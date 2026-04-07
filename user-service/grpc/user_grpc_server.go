package grpc

import (
	"context"

	"hmdp-microservices/common/proto/user"
	"hmdp-microservices/user-service/service"

	"google.golang.org/grpc"
)

// UserGrpcServer gRPC服务端实现
type UserGrpcServer struct {
	user.UnimplementedUserServiceServer
	userService *service.UserService
}

// NewUserGrpcServer 创建gRPC服务端
func NewUserGrpcServer(userService *service.UserService) *UserGrpcServer {
	return &UserGrpcServer{
		userService: userService,
	}
}

// Register 注册gRPC服务
func Register(server *grpc.Server, userService *service.UserService) {
	user.RegisterUserServiceServer(server, NewUserGrpcServer(userService))
}

// Login 用户登录
func (s *UserGrpcServer) Login(ctx context.Context, req *user.LoginRequest) (*user.LoginResponse, error) {
	result, err := s.userService.Login(ctx, req.Phone, req.Code, "")
	if err != nil {
		return &user.LoginResponse{
			Code:    500,
			Message: err.Error(),
		}, nil
	}

	dataMap, ok := result.(map[string]interface{})
	if !ok {
		return &user.LoginResponse{
			Code:    500,
			Message: "登录结果格式错误",
		}, nil
	}

	return &user.LoginResponse{
		Code:    200,
		Message: "登录成功",
		Token:   dataMap["accessToken"].(string),
	}, nil
}

// GetUserInfo 获取用户信息
func (s *UserGrpcServer) GetUserInfo(ctx context.Context, req *user.GetUserInfoRequest) (*user.GetUserInfoResponse, error) {
	userDTO, err := s.userService.GetUserInfo(ctx, req.UserId)
	if err != nil {
		return &user.GetUserInfoResponse{
			Code:    500,
			Message: err.Error(),
		}, nil
	}

	return &user.GetUserInfoResponse{
		Code:    200,
		Message: "获取成功",
		Data: &user.UserInfo{
			Id:       userDTO.ID,
			Phone:    userDTO.Phone,
			NickName: userDTO.NickName,
			Icon:     userDTO.Icon,
		},
	}, nil
}

// Sign 用户签到
func (s *UserGrpcServer) Sign(ctx context.Context, req *user.SignRequest) (*user.SignResponse, error) {
	err := s.userService.Sign(ctx, req.UserId)
	if err != nil {
		return &user.SignResponse{
			Code:    500,
			Message: err.Error(),
		}, nil
	}

	return &user.SignResponse{
		Code:    200,
		Message: "签到成功",
		Data:    true,
	}, nil
}

// GetSignCount 获取签到次数
func (s *UserGrpcServer) GetSignCount(ctx context.Context, req *user.GetSignCountRequest) (*user.GetSignCountResponse, error) {
	count, err := s.userService.GetSignCount(ctx, req.UserId)
	if err != nil {
		return &user.GetSignCountResponse{
			Code:    500,
			Message: err.Error(),
		}, nil
	}

	return &user.GetSignCountResponse{
		Code:    200,
		Message: "获取成功",
		Data:    count,
	}, nil
}

// ValidateToken 验证Token并返回用户信息
func (s *UserGrpcServer) ValidateToken(ctx context.Context, req *user.ValidateTokenRequest) (*user.ValidateTokenResponse, error) {
	userDTO, err := s.userService.ValidateToken(ctx, req.Token)
	if err != nil {
		return &user.ValidateTokenResponse{
			Code:    401,
			Message: "Token无效或已过期",
		}, nil
	}

	return &user.ValidateTokenResponse{
		Code:    200,
		Message: "验证成功",
		Data: &user.UserInfo{
			Id:       userDTO.ID,
			Phone:    userDTO.Phone,
			NickName: userDTO.NickName,
			Icon:     userDTO.Icon,
		},
	}, nil
}

// RefreshToken 刷新Token
func (s *UserGrpcServer) RefreshToken(ctx context.Context, req *user.RefreshTokenRequest) (*user.RefreshTokenResponse, error) {
	result, err := s.userService.RefreshToken(ctx, req.RefreshToken, req.DeviceId)
	if err != nil {
		return &user.RefreshTokenResponse{
			Code:    401,
			Message: "刷新失败",
		}, nil
	}

	dataMap := result // result已经是map[string]interface{}类型

	return &user.RefreshTokenResponse{
		Code:    200,
		Message: "刷新成功",
		Data: &user.TokenData{
			AccessToken:  dataMap["accessToken"].(string),
			RefreshToken: dataMap["refreshToken"].(string),
			ExpiresIn:    dataMap["expiresIn"].(int64),
			TokenType:    dataMap["tokenType"].(string),
		},
	}, nil
}

// 确保实现接口
var _ user.UserServiceServer = (*UserGrpcServer)(nil)
