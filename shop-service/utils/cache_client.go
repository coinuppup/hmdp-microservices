package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CacheClient 缓存客户端
type CacheClient struct {
	rdb                *redis.Client
	localLockValue     map[string]string
	localMu            sync.RWMutex
	shopBloomFilter    *BloomFilter
	voucherBloomFilter *BloomFilter
	whitelistDeleted   map[string]map[int64]bool // 用于记录软删除的ID
	whitelistMu        sync.RWMutex
}

// NewCacheClient 创建缓存客户端
func NewCacheClient(rdb *redis.Client) *CacheClient {
	return &CacheClient{
		rdb:                rdb,
		localLockValue:     make(map[string]string),
		shopBloomFilter:    NewBloomFilter(rdb, "bloom:shop", 100000, 0.01),
		voucherBloomFilter: NewBloomFilter(rdb, "bloom:voucher", 100000, 0.01),
		whitelistDeleted:   make(map[string]map[int64]bool),
	}
}

// InitBloomFilterWithData 使用数据初始化布隆过滤器
func (c *CacheClient) InitBloomFilterWithData(ctx context.Context, shopRepo interface{ FindAllIDs() ([]int64, error) }, voucherRepo interface{ FindAllIDs() ([]int64, error) }) error {
	// 初始化商铺布隆过滤器
	if shopRepo != nil {
		ids, err := shopRepo.FindAllIDs()
		if err != nil {
			return err
		}
		for _, id := range ids {
			if c.shopBloomFilter != nil {
				_ = c.shopBloomFilter.AddInt64(ctx, id)
			}
		}
	}

	// 初始化优惠券布隆过滤器
	if voucherRepo != nil {
		ids, err := voucherRepo.FindAllIDs()
		if err != nil {
			return err
		}
		for _, id := range ids {
			if c.voucherBloomFilter != nil {
				_ = c.voucherBloomFilter.AddInt64(ctx, id)
			}
		}
	}

	return nil
}

// CheckShopExists 检查商铺是否存在（布隆过滤器 + 白名单）
// 返回值：true=可能存在，false=一定不存在
func (c *CacheClient) CheckShopExists(ctx context.Context, id int64) (bool, error) {
	// 先检查白名单（软删除标记）
	c.whitelistMu.RLock()
	deleted, exists := c.whitelistDeleted["shop"][id]
	c.whitelistMu.RUnlock()

	if exists && deleted {
		// 在白名单中标记为已删除
		return false, nil
	}

	// 检查布隆过滤器
	if c.shopBloomFilter != nil {
		exists, err := c.shopBloomFilter.ExistsInt64(ctx, id)
		if err != nil {
			return true, nil // 错误时保守返回true
		}
		return exists, nil
	}

	// 过滤器未初始化，放行
	return true, nil
}

// CheckVoucherExists 检查优惠券是否存在
func (c *CacheClient) CheckVoucherExists(ctx context.Context, id int64) (bool, error) {
	c.whitelistMu.RLock()
	deleted, exists := c.whitelistDeleted["voucher"][id]
	c.whitelistMu.RUnlock()

	if exists && deleted {
		return false, nil
	}

	if c.voucherBloomFilter != nil {
		exists, err := c.voucherBloomFilter.ExistsInt64(ctx, id)
		if err != nil {
			return true, nil
		}
		return exists, nil
	}

	return true, nil
}

// AddShopToBloom 添加商铺ID到布隆过滤器
func (c *CacheClient) AddShopToBloom(ctx context.Context, id int64) error {
	if c.shopBloomFilter != nil {
		return c.shopBloomFilter.AddInt64(ctx, id)
	}
	return nil
}

// AddVoucherToBloom 添加优惠券ID到布隆过滤器
func (c *CacheClient) AddVoucherToBloom(ctx context.Context, id int64) error {
	if c.voucherBloomFilter != nil {
		return c.voucherBloomFilter.AddInt64(ctx, id)
	}
	return nil
}

