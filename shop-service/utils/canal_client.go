package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
============================================================
Cache Aside + Binlog 异步更新方案
============================================================

方案说明：
1. 应用层只负责读写缓存，不直接操作缓存
2. 通过监听MySQL Binlog获取数据变更
3. 异步消费变更事件，批量更新/删除缓存
4. 保证最终一致性，适合一致性要求不高的场景

架构：
  [MySQL] --Binlog--> [Canal Server] ---> [Canal Client] ---> [Redis]
                                       (解析事件)                 (更新缓存)

优点：
- 应用层代码简单，无需关注缓存更新
- 降低数据库压力，缓存更新异步进行
- 支持多个服务共享缓存更新逻辑

注意：
- 存在短暂不一致时间（通常秒级）
- 需要额外的基础设施（Canal）
- 需要处理重复消费和顺序性
============================================================
*/

// CanalEventType Canal事件类型
type CanalEventType string

const (
	CanalEventInsert CanalEventType = "INSERT"
	CanalEventUpdate CanalEventType = "UPDATE"
	CanalEventDelete CanalEventType = "DELETE"
)

// CanalEvent Canal事件结构
type CanalEvent struct {
	Table   string                 `json:"table"`
	Type    CanalEventType         `json:"type"`
	OldData map[string]interface{} `json:"oldData"` // 更新前的数据
	NewData map[string]interface{} `json:"newData"` // 更新后的数据
	// 额外字段用于缓存操作
	PrimaryKey   string `json:"primaryKey"`
	PrimaryValue int64  `json:"primaryValue"`
}

// CacheOperation 缓存操作类型
type CacheOperation string

const (
	CacheOpSet    CacheOperation = "SET"    // 设置缓存
	CacheOpDelete CacheOperation = "DELETE" // 删除缓存
)

// CacheWorkItem 缓存工作项
type CacheWorkItem struct {
	Operation CacheOperation
	CacheKey  string
	Data      interface{}
	TTL       time.Duration
}

// BinlogConsumer Binlog消费者
// 监听数据库变更，异步更新缓存
type BinlogConsumer struct {
	rdb         *redis.Client
	workQueue   chan *CacheWorkItem
	workerCount int
	stopCh      chan struct{}
	wg          sync.WaitGroup
	handlers    map[string]TableHandler
	handlersMu  sync.RWMutex
}

// TableHandler 表处理器接口
type TableHandler interface {
	// GetTableName 获取处理的表名
	GetTableName() string
	// HandleInsert 处理插入事件
	HandleInsert(event *CanalEvent) (*CacheWorkItem, error)
	// HandleUpdate 处理更新事件
	HandleUpdate(event *CanalEvent) (*CacheWorkItem, error)
	// HandleDelete 处理删除事件
	HandleDelete(event *CanalEvent) (*CacheWorkItem, error)
}

// TableHandlerFunc 函数式处理器
type TableHandlerFunc struct {
	tableName string
	onInsert  func(event *CanalEvent) (*CacheWorkItem, error)
	onUpdate  func(event *CanalEvent) (*CacheWorkItem, error)
	onDelete  func(event *CanalEvent) (*CacheWorkItem, error)
}

func (h *TableHandlerFunc) GetTableName() string {
	return h.tableName
}

func (h *TableHandlerFunc) HandleInsert(event *CanalEvent) (*CacheWorkItem, error) {
	if h.onInsert != nil {
		return h.onInsert(event)
	}
	return nil, nil
}

func (h *TableHandlerFunc) HandleUpdate(event *CanalEvent) (*CacheWorkItem, error) {
	if h.onUpdate != nil {
		return h.onUpdate(event)
	}
	return nil, nil
}

func (h *TableHandlerFunc) HandleDelete(event *CanalEvent) (*CacheWorkItem, error) {
	if h.onDelete != nil {
		return h.onDelete(event)
	}
	return nil, nil
}

// NewBinlogConsumer 创建Binlog消费者
func NewBinlogConsumer(rdb *redis.Client, workerCount int) *BinlogConsumer {
	if workerCount <= 0 {
		workerCount = 4
	}

	return &BinlogConsumer{
		rdb:         rdb,
		workQueue:   make(chan *CacheWorkItem, 10000),
		workerCount: workerCount,
		stopCh:      make(chan struct{}),
		handlers:    make(map[string]TableHandler),
	}
}

// RegisterHandler 注册表处理器
func (c *BinlogConsumer) RegisterHandler(handler TableHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[handler.GetTableName()] = handler
}

// RegisterHandlerFunc 注册函数式处理器
func (c *BinlogConsumer) RegisterHandlerFunc(tableName string,
	insertFn, updateFn, deleteFn func(event *CanalEvent) (*CacheWorkItem, error)) {
	handler := &TableHandlerFunc{
		tableName: tableName,
		onInsert:  insertFn,
		onUpdate:  updateFn,
		onDelete:  deleteFn,
	}
	c.RegisterHandler(handler)
}

