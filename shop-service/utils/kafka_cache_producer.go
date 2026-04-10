package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

/*
============================================================
Kafka 缓存一致性消息生产者

功能：
1. 封装缓存操作消息的发送
2. 支持批量发送提高性能
3. 支持消息追踪和监控

消息格式：
{
    "id": "唯一消息ID",
    "type": "DELETE|SET|DELETE_PATTERN",
    "key": "缓存key",
    "value": "缓存value(JSON)",
    "ttl": "过期时间(毫秒)",
    "timestamp": "时间戳",
    "retryCount": "重试次数"
}

注意：
- 使用 Kafka 保证消息持久化，避免缓存删除失败导致的不一致
- 消息生产者不关心消费者，由消费者负责缓存的实际操作
- 支持消息追踪，便于问题排查
============================================================
*/

// CacheMessageType 缓存消息类型
type CacheMessageType string

const (
	CacheMsgDelete        CacheMessageType = "DELETE"         // 删除单个缓存
	CacheMsgDeletePattern CacheMessageType = "DELETE_PATTERN" // 批量删除（支持通配符）
	CacheMsgSet           CacheMessageType = "SET"            // 设置缓存
)

// CacheMessage 缓存操作消息
type CacheMessage struct {
	ID         string           `json:"id"`                   // 唯一消息ID
	Type       CacheMessageType `json:"type"`                 // 消息类型
	Key        string           `json:"key"`                  // 缓存key
	Value      string           `json:"value,omitempty"`      // 缓存value（JSON）
	TTL        int64            `json:"ttl,omitempty"`        // 过期时间（毫秒）
	Timestamp  int64            `json:"timestamp"`            // 时间戳
	RetryCount int              `json:"retryCount"`           // 重试次数
	Source     string           `json:"source"`               // 消息来源
	TableName  string           `json:"tableName,omitempty"`  // 表名（binlog场景）
	PrimaryKey string           `json:"primaryKey,omitempty"` // 主键名
	PrimaryID  int64            `json:"primaryID,omitempty"`  // 主键值
}

// CacheMessageProducer 缓存消息生产者
type CacheMessageProducer struct {
	writer        *kafka.Writer
	topic         string
	broker        string
	messageCh     chan *CacheMessage
	workerCount   int
	flushInterval time.Duration
	flushSize     int
	closed        bool
}

// NewCacheMessageProducer 创建缓存消息生产者
func NewCacheMessageProducer(broker, topic string) *CacheMessageProducer {
	p := &CacheMessageProducer{
		broker: broker,
		topic:  topic,
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
			// 批量发送配置
			BatchSize:    10,                    // 每批消息数量
			BatchTimeout: 10 * time.Millisecond, // 批量等待时间
			// 异步写入
			Async: false,
			// 压缩
			Compression: kafka.Snappy,
		},
		messageCh:     make(chan *CacheMessage, 10000),
		workerCount:   4,
		flushInterval: 100 * time.Millisecond,
		flushSize:     100,
	}
	return p
}

// NewCacheMessageProducerWithConfig 创建带配置的消息生产者
func NewCacheMessageProducerWithConfig(broker, topic string, workerCount int, batchSize int, flushInterval time.Duration) *CacheMessageProducer {
	p := &CacheMessageProducer{
		broker: broker,
		topic:  topic,
		writer: &kafka.Writer{
			Addr:         kafka.TCP(broker),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			BatchSize:    batchSize,
			BatchTimeout: 10 * time.Millisecond,
			Async:        false,
			Compression:  kafka.Snappy,
		},
		messageCh:     make(chan *CacheMessage, 10000),
		workerCount:   workerCount,
		flushInterval: flushInterval,
		flushSize:     batchSize,
	}
	return p
}

// Start 启动生产者（异步批量发送）
func (p *CacheMessageProducer) Start(ctx context.Context) {
	for i := 0; i < p.workerCount; i++ {
		go p.worker(ctx, i)
	}
}