// MarkShopDeleted 标记商铺为已删除（软删除）
func (c *CacheClient) MarkShopDeleted(ctx context.Context, id int64) error {
	c.whitelistMu.Lock()
	if c.whitelistDeleted["shop"] == nil {
		c.whitelistDeleted["shop"] = make(map[int64]bool)
	}
	c.whitelistDeleted["shop"][id] = true
	c.whitelistMu.Unlock()

	// 同时存入Redis白名单（跨进程共享）
	return c.rdb.SAdd(ctx, "whitelist:shop:deleted", id).Err()
}

// MarkVoucherDeleted 标记优惠券为已删除
func (c *CacheClient) MarkVoucherDeleted(ctx context.Context, id int64) error {
	c.whitelistMu.Lock()
	if c.whitelistDeleted["voucher"] == nil {
		c.whitelistDeleted["voucher"] = make(map[int64]bool)
	}
	c.whitelistDeleted["voucher"][id] = true
	c.whitelistMu.Unlock()

	return c.rdb.SAdd(ctx, "whitelist:voucher:deleted", id).Err()
}

// IsShopDeleted 检查商铺是否已删除
func (c *CacheClient) IsShopDeleted(ctx context.Context, id int64) (bool, error) {
	// 先检查内存白名单
	c.whitelistMu.RLock()
	deleted, exists := c.whitelistDeleted["shop"][id]
	c.whitelistMu.RUnlock()

	if exists {
		return deleted, nil
	}

	// 检查Redis白名单
	return c.rdb.SIsMember(ctx, "whitelist:shop:deleted", id).Result()
}

// generateLockValue 生成锁的value（UUID）
func (c *CacheClient) generateLockValue() string {
	// 使用UUID作为唯一标识
	return uuid.New().String()
}

// ============================================================
// 基础分布式锁（不可重入）
// ============================================================

// TryLock 尝试获取分布式锁（不可重入版本）
// 使用 SET key value NX EX 命令，原子完成加锁和设置过期时间
// 返回值：true-获取成功，false-获取失败
func (c *CacheClient) TryLock(ctx context.Context, key string) bool {
	return c.TryLockWithTTL(ctx, key, time.Duration(LockShopTTL)*time.Second)
}

// TryLockWithTTL 尝试获取分布式锁，指定TTL
func (c *CacheClient) TryLockWithTTL(ctx context.Context, key string, ttl time.Duration) bool {
	// 生成锁的value（UUID）作为唯一标识符
	lockValue := c.generateLockValue()

	// NX: key不存在时才设置（保证互斥）
	// EX: 设置过期时间（防止死锁）
	ok, err := c.rdb.SetNX(ctx, key, lockValue, ttl).Result()
	if err != nil || !ok {
		return false
	}

	// 保存本地映射，用于后续释放锁时校验
	// 避免每一次释放锁的时候都要去查询redis获取value
	c.localMu.Lock()
	c.localLockValue[key] = lockValue
	c.localMu.Unlock()

	return true
}

// ============================================================
// 安全释放锁（使用Lua脚本）
// ============================================================

// safeUnlockScript Lua脚本：先判断value是否匹配，再删除
// 保证原子性，避免误删别人持有的锁
var safeUnlockScript = redis.NewScript(`
	if redis.call('get', KEYS[1]) == ARGV[1] then
		return redis.call('del', KEYS[1])
	else
		return 0
	end
`)

// SafeUnlock 安全释放分布式锁
// 使用Lua脚本保证"判断+删除"原子执行
// 避免以下问题：
// 1. 锁超时自动释放后被其他线程获取，原线程执行DEL删除新锁
// 2. 时钟漂移导致的问题
// 3. 非原子判断导致的误删
func (c *CacheClient) SafeUnlock(ctx context.Context, key string) error {
	// 获取本地存储的value
	c.localMu.RLock()
	lockValue, exists := c.localLockValue[key]
	c.localMu.RUnlock()

	if !exists {
		// 锁不存在，可能是超时自动释放了
		return nil
	}

	// 使用Lua脚本原子执行：先判断再删除
	_, err := safeUnlockScript.Run(ctx, c.rdb, []string{key}, lockValue).Result()

	// 清理本地映射
	c.localMu.Lock()
	delete(c.localLockValue, key)
	c.localMu.Unlock()

	return err
}