// Start 启动消费者
func (c *BinlogConsumer) Start(ctx context.Context) {
	// 启动工作池
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}
}

// Stop 停止消费者
func (c *BinlogConsumer) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	close(c.workQueue)
}

// worker 工作协程
func (c *BinlogConsumer) worker(ctx context.Context, id int) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case item, ok := <-c.workQueue:
			if !ok {
				return
			}
			c.processWorkItem(ctx, item)
		}
	}
}

// processWorkItem 处理工作项
func (c *BinlogConsumer) processWorkItem(ctx context.Context, item *CacheWorkItem) {
	switch item.Operation {
	case CacheOpSet:
		data, err := json.Marshal(item.Data)
		if err != nil {
			fmt.Printf("[BinlogConsumer] Marshal error: %v\n", err)
			return
		}
		if err := c.rdb.Set(ctx, item.CacheKey, data, item.TTL).Err(); err != nil {
			fmt.Printf("[BinlogConsumer] Set cache error: %v\n", err)
		}
	case CacheOpDelete:
		if err := c.rdb.Del(ctx, item.CacheKey).Err(); err != nil {
			fmt.Printf("[BinlogConsumer] Delete cache error: %v\n", err)
		}
	}
}

// OnEvent 处理Canal事件（由Canal客户端调用）
func (c *BinlogConsumer) OnEvent(event *CanalEvent) {
	c.handlersMu.RLock()
	handler, ok := c.handlers[event.Table]
	c.handlersMu.RUnlock()

	if !ok {
		// 没有注册处理器，跳过
		return
	}

	var workItem *CacheWorkItem
	var err error

	switch event.Type {
	case CanalEventInsert:
		workItem, err = handler.HandleInsert(event)
	case CanalEventUpdate:
		workItem, err = handler.HandleUpdate(event)
	case CanalEventDelete:
		workItem, err = handler.HandleDelete(event)
	}

	if err != nil {
		fmt.Printf("[BinlogConsumer] Handle event error: %v\n", err)
		return
	}

	if workItem != nil {
		select {
		case c.workQueue <- workItem:
		case <-time.After(time.Second):
			fmt.Printf("[BinlogConsumer] Queue full, discard work item\n")
		}
	}
}

// ============================================================
// 预定义的表处理器
// ============================================================

// NewShopTableHandler 创建商铺表处理器
func NewShopTableHandler() *ShopTableHandler {
	return &ShopTableHandler{}
}

type ShopTableHandler struct{}

func (h *ShopTableHandler) GetTableName() string {
	return "tb_shop"
}

func (h *ShopTableHandler) HandleInsert(event *CanalEvent) (*CacheWorkItem, error) {
	if event.NewData == nil {
		return nil, nil
	}

	id, ok := event.NewData["id"].(float64)
	if !ok {
		return nil, nil
	}

	return &CacheWorkItem{
		Operation: CacheOpSet,
		CacheKey:  CacheShopKey + strconv.FormatInt(int64(id), 10),
		Data:      event.NewData,
		TTL:       time.Duration(CacheShopTTL) * time.Minute,
	}, nil
}

func (h *ShopTableHandler) HandleUpdate(event *CanalEvent) (*CacheWorkItem, error) {
	if event.NewData == nil {
		return nil, nil
	}

	id, ok := event.NewData["id"].(float64)
	if !ok {
		return nil, nil
	}

	return &CacheWorkItem{
		Operation: CacheOpSet,
		CacheKey:  CacheShopKey + strconv.FormatInt(int64(id), 10),
		Data:      event.NewData,
		TTL:       time.Duration(CacheShopTTL) * time.Minute,
	}, nil
}

func (h *ShopTableHandler) HandleDelete(event *CanalEvent) (*CacheWorkItem, error) {
	if event.OldData == nil {
		return nil, nil
	}

	id, ok := event.OldData["id"].(float64)
	if !ok {
		return nil, nil
	}

	return &CacheWorkItem{
		Operation: CacheOpDelete,
		CacheKey:  CacheShopKey + strconv.FormatInt(int64(id), 10),
	}, nil
}

// NewVoucherTableHandler 创建优惠券表处理器
func (c *BinlogConsumer) NewVoucherTableHandler() *VoucherTableHandler {
	return &VoucherTableHandler{}
}

type VoucherTableHandler struct{}

func (h *VoucherTableHandler) GetTableName() string {
	return "tb_voucher"
}

func (h *VoucherTableHandler) HandleInsert(event *CanalEvent) (*CacheWorkItem, error) {
	if event.NewData == nil {
		return nil, nil
	}

	id, ok := event.NewData["id"].(float64)
	if !ok {
		return nil, nil
	}

	return &CacheWorkItem{
		Operation: CacheOpSet,
		CacheKey:  CacheVoucherKey + strconv.FormatInt(int64(id), 10),
		Data:      event.NewData,
		TTL:       time.Duration(CacheVoucherTTL) * time.Minute,
	}, nil
}

