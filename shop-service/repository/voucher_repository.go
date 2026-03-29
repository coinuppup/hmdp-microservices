package repository

import (
	"hmdp-microservices/shop-service/model"
	"gorm.io/gorm"
)

// VoucherRepository 优惠券仓库
type VoucherRepository struct {
	db *gorm.DB
}

// NewVoucherRepository 创建优惠券仓库
func NewVoucherRepository(db *gorm.DB) *VoucherRepository {
	return &VoucherRepository{db: db}
}

// FindByShopID 根据商铺ID查询优惠券
func (r *VoucherRepository) FindByShopID(shopID int64) ([]*model.Voucher, error) {
	var vouchers []*model.Voucher
	err := r.db.Where("shop_id = ?", shopID).Find(&vouchers).Error
	if err != nil {
		return nil, err
	}
	return vouchers, nil
}

// Create 创建优惠券
func (r *VoucherRepository) Create(voucher *model.Voucher) error {
	return r.db.Create(voucher).Error
}

// CreateSeckillVoucher 创建秒杀优惠券
func (r *VoucherRepository) CreateSeckillVoucher(seckillVoucher *model.SeckillVoucher) error {
	return r.db.Create(seckillVoucher).Error
}

// FindSeckillVoucherByVoucherID 根据优惠券ID查询秒杀优惠券
func (r *VoucherRepository) FindSeckillVoucherByVoucherID(voucherID int64) (*model.SeckillVoucher, error) {
	var seckillVoucher model.SeckillVoucher
	err := r.db.Where("voucher_id = ?", voucherID).First(&seckillVoucher).Error
	if err != nil {
		return nil, err
	}
	return &seckillVoucher, nil
}

// UpdateSeckillVoucherStock 更新秒杀优惠券库存
func (r *VoucherRepository) UpdateSeckillVoucherStock(voucherID int64) error {
	return r.db.Model(&model.SeckillVoucher{}).Where("voucher_id = ? AND stock > 0", voucherID).Update("stock", gorm.Expr("stock - ?", 1)).Error
}

// VoucherOrderRepository 优惠券订单仓库
type VoucherOrderRepository struct {
	db *gorm.DB
}

// NewVoucherOrderRepository 创建优惠券订单仓库
func NewVoucherOrderRepository(db *gorm.DB) *VoucherOrderRepository {
	return &VoucherOrderRepository{db: db}
}

// FindByUserAndVoucher 根据用户ID和优惠券ID查询订单
func (r *VoucherOrderRepository) FindByUserAndVoucher(userID, voucherID int64) (*model.VoucherOrder, error) {
	var order model.VoucherOrder
	err := r.db.Where("user_id = ? AND voucher_id = ?", userID, voucherID).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Create 创建订单
func (r *VoucherOrderRepository) Create(order *model.VoucherOrder) error {
	return r.db.Create(order).Error
}

// FindByUser 根据用户ID查询订单
func (r *VoucherOrderRepository) FindByUser(userID int64, page, pageSize int) ([]*model.VoucherOrder, error) {
	var orders []*model.VoucherOrder
	offset := (page - 1) * pageSize
	err := r.db.Where("user_id = ?", userID).Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}
