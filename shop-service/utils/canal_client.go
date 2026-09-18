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
	"github.com/withlin/canal-go/client"
	pbe "github.com/withlin/canal-go/protocol/entry"
	"google.golang.org/protobuf/proto"
)

/*
============================================================
Canal 订阅 MySQL Binlog 完整实现

方案说明：
1. Canal模拟MySQL从库，订阅MySQL的Binlog
2. 解析Binlog事件，转换为业务可用的数据变更事件
3. 异步执行缓存删除/更新操作
4. 实现缓存的最终一致性

架构：
  [MySQL] --Binlog--> [Canal Server] ---> [Canal Client] ---> [Redis/本地缓存]
                                      (解析事件)                 (更新缓存)

优点：
- 完全解耦：不需要在业务代码中显式操作缓存
- 自动感知：Canal自动监听数据库变化
- 跨应用有效：多个应用可以共享同一个Canal消费逻辑
============================================================
*/

// CanalEventType Canal事件类型
type CanalEventType string

const (
	CanalEventInsert CanalEventType = "INSERT"
	CanalEventUpdate CanalEventType = "UPDATE"
	CanalEventDelete CanalEventType = "DELETE"
)

// CanalEvent Canal事件结构（业务层使用）
type CanalEvent struct {
	TableName  string                 `json:"tableName"`  // 表名
	EventType CanalEventType         `json:"eventType"` // 事件类型
	PrimaryKey string                 `json:"primaryKey"` // 主键名
	PrimaryID  int64                  `json:"primaryID"` // 主键值
	OldData    map[string]interface{} `json:"oldData"`    // 变更前数据
	NewData    map[string]interface{} `json:"newData"`    // 变更后数据
	Timestamp  int64                  `json:"timestamp"` // 事件时间戳
}

// TableHandler 表处理器接口
type TableHandler interface {
	GetTableName() string
	HandleInsert(event *CanalEvent) error
	HandleUpdate(event *CanalEvent) error
	HandleDelete(event *CanalEvent) error
}

// TableHandlerFunc 函数式处理器
type TableHandlerFunc struct {
	tableName string
	onInsert  func(event *CanalEvent) error
	onUpdate  func(event *CanalEvent) error
	onDelete  func(event *CanalEvent) error
}

func (h *TableHandlerFunc) GetTableName() string {
	return h.tableName
}

func (h *TableHandlerFunc) HandleInsert(event *CanalEvent) error {
	if h.onInsert != nil {
		return h.onInsert(event)
	}
	return nil
}

func (h *TableHandlerFunc) HandleUpdate(event *CanalEvent) error {
	if h.onUpdate != nil {
		return h.onUpdate(event)
	}
	return nil
}

func (h *TableHandlerFunc) HandleDelete(event *CanalEvent) error {
	if h.onDelete != nil {
		return h.onDelete(event)
	}
	return nil
}

// CanalConfig Canal客户端配置
type CanalConfig struct {
	Server     string // Canal服务端地址 (ip:port)
	Destination string // Canal实例名
	Username   string // Canal用户名
	Password   string // Canal密码
	Filter     string // 过滤规则，如: db1\\.table1,db2\\.table2
	BatchSize  int32  // 批量获取大小
	Timeout    int32  // 超时时间(毫秒)
}

// CanalCacheClient Canal缓存客户端
// 监听MySQL Binlog变化，自动更新/删除缓存
type CanalCacheClient struct {
	config      *CanalConfig
	rdb         *redis.Client
	localCache  *LocalCache
	connector   client.CanalConnector
	handlers    map[string]TableHandler
	handlersMu  sync.RWMutex
	workerCount int
	stopCh      chan struct{}
	wg          sync.WaitGroup
	running     bool
	// 统计
	stats *CanalStats
}

// CanalStats Canal客户端统计
type CanalStats struct {
	Processed int64 `json:"processed"`
	Deleted   int64 `json:"deleted"`
	Updated   int64 `json:"updated"`
	Errors    int64 `json:"errors"`
	mu        sync.RWMutex
}

