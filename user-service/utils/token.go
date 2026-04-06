package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// TokenPair 双Token结构
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"` // Access Token剩余秒数
	TokenType    string `json:"tokenType"` // Bearer
}

// AccessTokenInfo Access Token信息
type AccessTokenInfo struct {
	UserID    int64     `json:"userId"`
	TokenID   string    `json:"tokenId"`
	DeviceID  string    `json:"deviceId"` // 设备ID，用于多端识别
	CreatedAt time.Time `json:"createdAt"`
	ExpireAt  time.Time `json:"expireAt"`
}

// RefreshTokenInfo Refresh Token信息
type RefreshTokenInfo struct {
	UserID   int64     `json:"userId"`
	TokenID  string    `json:"tokenId"`
	DeviceID string    `json:"deviceId"` // 设备ID，用于多设备区分
	ExpireAt time.Time `json:"expireAt"`
}

// 常量定义
const (
	// TokenSecret JWT签名密钥
	TokenSecret = "your-secret-key"
)

// JWTClaims JWT声明结构
type JWTClaims struct {
	UserID   int64  `json:"userId"`
	DeviceID string `json:"deviceId"`
	TokenID  string `json:"tokenId"`
	jwt.RegisteredClaims
}

// TokenService Token服务
type TokenService struct {
	rdb *redis.Client
}

// NewTokenService 创建Token服务
func NewTokenService(rdb *redis.Client) *TokenService {
	return &TokenService{rdb: rdb}
}

// GenerateTokenPair 生成双Token
// - Access Token: 纯JWT，前端存储，不存Redis
// - Refresh Token: 存Redis白名单，支持多设备区分
func (s *TokenService) GenerateTokenPair(ctx context.Context, userID int64, deviceID string) (*TokenPair, error) {
	// 1. 生成Access Token ID
	accessTokenID := generateRandomToken(AccessTokenLength)

	// 2. 生成Access Token (纯JWT，不存Redis)
	accessToken, err := s.generateJWT(userID, deviceID, accessTokenID, AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("生成access token失败: %w", err)
	}

	// 3. 生成Refresh Token ID
	refreshTokenID := generateRandomToken(RefreshTokenLength)

	// 4. 使用Pipeline批量写入Redis
	pipe := s.rdb.Pipeline()

	// Refresh Token 白名单存储 (Hash结构，支持多设备区分)
	// Key: token:whitelist:<userID>:<deviceID>
	// Field: refreshTokenID
	// Value: 过期时间
	whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, deviceID)
	pipe.HSet(ctx, whitelistKey, refreshTokenID, time.Now().Add(RefreshTokenTTL).Format(time.RFC3339))
	pipe.Expire(ctx, whitelistKey, RefreshTokenTTL)

	// 同时使用简单的String结构存储，便于验证
	// Key: token:refresh:<refreshTokenID>
	// Value: userID:deviceID
	refreshKey := RefreshTokenKey + refreshTokenID
	pipe.Set(ctx, refreshKey, fmt.Sprintf("%d:%s", userID, deviceID), RefreshTokenTTL)

	// 用户的设备列表 (Set结构，记录该用户所有登录的设备)
	userDevicesKey := UserDevicesKey + fmt.Sprintf("%d", userID)
	pipe.SAdd(ctx, userDevicesKey, deviceID)
	pipe.Expire(ctx, userDevicesKey, RefreshTokenTTL)

	// 执行Pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("保存token失败: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenID,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
		TokenType:    TokenTypeBearer,
	}, nil
}

// generateJWT 生成JWT Token
func (s *TokenService) generateJWT(userID int64, deviceID string, tokenID string, expiration time.Duration) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		DeviceID: deviceID,
		TokenID:  tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "hmdp-user-service",
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(TokenSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateAccessToken 验证Access Token (纯JWT验证，不查Redis白名单/黑名单)
func (s *TokenService) ValidateAccessToken(ctx context.Context, tokenString string) (*AccessTokenInfo, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token不能为空")
	}

	// 1. 解析JWT Token (纯JWT验证，不依赖Redis)
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(TokenSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("token解析失败: %w", err)
	}

	// 2. 验证Token有效性
	if !token.Valid {
		return nil, fmt.Errorf("token无效")
	}

	// 3. 提取Claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("token claims无效")
	}

	return &AccessTokenInfo{
		UserID:    claims.UserID,
		TokenID:   claims.TokenID,
		DeviceID:  claims.DeviceID,
		CreatedAt: claims.IssuedAt.Time,
		ExpireAt:  claims.ExpiresAt.Time,
	}, nil
}