// ============================================================
// 可重入分布式锁（使用Hash结构）
// ============================================================

// ReentrantLock 加锁（可重入版本）
// 使用Hash结构存储：Key=锁key, Field=客户端标识, Value=重入次数
// 优点：同一线程可以多次获取锁，不会阻塞
// 适用场景：递归调用、嵌套方法加锁
func (c *CacheClient) ReentrantLock(ctx context.Context, key string) bool {
	lockValue := c.generateLockValue()
	return c.ReentrantLockWithValue(ctx, key, lockValue)
}

// ReentrantLockWithValue 使用指定value加锁（可重入）
func (c *CacheClient) ReentrantLockWithValue(ctx context.Context, key, lockValue string) bool {
	return c.ReentrantLockWithValueAndTTL(ctx, key, lockValue, time.Duration(LockShopTTL)*time.Second)
}

// ReentrantLockWithValueAndTTL 可重入锁，指定TTL
func (c *CacheClient) ReentrantLockWithValueAndTTL(ctx context.Context, key, lockValue string, ttl time.Duration) bool {
	// Lua脚本：可重入锁加锁
	// 1. 锁不存在：创建并设置重入次数为1
	// 2. 锁存在且属于当前客户端：重入次数+1
	// 3. 锁存在但不属于当前客户端：加锁失败
	reentrantLockScript := redis.NewScript(`
		if redis.call('exists', KEYS[1]) == 0 then
			redis.call('hset', KEYS[1], ARGV[1], 1)
			redis.call('pexpire', KEYS[1], ARGV[2])
			return 1
		end
		if redis.call('hexists', KEYS[1], ARGV[1]) == 1 then
			redis.call('hincrby', KEYS[1], ARGV[1], 1)
			redis.call('pexpire', KEYS[1], ARGV[2])
			return redis.call('hget', KEYS[1], ARGV[1])
		end
		return 0
	`)

	result, err := reentrantLockScript.Run(ctx, c.rdb,
		[]string{key},
		lockValue,
		ttl.Milliseconds(),
	).Result()

	if err != nil {
		return false
	}

	count, ok := result.(int64)
	if !ok || count == 0 {
		return false
	}

	// 保存本地映射
	c.localMu.Lock()
	c.localLockValue[key] = lockValue
	c.localMu.Unlock()

	return true
}

// ReentrantUnlock 释放可重入锁
// 使用Hash结构，重入次数-1，次数为0时删除锁
func (c *CacheClient) ReentrantUnlock(ctx context.Context, key string) error {
	// 获取本地存储的value
	c.localMu.RLock()
	lockValue, exists := c.localLockValue[key]
	c.localMu.RUnlock()

	if !exists {
		return nil
	}

	// Lua脚本：可重入锁释放
	// 1. 锁不属于当前客户端：返回0
	// 2. 锁属于当前客户端：重入次数-1
	// 3. 重入次数为0：删除锁
	reentrantUnlockScript := redis.NewScript(`
		if redis.call('hexists', KEYS[1], ARGV[1]) == 0 then
			return 0
		end
		local count = redis.call('hincrby', KEYS[1], ARGV[1], -1)
		if count == 0 then
			redis.call('del', KEYS[1])
		end
		return count
	`)

	result, err := reentrantUnlockScript.Run(ctx, c.rdb, []string{key}, lockValue).Result()
	if err != nil {
		return err
	}

	// 清理本地映射（只在重入次数为0时清理）
	count, _ := result.(int64)
	if count == 0 {
		c.localMu.Lock()
		delete(c.localLockValue, key)
		c.localMu.Unlock()
	}

	return nil
}

// ============================================================
// 看门狗机制（自动续期）
// ============================================================

// WatchdogLock 加锁并启动看门狗
func (c *CacheClient) WatchdogLock(ctx context.Context, key string) bool {
	lockValue := c.generateLockValue()
	return c.WatchdogLockWithValue(ctx, key, lockValue)
}

