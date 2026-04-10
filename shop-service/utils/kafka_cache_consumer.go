package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

/*
============================================================
Kafka 缓存一致性消息消费者

功能：
1. 订阅 Kafka 消息，接收缓存删除/更新请求
2. 执行实际的缓存操作（Delete/Set）
3. 实现重试机制，失败消息进入重试队列
4. 支持死信队列（DLQ），记录失败消息
5. 支持通配符批量删除

重试机制：
1. 首次失败：等待一定时间后重试
2. 超过最大重试次数：进入死信队列
3. 死信队列：记录失败消息，便于后续人工处理

架构：
[Kafka Topic: cache-invalidate]
        │
        ▼
┌─────────────────┐
│   Kafka         │
│   Consumer      │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
成功执行   失败重试
缓存操作    │
            ▼
      ┌─────────────┐
      │  Retry      │
      │  Topic      │
      └─────────────┘
            │
            ▼ (超过最大重试次数)
      ┌─────────────┐
      │  DLQ        │
      │  (死信队列)  │
      └─────────────┘
============================================================
*/

// CacheConsumerConfig 消费者配置
type CacheConsumerConfig struct {
	Brokers        []string      // Kafka broker地址
	Topic          string        // 消费主题
	GroupID        string        // 消费者组ID
	RetryTopic     string        // 重试主题
	DLQTopic       string        // 死信队列主题
	MaxRetries     int           // 最大重试次数
	RetryInterval  time.Duration // 重试间隔
	ReadBatchSize  int           // 每次读取消息数量
	CommitInterval time.Duration // 提交偏移量间隔
	WorkerCount    int           // 工作协程数量
}

// DefaultCacheConsumerConfig 默认配置
func DefaultCacheConsumerConfig() *CacheConsumerConfig {
	return &CacheConsumerConfig{
		Brokers:        []string{"localhost:9092"},
		Topic:          "cache-invalidate",
		GroupID:        "cache-consumer-group",
		RetryTopic:     "cache-invalidate-retry",
		DLQTopic:       "cache-invalidate-dlq",
		MaxRetries:     3,
		RetryInterval:  1 * time.Second,
		ReadBatchSize:  10,
		CommitInterval: 100 * time.Millisecond,
		WorkerCount:    4,
	}
}

// CacheConsumer 缓存消息消费者
type CacheConsumer struct {
	config  *CacheConsumerConfig
	rdb     *redis.Client
	reader  *kafka.Reader
	workers int
	wg      sync.WaitGroup
	stopCh  chan struct{}
	// 重试相关
	retryWriter *kafka.Writer
	dlqWriter   *kafka.Writer
}

// NewCacheConsumer 创建缓存消息消费者
func NewCacheConsumer(rdb *redis.Client, cfg *CacheConsumerConfig) *CacheConsumer {
	if cfg == nil {
		cfg = DefaultCacheConsumerConfig()
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: cfg.CommitInterval,
		// 从最新位置开始消费
		StartOffset: kafka.LastOffset,
	})

	// 创建重试写入器
	retryWriter := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.RetryTopic,
		Balancer: &kafka.LeastBytes{},
	}

	// 创建死信队列写入器
	dlqWriter := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.DLQTopic,
		Balancer: &kafka.LeastBytes{},
	}

	return &CacheConsumer{
		config:      cfg,
		rdb:         rdb,
		reader:      reader,
		workers:     cfg.WorkerCount,
		stopCh:      make(chan struct{}),
		retryWriter: retryWriter,
		dlqWriter:   dlqWriter,
	}
}

// Start 启动消费者
func (c *CacheConsumer) Start(ctx context.Context) {
	// 启动工作协程
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}

	fmt.Printf("[CacheConsumer] Started with %d workers\n", c.workers)
}

