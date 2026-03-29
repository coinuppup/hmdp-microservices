package model

import (
	"time"
)

// User 用户模型
type User struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	Phone      string    `json:"phone" gorm:"uniqueIndex"`
	Password   string    `json:"password"`
	NickName   string    `json:"nickName"`
	Icon       string    `json:"icon"`
	CreateTime time.Time `json:"createTime" gorm:"autoCreateTime"`
	UpdateTime time.Time `json:"updateTime" gorm:"autoUpdateTime"`
}

/*
在用户登录时，UserDTO 被用来存储用户会话信息到 Redis，以 Hash 形式存储在 Redis 中
Key: login:user:{token}
Fields: id, phone, nickName, i
con
Expire: 30分钟（这个可以修改）
*/
type UserDTO struct {
	ID       int64  `json:"id"`
	Phone    string `json:"phone"`
	NickName string `json:"nickName"`
	Icon     string `json:"icon"`
}

// LoginFormDTO 登录表单
type LoginFormDTO struct {
	Phone    string `json:"phone" binding:"required"`
	Code     string `json:"code" binding:"required"`
	DeviceID string `json:"deviceId"` // 设备ID，可选，用于多端识别
}

// TokenPairDTO 双Token响应
type TokenPairDTO struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"` // Access Token剩余秒数
	TokenType    string `json:"tokenType"` // Bearer
}

// RefreshTokenFormDTO 刷新Token请求
type RefreshTokenFormDTO struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
	DeviceID     string `json:"deviceId"` // 设备ID，可选
}

// TokenInfoDTO Token信息响应
type TokenInfoDTO struct {
	TokenID   string    `json:"tokenId"`
	DeviceID  string    `json:"deviceId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpireAt  time.Time `json:"expireAt"`
}
