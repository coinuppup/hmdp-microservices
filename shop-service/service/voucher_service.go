package service

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hmdp-microservices/shop-service/model"
	"hmdp-microservices/shop-service/repository"
	"hmdp-microservices/shop-service/utils"
)

// VoucherService 优惠券服务
type VoucherService struct {
	db           *gorm.DB
	rdb          *redis.Client
	voucherRepo  *repository.VoucherRepository
}

// NewVoucherService 创建优惠券服务
func NewVoucherService(db *gorm.DB, rdb *redis.Client) *VoucherService {
	return &VoucherService{
		db:           db,
		rdb:          rdb,
		voucherRepo:  repository.NewVoucherRepository(db),
	}
}

// ListVouchers 获取优惠券列表
func (s *VoucherService) ListVouchers(ctx context.Context, shopId int64) ([]*model.Voucher, error) {
	return s.voucherRepo.FindByShopID(shopId)
}

// CreateVoucher 创建优惠券
func (s *VoucherService) CreateVoucher(ctx context.Context, voucher *model.Voucher) (int64, error) {
	err := s.voucherRepo.Create(voucher)
	if err != nil {
		return 0, err
	}
	return voucher.ID, nil
}

// CreateSeckillVoucher 创建秒杀优惠券
func (s *VoucherService) CreateSeckillVoucher(ctx context.Context, voucher *model.Voucher, stock int) (int64, error) {
	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建优惠券
	err := tx.Create(voucher).Error
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	// 创建秒杀优惠券
	seckillVoucher := &model.SeckillVoucher{
		VoucherID: voucher.ID,
		Stock:     stock,
		BeginTime: time.Now(),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
	err = tx.Create(seckillVoucher).Error
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	// 同步库存到 Redis
	stockKey := utils.SeckillVoucherStockKey + strconv.FormatInt(voucher.ID, 10)
	s.rdb.Set(ctx, stockKey, stock, 24*time.Hour)

	return voucher.ID, nil
}
