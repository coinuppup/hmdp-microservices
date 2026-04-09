package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hmdp-microservices/shop-service/model"
	"hmdp-microservices/shop-service/repository"
	"hmdp-microservices/shop-service/utils"
)

// ShopService 商铺服务
type ShopService struct {
	db          *gorm.DB
	rdb         *redis.Client
	shopRepo    *repository.ShopRepository
	cacheClient *utils.CacheClient
}

// NewShopService 创建商铺服务
// 传入 shopRepo 用于布隆过滤器预热
func NewShopService(db *gorm.DB, rdb *redis.Client, shopRepo ...*repository.ShopRepository) *ShopService {
	cacheClient := utils.NewCacheClient(rdb)

	// 如果提供了 shopRepo，则预热布隆过滤器
	if len(shopRepo) > 0 && shopRepo[0] != nil {
		ctx := context.Background()
		_ = cacheClient.InitBloomFilterWithData(ctx, shopRepo[0], nil)
		log.Printf("[ShopService] Bloom filter initialized with shop data")
	}

	return &ShopService{
		db:          db,
		rdb:         rdb,
		shopRepo:    repository.NewShopRepository(db),
		cacheClient: cacheClient,
	}
}

// GetShop 获取商铺信息
func (s *ShopService) GetShop(ctx context.Context, id int64) (*model.Shop, error) {
	// 第一步：布隆过滤器检查，防止缓存穿透
	// 如果布隆过滤器判断一定不存在，直接返回错误
	exists, err := s.cacheClient.CheckShopExists(ctx, id)
	if err == nil && !exists {
		log.Printf("[ShopService] Bloom filter: shop %d does not exist", id)
		return nil, fmt.Errorf("shop not found")
	}

	// 第二步：检查是否已删除（白名单）
	isDeleted, _ := s.cacheClient.IsShopDeleted(ctx, id)
	if isDeleted {
		return nil, fmt.Errorf("shop has been deleted")
	}

	// 缓存键
	key := utils.CacheShopKey + strconv.FormatInt(id, 10)
	var shop model.Shop

	// 第三步：使用安全版互斥锁处理缓存击穿（生产环境推荐）
	err = s.cacheClient.SafeMutex(ctx, key, &shop, time.Duration(utils.CacheShopTTL)*time.Minute, func() (interface{}, error) {
		// 查询数据库
		dbShop, err := s.shopRepo.FindByID(id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("shop not found")
			}
			return nil, err
		}
		// 将新查询到的ID添加到布隆过滤器
		_ = s.cacheClient.AddShopToBloom(ctx, dbShop.ID)

		data, err := json.Marshal(dbShop)
		if err != nil {
			return nil, err
		}
		return data, nil
	})

	if err != nil {
		return nil, err
	}

	return &shop, nil
}

// ListShops 分页查询商铺
func (s *ShopService) ListShops(ctx context.Context, typeId int64, current, size int32) ([]*model.Shop, error) {
	return s.shopRepo.FindByType(typeId, int(current), int(size))
}

// ListShopTypes 获取商铺类型
func (s *ShopService) ListShopTypes(ctx context.Context) ([]*model.ShopType, error) {
	// 缓存键
	key := utils.CacheShopTypeKey
	var shopTypes []*model.ShopType

	// 使用安全版互斥锁处理缓存击穿（生产环境推荐）
	err := s.cacheClient.SafeMutex(ctx, key, &shopTypes, time.Duration(utils.CacheShopTypeTTL)*time.Minute, func() (interface{}, error) {
		// 查询数据库
		dbShopTypes, err := s.shopRepo.FindShopTypes()
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(dbShopTypes)
		if err != nil {
			return nil, err
		}
		return data, nil
	})

	if err != nil {
		return nil, err
	}

	return shopTypes, nil
}

// CreateShop 创建商铺
func (s *ShopService) CreateShop(ctx context.Context, shop *model.Shop) (int64, error) {
	err := s.shopRepo.Create(shop)
	if err != nil {
		return 0, err
	}

	// 将新创建的 ID 添加到布隆过滤器
	_ = s.cacheClient.AddShopToBloom(ctx, shop.ID)

	return shop.ID, nil
}

// UpdateShop 更新商铺
func (s *ShopService) UpdateShop(ctx context.Context, shop *model.Shop) error {
	// 更新数据库
	err := s.shopRepo.Update(shop)
	if err != nil {
		return err
	}

	// 删除缓存（Cache Aside 策略：先更新数据库，后删除缓存）
	key := utils.CacheShopKey + strconv.FormatInt(shop.ID, 10)
	s.cacheClient.Delete(ctx, key)

	return nil
}

// DeleteShop 删除商铺
func (s *ShopService) DeleteShop(ctx context.Context, id int64) error {
	// 删除数据库
	err := s.shopRepo.Delete(id)
	if err != nil {
		return err
	}

	// 删除缓存
	key := utils.CacheShopKey + strconv.FormatInt(id, 10)
	s.cacheClient.Delete(ctx, key)

	// 标记已删除（布隆过滤器无法删除，使用白名单）
	_ = s.cacheClient.MarkShopDeleted(ctx, id)

	return nil
}
