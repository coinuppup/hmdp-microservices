package model

import (
	"time"
)

// Follow 关注模型
type Follow struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	UserID       int64     `json:"userId"`
	FollowUserID int64     `json:"followUserId"`
	CreateTime   time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime   time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}
