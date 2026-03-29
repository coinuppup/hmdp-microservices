package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"hmdp-microservices/content-service/model"

	"github.com/gin-gonic/gin"
)

// UserServiceURL user-service 的地址
const UserServiceURL = "http://localhost:8081"

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 从请求头获取token
		authorization := ctx.GetHeader("Authorization")
		if authorization == "" {
			ctx.Next()
			return
		}

		// 解析token
		parts := strings.SplitN(authorization, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			ctx.Next()
			return
		}

		token := parts[1]

		// 调用 user-service 验证 token
		user, err := validateToken(ctx.Request.Context(), token)
		if err != nil {
			// token 无效或过期，继续但不设置用户
			ctx.Next()
			return
		}

		// 设置用户到上下文
		ctx.Request = ctx.Request.WithContext(SetUser(ctx.Request.Context(), user))
		ctx.Next()
	}
}

// validateToken 调用 user-service 验证 token
func validateToken(ctx context.Context, token string) (*model.UserDTO, error) {
	// 构建请求
	req, err := http.NewRequestWithContext(ctx, "POST", UserServiceURL+"/user/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	// 解析响应
	var result struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    *model.UserDTO `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 200 {
		return nil, err
	}

	return result.Data, nil
}

// GetUserID 从上下文获取用户ID
func GetUserID(ctx *gin.Context) int64 {
	user := GetUser(ctx.Request.Context())
	if user == nil {
		return 0
	}
	return user.ID
}

// SetUserID 设置用户ID到上下文
func SetUserID(ctx context.Context, userID int64) context.Context {
	user := &model.UserDTO{
		ID: userID,
	}
	return SetUser(ctx, user)
}
