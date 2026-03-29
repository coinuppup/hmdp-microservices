package utils

// Redis 常量
const (
	// 商铺缓存
	CacheShopKey  = "cache:shop:"
	CacheShopTTL  = 30 // 30分钟

	// 空值缓存
	CacheNullKey  = "cache:null:"
	CacheNullTTL  = 5 // 5分钟

	// 锁
	LockShopKey   = "lock:shop:"
	LockShopTTL   = 10 // 10秒

	// 商铺类型缓存
	CacheShopTypeKey = "cache:shop:type"
	CacheShopTypeTTL = 30 // 30分钟

	// 优惠券
	CacheVoucherKey = "cache:voucher:"
	CacheVoucherTTL = 30 // 30分钟

	// 秒杀
	SeckillVoucherStockKey = "seckill:stock:"
	SeckillVoucherOrderKey = "seckill:order:"

	// 订单
	StreamOrdersKey = "stream:orders"
)