// NewCanalCacheClient 创建Canal缓存客户端
func NewCanalCacheClient(cfg *CanalConfig, rdb *redis.Client, workerCount int) *CanalCacheClient {
	if workerCount <= 0 {
		workerCount = 4
	}

	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60000
	}

	return &CanalCacheClient{
		config:      cfg,
		rdb:         rdb,
		localCache:  NewLocalCache(DefaultLocalCacheConfig()),
		handlers:    make(map[string]TableHandler),
		workerCount: workerCount,
		stopCh:      make(chan struct{}),
		stats:       &CanalStats{},
	}
}

// RegisterHandler 注册表处理器
func (c *CanalCacheClient) RegisterHandler(handler TableHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[handler.GetTableName()] = handler
}

// RegisterHandlerFunc 注册函数式处理器
func (c *CanalCacheClient) RegisterHandlerFunc(tableName string,
	insertFn, updateFn, deleteFn func(event *CanalEvent) error) {
	handler := &TableHandlerFuncWrapper{
		tableName: tableName,
		onInsert:  insertFn,
		onUpdate:  updateFn,
		onDelete:  deleteFn,
	}
	c.RegisterHandler(handler)
}

// RegisterTableCacheHandler 注册表缓存处理器（简化版）
// 根据表名和缓存前缀自动生成处理器
func (c *CanalCacheClient) RegisterTableCacheHandler(tableName, cacheKeyPrefix string, ttl time.Duration) {
	handler := NewTableCacheHandler(tableName, cacheKeyPrefix, ttl, c.rdb, c.localCache)
	c.RegisterHandler(handler)
}

// Start 启动Canal客户端
func (c *CanalCacheClient) Start(ctx context.Context) error {
	if c.running {
		return nil
	}

	// 解析server地址和端口
	serverParts := strings.Split(c.config.Server, ":")
	var serverHost string
	var serverPort int
	if len(serverParts) == 2 {
		serverHost = serverParts[0]
		serverPort, _ = strconv.Atoi(serverParts[1])
	} else {
		serverHost = c.config.Server
		serverPort = 11111
	}

	// 创建Canal连接器
	c.connector = client.NewSimpleCanalConnector(
		serverHost,
		serverPort,
		c.config.Username,
		c.config.Password,
		c.config.Destination,
		c.config.Timeout,
		c.config.BatchSize,
	)

	// 连接Canal
	if err := c.connector.Connect(); err != nil {
		return fmt.Errorf("failed to connect to Canal: %w", err)
	}

	// 订阅
	if err := c.connector.Subscribe(c.config.Filter); err != nil {
		c.connector.DisConnection()
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	c.running = true

	// 启动消费协程
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.consume(ctx, i)
	}

	fmt.Printf("[CanalCacheClient] Started, watching %s\n", c.config.Filter)
	return nil
}

// Stop 停止Canal客户端
func (c *CanalCacheClient) Stop() {
	if !c.running {
		return
	}

	close(c.stopCh)
	c.wg.Wait()

	if c.connector != nil {
		c.connector.UnSubscribe()
		c.connector.DisConnection()
	}

	c.localCache.Stop()
	c.running = false

	fmt.Println("[CanalCacheClient] Stopped")
}

// consume 消费Canal消息
func (c *CanalCacheClient) consume(ctx context.Context, id int) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// 获取消息
			message, err := c.connector.Get(c.config.BatchSize, nil, nil)
			if err != nil {
				fmt.Printf("[CanalCacheClient] Get error: %v\n", err)
				time.Sleep(time.Second)
				continue
			}

			// 处理消息
			for _, entry := range message.Entries {
				c.processEntry(entry)
			}
		}
	}
}

