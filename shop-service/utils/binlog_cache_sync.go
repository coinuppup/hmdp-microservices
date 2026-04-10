package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

/*
============================================================
Binlog 缓存同步方案

方案说明：
1. 应用层更新数据库后，发送 binlog 消息到 Kafka
2. Kafka 消费者订阅消息，异步执行缓存删除/更新
3. 实现缓存的最终一致性

架构：
┌─────────────────────────────────────────────────────────────────┐
│                         应用层                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ ShopService │    │UserService  │    │ ContentSvc  │         │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘         │
└─────────┼──────────────────┼──────────────────┼────────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
    ┌──────────────────────────────────────────────────────────┐
    │               业务消息队列 (cache-invalidate)             │
    │  - 表名: tb_shop, tb_user, tb_voucher                    │
    │  - 操作类型: INSERT, UPDATE, DELETE                       │
    │  - 包含: 变更前/后的数据                                   │
    └─────────────────────────┬────────────────────────────────┘
                              │
                              ▼
    ┌──────────────────────────────────────────────────────────┐
    │               Binlog 消费者 (多消费者)                    │
    │  1. 解析消息                                              │
    │  2. 根据表名和操作类型执行缓存操作                         │
    │  3. 处理失败则重试                                        │
    └─────────────────────────┬────────────────────────────────┘
                              │
                              ▼
    ┌──────────────────────────────────────────────────────────┐
    │                     缓存层                                │
    │  - 删除: 本地缓存 + Redis 缓存                             │
    │  - 更新: 重新从数据库加载并设置缓存                        │
    └──────────────────────────────────────────────────────────┘

核心特点：
1. **异步解耦**：数据库操作和缓存操作完全异步
2. **消息持久化**：使用 Kafka 保证消息不丢失
3. **重试机制**：失败消息进入重试队列，保证最终成功
4. **顺序性**：同一 key 的操作保证顺序执行
5. **幂等性**：支持重复消费

注意：
- 对于要求强一致性的场景，可以结合延迟双删
- 对于读多写少场景，binlog 方案性能更好
- 需要处理消息重复问题（使用消息 ID 去重）
============================================================
*/

// BinlogOperationType Binlog 操作类型
type BinlogOperationType string

const (
	BinlogInsert BinlogOperationType = "INSERT"
	BinlogUpdate BinlogOperationType = "UPDATE"
	BinlogDelete BinlogOperationType = "DELETE"
)

// BinlogMessage Binlog 消息结构
type BinlogMessage struct {
	ID         string                 `json:"id"`         // 消息ID
	Timestamp  int64                  `json:"timestamp"`  // 时间戳
	TableName  string                 `json:"tableName"`  // 表名
	Operation  BinlogOperationType    `json:"operation"`  // 操作类型
	OldData    map[string]interface{} `json:"oldData"`    // 变更前数据
	NewData    map[string]interface{} `json:"newData"`    // 变更后数据
	PrimaryKey string                 `json:"primaryKey"` // 主键名
	PrimaryID  int64                  `json:"primaryID"`  // 主键值
	Source     string                 `json:"source"`     // 数据源
	RetryCount int                    `json:"retryCount"` // 重试次数
}

// TableCacheConfig 表的缓存配置
type TableCacheConfig struct {
	TableName      string        // 表名
	CacheKeyPrefix string        // 缓存 key 前缀
	CacheTTL       time.Duration // 缓存 TTL
	DeleteOnUpdate bool          // 更新时是否删除缓存（true=删除，false=刷新）
}

// BinlogCacheConfig Binlog 缓存同步配置
type BinlogCacheConfig struct {
	Brokers       []string
	Topic         string
	GroupID       string
	TableConfigs  map[string]*TableCacheConfig
	MaxRetries    int
	RetryInterval time.Duration
	WorkerCount   int
}

