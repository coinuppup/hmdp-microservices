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

	// ========== 双Token机制 (新方案) ==========
	// Access Token: 纯JWT，不存Redis

	// Refresh Token 存储前缀 (白名单)
	RefreshTokenKey = "token:refresh:"
	// Refresh Token 有效期: 7天
	RefreshTokenTTL = 7 * 24 * time.Hour

	// Refresh Token 白名单前缀 (按用户ID:设备ID 分组)
	// Key: token:whitelist:<userID>:<deviceID>
	// Hash 结构，field 为 refreshTokenID，value 为过期时间
	RefreshWhitelistKey = "token:whitelist:"

	// 用户设备列表前缀 (用于多设备管理)
	UserDevicesKey = "user:devices:"

	// Access Token 有效期: 15分钟
	AccessTokenTTL = 15 * time.Minute

	// 已吊销的Refresh Token前缀 (旧版本兼容，用于防重放)
	RevokedRefreshTokenKey = "token:revoked:"

	// 用户Token列表前缀 (旧版本兼容)
	UserTokensKey = "user:tokens:"
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