// processEntry 处理Entry
func (c *CanalCacheClient) processEntry(entry pbe.Entry) {
	// 只处理RowData
	if entry.GetEntryType() != pbe.EntryType_ROWDATA {
		return
	}

	// 解析表名
	tableName := entry.GetHeader().GetTableName()
	if tableName == "" {
		return
	}

	// 获取事件类型
	var eventType CanalEventType
	switch entry.GetHeader().GetEventType() {
	case pbe.EventType_INSERT:
		eventType = CanalEventInsert
	case pbe.EventType_UPDATE:
		eventType = CanalEventUpdate
	case pbe.EventType_DELETE:
		eventType = CanalEventDelete
	default:
		return
	}

	// 解析RowChange
	rowChange := new(pbe.RowChange)
	if err := proto.Unmarshal(entry.GetStoreValue(), rowChange); err != nil {
		fmt.Printf("[CanalCacheClient] Unmarshal error: %v\n", err)
		return
	}

	// 获取主键
	primaryKey := "id"
	primaryID := int64(0)

	// 根据事件类型获取数据
	var oldData, newData map[string]interface{}

	rowDatas := rowChange.GetRowDatas()
	if len(rowDatas) == 0 {
		return
	}

	if eventType == CanalEventDelete {
		for _, col := range rowDatas[0].GetBeforeColumns() {
			colName := col.GetName()

			if colName == "id" {
				primaryID = parseInt64(col.GetValue())
			}

			if oldData == nil {
				oldData = make(map[string]interface{})
			}
			oldData[colName] = parseColumnValue(col)
		}
	} else {
		for _, col := range rowDatas[0].GetAfterColumns() {
			colName := col.GetName()

			if colName == "id" {
				primaryID = parseInt64(col.GetValue())
			}

			if newData == nil {
				newData = make(map[string]interface{})
			}
			newData[colName] = parseColumnValue(col)
		}
	}

	// 构建事件
	event := &CanalEvent{
		TableName:  tableName,
		EventType:  eventType,
		PrimaryKey: primaryKey,
		PrimaryID:  primaryID,
		OldData:    oldData,
		NewData:    newData,
		Timestamp:  time.Now().UnixMilli(),
	}

	// 查找处理器并处理
	c.handlersMu.RLock()
	handler, ok := c.handlers[tableName]
	c.handlersMu.RUnlock()

	if !ok {
		// 没有特定处理器，跳过
		return
	}

	var err error
	switch eventType {
	case CanalEventInsert:
		err = handler.HandleInsert(event)
	case CanalEventUpdate:
		err = handler.HandleUpdate(event)
	case CanalEventDelete:
		err = handler.HandleDelete(event)
	}

	c.stats.mu.Lock()
	c.stats.Processed++
	if err != nil {
		c.stats.Errors++
		fmt.Printf("[CanalCacheClient] Handle event error: %v\n", err)
	} else {
		if eventType == CanalEventDelete {
			c.stats.Deleted++
		} else {
			c.stats.Updated++
		}
	}
	c.stats.mu.Unlock()
}

// parseInt64 解析int64
func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// parseColumnValue 解析列值
func parseColumnValue(col *pbe.Column) interface{} {
	if col.GetIsNull() {
		return nil
	}

	sqlType := col.GetSqlType()
	value := col.GetValue()

	// 根据sqlType判断类型 (sqlType是int32类型)
	switch sqlType {
	case 0: // VARCHAR, CHAR, TEXT 等字符串类型
		return value
	case 1: // BIGINT
		return parseInt64(value)
	case 2: // LONG
		return parseInt64(value)
	case 3: // SHOLT
		v, _ := strconv.ParseInt(value, 10, 16)
		return v
	case 4: // INT24
		v, _ := strconv.ParseInt(value, 10, 32)
		return v
	case 5: // FLOAT
		v, _ := strconv.ParseFloat(value, 32)
		return v
	case 6: // DOUBLE
		v, _ := strconv.ParseFloat(value, 64)
		return v
	case 7: // NULL
		return nil
	case 8: // TIMESTAMP
		return value
	case 9: // LONGLONG (BIGINT)
		return parseInt64(value)
	case 10: // DATE
		return value
	case 11: // TIME
		return value
	case 12: // DATETIME
		return value
	case 13: // YEAR
		return value
	default:
		// 尝试解析为数字
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			return v
		}
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
		return value
	}
}

