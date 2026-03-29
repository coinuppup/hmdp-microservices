package config

import (
	"github.com/redis/go-redis/v9"
)

// InitRedis 初始化Redis连接
func InitRedis(cfg *Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return rdb
}