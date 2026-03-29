package main

import (
	"log"
	"net"

	"hmdp-microservices/user-service/config"
	"hmdp-microservices/user-service/controller"
	"hmdp-microservices/user-service/service"
	"hmdp-microservices/user-service/utils"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	// 初始化配置
	cfg := config.Load()

	// 初始化数据库
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 初始化Redis
	rdb := config.InitRedis(cfg)

	// 初始化服务
	userService := service.NewUserService(db, rdb)

	// 启动gRPC服务
	go startGRPCServer(userService, cfg.GRPC.Port)

	// 启动HTTP服务
	startHTTPServer(userService, cfg.Server.Port)
}

// startGRPCServer 启动gRPC服务
func startGRPCServer(userService *service.UserService, grpcPort string) {
	// 注册gRPC服务
	server := grpc.NewServer()
	// TODO: 注册用户服务的gRPC实现

	// 启动服务
	listener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("gRPC server starting on :%s", grpcPort)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}
}

// startHTTPServer 启动HTTP服务
func startHTTPServer(userService *service.UserService, httpPort string) {
	// 初始化Gin
	r := gin.New()
	r.Use(gin.Recovery())

	// 创建控制器
	userController := controller.NewUserController(userService)

	// 注册路由
	r.POST("/user/code", userController.SendCode)
	r.POST("/user/login", userController.Login)
	r.GET("/user/me", userController.GetCurrentUser)
	r.GET("/user/info/:id", userController.GetUserInfo)
	r.POST("/user/sign", utils.AuthMiddleware(), userController.Sign)
	r.GET("/user/sign/count", utils.AuthMiddleware(), userController.GetSignCount)

	// Token相关接口
	r.POST("/user/validate", userController.ValidateToken)
	r.POST("/user/refresh", userController.RefreshToken)

	// 启动服务器
	log.Printf("HTTP server starting on :%s", httpPort)
	if err := r.Run(":" + httpPort); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