// GetStats 获取统计信息
func (c *CanalCacheClient) GetStats() (processed, deleted, updated, errors int64) {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()
	return c.stats.Processed, c.stats.Deleted, c.stats.Updated, c.stats.Errors
}

// ============================================================
// 预定义的表处理器
// ============================================================

// TableCacheHandler 表缓存处理器
type TableCacheHandler struct {
	tableName      string
	cacheKeyPrefix string
	ttl            time.Duration
	rdb            *redis.Client
	localCache     *LocalCache
}

// NewTableCacheHandler 创建表缓存处理器
func NewTableCacheHandler(tableName, cacheKeyPrefix string, ttl time.Duration, rdb *redis.Client, localCache *LocalCache) *TableCacheHandler {
	return &TableCacheHandler{
		tableName:      tableName,
		cacheKeyPrefix: cacheKeyPrefix,
		ttl:            ttl,
		rdb:            rdb,
		localCache:     localCache,
	}
}

func (h *TableCacheHandler) GetTableName() string {
	return h.tableName
}

func (h *TableCacheHandler) HandleInsert(event *CanalEvent) error {
	if event.NewData == nil {
		return nil
	}

	// 插入数据后，删除缓存（让下次查询重新加载）
	cacheKey := h.cacheKeyPrefix + strconv.FormatInt(event.PrimaryID, 10)
	return h.deleteCache(cacheKey)
}

func (h *TableCacheHandler) HandleUpdate(event *CanalEvent) error {
	if event.NewData == nil {
		return nil
	}

	// 更新数据后，删除缓存
	cacheKey := h.cacheKeyPrefix + strconv.FormatInt(event.PrimaryID, 10)
	return h.deleteCache(cacheKey)
}

func (h *TableCacheHandler) HandleDelete(event *CanalEvent) error {
	// 删除数据后，删除缓存
	cacheKey := h.cacheKeyPrefix + strconv.FormatInt(event.PrimaryID, 10)
	return h.deleteCache(cacheKey)
}

func (h *TableCacheHandler) deleteCache(cacheKey string) error {
	// 删除本地缓存
	h.localCache.Delete(context.Background(), cacheKey)

	// 删除Redis缓存
	if err := h.rdb.Del(context.Background(), cacheKey).Err(); err != nil {
		return fmt.Errorf("delete redis cache error: %w", err)
	}

	fmt.Printf("[TableCacheHandler] Deleted cache: %s\n", cacheKey)
	return nil
}

// TableHandlerFuncWrapper 函数式处理器包装
type TableHandlerFuncWrapper struct {
	tableName string
	onInsert  func(event *CanalEvent) error
	onUpdate  func(event *CanalEvent) error
	onDelete  func(event *CanalEvent) error
}

func (h *TableHandlerFuncWrapper) GetTableName() string {
	return h.tableName
}

func (h *TableHandlerFuncWrapper) HandleInsert(event *CanalEvent) error {
	if h.onInsert != nil {
		return h.onInsert(event)
	}
	return nil
}

func (h *TableHandlerFuncWrapper) HandleUpdate(event *CanalEvent) error {
	if h.onUpdate != nil {
		return h.onUpdate(event)
	}
	return nil
}

func (h *TableHandlerFuncWrapper) HandleDelete(event *CanalEvent) error {
	if h.onDelete != nil {
		return h.onDelete(event)
	}
	return nil
}

// ============================================================
// 便捷构造函数
// ============================================================

// NewCanalCacheClientFromConfig 从配置创建Canal客户端
func NewCanalCacheClientFromConfig(server, destination, username, password, filter string, rdb *redis.Client) *CanalCacheClient {
	cfg := &CanalConfig{
		Server:      server,
		Destination: destination,
		Username:    username,
		Password:    password,
		Filter:      filter,
		BatchSize:   1000,
		Timeout:     60000,
	}
	return NewCanalCacheClient(cfg, rdb, 4)
}