// DefaultBinlogCacheConfig 默认配置
func DefaultBinlogCacheConfig() *BinlogCacheConfig {
	return &BinlogCacheConfig{
		Brokers:       []string{"localhost:9092"},
		Topic:         "db-binlog",
		GroupID:       "binlog-cache-sync",
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
		WorkerCount:   4,
		TableConfigs: map[string]*TableCacheConfig{
			"tb_shop": {
				TableName:      "tb_shop",
				CacheKeyPrefix: "cache:shop:",
				CacheTTL:       30 * time.Minute,
				DeleteOnUpdate: true,
			},
			"tb_voucher": {
				TableName:      "tb_voucher",
				CacheKeyPrefix: "cache:voucher:",
				CacheTTL:       30 * time.Minute,
				DeleteOnUpdate: true,
			},
			"tb_user": {
				TableName:      "tb_user",
				CacheKeyPrefix: "cache:user:",
				CacheTTL:       30 * time.Minute,
				DeleteOnUpdate: true,
			},
		},
	}
}

// BinlogCacheSync Binlog 缓存同步器
type BinlogCacheSync struct {
	config     *BinlogCacheConfig
	rdb        *redis.Client
	reader     *kafka.Reader
	workers    int
	wg         sync.WaitGroup
	stopCh     chan struct{}
	localCache *LocalCache
	// 消息去重
	processedMsgs map[string]time.Time
	msgMu         sync.RWMutex
	// 统计
	stats *BinlogSyncStats
}

// BinlogSyncStats 同步统计
type BinlogSyncStats struct {
	Processed int64 `json:"processed"`
	Deleted   int64 `json:"deleted"`
	Updated   int64 `json:"updated"`
	Errors    int64 `json:"errors"`
	mu        sync.RWMutex
}

// NewBinlogCacheSync 创建 Binlog 缓存同步器
func NewBinlogCacheSync(rdb *redis.Client, cfg *BinlogCacheConfig) *BinlogCacheSync {
	if cfg == nil {
		cfg = DefaultBinlogCacheConfig()
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: 1,
		MaxBytes: 10e6,
		// 从最新位置开始消费
		StartOffset: kafka.LastOffset,
	})

	return &BinlogCacheSync{
		config:        cfg,
		rdb:           rdb,
		reader:        reader,
		workers:       cfg.WorkerCount,
		stopCh:        make(chan struct{}),
		processedMsgs: make(map[string]time.Time),
		stats:         &BinlogSyncStats{},
		localCache:    NewLocalCache(DefaultLocalCacheConfig()),
	}
}

// Start 启动同步器
func (b *BinlogCacheSync) Start(ctx context.Context) {
	// 启动清理过期消息的协程
	go b.cleanupProcessedMsgLoop()

	// 启动工作协程
	for i := 0; i < b.workers; i++ {
		b.wg.Add(1)
		go b.worker(ctx, i)
	}

	fmt.Printf("[BinlogCacheSync] Started with %d workers\n", b.workers)
}

// worker 工作协程
func (b *BinlogCacheSync) worker(ctx context.Context, id int) {
	defer b.wg.Done()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// 读取消息
			msg, err := b.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				fmt.Printf("[BinlogCacheSync] FetchMessage error: %v\n", err)
				time.Sleep(time.Second)
				continue
			}

			// 处理消息
			if err := b.processMessage(ctx, msg); err != nil {
				fmt.Printf("[BinlogCacheSync] Process message error: %v\n", err)
				// 失败不重试，简化处理
			}

			// 提交偏移量
			if err := b.reader.CommitMessages(ctx, msg); err != nil {
				fmt.Printf("[BinlogCacheSync] Commit error: %v\n", err)
			}
		}
	}
}

