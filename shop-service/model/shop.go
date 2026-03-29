package model

import (
	"time"
)

// Shop 商铺模型
type Shop struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	TypeID      int64     `json:"typeId"`
	Area        string    `json:"area"`
	Address     string    `json:"address"`
	Longitude   float64   `json:"longitude"`
	Latitude    float64   `json:"latitude"`
	AvgPrice    float64   `json:"avgPrice"`
	Sale        int       `json:"sale"`
	Comments    int       `json:"comments"`
	Score       float64   `json:"score"`
	Status      int       `json:"status"`
	CreateTime  time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime  time.Time `json:"updateTime" gorm:"autoUpdateTime"`
	Distance    float64   `json:"distance,omitempty" gorm:"-"`
}

// ShopType 商铺类型模型
type ShopType struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Icon        string    `json:"icon"`
	Sort        int       `json:"sort"`
	CreateTime  time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime  time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}