// NewShopCanalHandler 创建商铺表Canal处理器
func NewShopCanalHandler(rdb *redis.Client, localCache *LocalCache) *TableCacheHandler {
	return NewTableCacheHandler("tb_shop", CacheShopKey, time.Duration(CacheShopTTL)*time.Minute, rdb, localCache)
}

// NewVoucherCanalHandler 创建优惠券表Canal处理器
func NewVoucherCanalHandler(rdb *redis.Client, localCache *LocalCache) *TableCacheHandler {
	return NewTableCacheHandler("tb_voucher", CacheVoucherKey, time.Duration(CacheVoucherTTL)*time.Minute, rdb, localCache)
}

// NewUserCanalHandler 创建用户表Canal处理器
func NewUserCanalHandler(rdb *redis.Client, localCache *LocalCache) *TableCacheHandler {
	return NewTableCacheHandler("tb_user", CacheUserKey, time.Duration(CacheUserTTL)*time.Minute, rdb, localCache)
}

// ============================================================
// Canal适配器：将Canal事件转发到现有的BinlogCacheSync
// ============================================================

// CanalToBinlogAdapter Canal适配器
// 将Canal事件转换为BinlogMessage，转发给BinlogCacheSync
type CanalToBinlogAdapter struct {
	canalClient   *CanalCacheClient
	binlogSync    *BinlogCacheSync
	tableMappings map[string]string // Canal表名 -> Binlog表名
}

// NewCanalToBinlogAdapter 创建适配器
func NewCanalToBinlogAdapter(canalClient *CanalCacheClient, binlogSync *BinlogCacheSync) *CanalToBinlogAdapter {
	return &CanalToBinlogAdapter{
		canalClient:   canalClient,
		binlogSync:    binlogSync,
		tableMappings: make(map[string]string),
	}
}

// AddTableMapping 添加表名映射
func (a *CanalToBinlogAdapter) AddTableMapping(canalTable, binlogTable string) {
	a.tableMappings[canalTable] = binlogTable
}

// Start 启动适配器
func (a *CanalToBinlogAdapter) Start(ctx context.Context) error {
	return a.canalClient.Start(ctx)
}

// Stop 停止适配器
func (a *CanalToBinlogAdapter) Stop() {
	a.canalClient.Stop()
}

// GetStats 获取统计
func (a *CanalToBinlogAdapter) GetStats() (processed, deleted, updated, errors int64) {
	return a.canalClient.GetStats()
}

// ============================================================
// 模拟Canal事件（用于测试）
// ============================================================

// SimulateCanalEvent 模拟Canal事件（用于测试）
func SimulateCanalEvent(tableName string, eventType CanalEventType, oldData, newData map[string]interface{}) *CanalEvent {
	primaryID := int64(0)

	if newData != nil {
		if id, ok := newData["id"].(float64); ok {
			primaryID = int64(id)
		}
	} else if oldData != nil {
		if id, ok := oldData["id"].(float64); ok {
			primaryID = int64(id)
		}
	}

	return &CanalEvent{
		TableName:  tableName,
		EventType:  eventType,
		PrimaryKey: "id",
		PrimaryID:  primaryID,
		OldData:    oldData,
		NewData:    newData,
		Timestamp:  time.Now().UnixMilli(),
	}
}

// ============================================================
// JSON序列化辅助
// ============================================================

