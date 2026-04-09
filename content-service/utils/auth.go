package utils

import (
	"context"
	"strings"
	"sync"

	"hmdp-microservices/content-service/grpc"
	"hmdp-microservices/content-service/model"

	"github.com/gin-gonic/gin"
)

var (
	userGrpcClient  *grpc.UserGrpcClient
	once            sync.Once
	etcdEndpoints   []string
	userServiceName string
)

// InitUserGrpcClient 初始化gRPC客户端配置
func InitUserGrpcClient(endpoints []string, serviceName string) {
	etcdEndpoints = endpoints
	userServiceName = serviceName
}

// getUserGrpcClient 获取gRPC客户端(单例)
func getUserGrpcClient() (*grpc.UserGrpcClient, error) {
	var err error
	once.Do(func() {
		userGrpcClient, err = grpc.NewUserGrpcClient(etcdEndpoints, userServiceName)
	})
	return userGrpcClient, err
}

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

		// 调用 user-service 验证 token (使用gRPC)
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

// validateToken 调用 user-service 验证 token (使用gRPC)
func validateToken(ctx context.Context, token string) (*model.UserDTO, error) {
	// 获取gRPC客户端
	client, err := getUserGrpcClient()
	if err != nil {
		return nil, err
	}

	// 调用gRPC
	userInfo, err := client.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if userInfo == nil {
		return nil, nil
	}

	// 转换为UserDTO
	return &model.UserDTO{
		ID:       userInfo.Id,
		Phone:    userInfo.Phone,
		NickName: userInfo.NickName,
		Icon:     userInfo.Icon,
	}, nil
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
