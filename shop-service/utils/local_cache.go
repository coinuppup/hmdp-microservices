package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

/*
============================================================
多级缓存架构

架构设计：
┌─────────────────────────────────────────────────────────────────┐
│                         应用层                                  │
└───────────────────────────┬─────────────────────────────────────┘
                            │
              ┌─────────────┴─────────────┐
              │                           │
              ▼                           ▼
    ┌─────────────────┐         ┌─────────────────┐
    │   本地缓存       │         │   分布式缓存     │
    │   (Local Cache) │◀───────▶│   (Redis)       │
    │                 │  同步    │                 │
    │  - 内存缓存      │         │  - 跨进程共享    │
    │  - 读写速度快    │         │  - 数据一致性    │
    │  - 无网络开销    │         │  - 持久化        │
    └────────┬────────┘         └────────┬────────┘
             │                           │
             │                           │
             └───────────┬───────────────┘
                         │
                         ▼
              ┌─────────────────────────┐
              │        数据库            │
              │        MySQL            │
              └─────────────────────────┘

特点：
1. 本地缓存作为一级缓存，Redis作为二级缓存
2. 读写流程：先读本地，本地未命中读Redis，Redis未命中读数据库
3. 写操作：先写数据库，再删本地缓存和Redis缓存
4. 通过消息队列保证跨节点缓存一致性

优势：
- 读性能：本地缓存无网络开销，性能最高
- 命中率：本地缓存+Redis二级缓存提高命中率
- 扩展性：支持水平扩展多节点
============================================================
*/

// LocalCacheConfig 本地缓存配置
type LocalCacheConfig struct {
	// 缓存条目上限
	MaxSize int
	// 默认过期时间
	DefaultTTL time.Duration
	// 清理间隔
	CleanInterval time.Duration
	// 读写锁配置
	UseRWMutex bool
	// 统计信息
	EnableStats bool
}

// DefaultLocalCacheConfig 默认配置
func DefaultLocalCacheConfig() *LocalCacheConfig {
	return &LocalCacheConfig{
		MaxSize:       10000,           // 默认10000条
		DefaultTTL:    5 * time.Minute, // 默认5分钟
		CleanInterval: 1 * time.Minute, // 每分钟清理
		UseRWMutex:    true,
		EnableStats:   true,
	}
}

// LocalCacheEntry 本地缓存条目
type LocalCacheEntry struct {
	Value       interface{}
	ExpireAt    time.Time
	AccessTime  time.Time
	AccessCount int64
}

// LocalCache 本地缓存（实现多级缓存的一级缓存）
type LocalCache struct {
	config *LocalCacheConfig
	cache  map[string]*LocalCacheEntry
	mu     sync.RWMutex
	// 统计信息
	stats *CacheStats
	// 过期处理回调
	onExpire func(key string, value interface{})
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits      int64 `json:"hits"`      // 命中次数
	Misses    int64 `json:"misses"`    // 未命中次数
	Evictions int64 `json:"evictions"` // 驱逐次数
	Sets      int64 `json:"sets"`      // 设置次数
	Deletes   int64 `json:"deletes"`   // 删除次数
	hitRate   float64
	mu        sync.RWMutex
}

// NewLocalCache 创建本地缓存
func NewLocalCache(cfg *LocalCacheConfig) *LocalCache {
	if cfg == nil {
		cfg = DefaultLocalCacheConfig()
	}

	cache := &LocalCache{
		config: cfg,
		cache:  make(map[string]*LocalCacheEntry, cfg.MaxSize),
		stats:  &CacheStats{},
		stopCh: make(chan struct{}),
	}

	// 启动定时清理过期条目
	if cfg.CleanInterval > 0 {
		cache.wg.Add(1)
		go cache.cleanupLoop()
	}

	return cache
}

// cleanupLoop 清理过期条目
func (c *LocalCache) cleanupLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanupExpired()
		}
	}
}

// cleanupExpired 清理过期条目
func (c *LocalCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evicted := 0

	for key, entry := range c.cache {
		if entry.ExpireAt.Before(now) {
			// 调用过期回调
			if c.onExpire != nil {
				c.onExpire(key, entry.Value)
			}
			delete(c.cache, key)
			evicted++
		}
	}

	if evicted > 0 {
		c.stats.mu.Lock()
		c.stats.Evictions += int64(evicted)
		c.stats.mu.Unlock()
	}
}

// Set 设置缓存
func (c *LocalCache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expireTime := time.Now().Add(c.config.DefaultTTL)
	if len(ttl) > 0 && ttl[0] > 0 {
		expireTime = time.Now().Add(ttl[0])
	}

	c.cache[key] = &LocalCacheEntry{
		Value:       value,
		ExpireAt:    expireTime,
		AccessTime:  time.Now(),
		AccessCount: 0,
	}

	// 超过容量，清理最旧的条目
	if len(c.cache) > c.config.MaxSize {
		c.evictOldest()
	}

	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.Sets++
		c.stats.mu.Unlock()
	}
}