// worker 工作协程
func (p *CacheMessageProducer) worker(ctx context.Context, id int) {
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	var batch []kafka.Message
	forceFlush := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			// 关闭前发送剩余消息
			if len(batch) > 0 {
				p.flushBatch(batch)
			}
			return
		case <-ticker.C:
			// 定时刷新
			if len(batch) > 0 {
				p.flushBatch(batch)
				batch = nil
			}
		case <-forceFlush:
			// 强制刷新
			if len(batch) > 0 {
				p.flushBatch(batch)
				batch = nil
			}
		case msg, ok := <-p.messageCh:
			if !ok {
				// 通道关闭，发送剩余消息
				if len(batch) > 0 {
					p.flushBatch(batch)
				}
				return
			}

			// 序列化消息
			data, err := json.Marshal(msg)
			if err != nil {
				fmt.Printf("[CacheMessageProducer] Marshal error: %v\n", err)
				continue
			}

			// 添加到批次
			batch = append(batch, kafka.Message{
				Key:   []byte(msg.Key), // 使用缓存key作为message key，便于分区
				Value: data,
			})

			// 达到批次大小，发送
			if len(batch) >= p.flushSize {
				p.flushBatch(batch)
				batch = nil
			}
		}
	}
}

// flushBatch 发送批次消息
func (p *CacheMessageProducer) flushBatch(batch []kafka.Message) {
	if len(batch) == 0 {
		return
	}

	err := p.writer.WriteMessages(context.Background(), batch...)
	if err != nil {
		fmt.Printf("[CacheMessageProducer] WriteMessages error: %v\n", err)
	}
}

// Send 发送缓存消息（异步）
func (p *CacheMessageProducer) Send(ctx context.Context, msg *CacheMessage) error {
	if p.closed {
		return fmt.Errorf("producer closed")
	}

	msg.Timestamp = time.Now().UnixMilli()

	select {
	case p.messageCh <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	}
}

// SendDelete 发送删除缓存消息
func (p *CacheMessageProducer) SendDelete(ctx context.Context, key string) error {
	msg := &CacheMessage{
		ID:     generateMessageID(),
		Type:   CacheMsgDelete,
		Key:    key,
		Source: "application",
	}
	return p.Send(ctx, msg)
}

// SendDeletePattern 发送批量删除缓存消息
func (p *CacheMessageProducer) SendDeletePattern(ctx context.Context, pattern string) error {
	msg := &CacheMessage{
		ID:     generateMessageID(),
		Type:   CacheMsgDeletePattern,
		Key:    pattern,
		Source: "application",
	}
	return p.Send(ctx, msg)
}

// SendSet 发送设置缓存消息
func (p *CacheMessageProducer) SendSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	msg := &CacheMessage{
		ID:     generateMessageID(),
		Type:   CacheMsgSet,
		Key:    key,
		Value:  string(data),
		TTL:    ttl.Milliseconds(),
		Source: "application",
	}
	return p.Send(ctx, msg)
}

// SendBinlogMessage 发送Binlog消息（由Binlog消费者调用）
func (p *CacheMessageProducer) SendBinlogMessage(ctx context.Context, tableName string, msgType CacheMessageType, primaryID int64, data map[string]interface{}) error {
	// 根据表名生成缓存key
	key := generateCacheKeyFromBinlog(tableName, primaryID)

	msg := &CacheMessage{
		ID:         generateMessageID(),
		Type:       msgType,
		Key:        key,
		Source:     "binlog",
		TableName:  tableName,
		PrimaryID:  primaryID,
		PrimaryKey: "id",
	}

	// 如果是SET操作，序列化数据
	if msgType == CacheMsgSet && data != nil {
		dataBytes, err := json.Marshal(data)
		if err == nil {
			msg.Value = string(dataBytes)
		}
	}

	return p.Send(ctx, msg)
}

// Close 关闭生产者
func (p *CacheMessageProducer) Close() error {
	p.closed = true
	close(p.messageCh)
	return p.writer.Close()
}

// generateMessageID 生成唯一消息ID
func generateMessageID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// generateCacheKeyFromBinlog 根据Binlog事件生成缓存key
func generateCacheKeyFromBinlog(tableName string, id int64) string {
	switch tableName {
	case "tb_shop":
		return CacheShopKey + fmt.Sprintf("%d", id)
	case "tb_voucher":
		return CacheVoucherKey + fmt.Sprintf("%d", id)
	default:
		return fmt.Sprintf("cache:%s:%d", tableName, id)
	}
}

// ============================================================
// 便捷函数：同步发送（用于需要立即删除缓存的场景）
// ============================================================

// SendDeleteSync 同步发送删除缓存消息
func SendDeleteSync(ctx context.Context, broker, topic, key string) error {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	msg := &CacheMessage{
		ID:     generateMessageID(),
		Type:   CacheMsgDelete,
		Key:    key,
		Source: "application",
	}

	data, _ := json.Marshal(msg)
	return writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
	})
}