// worker 工作协程
func (c *CacheConsumer) worker(ctx context.Context, id int) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// 读取消息
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				fmt.Printf("[CacheConsumer] FetchMessage error: %v\n", err)
				time.Sleep(time.Second)
				continue
			}

			// 处理消息
			if err := c.processMessage(ctx, msg); err != nil {
				fmt.Printf("[CacheConsumer] Process message error: %v\n", err)
				// 处理失败，发送到重试队列
				c.sendToRetry(ctx, msg, err.Error())
			}

			// 提交偏移量
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				fmt.Printf("[CacheConsumer] Commit error: %v\n", err)
			}
		}
	}
}

// processMessage 处理单条消息
func (c *CacheConsumer) processMessage(ctx context.Context, msg kafka.Message) error {
	// 解析消息
	var cacheMsg CacheMessage
	if err := json.Unmarshal(msg.Value, &cacheMsg); err != nil {
		fmt.Printf("[CacheConsumer] Unmarshal error: %v, message: %s\n", err, string(msg.Value))
		return fmt.Errorf("invalid message format")
	}

	// 记录处理日志
	fmt.Printf("[CacheConsumer] Processing: type=%s, key=%s, retry=%d\n",
		cacheMsg.Type, cacheMsg.Key, cacheMsg.RetryCount)

	// 根据消息类型执行缓存操作
	switch cacheMsg.Type {
	case CacheMsgDelete:
		return c.handleDelete(ctx, cacheMsg)
	case CacheMsgDeletePattern:
		return c.handleDeletePattern(ctx, cacheMsg)
	case CacheMsgSet:
		return c.handleSet(ctx, cacheMsg)
	default:
		return fmt.Errorf("unknown message type: %s", cacheMsg.Type)
	}
}

// handleDelete 处理删除缓存消息
func (c *CacheConsumer) handleDelete(ctx context.Context, msg CacheMessage) error {
	err := c.rdb.Del(ctx, msg.Key).Err()
	if err != nil {
		return fmt.Errorf("delete cache error: %w", err)
	}
	fmt.Printf("[CacheConsumer] Deleted cache key: %s\n", msg.Key)
	return nil
}

