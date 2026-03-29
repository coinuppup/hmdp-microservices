package utils

import (
	"context"
	"hmdp-microservices/content-service/model"
)

// 用户上下文键
var userKey = &struct{}{}

// SetUser 设置用户到上下文
func SetUser(ctx context.Context, user *model.UserDTO) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetUser 从上下文获取用户
func GetUser(ctx context.Context) *model.UserDTO {
	user, ok := ctx.Value(userKey).(*model.UserDTO)
	if !ok {
		return nil
	}
	return user
}
