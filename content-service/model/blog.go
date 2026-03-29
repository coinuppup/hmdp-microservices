package model

import (
	"time"
)

// Blog 博客模型
type Blog struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	UserID    int64     `json:"userId"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Images    string    `json:"images"`
	Liked     int       `json:"liked"`
	Status    int       `json:"status"`
	CreateTime time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime time.Time `json:"updateTime" gorm:"autoUpdateTime"`
	Name      string    `json:"name" gorm:"-"`
	Icon      string    `json:"icon" gorm:"-"`
	IsLike    bool      `json:"isLike" gorm:"-"`
}

// BlogComments 博客评论模型
type BlogComments struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	BlogID    int64     `json:"blogId"`
	UserID    int64     `json:"userId"`
	Content   string    `json:"content"`
	CreateTime time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}