// handleDeletePattern 处理批量删除缓存消息
func (c *CacheConsumer) handleDeletePattern(ctx context.Context, msg CacheMessage) error {
	// 使用 SCAN 遍历匹配的 key
	var cursor uint64
	var deletedCount int64

	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, msg.Key, 100).Result()
		if err != nil {
			return fmt.Errorf("scan keys error: %w", err)
		}

		if len(keys) > 0 {
			count, err := c.rdb.Del(ctx, keys...).Result()
			if err != nil {
				fmt.Printf("[CacheConsumer] Delete keys error: %v\n", err)
			} else {
				deletedCount += count
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	fmt.Printf("[CacheConsumer] Deleted %d keys matching pattern: %s\n", deletedCount, msg.Key)
	return nil
}

// handleSet 处理设置缓存消息
func (c *CacheConsumer) handleSet(ctx context.Context, msg CacheMessage) error {
	if msg.Value == "" {
		return fmt.Errorf("empty cache value")
	}

	ttl := time.Duration(msg.TTL) * time.Millisecond
	if ttl <= 0 {
		ttl = 30 * time.Minute // 默认30分钟
	}

	err := c.rdb.Set(ctx, msg.Key, msg.Value, ttl).Err()
	if err != nil {
		return fmt.Errorf("set cache error: %w", err)
	}
	fmt.Printf("[CacheConsumer] Set cache key: %s, TTL: %v\n", msg.Key, ttl)
	return nil
}

// sendToRetry 发送消息到重试队列
func (c *CacheConsumer) sendToRetry(ctx context.Context, msg kafka.Message, errMsg string) error {
	// 解析原始消息
	var cacheMsg CacheMessage
	if err := json.Unmarshal(msg.Value, &cacheMsg); err != nil {
		return err
	}

	// 增加重试次数
	cacheMsg.RetryCount++

	// 检查是否超过最大重试次数
	if cacheMsg.RetryCount > c.config.MaxRetries {
		// 发送到死信队列
		return c.sendToDLQ(ctx, cacheMsg, errMsg)
	}

	// 序列化消息
	data, err := json.Marshal(cacheMsg)
	if err != nil {
		return err
	}

	// 添加延迟后发送（使用消息的时间戳实现延迟）
	retryMsg := kafka.Message{
		Key:     msg.Key,
		Value:   data,
		Time:    time.Now().Add(c.config.RetryInterval), // 延迟发送
		Headers: msg.Headers,
	}

	// 立即发送到重试主题（消费者可以根据时间戳判断是否可消费）
	// 这里简化处理：直接发送到重试主题，消费者立即消费
	// 实际生产中可以使用延迟队列或定时任务处理
	err = c.retryWriter.WriteMessages(ctx, retryMsg)
	if err != nil {
		fmt.Printf("[CacheConsumer] Send to retry error: %v\n", err)
		// 重试也失败，发送到死信队列
		return c.sendToDLQ(ctx, cacheMsg, errMsg)
	}

	fmt.Printf("[CacheConsumer] Sent to retry: key=%s, retry=%d\n",
		cacheMsg.Key, cacheMsg.RetryCount)
	return nil
}

// sendToDLQ 发送消息到死信队列
func (c *CacheConsumer) sendToDLQ(ctx context.Context, msg CacheMessage, errMsg string) error {
	// 添加错误信息到消息
	msgData, _ := json.Marshal(msg)

	dlqMsg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%s-dlq", msg.Key)),
		Value: msgData,
		Headers: []kafka.Header{
			{Key: "error", Value: []byte(errMsg)},
			{Key: "failed_at", Value: []byte(time.Now().Format(time.RFC3339))},
		},
	}

	err := c.dlqWriter.WriteMessages(ctx, dlqMsg)
	if err != nil {
		fmt.Printf("[CacheConsumer] Send to DLQ error: %v\n", err)
		return err
	}

	fmt.Printf("[CacheConsumer] Sent to DLQ: key=%s, error=%s\n", msg.Key, errMsg)
	return nil
}

// Stop 停止消费者
func (c *CacheConsumer) Stop() error {
	close(c.stopCh)
	c.wg.Wait()

	// 关闭reader
	if err := c.reader.Close(); err != nil {
		return err
	}

	// 关闭写入器
	c.retryWriter.Close()
	c.dlqWriter.Close()

	fmt.Println("[CacheConsumer] Stopped")
	return nil
}

// ============================================================
// 便捷函数：创建带有重试机制的缓存消费者
// ============================================================

// CreateCacheConsumerWithRetry 创建带重试的缓存消费者
func CreateCacheConsumerWithRetry(rdb *redis.Client, brokers []string) *CacheConsumer {
	cfg := &CacheConsumerConfig{
		Brokers:       brokers,
		Topic:         "cache-invalidate",
		GroupID:       "cache-consumer-group",
		RetryTopic:    "cache-invalidate-retry",
		DLQTopic:      "cache-invalidate-dlq",
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
		WorkerCount:   4,
	}
	return NewCacheConsumer(rdb, cfg)
}

// ============================================================
// 用于测试的模拟消息处理
// ============================================================

// ProcessCacheMessageDirect 直接处理缓存消息（不通过Kafka）
func ProcessCacheMessageDirect(ctx context.Context, rdb *redis.Client, msg *CacheMessage) error {
	consumer := &CacheConsumer{
		rdb:    rdb,
		config: DefaultCacheConsumerConfig(),
	}

	switch msg.Type {
	case CacheMsgDelete:
		return consumer.handleDelete(ctx, *msg)
	case CacheMsgDeletePattern:
		return consumer.handleDeletePattern(ctx, *msg)
	case CacheMsgSet:
		return consumer.handleSet(ctx, *msg)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}