// RefreshToken 使用Refresh Token刷新Access Token
// 使用白名单验证，支持多设备
func (s *TokenService) RefreshToken(ctx context.Context, refreshTokenID string, deviceID string) (*TokenPair, error) {
	if refreshTokenID == "" {
		return nil, fmt.Errorf("refresh token不能为空")
	}

	// 1. 验证Refresh Token是否存在 (查白名单)
	refreshKey := RefreshTokenKey + refreshTokenID
	value, err := s.rdb.Get(ctx, refreshKey).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("refresh token已过期或无效")
	}
	if err != nil {
		return nil, fmt.Errorf("查询refresh token失败: %w", err)
	}

	// 2. 解析存储的 userID:deviceID
	var userID int64
	var originalDeviceID string
	fmt.Sscanf(value, "%d:%s", &userID, &originalDeviceID)
	if userID == 0 {
		return nil, fmt.Errorf("token数据无效")
	}

	// 3. 吊销旧的Refresh Token (从白名单中删除)
	s.rdb.Del(ctx, refreshKey)

	// 从用户设备白名单中移除
	whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, originalDeviceID)
	s.rdb.HDel(ctx, whitelistKey, refreshTokenID)

	// 4. 生成新的双Token (使用新的deviceID)
	return s.GenerateTokenPair(ctx, userID, deviceID)
}

// RevokeToken 吊销Token (用户登出)
// 只吊销Refresh Token，Access Token 会在 JWT 过期后自然失效
func (s *TokenService) RevokeToken(ctx context.Context, accessTokenString string) error {
	if accessTokenString == "" {
		return fmt.Errorf("token不能为空")
	}

	// 1. 解析JWT Token获取deviceID
	token, err := jwt.ParseWithClaims(accessTokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(TokenSecret), nil
	})

	if err != nil {
		return fmt.Errorf("token解析失败: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return fmt.Errorf("token无效")
	}

	userID := claims.UserID
	deviceID := claims.DeviceID

	// 2. 获取该设备对应的Refresh Token
	// 从白名单中获取该用户的设备列表，找到对应的refreshTokenID
	whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, deviceID)
	refreshTokenIDs, err := s.rdb.HKeys(ctx, whitelistKey).Result()
	if err != nil || len(refreshTokenIDs) == 0 {
		// 如果白名单中没有，可能是旧的token格式，尝试使用SCAN查找
		return s.revokeTokenByScan(ctx, userID, deviceID)
	}

	// 吊销该设备的所有Refresh Token
	pipe := s.rdb.Pipeline()
	for _, rtID := range refreshTokenIDs {
		pipe.Del(ctx, RefreshTokenKey+rtID)
	}
	pipe.Del(ctx, whitelistKey)
	_, err = pipe.Exec(ctx)

	return err
}

// revokeTokenByScan 通过SCAN查找并吊销Token (兼容旧版本)
func (s *TokenService) revokeTokenByScan(ctx context.Context, userID int64, deviceID string) error {
	// 查找用户的所有设备
	userDevicesKey := UserDevicesKey + fmt.Sprintf("%d", userID)
	deviceIDs, err := s.rdb.SMembers(ctx, userDevicesKey).Result()
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	for _, devID := range deviceIDs {
		if devID == deviceID || deviceID == "" {
			// 删除该设备的Refresh Token白名单
			whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, devID)
			pipe.Del(ctx, whitelistKey)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

// RevokeDeviceToken 吊销指定设备的Token
func (s *TokenService) RevokeDeviceToken(ctx context.Context, userID int64, deviceID string) error {
	whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, deviceID)

	// 获取该设备的所有Refresh Token
	refreshTokenIDs, err := s.rdb.HKeys(ctx, whitelistKey).Result()
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	for _, rtID := range refreshTokenIDs {
		pipe.Del(ctx, RefreshTokenKey+rtID)
	}
	pipe.Del(ctx, whitelistKey)
	_, err = pipe.Exec(ctx)

	return err
}

// RevokeAllUserTokens 吊销用户的所有Token (强制登出所有设备)
func (s *TokenService) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	// 获取用户的所有设备
	userDevicesKey := UserDevicesKey + fmt.Sprintf("%d", userID)
	deviceIDs, err := s.rdb.SMembers(ctx, userDevicesKey).Result()
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	for _, deviceID := range deviceIDs {
		whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, deviceID)
		// 获取该设备的所有Refresh Token并删除
		refreshTokenIDs, _ := s.rdb.HKeys(ctx, whitelistKey).Result()
		for _, rtID := range refreshTokenIDs {
			pipe.Del(ctx, RefreshTokenKey+rtID)
		}
		pipe.Del(ctx, whitelistKey)
	}

	// 删除用户设备列表
	pipe.Del(ctx, userDevicesKey)

	_, err = pipe.Exec(ctx)
	return err
}

// GetUserDevices 获取用户的所有登录设备
func (s *TokenService) GetUserDevices(ctx context.Context, userID int64) ([]string, error) {
	userDevicesKey := UserDevicesKey + fmt.Sprintf("%d", userID)
	return s.rdb.SMembers(ctx, userDevicesKey).Result()
}

// GetDeviceTokenCount 获取指定设备的Token数量
func (s *TokenService) GetDeviceTokenCount(ctx context.Context, userID int64, deviceID string) (int64, error) {
	whitelistKey := RefreshWhitelistKey + fmt.Sprintf("%d:%s", userID, deviceID)
	return s.rdb.HLen(ctx, whitelistKey).Result()
}

// generateRandomToken 生成随机Token字符串
func generateRandomToken(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// 降级方案：使用时间和随机数
		return fmt.Sprintf("%d%d", time.Now().UnixNano(), time.Now().Unix())[:length]
	}
	return hex.EncodeToString(bytes)
}
