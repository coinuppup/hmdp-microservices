package utils

import "time"

// Redis 常量
const (
	// 登录验证码
	LoginCodeKey = "login:code:"
	LoginCodeTTL = 5 // 5分钟

	// 登录用户 (旧版兼容，后续废弃)
	LoginUserKey = "login:user:"
	LoginUserTTL = 30 // 30分钟

	// 用户签到
	UserSignKey = "user:sign:"

	// ========== 双Token机制 ==========
	// Access Token 存储前缀
	AccessTokenKey = "token:access:"
	// Access Token 有效期: 15分钟
	AccessTokenTTL = 15 * time.Minute

	// Refresh Token 存储前缀
	RefreshTokenKey = "token:refresh:"
	// Refresh Token 有效期: 7天
	RefreshTokenTTL = 7 * 24 * time.Hour

	// 用户Token列表前缀 (用于多端管理和强制登出)
	UserTokensKey = "user:tokens:"
	// 已吊销的Refresh Token前缀
	RevokedRefreshTokenKey = "token:revoked:"
)

// Token 常量
const (
	// Access Token 字符串长度
	AccessTokenLength = 32
	// Refresh Token 字符串长度
	RefreshTokenLength = 64
	// Token 类型
	TokenTypeBearer = "Bearer"
)