// WatchdogLockWithValue 使用指定value加锁并启动看门狗
func (c *CacheClient) WatchdogLockWithValue(ctx context.Context, key, lockValue string) bool {
	// 先尝试获取锁，TTL设为30秒
	ok := c.ReentrantLockWithValueAndTTL(ctx, key, lockValue, 30*time.Second)
	if !ok {
		return false
	}

	// 启动看门狗协程
	go c.runWatchdog(ctx, key, lockValue)

	return true
}

// runWatchdog 看门狗运行协程
// 每隔1/3TTL检查一次，如果仍持有锁则续期
// 如果客户端断开连接，看门狗停止，锁自动过期
func (c *CacheClient) runWatchdog(ctx context.Context, key, lockValue string) {
	// 续期间隔：TTL的1/3，30秒TTL则10秒续期一次
	watchdogInterval := 10 * time.Second
	ticker := time.NewTicker(watchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查锁是否还是自己持有
			c.localMu.RLock()
			_, exists := c.localLockValue[key]
			c.localMu.RUnlock()

			if !exists {
				// 锁已被释放，停止看门狗
				return
			}

			// 检查锁的value是否还是自己
			currentValue, err := c.rdb.Get(ctx, key).Result()
			if err != nil || currentValue != lockValue {
				// 锁已经不是自己的，停止看门狗
				return
			}

			// 续期：将TTL重置为30秒
			_ = c.rdb.Expire(ctx, key, 30*time.Second)

		case <-ctx.Done():
			// 上下文取消，停止看门狗
			return
		}
	}
}

// WatchdogUnlock 释放看门狗锁
func (c *CacheClient) WatchdogUnlock(ctx context.Context, key string) error {
	// 获取本地存储的value
	c.localMu.RLock()
	_, exists := c.localLockValue[key]
	c.localMu.RUnlock()

	if !exists {
		return nil
	}

	// 直接删除（因为看门狗会自动停止）
	err := c.rdb.Del(ctx, key).Err()

	// 清理本地映射
	c.localMu.Lock()
	delete(c.localLockValue, key)
	c.localMu.Unlock()

	return err
}

// ============================================================
// 缓存操作方法
// ============================================================