// ToJSON 转换为JSON
func (e *CanalEvent) ToJSON() (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON解析
func CanalEventFromJSON(s string) (*CanalEvent, error) {
	var event CanalEvent
	if err := json.Unmarshal([]byte(s), &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// ============================================================
// 分布式锁辅助（用于缓存更新）
// ============================================================

// CanalCacheLocker Canal缓存锁
type CanalCacheLocker struct {
	rdb *redis.Client
}

// NewCanalCacheLocker 创建缓存锁
func NewCanalCacheLocker(rdb *redis.Client) *CanalCacheLocker {
	return &CanalCacheLocker{rdb: rdb}
}

// LockWithTimeout 带超时锁
func (l *CanalCacheLocker) LockWithTimeout(ctx context.Context, key, value string, timeout time.Duration) (bool, error) {
	return l.rdb.SetNX(ctx, "lock:"+key, value, timeout).Result()
}

// Unlock 解锁
func (l *CanalCacheLocker) Unlock(ctx context.Context, key, value string) error {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)
	_, err := script.Run(ctx, l.rdb, []string{"lock:" + key}, value).Result()
	return err
}

// ============================================================
// 兼容旧版接口
// ============================================================

// BinlogConsumer 兼容旧版接口
type BinlogConsumer = CanalCacheClient

// NewBinlogConsumer 兼容旧版接口
func NewBinlogConsumer(rdb *redis.Client, workerCount int) *BinlogConsumer {
	return NewCanalCacheClient(&CanalConfig{
		Server:      "127.0.0.1:11111",
		Destination: "example",
		Username:    "canal",
		Password:    "canal",
		Filter:      ".*\\..*",
	}, rdb, workerCount)
}

// CanalClient 兼容旧版接口
type CanalClient = CanalCacheClient

// NewCanalClient 兼容旧版接口
func NewCanalClient(config *CanalConfig, rdb *redis.Client, consumer *BinlogConsumer) *CanalClient {
	return NewCanalCacheClient(config, rdb, 4)
}

// Connect 兼容旧版接口
func (c *CanalCacheClient) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.Start(ctx)
}

// ============================================================
// Canal健康检查
// ============================================================

// CanalHealthCheck Canal健康检查
type CanalHealthCheck struct {
	server        string
	timeout       int32
	checkInterval time.Duration
}

// NewCanalHealthCheck 创建健康检查
func NewCanalHealthCheck(server string, timeout int32, checkInterval time.Duration) *CanalHealthCheck {
	return &CanalHealthCheck{
		server:        server,
		timeout:       timeout,
		checkInterval: checkInterval,
	}
}

// Check 检查Canal服务是否可用
func (h *CanalHealthCheck) Check() error {
	// 解析server地址和端口
	serverParts := strings.Split(h.server, ":")
	var serverHost string
	var serverPort int
	if len(serverParts) == 2 {
		serverHost = serverParts[0]
		serverPort, _ = strconv.Atoi(serverParts[1])
	} else {
		serverHost = h.server
		serverPort = 11111
	}

	// 尝试连接
	connector := client.NewSimpleCanalConnector(
		serverHost,
		serverPort,
		"canal",
		"canal",
		"example",
		5000,
		5000,
	)

	if err := connector.Connect(); err != nil {
		return fmt.Errorf("cannot connect to Canal server %s: %w", h.server, err)
	}

	connector.DisConnection()
	return nil
}

// StartLoop 启动健康检查循环
func (h *CanalHealthCheck) StartLoop(ctx context.Context, onSuccess, onFailure func()) {
	go func() {
		ticker := time.NewTicker(h.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := h.Check(); err != nil {
					fmt.Printf("[CanalHealthCheck] Check failed: %v\n", err)
					if onFailure != nil {
						onFailure()
					}
				} else {
					if onSuccess != nil {
						onSuccess()
					}
				}
			}
		}
	}()
}

// ============================================================
// Canal表过滤器
// ============================================================

// ParseFilter 解析过滤规则
// 支持格式: db1.table1,db2.table2 或正则表达式
func ParseFilter(filter string) []string {
	if filter == "" {
		return nil
	}

	// 分割多个规则
	rules := strings.Split(filter, ",")
	result := make([]string, 0, len(rules))

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule != "" {
			result = append(result, rule)
		}
	}

	return result
}

// BuildFilter 构建过滤规则
func BuildFilter(database string, tables []string) string {
	if len(tables) == 0 {
		return database + "\\..*"
	}

	var rules []string
	for _, table := range tables {
		rules = append(rules, database+"\\."+table)
	}

	return strings.Join(rules, ",")
}
