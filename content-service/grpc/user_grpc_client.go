package grpc

import (
	"context"
	"fmt"

	"hmdp-microservices/common/etcd"
	"hmdp-microservices/common/proto/user"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserGrpcClient gRPC客户端
type UserGrpcClient struct {
	client    user.UserServiceClient
	conn      *grpc.ClientConn
	discovery *etcd.ServiceDiscovery
}

// NewUserGrpcClient 创建gRPC客户端
func NewUserGrpcClient() (*UserGrpcClient, error) {
	// 服务发现
	etcdEndpoints := []string{"localhost:2379"}
	discovery, err := etcd.NewServiceDiscovery(etcdEndpoints, "user-service")
	if err != nil {
		return nil, fmt.Errorf("failed to create service discovery: %w", err)
	}

	// 获取服务地址
	addr, err := discovery.GetServiceAddr()
	if err != nil {
		return nil, fmt.Errorf("failed to get service address: %w", err)
	}

	// 建立连接
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// 创建客户端
	client := user.NewUserServiceClient(conn)

	return &UserGrpcClient{
		client:    client,
		conn:      conn,
		discovery: discovery,
	}, nil
}

// Close 关闭连接
func (c *UserGrpcClient) Close() error {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return err
		}
	}

	if c.discovery != nil {
		if err := c.discovery.Close(); err != nil {
			return err
		}
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