// Set 设置缓存
func (c *CacheClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// Get 获取缓存
func (c *CacheClient) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete 删除缓存
func (c *CacheClient) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// QueryWithPassThrough 缓存穿透处理
func (c *CacheClient) QueryWithPassThrough(ctx context.Context, key string, dest interface{}, ttl time.Duration, query func() (interface{}, error)) error {
	// 尝试从缓存获取
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// 缓存未命中，查询数据库
	data, err := query()
	if err != nil {
		// 数据库不存在，设置空值缓存
		_ = c.Set(ctx, CacheNullKey+key, "", time.Duration(CacheNullTTL)*time.Minute)
		return err
	}

	// 设置缓存
	_ = c.Set(ctx, key, data, ttl)
	return json.Unmarshal(data.([]byte), dest)
}

// QueryWithMutex 缓存击穿处理（互斥锁）
// 使用SafeUnlock释放锁，避免误删
func (c *CacheClient) QueryWithMutex(ctx context.Context, key string, dest interface{}, ttl time.Duration, query func() (interface{}, error)) error {
	// 尝试从缓存获取
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// 缓存未命中，获取锁
	lockKey := LockShopKey + key
	if !c.TryLock(ctx, lockKey) {
		// 锁获取失败，短暂等待后重试
		time.Sleep(50 * time.Millisecond)
		return c.QueryWithMutex(ctx, key, dest, ttl, query)
	}

	// 锁获取成功，查询数据库
	defer func() {
		_ = c.SafeUnlock(ctx, lockKey)
	}()

	// 双重检查缓存
	err = c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// 查询数据库并设置缓存
	data, err := query()
	if err != nil {
		_ = c.Set(ctx, CacheNullKey+key, "", time.Duration(CacheNullTTL)*time.Minute)
		return err
	}

	_ = c.Set(ctx, key, data, ttl)
	return json.Unmarshal(data.([]byte), dest)
}

// SafeMutex 缓存击穿处理（安全版，使用看门狗+Lua脚本释放锁）
// 推荐生产环境使用
func (c *CacheClient) SafeMutex(ctx context.Context, key string, dest interface{}, ttl time.Duration, query func() (interface{}, error)) error {
	// 尝试从缓存获取
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// 缓存未命中，获取锁（带看门狗）
	lockKey := LockShopKey + key
	if !c.WatchdogLock(ctx, lockKey) {
		// 锁获取失败，短暂等待后重试
		time.Sleep(50 * time.Millisecond)
		return c.SafeMutex(ctx, key, dest, ttl, query)
	}

	// 锁获取成功，查询数据库
	defer func() {
		_ = c.WatchdogUnlock(ctx, lockKey)
	}()

	// 双重检查缓存
	err = c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// 查询数据库并设置缓存
	data, err := query()
	if err != nil {
		_ = c.Set(ctx, CacheNullKey+key, "", time.Duration(CacheNullTTL)*time.Minute)
		return err
	}

	_ = c.Set(ctx, key, data, ttl)
	return json.Unmarshal(data.([]byte), dest)
}

// GetLockValue 获取指定锁的value（用于测试）
func (c *CacheClient) GetLockValue(ctx context.Context, key string) string {
	c.localMu.RLock()
	defer c.localMu.RUnlock()
	return c.localLockValue[key]
}

// IsLocked 检查指定锁是否被当前客户端持有
func (c *CacheClient) IsLocked(ctx context.Context, key string) bool {
	c.localMu.RLock()
	_, exists := c.localLockValue[key]
	c.localMu.RUnlock()
	return exists
}

// GetLockTTL 获取锁的剩余TTL
func (c *CacheClient) GetLockTTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// ExtendLock 手动延长锁的TTL
func (c *CacheClient) ExtendLock(ctx context.Context, key string, ttl time.Duration) error {
	c.localMu.RLock()
	lockValue, exists := c.localLockValue[key]
	c.localMu.RUnlock()

	if !exists {
		return fmt.Errorf("锁不存在或已释放")
	}

	// 检查是否是自己的锁
	currentValue, err := c.rdb.Get(ctx, key).Result()
	if err != nil || currentValue != lockValue {
		return fmt.Errorf("锁不是当前客户端持有")
	}

	return c.rdb.Expire(ctx, key, ttl).Err()
}

// IsHeldByCurrentThread 检查锁是否由当前客户端持有
func (c *CacheClient) IsHeldByCurrentThread(ctx context.Context, key string) bool {
	c.localMu.RLock()
	lockValue, exists := c.localLockValue[key]
	c.localMu.RUnlock()

	if !exists {
		return false
	}

	currentValue, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return false
	}

	return currentValue == lockValue
}

// TryLockSimple 简单的分布式锁尝试（不保存本地状态）
// 返回锁的value，需要手动保存用于释放
func TryLockSimple(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (bool, string, error) {
	lockValue := uuid.New().String()

	ok, err := rdb.SetNX(ctx, key, lockValue, ttl).Result()
	if err != nil || !ok {
		return false, "", err
	}

	return true, lockValue, nil
}

// UnlockWithValue 使用value释放锁（安全版）
// 使用Lua脚本，先判断再删除
func UnlockWithValue(ctx context.Context, rdb *redis.Client, key, lockValue string) (bool, error) {
	result, err := safeUnlockScript.Run(ctx, rdb, []string{key}, lockValue).Result()
	if err != nil {
		return false, err
	}

	count, ok := result.(int64)
	return ok && count > 0, nil
}

// IsLockExists 检查锁是否存在
func IsLockExists(ctx context.Context, rdb *redis.Client, key string) (bool, error) {
	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// GetLockHolder 获取锁的持有者value
func GetLockHolder(ctx context.Context, rdb *redis.Client, key string) (string, error) {
	return rdb.Get(ctx, key).Result()
}
