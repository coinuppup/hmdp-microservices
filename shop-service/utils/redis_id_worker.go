package utils

import (
	"context"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisIDWorker Redis ID 生成器
type RedisIDWorker struct {
	rdb        *redis.Client
	lastStamp  int64
	seq        int64
	serverID   int64
	serverBits int64
	seqBits    int64
	maxSeq     int64
}

// NewRedisIDWorker 创建 Redis ID 生成器
func NewRedisIDWorker(rdb *redis.Client, serverID int64) *RedisIDWorker {
	const (
		serverBits = 2  // 服务器 ID 位数
		seqBits    = 12 // 序列号位数
	)

	maxSeq := int64(math.Pow(2, float64(seqBits)) - 1)

	return &RedisIDWorker{
		rdb:        rdb,
		serverID:   serverID,
		serverBits: serverBits,
		seqBits:    seqBits,
		maxSeq:     maxSeq,
	}
}

// ID结构：41位时间戳 + 10位机器ID + 12位序列号 = 63位
func (w *RedisIDWorker) NextId(ctx context.Context, key string) (int64, error) {
	// 获取当前时间戳毫秒级别
	now := time.Now().UnixMilli()

	// 生成序列号
	if now == w.lastStamp {
		w.seq++
		if w.seq > w.maxSeq {
			// 等待下一个毫秒
			for now <= w.lastStamp {
				now = time.Now().UnixMilli()
			}
			w.seq = 0
		}
	} else {
		w.seq = 0
	}

	w.lastStamp = now

	// 生成 ID
	id := now<<(w.serverBits+w.seqBits) | (w.serverID << w.seqBits) | w.seq

	return id, nil
}
