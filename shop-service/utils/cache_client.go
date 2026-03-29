package utils

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheClient 缓存客户端
type CacheClient struct {
	rdb *redis.Client
}

// NewCacheClient 创建缓存客户端
func NewCacheClient(rdb *redis.Client) *CacheClient {
	return &CacheClient{rdb: rdb}
}

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
		c.Set(ctx, CacheNullKey+key, "", time.Duration(CacheNullTTL)*time.Minute)
		return err
	}

	// 设置缓存
	c.Set(ctx, key, data, ttl)
	return json.Unmarshal(json.RawMessage(data.([]byte)), dest)
}

// QueryWithMutex 缓存击穿处理（互斥锁）
func (c *CacheClient) QueryWithMutex(ctx context.Context, key string, dest interface{}, ttl time.Duration, query func() (interface{}, error)) error {
	// 尝试从缓存获取
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// 缓存未命中，获取锁
	lockKey := LockShopKey + key
	if !c.TryLock(ctx, lockKey) {
		// 锁获取失败，重试
		time.Sleep(50 * time.Millisecond)
		return c.QueryWithMutex(ctx, key, dest, ttl, query)
	}

	// 锁获取成功，查询数据库
	defer c.Unlock(ctx, lockKey)

	data, err := query()
	if err != nil {
		// 数据库不存在，设置空值缓存
		c.Set(ctx, CacheNullKey+key, "", time.Duration(CacheNullTTL)*time.Minute)
		return err
	}

	// 设置缓存
	c.Set(ctx, key, data, ttl)
	return json.Unmarshal(json.RawMessage(data.([]byte)), dest)
}

// TryLock 尝试获取锁
func (c *CacheClient) TryLock(ctx context.Context, key string) bool {
	return c.rdb.SetNX(ctx, key, "1", time.Duration(LockShopTTL)*time.Second).Val()
}

// Unlock 释放锁
func (c *CacheClient) Unlock(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}