// Get 获取缓存
func (c *LocalCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Misses++
			c.stats.mu.Unlock()
		}
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpireAt) {
		c.mu.RUnlock()
		c.mu.Lock()
		delete(c.cache, key)
		c.mu.Unlock()

		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Misses++
			c.stats.mu.Unlock()
		}
		return nil, false
	}

	// 更新访问信息
	entry.AccessTime = time.Now()
	entry.AccessCount++

	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.Hits++
		c.stats.mu.Unlock()
	}

	return entry.Value, true
}

// Delete 删除缓存
func (c *LocalCache) Delete(ctx context.Context, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.cache[key]; exists {
		delete(c.cache, key)

		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Deletes++
			c.stats.mu.Unlock()
		}
	}
}

// Clear 清空缓存
func (c *LocalCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*LocalCacheEntry)
}

// evictOldest 驱逐最旧的条目（使用LRU策略）
func (c *LocalCache) evictOldest() {
	var oldestKey string
	var oldestTime = time.Now()

	for key, entry := range c.cache {
		if entry.AccessTime.Before(oldestTime) {
			oldestTime = entry.AccessTime
			oldestKey = key
		}
	}

	if oldestKey != "" {
		if c.onExpire != nil {
			c.onExpire(oldestKey, c.cache[oldestKey].Value)
		}
		delete(c.cache, oldestKey)

		if c.config.EnableStats {
			c.stats.Evictions++
		}
	}
}

// GetStats 获取统计信息
func (c *LocalCache) GetStats() (hits, misses, evictions, sets, deletes int64, hitRate float64) {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	hits = c.stats.Hits
	misses = c.stats.Misses
	evictions = c.stats.Evictions
	sets = c.stats.Sets
	deletes = c.stats.Deletes

	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return
}

// ResetStats 重置统计信息
func (c *LocalCache) ResetStats() {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()
	c.stats.Hits = 0
	c.stats.Misses = 0
	c.stats.Evictions = 0
	c.stats.Sets = 0
	c.stats.Deletes = 0
}

// SetOnExpire 设置过期回调
func (c *LocalCache) SetOnExpire(callback func(key string, value interface{})) {
	c.onExpire = callback
}

// Stop 停止缓存
func (c *LocalCache) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// Size 获取缓存大小
func (c *LocalCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// ============================================================
// 多级缓存客户端
// ============================================================

// MultiLevelCacheClient 多级缓存客户端
// 结合本地缓存和分布式缓存（Redis）实现多级缓存
type MultiLevelCacheClient struct {
	rdb        *redis.Client
	localCache *LocalCache
	producer   *CacheMessageProducer
	broker     string
	topic      string
}

// NewMultiLevelCacheClient 创建多级缓存客户端
func NewMultiLevelCacheClient(rdb *redis.Client, localConfig *LocalCacheConfig, broker, topic string) *MultiLevelCacheClient {
	client := &MultiLevelCacheClient{
		rdb:        rdb,
		localCache: NewLocalCache(localConfig),
		broker:     broker,
		topic:      topic,
	}

	// 设置本地缓存过期回调：当本地缓存过期时，同步删除Redis缓存
	client.localCache.SetOnExpire(func(key string, value interface{}) {
		fmt.Printf("[MultiLevelCache] Local cache expired: %s\n", key)
		// 可以选择同步删除Redis缓存，或者通过消息队列
	})

	return client
}

// SetProducer 设置消息生产者（用于缓存删除通知）
func (m *MultiLevelCacheClient) SetProducer(producer *CacheMessageProducer) {
	m.producer = producer
}

// Get 多级缓存获取
// 1. 先从本地缓存获取
// 2. 本地缓存未命中，从Redis获取
// 3. Redis未命中，返回错误
func (m *MultiLevelCacheClient) Get(ctx context.Context, key string, dest interface{}) error {
	// 1. 尝试从本地缓存获取
	value, found := m.localCache.Get(ctx, key)
	if found {
		return m.unmarshal(value, dest)
	}

	// 2. 本地缓存未命中，从Redis获取
	data, err := m.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}

	// 3. 写入本地缓存（异步，不阻塞返回）
	value, err = m.unmarshalBytes(data, dest)
	if err == nil && value != nil {
		// 使用较短的TTL同步到本地缓存，避免多节点不一致时间过长
		m.localCache.Set(ctx, key, data, 30*time.Second)
	}

	return err
}

// Set 多级缓存设置
// 1. 直接设置Redis缓存
// 2. 本地缓存通过后续读取时填充
func (m *MultiLevelCacheClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := m.marshal(value)
	if err != nil {
		return err
	}

	// 设置Redis缓存
	if err := m.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return err
	}

	// 同步更新本地缓存（使用较短的TTL）
	m.localCache.Set(ctx, key, data, minDuration(ttl, 30*time.Second))

	return nil
}