func (h *VoucherTableHandler) HandleUpdate(event *CanalEvent) (*CacheWorkItem, error) {
	if event.NewData == nil {
		return nil, nil
	}

	id, ok := event.NewData["id"].(float64)
	if !ok {
		return nil, nil
	}

	return &CacheWorkItem{
		Operation: CacheOpSet,
		CacheKey:  CacheVoucherKey + strconv.FormatInt(int64(id), 10),
		Data:      event.NewData,
		TTL:       time.Duration(CacheVoucherTTL) * time.Minute,
	}, nil
}

func (h *VoucherTableHandler) HandleDelete(event *CanalEvent) (*CacheWorkItem, error) {
	if event.OldData == nil {
		return nil, nil
	}

	id, ok := event.OldData["id"].(float64)
	if !ok {
		return nil, nil
	}

	return &CacheWorkItem{
		Operation: CacheOpDelete,
		CacheKey:  CacheVoucherKey + strconv.FormatInt(int64(id), 10),
	}, nil
}

// ============================================================
// Canal协议解析（简化版）
// ============================================================

// ParseCanalEntry 解析Canal Entry（需要配合Canal客户端使用）
// 这里提供模拟实现，用于测试
func ParseCanalEntry(data []byte) (*CanalEvent, error) {
	var event CanalEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}

	// 解析主键
	if event.NewData != nil {
		if id, ok := event.NewData["id"].(float64); ok {
			event.PrimaryKey = "id"
			event.PrimaryValue = int64(id)
		}
	} else if event.OldData != nil {
		if id, ok := event.OldData["id"].(float64); ok {
			event.PrimaryKey = "id"
			event.PrimaryValue = int64(id)
		}
	}

	return &event, nil
}

// SimulateBinlogEvent 模拟Binlog事件（用于测试）
func SimulateBinlogEvent(table string, eventType CanalEventType, oldData, newData map[string]interface{}) *CanalEvent {
	event := &CanalEvent{
		Table:   table,
		Type:    eventType,
		OldData: oldData,
		NewData: newData,
	}

	// 提取主键
	if newData != nil {
		if id, ok := newData["id"].(float64); ok {
			event.PrimaryValue = int64(id)
			event.PrimaryKey = "id"
		}
	} else if oldData != nil {
		if id, ok := oldData["id"].(float64); ok {
			event.PrimaryValue = int64(id)
			event.PrimaryKey = "id"
		}
	}

	return event
}

// ============================================================
// 缓存一致性保证
// ============================================================

// ConsistencyManager 一致性管理器
// 结合本地消息表和Binlog实现最终一致性
type ConsistencyManager struct {
	rdb            *redis.Client
	binlogConsumer *BinlogConsumer
	// 使用字符串存储键名，便于Redis List和Set操作
	pendingOpsKey string // 待执行的缓存操作队列 key
	processedKey  string // 已处理的记录 key（防重）
}

// NewConsistencyManager 创建一致性管理器
func NewConsistencyManager(rdb *redis.Client) *ConsistencyManager {
	return &ConsistencyManager{
		rdb:           rdb,
		pendingOpsKey: "pending_cache_ops",
		processedKey:  "processed_op_ids",
	}
}

// InitPendingOps 初始化待处理操作队列
func (m *ConsistencyManager) InitPendingOps(ctx context.Context) error {
	// 初始化Redis List和Set
	// pending_ops:list - 待处理的操作
	// processed:hash - 已处理的记录，key=操作ID，value=处理时间
	return nil
}

// AddPendingOp 添加待处理操作
func (m *ConsistencyManager) AddPendingOp(ctx context.Context, opID string, operation *CacheWorkItem) error {
	data, err := json.Marshal(operation)
	if err != nil {
		return err
	}

	// 添加到队列尾部
	err = m.rdb.RPush(ctx, "pending_cache_ops", data).Err()
	if err != nil {
		return err
	}

	// 记录到已处理集合（用于去重）
	return m.rdb.SAdd(ctx, "processed_op_ids", opID).Err()
}

// ProcessPendingOps 处理待处理操作
func (m *ConsistencyManager) ProcessPendingOps(ctx context.Context) (int, error) {
	count := 0

	for {
		// 从队列头部取出
		result, err := m.rdb.LPop(ctx, "pending_cache_ops").Result()
		if err == redis.Nil {
			break // 队列为空
		}
		if err != nil {
			return count, err
		}

		var op CacheWorkItem
		if err := json.Unmarshal([]byte(result), &op); err != nil {
			continue
		}

		// 执行缓存操作
		switch op.Operation {
		case CacheOpSet:
			data, _ := json.Marshal(op.Data)
			_ = m.rdb.Set(ctx, op.CacheKey, data, op.TTL).Err()
		case CacheOpDelete:
			_ = m.rdb.Del(ctx, op.CacheKey).Err()
		}

		count++
	}

	return count, nil
}

