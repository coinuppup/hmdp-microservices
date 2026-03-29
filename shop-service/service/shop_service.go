package service

import (
	"context"
	"encoding/json"
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
func NewShopService(db *gorm.DB, rdb *redis.Client) *ShopService {
	return &ShopService{
		db:          db,
		rdb:         rdb,
		shopRepo:    repository.NewShopRepository(db),
		cacheClient: utils.NewCacheClient(rdb),
	}
}

// GetShop 获取商铺信息
func (s *ShopService) GetShop(ctx context.Context, id int64) (*model.Shop, error) {
	// 缓存键
	key := utils.CacheShopKey + strconv.FormatInt(id, 10)
	var shop model.Shop

	// 使用缓存穿透处理
	err := s.cacheClient.QueryWithPassThrough(ctx, key, &shop, time.Duration(utils.CacheShopTTL)*time.Minute, func() (interface{}, error) {
		// 查询数据库
		dbShop, err := s.shopRepo.FindByID(id)
		if err != nil {
			return nil, err
		}
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

	// 使用缓存穿透处理
	err := s.cacheClient.QueryWithPassThrough(ctx, key, &shopTypes, time.Duration(utils.CacheShopTypeTTL)*time.Minute, func() (interface{}, error) {
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
	return shop.ID, nil
}

// UpdateShop 更新商铺
func (s *ShopService) UpdateShop(ctx context.Context, shop *model.Shop) error {
	// 更新数据库
	err := s.shopRepo.Update(shop)
	if err != nil {
		return err
	}

	// 删除缓存
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

	return nil
}