// Delete 多级缓存删除
// 1. 删除本地缓存
// 2. 删除Redis缓存
// 3. 发送消息通知其他节点
func (m *MultiLevelCacheClient) Delete(ctx context.Context, key string) error {
	// 删除本地缓存
	m.localCache.Delete(ctx, key)

	// 删除Redis缓存
	if err := m.rdb.Del(ctx, key).Err(); err != nil {
		return err
	}

	// 发送消息通知其他节点删除缓存
	if m.producer != nil {
		_ = m.producer.SendDelete(ctx, key)
	}

	return nil
}

// DeletePattern 批量删除
func (m *MultiLevelCacheClient) DeletePattern(ctx context.Context, pattern string) error {
	// 使用SCAN遍历匹配的key
	var cursor uint64
	var keys []string

	for {
		var nextKeys []string
		var err error
		nextKeys, cursor, err = m.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		keys = append(keys, nextKeys...)
		if cursor == 0 {
			break
		}
	}

	// 删除本地缓存匹配的key
	for _, key := range keys {
		m.localCache.Delete(ctx, key)
	}

	// 删除Redis缓存
	if len(keys) > 0 {
		if err := m.rdb.Del(ctx, keys...).Err(); err != nil {
			return err
		}
	}

	// 发送消息通知其他节点
	if m.producer != nil {
		_ = m.producer.SendDeletePattern(ctx, pattern)
	}

	return nil
}

// InvalidateLocalCache 使本地缓存失效（由消息驱动）
func (m *MultiLevelCacheClient) InvalidateLocalCache(ctx context.Context, key string) {
	m.localCache.Delete(ctx, key)
}

// SafeMutex 安全版互斥锁（带看门狗+Lua脚本释放锁）
func (m *MultiLevelCacheClient) SafeMutex(ctx context.Context, key string, dest interface{}, ttl time.Duration, query func() (interface{}, error)) error {
	// 1. 尝试从多级缓存获取
	if err := m.Get(ctx, key, dest); err == nil {
		return nil // 命中缓存，直接返回
	}

	// 2. 获取分布式锁（带看门狗）
	lockKey := "lock:" + key
	cacheClient := &CacheClient{rdb: m.rdb}
	if !cacheClient.WatchdogLock(ctx, lockKey) {
		// 获取失败，等待后重试
		time.Sleep(50 * time.Millisecond)
		return m.SafeMutex(ctx, key, dest, ttl, query)
	}

	// 3. 获取锁成功，查询数据库
	defer func() {
		_ = cacheClient.WatchdogUnlock(ctx, lockKey)
	}()

	// 4. 双重检查缓存（防止其他线程已加载）
	if err := m.Get(ctx, key, dest); err == nil {
		return nil
	}

	// 5. 查询数据库
	data, err := query()
	if err != nil {
		// 数据库不存在，设置空值缓存
		_ = m.Set(ctx, "cache:null:"+key, "", 5*time.Minute)
		return err
	}

	// 6. 设置缓存（Redis + 本地）
	_ = m.Set(ctx, key, data, ttl)

	return m.unmarshal(data, dest)
}

// GetStats 获取统计信息
func (m *MultiLevelCacheClient) GetStats() (hits, misses, evictions, sets, deletes int64, hitRate float64) {
	return m.localCache.GetStats()
}

// Stop 停止缓存客户端
func (m *MultiLevelCacheClient) Stop() {
	m.localCache.Stop()
}

// marshal 序列化
func (m *MultiLevelCacheClient) marshal(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

// unmarshal 反序列化
func (m *MultiLevelCacheClient) unmarshal(value interface{}, dest interface{}) error {
	if value == nil {
		return fmt.Errorf("cache value is nil")
	}

	// 如果是字节数组，直接反序列化
	if data, ok := value.([]byte); ok {
		return json.Unmarshal(data, dest)
	}

	// 如果是字符串，转换为字节数组再反序列化
	if str, ok := value.(string); ok {
		return json.Unmarshal([]byte(str), dest)
	}

	// 如果是目标类型直接返回
	return json.Unmarshal([]byte(fmt.Sprintf("%v", value)), dest)
}

// unmarshalBytes 从字节数组反序列化
func (m *MultiLevelCacheClient) unmarshalBytes(data []byte, dest interface{}) (interface{}, error) {
	if err := json.Unmarshal(data, dest); err != nil {
		return nil, err
	}
	return dest, nil
}

// minDuration 返回较小的时间
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// ============================================================
// 便捷函数
// ============================================================

// NewMultiLevelCacheClientWithDefaults 创建默认配置的多级缓存客户端
func NewMultiLevelCacheClientWithDefaults(rdb *redis.Client, broker string) *MultiLevelCacheClient {
	return NewMultiLevelCacheClient(rdb, DefaultLocalCacheConfig(), broker, "cache-invalidate")
}