// CleanExpiredProcessed 清理过期的已处理记录
func (m *ConsistencyManager) CleanExpiredProcessed(ctx context.Context) error {
	// 清理超过24小时的记录
	// 使用SCAN遍历并删除过期的记录
	// 这里简化处理，实际可以使用Redis Sorted Set
	threshold := time.Now().Add(-24 * time.Hour).Unix()
	_ = threshold // 抑制未使用警告
	return nil
}

// ============================================================
// Canal Client 连接配置
// ============================================================

// CanalConfig Canal客户端配置
type CanalConfig struct {
	ServerAddr  string // Canal服务端地址
	Destination string // Canal实例名
	Username    string
	Password    string
	Filter      string // 表过滤规则，如: db1.tb1,db2.tb2
}

// CanalClient Canal客户端（简化版，实际需要使用官方客户端）
type CanalClient struct {
	config   *CanalConfig
	rdb      *redis.Client
	consumer *BinlogConsumer
	running  bool
}

// NewCanalClient 创建Canal客户端
func NewCanalClient(config *CanalConfig, rdb *redis.Client, consumer *BinlogConsumer) *CanalClient {
	return &CanalClient{
		config:   config,
		rdb:      rdb,
		consumer: consumer,
		running:  false,
	}
}

// Connect 连接Canal服务端
func (c *CanalClient) Connect() error {
	// 实际实现需要使用 Canal 官方 Go 客户端
	// github.com/CanalClient/canal-go
	// 这里只提供接口定义
	return nil
}

// Start 启动消费
func (c *CanalClient) Start(ctx context.Context) error {
	if c.running {
		return nil
	}

	c.consumer.Start(ctx)
	c.running = true
	return nil
}

// Stop 停止消费
func (c *CanalClient) Stop() {
	if !c.running {
		return
	}

	c.consumer.Stop()
	c.running = false
}

// ============================================================
// 简化版：使用Redis Stream模拟Binlog消费
// ============================================================

// StreamConsumer Redis Stream消费者
// 用于在没有Canal的情况下模拟Binlog消费
type StreamConsumer struct {
	rdb       *redis.Client
	streamKey string
	groupName string
	consumer  *BinlogConsumer
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewStreamConsumer 创建Stream消费者
func NewStreamConsumer(rdb *redis.Client, streamKey, groupName string, consumer *BinlogConsumer) *StreamConsumer {
	return &StreamConsumer{
		rdb:       rdb,
		streamKey: streamKey,
		groupName: groupName,
		consumer:  consumer,
		stopCh:    make(chan struct{}),
	}
}

// Init 初始化消费者组
func (s *StreamConsumer) Init(ctx context.Context) error {
	// 创建消费者组
	err := s.rdb.XGroupCreateMkStream(ctx, s.streamKey, s.groupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// Start 启动消费
func (s *StreamConsumer) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.consume(ctx)
}

// Stop 停止消费
func (s *StreamConsumer) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// consume 消费循环
func (s *StreamConsumer) consume(ctx context.Context) {
	defer s.wg.Done()

	consumerName := "consumer-" + strconv.FormatInt(time.Now().Unix(), 10)

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// 读取消息
			streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    s.groupName,
				Consumer: consumerName,
				Streams:  []string{s.streamKey, ">"},
				Count:    10,
				Block:    5 * time.Second,
			}).Result()

			if err == redis.Nil {
				continue
			}
			if err != nil {
				fmt.Printf("[StreamConsumer] XReadGroup error: %v\n", err)
				time.Sleep(time.Second)
				continue
			}

			// 处理消息
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					s.processMessage(ctx, msg)
					// 确认消息
					s.rdb.XAck(ctx, s.streamKey, s.groupName, msg.ID)
				}
			}
		}
	}
}

// processMessage 处理消息
func (s *StreamConsumer) processMessage(ctx context.Context, msg redis.XMessage) {
	// 解析消息
	var event CanalEvent
	data, ok := msg.Values["data"]
	if !ok {
		return
	}

	dataStr, ok := data.(string)
	if !ok {
		return
	}

	if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
		fmt.Printf("[StreamConsumer] Unmarshal error: %v\n", err)
		return
	}

	// 交给BinlogConsumer处理
	s.consumer.OnEvent(&event)
}

// PublishBinlogEvent 发布Binlog事件（由数据库触发器或应用层调用）
func (s *StreamConsumer) PublishBinlogEvent(ctx context.Context, event *CanalEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: s.streamKey,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Err()
}
