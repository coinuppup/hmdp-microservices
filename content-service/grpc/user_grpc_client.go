package grpc

import (
	"context"

	"hmdp-microservices/common/proto/user"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// UserServiceAddr user-service的gRPC地址
	UserServiceAddr = "localhost:50051"
)

// UserGrpcClient gRPC客户端
type UserGrpcClient struct {
	client user.UserServiceClient
	conn   *grpc.ClientConn
}

// NewUserGrpcClient 创建gRPC客户端
func NewUserGrpcClient() (*UserGrpcClient, error) {
	// 建立连接
	conn, err := grpc.NewClient(UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// 创建客户端
	client := user.NewUserServiceClient(conn)

	return &UserGrpcClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close 关闭连接
func (c *UserGrpcClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ValidateToken 验证Token并返回用户信息
func (c *UserGrpcClient) ValidateToken(ctx context.Context, token string) (*user.UserInfo, error) {
	resp, err := c.client.ValidateToken(ctx, &user.ValidateTokenRequest{Token: token})
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, nil
	}

	return resp.Data, nil
}

// RefreshToken 刷新Token
func (c *UserGrpcClient) RefreshToken(ctx context.Context, refreshToken, deviceId string) (*user.TokenData, error) {
	resp, err := c.client.RefreshToken(ctx, &user.RefreshTokenRequest{
		RefreshToken: refreshToken,
		DeviceId:     deviceId,
	})
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, nil
	}

	return resp.Data, nil
}
