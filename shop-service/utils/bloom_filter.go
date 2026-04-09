package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"

	"github.com/redis/go-redis/v9"
)

// ============================================================
// 布隆过滤器 - 防止缓存穿透
// ============================================================

// BloomFilter 布隆过滤器
// 使用Redis Bitmap实现，空间效率高
type BloomFilter struct {
	rdb       *redis.Client
	key       string
	size      uint64 // 位数组大小
	hashFuncs uint   // 哈希函数数量
}

// NewBloomFilter 创建布隆过滤器
// size: 预期存储的元素数量
// fpRate: 期望的误判率
func NewBloomFilter(rdb *redis.Client, key string, size uint64, fpRate float64) *BloomFilter {
	// 根据期望误判率计算哈希函数数量
	// k = -log2(p)，当p=0.01时，k≈7
	hashFuncs := uint(7)
	if fpRate > 0 && fpRate < 1 {
		hashFuncs = uint(-1 * math.Log(fpRate) / math.Ln2)
	}
	if hashFuncs < 1 {
		hashFuncs = 7
	}
	if hashFuncs > 20 {
		hashFuncs = 20 // 限制最大数量
	}

	// 优化位数组大小：m = -n * ln(p) / (ln(2)^2)
	// 使用更大的size以减少冲突
	optimalSize := uint64(float64(size) * (-math.Log(fpRate)) / (math.Ln2 * math.Ln2))
	if optimalSize < size || optimalSize == 0 {
		optimalSize = size * 10
	}
	if optimalSize > 10000000 {
		optimalSize = 10000000 // 限制最大大小
	}

	return &BloomFilter{
		rdb:       rdb,
		key:       key,
		size:      optimalSize,
		hashFuncs: hashFuncs,
	}
}

