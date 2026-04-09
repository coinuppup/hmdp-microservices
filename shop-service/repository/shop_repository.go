package repository

import (
	"gorm.io/gorm"
	"hmdp-microservices/shop-service/model"
)

// ShopRepository 商铺仓库
type ShopRepository struct {
	db *gorm.DB
}

// NewShopRepository 创建商铺仓库
func NewShopRepository(db *gorm.DB) *ShopRepository {
	return &ShopRepository{db: db}
}

// FindByID 根据ID查询商铺
func (r *ShopRepository) FindByID(id int64) (*model.Shop, error) {
	var shop model.Shop
	err := r.db.First(&shop, id).Error
	if err != nil {
		return nil, err
	}
	return &shop, nil
}

// FindByType 根据类型查询商铺
func (r *ShopRepository) FindByType(typeID int64, page, pageSize int) ([]*model.Shop, error) {
	var shops []*model.Shop
	offset := (page - 1) * pageSize
	err := r.db.Where("type_id = ?", typeID).Offset(offset).Limit(pageSize).Find(&shops).Error
	if err != nil {
		return nil, err
	}
	return shops, nil
}

// Create 创建商铺
func (r *ShopRepository) Create(shop *model.Shop) error {
	return r.db.Create(shop).Error
}

// Update 更新商铺
func (r *ShopRepository) Update(shop *model.Shop) error {
	return r.db.Save(shop).Error
}

// Delete 删除商铺
func (r *ShopRepository) Delete(id int64) error {
	return r.db.Delete(&model.Shop{}, id).Error
}

// FindShopTypes 获取所有商铺类型
func (r *ShopRepository) FindShopTypes() ([]*model.ShopType, error) {
	var shopTypes []*model.ShopType
	err := r.db.Order("sort").Find(&shopTypes).Error
	if err != nil {
		return nil, err
	}
	return shopTypes, nil
}

// FindAllIDs 获取所有商铺ID（用于布隆过滤器预热）
func (r *ShopRepository) FindAllIDs() ([]int64, error) {
	var ids []int64
	err := r.db.Model(&model.Shop{}).Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// FindAllVoucherIDs 获取所有优惠券ID（用于布隆过滤器预热）
func (r *ShopRepository) FindAllVoucherIDs() ([]int64, error) {
	var ids []int64
	err := r.db.Model(&model.Voucher{}).Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}