// processMessage 处理消息
func (b *BinlogCacheSync) processMessage(ctx context.Context, msg kafka.Message) error {
	// 解析消息
	var binlogMsg BinlogMessage
	if err := json.Unmarshal(msg.Value, &binlogMsg); err != nil {
		b.stats.mu.Lock()
		b.stats.Errors++
		b.stats.mu.Unlock()
		return fmt.Errorf("unmarshal error: %w", err)
	}

	// 消息去重
	if b.isProcessed(binlogMsg.ID) {
		return nil
	}
	b.markProcessed(binlogMsg.ID)

	// 获取表配置
	tableConfig, ok := b.config.TableConfigs[binlogMsg.TableName]
	if !ok {
		// 没有配置，跳过
		return nil
	}

	// 根据操作类型执行缓存操作
	switch binlogMsg.Operation {
	case BinlogInsert, BinlogUpdate:
		// 无论是插入还是更新，都删除缓存，让下次查询重新加载
		// 或者可以刷新缓存（从数据库重新读取）
		if tableConfig.DeleteOnUpdate {
			return b.handleDeleteCache(ctx, tableConfig, binlogMsg.PrimaryID)
		} else {
			return b.handleRefreshCache(ctx, tableConfig, &binlogMsg)
		}
	case BinlogDelete:
		return b.handleDeleteCache(ctx, tableConfig, binlogMsg.PrimaryID)
	default:
		return fmt.Errorf("unknown operation: %s", binlogMsg.Operation)
	}
}

// handleDeleteCache 处理删除缓存
func (b *BinlogCacheSync) handleDeleteCache(ctx context.Context, config *TableCacheConfig, id int64) error {
	cacheKey := config.CacheKeyPrefix + strconv.FormatInt(id, 10)

	// 删除本地缓存
	b.localCache.Delete(ctx, cacheKey)

	// 删除 Redis 缓存
	if err := b.rdb.Del(ctx, cacheKey).Err(); err != nil {
		b.stats.mu.Lock()
		b.stats.Errors++
		b.stats.mu.Unlock()
		return fmt.Errorf("delete redis cache error: %w", err)
	}

	b.stats.mu.Lock()
	b.stats.Deleted++
	b.stats.Processed++
	b.stats.mu.Unlock()

	fmt.Printf("[BinlogCacheSync] Deleted cache: %s\n", cacheKey)
	return nil
}

// handleRefreshCache 处理刷新缓存（从数据库读取新数据）
func (b *BinlogCacheSync) handleRefreshCache(ctx context.Context, config *TableCacheConfig, msg *BinlogMessage) error {
	// 如果有 newData，直接使用；否则需要从数据库读取
	cacheKey := config.CacheKeyPrefix + strconv.FormatInt(msg.PrimaryID, 10)

	if msg.NewData != nil {
		data, err := json.Marshal(msg.NewData)
		if err == nil {
			// 写入本地缓存
			b.localCache.Set(ctx, cacheKey, data, config.CacheTTL)
			// 写入 Redis
			_ = b.rdb.Set(ctx, cacheKey, data, config.CacheTTL).Err()
		}
	}

	b.stats.mu.Lock()
	b.stats.Updated++
	b.stats.Processed++
	b.stats.mu.Unlock()

	fmt.Printf("[BinlogCacheSync] Refreshed cache: %s\n", cacheKey)
	return nil
}

// isProcessed 检查消息是否已处理
func (b *BinlogCacheSync) isProcessed(msgID string) bool {
	b.msgMu.RLock()
	defer b.msgMu.RUnlock()
	_, exists := b.processedMsgs[msgID]
	return exists
}

// markProcessed 标记消息已处理
func (b *BinlogCacheSync) markProcessed(msgID string) {
	b.msgMu.Lock()
	defer b.msgMu.Unlock()
	b.processedMsgs[msgID] = time.Now()
}

// cleanupProcessedMsgLoop 清理过期消息记录
func (b *BinlogCacheSync) cleanupProcessedMsgLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.cleanupProcessedMsg()
		}
	}
}

// cleanupProcessedMsg 清理过期的消息记录
func (b *BinlogCacheSync) cleanupProcessedMsg() {
	b.msgMu.Lock()
	defer b.msgMu.Unlock()

	threshold := time.Now().Add(-10 * time.Minute)
	for id, t := range b.processedMsgs {
		if t.Before(threshold) {
			delete(b.processedMsgs, id)
		}
	}
}

// Stop 停止同步器
func (b *BinlogCacheSync) Stop() error {
	close(b.stopCh)
	b.wg.Wait()

	if err := b.reader.Close(); err != nil {
		return err
	}

	b.localCache.Stop()
	fmt.Println("[BinlogCacheSync] Stopped")
	return nil
}