// Add 添加元素到布隆过滤器
func (bf *BloomFilter) Add(ctx context.Context, value string) error {
	// 计算多个哈希值
	hashes := bf.hash(value)
	for _, hash := range hashes {
		offset := hash % bf.size
		err := bf.rdb.SetBit(ctx, bf.key, int64(offset), 1).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

// AddInt64 添加int64类型元素
func (bf *BloomFilter) AddInt64(ctx context.Context, value int64) error {
	return bf.Add(ctx, fmt.Sprintf("%d", value))
}

// Exists 检查元素是否存在
// 可能存在误判（false一定不存在，true可能存在）
func (bf *BloomFilter) Exists(ctx context.Context, value string) (bool, error) {
	hashes := bf.hash(value)
	for _, hash := range hashes {
		offset := hash % bf.size
		bit, err := bf.rdb.GetBit(ctx, bf.key, int64(offset)).Result()
		if err != nil {
			return false, err
		}
		if bit == 0 {
			return false, nil // 一定不存在
		}
	}
	return true, nil // 可能存在（误判）
}

// ExistsInt64 检查int64类型元素是否存在
func (bf *BloomFilter) ExistsInt64(ctx context.Context, value int64) (bool, error) {
	return bf.Exists(ctx, fmt.Sprintf("%d", value))
}

// Remove 删除元素（不支持真正删除，只能重置）
// 由于布隆过滤器的特性，无法准确删除元素
// 可以通过重新创建过滤器来解决
func (bf *BloomFilter) Clear(ctx context.Context) error {
	return bf.rdb.Del(ctx, bf.key).Err()
}

// GetSize 获取位数组大小
func (bf *BloomFilter) GetSize() uint64 {
	return bf.size
}

// GetHashFuncs 获取哈希函数数量
func (bf *BloomFilter) GetHashFuncs() uint {
	return bf.hashFuncs
}

// hash 计算多个哈希值
// 使用双重哈希技术：h(i) = h1 + i * h2
func (bf *BloomFilter) hash(value string) []uint64 {
	h := fnv.New64a()
	h.Write([]byte(value))
	h1 := h.Sum64()

	h.Reset()
	h.Write([]byte(value + "_salt"))
	h2 := h.Sum64()

	hashes := make([]uint64, bf.hashFuncs)
	for i := uint(0); i < bf.hashFuncs; i++ {
		hashes[i] = h1 + uint64(i)*h2
	}
	return hashes
}

// ============================================================
// 预热布隆过滤器
// ============================================================

// BloomFilterManager 布隆过滤器管理器
type BloomFilterManager struct {
	rdb           *redis.Client
	shopFilter    *BloomFilter
	voucherFilter *BloomFilter
	initialized   bool
}

// NewBloomFilterManager 创建布隆过滤器管理器
func NewBloomFilterManager(rdb *redis.Client) *BloomFilterManager {
	return &BloomFilterManager{
		rdb:           rdb,
		shopFilter:    NewBloomFilter(rdb, "bloom:shop", 100000, 0.01),
		voucherFilter: NewBloomFilter(rdb, "bloom:voucher", 100000, 0.01),
	}
}

// InitShopFilter 初始化商铺布隆过滤器（从数据库加载）
func (m *BloomFilterManager) InitShopFilter(ctx context.Context, shopRepo interface {
	FindAllIDs() ([]int64, error)
}) error {
	if m.shopFilter == nil {
		m.shopFilter = NewBloomFilter(m.rdb, "bloom:shop", 100000, 0.01)
	}

	// 查询所有商铺ID
	ids, err := shopRepo.(interface{ FindAllIDs() ([]int64, error) }).FindAllIDs()
	if err != nil {
		return err
	}

	// 添加到布隆过滤器
	for _, id := range ids {
		if err := m.shopFilter.AddInt64(ctx, id); err != nil {
			return err
		}
	}

	m.initialized = true
	return nil
}

// InitVoucherFilter 初始化优惠券布隆过滤器
func (m *BloomFilterManager) InitVoucherFilter(ctx context.Context, voucherRepo interface {
	FindAllIDs() ([]int64, error)
}) error {
	if m.voucherFilter == nil {
		voucherFilter := NewBloomFilter(m.rdb, "bloom:voucher", 100000, 0.01)
		m.voucherFilter = voucherFilter
	}

	ids, err := voucherRepo.(interface{ FindAllIDs() ([]int64, error) }).FindAllIDs()
	if err != nil {
		return err
	}

	for _, id := range ids {
		if err := m.voucherFilter.AddInt64(ctx, id); err != nil {
			return err
		}
	}

	m.initialized = true
	return nil
}

// CheckShopExists 检查商铺ID是否可能存在（用于缓存穿透防护）
func (m *BloomFilterManager) CheckShopExists(ctx context.Context, id int64) (bool, error) {
	if m.shopFilter == nil {
		// 过滤器未初始化，放行请求（保守策略）
		return true, nil
	}
	return m.shopFilter.ExistsInt64(ctx, id)
}

// CheckVoucherExists 检查优惠券ID是否可能存在
func (m *BloomFilterManager) CheckVoucherExists(ctx context.Context, id int64) (bool, error) {
	if m.voucherFilter == nil {
		return true, nil
	}
	return m.voucherFilter.ExistsInt64(ctx, id)
}

// AddShop 添加商铺ID到过滤器
func (m *BloomFilterManager) AddShop(ctx context.Context, id int64) error {
	if m.shopFilter == nil {
		return nil
	}
	return m.shopFilter.AddInt64(ctx, id)
}

// AddVoucher 添加优惠券ID到过滤器
func (m *BloomFilterManager) AddVoucher(ctx context.Context, id int64) error {
	if m.voucherFilter == nil {
		return nil
	}
	return m.voucherFilter.AddInt64(ctx, id)
}

// RemoveShop 从过滤器移除（标记删除，使用白名单辅助）
func (m *BloomFilterManager) MarkShopDeleted(ctx context.Context, id int64) error {
	// 布隆过滤器无法删除，使用白名单辅助
	// 将删除的ID存入Redis Set，后续查询时先检查白名单
	return m.rdb.SAdd(ctx, "whitelist:shop:deleted", id).Err()
}

// IsShopDeleted 检查商铺是否已删除
func (m *BloomFilterManager) IsShopDeleted(ctx context.Context, id int64) (bool, error) {
	exists, err := m.rdb.SIsMember(ctx, "whitelist:shop:deleted", id).Result()
	return exists, err
}

// ============================================================
// 布隆过滤器辅助工具函数
// ============================================================

// CheckBloomFilterStats 获取布隆过滤器统计信息
func CheckBloomFilterStats(ctx context.Context, rdb *redis.Client, key string) (int64, int64, error) {
	// 获取位数组中设置为1的位数
	bitCount, err := rdb.BitCount(ctx, key, &redis.BitCount{Start: 0, End: -1}).Result()
	if err != nil {
		return 0, 0, err
	}

	// 估算已存储的元素数量
	// n = -m * ln(1 - bitCount/m) / k
	// 这里简化处理，返回位数组使用率
	var info redis.StringCmd
	_ = info
	return bitCount, 0, nil
}

// SerializeBloomFilter 序列化布隆过滤器配置（用于分布式环境）
func SerializeBloomFilter(key string, size uint64, hashFuncs uint) string {
	data := map[string]interface{}{
		"key":       key,
		"size":      size,
		"hashFuncs": hashFuncs,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// DeserializeBloomFilter 反序列化布隆过滤器配置
func DeserializeBloomFilter(data string) (string, uint64, uint, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return "", 0, 0, err
	}

	key, _ := config["key"].(string)
	size, _ := config["size"].(float64)
	hashFuncs, _ := config["hashFuncs"].(float64)

	return key, uint64(size), uint(hashFuncs), nil
}
