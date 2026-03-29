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
	UserID        int64     `json:"userId"`
	TokenID       string    `json:"tokenId"`
	AccessTokenID string    `json:"accessTokenId"` // 关联的Access Token ID
	CreatedAt     time.Time `json:"createdAt"`
	ExpireAt      time.Time `json:"expireAt"`
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
func (s *TokenService) GenerateTokenPair(ctx context.Context, userID int64, deviceID string) (*TokenPair, error) {
	// 1. 生成Access Token (使用JWT)
	accessTokenID := generateRandomToken(AccessTokenLength)
	accessToken, err := s.generateJWT(userID, deviceID, accessTokenID, AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("生成access token失败: %w", err)
	}

	// 2. 生成Refresh Token (使用随机字符串)
	refreshTokenID := generateRandomToken(RefreshTokenLength)

	// 3. 使用Pipeline批量写入Redis
	pipe := s.rdb.Pipeline()

	// Refresh Token 存储 (String结构，值为userID)
	refreshKey := RefreshTokenKey + refreshTokenID
	pipe.Set(ctx, refreshKey, userID, RefreshTokenTTL)

	// Access Token 与 Refresh Token 关联 (用于吊销时清理)
	mappingKey := RefreshTokenKey + "mapping:" + accessTokenID
	pipe.Set(ctx, mappingKey, refreshTokenID, RefreshTokenTTL)

	// 用户的Token列表 (Set结构，记录该用户所有有效的Access Token)
	userTokensKey := UserTokensKey + fmt.Sprintf("%d", userID)
	pipe.SAdd(ctx, userTokensKey, accessTokenID)
	pipe.Expire(ctx, userTokensKey, RefreshTokenTTL)

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

// ValidateAccessToken 验证Access Token
func (s *TokenService) ValidateAccessToken(ctx context.Context, tokenString string) (*AccessTokenInfo, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token不能为空")
	}

	// 1. 解析JWT Token
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

	// 4. 检查Token是否已被吊销
	// 注意：由于使用JWT，我们需要通过Redis检查token是否被吊销
	// 这里通过tokenID来检查
	accessTokenID := claims.TokenID
	userID := claims.UserID

	// 检查用户Token列表中是否存在该token
	userTokensKey := UserTokensKey + fmt.Sprintf("%d", userID)
	exists, err := s.rdb.SIsMember(ctx, userTokensKey, accessTokenID).Result()
	if err != nil {
		return nil, fmt.Errorf("检查token状态失败: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("token已被吊销")
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
func (s *TokenService) RefreshToken(ctx context.Context, refreshTokenID string, deviceID string) (*TokenPair, error) {
	if refreshTokenID == "" {
		return nil, fmt.Errorf("refresh token不能为空")
	}

	// 1. 验证Refresh Token是否存在
	refreshKey := RefreshTokenKey + refreshTokenID
	userIDStr, err := s.rdb.Get(ctx, refreshKey).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("refresh token已过期或无效")
	}
	if err != nil {
		return nil, fmt.Errorf("查询refresh token失败: %w", err)
	}

	// 2. 检查Refresh Token是否已被吊销
	revokedKey := RevokedRefreshTokenKey + refreshTokenID
	exists, _ := s.rdb.Exists(ctx, revokedKey).Result()
	if exists > 0 {
		return nil, fmt.Errorf("refresh token已被吊销")
	}

	userID := ParseInt64(userIDStr)
	if userID == 0 {
		return nil, fmt.Errorf("token数据无效")
	}

	// 3. 获取关联的旧Access Token ID
	// 使用SCAN查找关联
	var oldAccessTokenID string
	iter := s.rdb.Scan(ctx, 0, RefreshTokenKey+"mapping:*", 1000).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		val, _ := s.rdb.Get(ctx, key).Result()
		if val == refreshTokenID {
			// 提取access token ID
			oldAccessTokenID = key[len(RefreshTokenKey+"mapping:"):]
			break
		}
	}

	// 4. 吊销旧的Refresh Token (一次性使用，防止重放攻击)
	s.rdb.Set(ctx, revokedKey, "1", RefreshTokenTTL)
	s.rdb.Del(ctx, refreshKey)

	// 5. 删除旧的Access Token
	if oldAccessTokenID != "" {
		s.rdb.Del(ctx, RefreshTokenKey+"mapping:"+oldAccessTokenID)

		// 从用户Token列表中移除
		userTokensKey := UserTokensKey + fmt.Sprintf("%d", userID)
		s.rdb.SRem(ctx, userTokensKey, oldAccessTokenID)
	}

	// 6. 生成新的双Token
	return s.GenerateTokenPair(ctx, userID, deviceID)
}

// RevokeToken 吊销Token (用户登出)
func (s *TokenService) RevokeToken(ctx context.Context, accessTokenString string) error {
	if accessTokenString == "" {
		return fmt.Errorf("token不能为空")
	}

	// 1. 解析JWT Token获取tokenID和userID
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

	accessTokenID := claims.TokenID
	userID := claims.UserID

	// 2. 查找关联的Refresh Token
	mappingKey := RefreshTokenKey + "mapping:" + accessTokenID
	refreshTokenID, _ := s.rdb.Get(ctx, mappingKey).Result()

	pipe := s.rdb.Pipeline()

	// 3. 吊销Refresh Token
	if refreshTokenID != "" {
		pipe.Set(ctx, RevokedRefreshTokenKey+refreshTokenID, "1", RefreshTokenTTL)
		pipe.Del(ctx, RefreshTokenKey+refreshTokenID)
		pipe.Del(ctx, mappingKey)
	}

	// 4. 从用户Token列表中移除
	userTokensKey := UserTokensKey + fmt.Sprintf("%d", userID)
	pipe.SRem(ctx, userTokensKey, accessTokenID)

	_, err = pipe.Exec(ctx)
	return err
}

// RevokeAllUserTokens 吊销用户的所有Token (强制登出所有设备)
func (s *TokenService) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	userTokensKey := UserTokensKey + fmt.Sprintf("%d", userID)

	// 获取该用户所有有效的Access Token ID
	tokenIDs, err := s.rdb.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()

	for _, tokenID := range tokenIDs {
		// 查找并吊销关联的Refresh Token
		mappingKey := RefreshTokenKey + "mapping:" + tokenID
		refreshTokenID, _ := s.rdb.Get(ctx, mappingKey).Result()
		if refreshTokenID != "" {
			pipe.Set(ctx, RevokedRefreshTokenKey+refreshTokenID, "1", RefreshTokenTTL)
			pipe.Del(ctx, RefreshTokenKey+refreshTokenID)
			pipe.Del(ctx, mappingKey)
		}
	}

	// 删除用户Token列表
	pipe.Del(ctx, userTokensKey)

	_, err = pipe.Exec(ctx)
	return err
}

// GetUserTokens 获取用户的所有有效Token (用于查看登录设备)
func (s *TokenService) GetUserTokens(ctx context.Context, userID int64) ([]AccessTokenInfo, error) {
	userTokensKey := UserTokensKey + fmt.Sprintf("%d", userID)

	// 获取所有Token ID
	tokenIDs, err := s.rdb.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		return nil, err
	}

	// 注意：由于使用JWT，我们无法直接通过tokenID获取完整的token信息
	// 这里返回的是基于tokenID的基本信息
	// 实际使用中，前端需要提供完整的token才能验证
	var tokens []AccessTokenInfo
	for _, tokenID := range tokenIDs {
		// 创建一个基本的AccessTokenInfo
		tokenInfo := AccessTokenInfo{
			UserID:    userID,
			TokenID:   tokenID,
			DeviceID:  "", // 由于使用JWT，我们无法直接获取设备ID
			CreatedAt: time.Now(),
			ExpireAt:  time.Now().Add(AccessTokenTTL),
		}
		tokens = append(tokens, tokenInfo)
	}

	return tokens, nil
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

// ParseInt64 字符串转int64
func ParseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// parseTime 解析时间字符串
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