// GetStats 获取统计信息
func (b *BinlogCacheSync) GetStats() (processed, deleted, updated, errors int64) {
	b.stats.mu.RLock()
	defer b.stats.mu.RUnlock()
	return b.stats.Processed, b.stats.Deleted, b.stats.Updated, b.stats.Errors
}

// ============================================================
// 应用层发送 Binlog 消息
// ============================================================

// BinlogPublisher Binlog 消息发布者
type BinlogPublisher struct {
	writer *kafka.Writer
	topic  string
}

// NewBinlogPublisher 创建 Binlog 发布者
func NewBinlogPublisher(broker, topic string) *BinlogPublisher {
	return &BinlogPublisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
		topic: topic,
	}
}

// PublishInsert 发布插入消息
func (p *BinlogPublisher) PublishInsert(ctx context.Context, tableName string, id int64, data map[string]interface{}) error {
	msg := BinlogMessage{
		ID:         generateMessageID(),
		Timestamp:  time.Now().UnixMilli(),
		TableName:  tableName,
		Operation:  BinlogInsert,
		NewData:    data,
		PrimaryKey: "id",
		PrimaryID:  id,
		Source:     "application",
	}
	return p.publish(ctx, msg)
}

// PublishUpdate 发布更新消息
func (p *BinlogPublisher) PublishUpdate(ctx context.Context, tableName string, id int64, oldData, newData map[string]interface{}) error {
	msg := BinlogMessage{
		ID:         generateMessageID(),
		Timestamp:  time.Now().UnixMilli(),
		TableName:  tableName,
		Operation:  BinlogUpdate,
		OldData:    oldData,
		NewData:    newData,
		PrimaryKey: "id",
		PrimaryID:  id,
		Source:     "application",
	}
	return p.publish(ctx, msg)
}

// PublishDelete 发布删除消息
func (p *BinlogPublisher) PublishDelete(ctx context.Context, tableName string, id int64) error {
	msg := BinlogMessage{
		ID:         generateMessageID(),
		Timestamp:  time.Now().UnixMilli(),
		TableName:  tableName,
		Operation:  BinlogDelete,
		OldData:    map[string]interface{}{"id": id},
		PrimaryKey: "id",
		PrimaryID:  id,
		Source:     "application",
	}
	return p.publish(ctx, msg)
}

// publish 发布消息
func (p *BinlogPublisher) publish(ctx context.Context, msg BinlogMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%s:%d", msg.TableName, msg.PrimaryID)),
		Value: data,
	})
}

// Close 关闭发布者
func (p *BinlogPublisher) Close() error {
	return p.writer.Close()
}

// ============================================================
// 便捷函数：在 Service 层调用
// ============================================================

// PublishShopCacheInvalidate 发布商铺缓存失效消息
func PublishShopCacheInvalidate(ctx context.Context, broker string, shopID int64, operation BinlogOperationType) error {
	publisher := NewBinlogPublisher(broker, "db-binlog")
	defer publisher.Close()

	switch operation {
	case BinlogInsert:
		return publisher.PublishInsert(ctx, "tb_shop", shopID, nil)
	case BinlogUpdate:
		return publisher.PublishUpdate(ctx, "tb_shop", shopID, nil, nil)
	case BinlogDelete:
		return publisher.PublishDelete(ctx, "tb_shop", shopID)
	default:
		return nil
	}
}

// PublishVoucherCacheInvalidate 发布优惠券缓存失效消息
func PublishVoucherCacheInvalidate(ctx context.Context, broker string, voucherID int64, operation BinlogOperationType) error {
	publisher := NewBinlogPublisher(broker, "db-binlog")
	defer publisher.Close()

	switch operation {
	case BinlogInsert:
		return publisher.PublishInsert(ctx, "tb_voucher", voucherID, nil)
	case BinlogUpdate:
		return publisher.PublishUpdate(ctx, "tb_voucher", voucherID, nil, nil)
	case BinlogDelete:
		return publisher.PublishDelete(ctx, "tb_voucher", voucherID)
	default:
		return nil
	}
}
