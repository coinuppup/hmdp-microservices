package utils

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"hmdp-microservices/user-service/model"
)

// AuthMiddleware 认证中间件（使用Redis双Token）
func AuthMiddleware(rdb *redis.Client, secret string) gin.HandlerFunc {
	// 创建TokenService实例
	tokenService := NewTokenService(rdb, secret)

	return func(ctx *gin.Context) {
		// 获取token
		auth := ctx.GetHeader("Authorization")
		if auth == "" {
			ctx.JSON(http.StatusUnauthorized, model.Fail("未登录"))
			ctx.Abort()
			return
		}

		// 解析token
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			ctx.JSON(http.StatusUnauthorized, model.Fail("未登录"))
			ctx.Abort()
			return
		}

		token := parts[1]

		// 验证Token并自动续期
		tokenInfo, err := tokenService.ValidateAccessToken(ctx.Request.Context(), token)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, model.Fail("登录已过期，请重新登录"))
			ctx.Abort()
			return
		}

		// 设置用户ID到上下文
		ctx.Set("userID", tokenInfo.UserID)

		ctx.Next()
	}
}
