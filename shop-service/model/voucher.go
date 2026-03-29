package model

import (
	"time"
)

// Voucher 优惠券模型
type Voucher struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	ShopID       int64     `json:"shopId"`
	Title        string    `json:"title"`
	SubTitle     string    `json:"subTitle"`
	Rules        string    `json:"rules"`
	PayValue     int64     `json:"payValue"`
	ActualValue  int64     `json:"actualValue"`
	Type         int       `json:"type"`
	Status       int       `json:"status"`
	CreateTime   time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime   time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}

// SeckillVoucher 秒杀优惠券模型
type SeckillVoucher struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	VoucherID    int64     `json:"voucherId"`
	Stock        int       `json:"stock"`
	BeginTime    time.Time `json:"beginTime"`
	EndTime      time.Time `json:"endTime"`
	CreateTime   time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime   time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}

// VoucherOrder 优惠券订单模型
type VoucherOrder struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	UserID       int64     `json:"userId"`
	VoucherID    int64     `json:"voucherId"`
	Status       int       `json:"status"`
	CreateTime   time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime   time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}
