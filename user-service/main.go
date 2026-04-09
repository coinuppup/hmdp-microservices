package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"hmdp-microservices/common/etcd"
	"hmdp-microservices/user-service/config"
	"hmdp-microservices/user-service/controller"
	grpcserver "hmdp-microservices/user-service/grpc"
	"hmdp-microservices/user-service/service"
	"hmdp-microservices/user-service/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
	userService := service.NewUserService(db, rdb, cfg.Token.Secret)

	// 启动gRPC服务
	go startGRPCServer(userService, cfg.GRPC.Port)

	// 注册服务到etcd
	serviceAddr := "127.0.0.1:" + cfg.GRPC.Port
	etcdEndpoints := []string{"localhost:2379"}
	serviceRegister, err := etcd.NewServiceRegister(etcdEndpoints, "user-service", serviceAddr, 10)
	if err != nil {
		log.Fatalf("Failed to create service register: %v", err)
	}

	if err := serviceRegister.Register(); err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}

	// 处理优雅关闭
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c

		log.Println("Shutting down service...")
		if err := serviceRegister.Unregister(); err != nil {
			log.Printf("Failed to unregister service: %v", err)
		}
		os.Exit(0)
	}()

	// 启动HTTP服务
	startHTTPServer(userService, cfg.Server.Port, rdb, cfg)
}

// startGRPCServer 启动gRPC服务
func startGRPCServer(userService *service.UserService, grpcPort string) {
	// 注册gRPC服务
	server := grpc.NewServer()

	// 注册用户服务的gRPC实现
	grpcserver.Register(server, userService)

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
func startHTTPServer(userService *service.UserService, httpPort string, rdb *redis.Client, cfg *config.Config) {
	// 初始化Gin
	r := gin.New()
	r.Use(gin.Recovery())

	// 手动添加CORS支持
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 创建控制器
	userController := controller.NewUserController(userService)

	// 创建认证中间件
	authMiddleware := utils.AuthMiddleware(rdb, cfg.Token.Secret)

	// 注册路由
	r.POST("/user/code", userController.SendCode)
	r.POST("/user/login", userController.Login)
	r.GET("/user/me", authMiddleware, userController.GetCurrentUser)
	r.GET("/user/info/:id", userController.GetUserInfo)
	r.POST("/user/sign", authMiddleware, userController.Sign)
	r.GET("/user/sign/count", authMiddleware, userController.GetSignCount)

	// Token相关接口
	r.POST("/user/validate", userController.ValidateToken)
	r.POST("/user/refresh", userController.RefreshToken)

	// 启动服务器
	log.Printf("HTTP server starting on :%s", httpPort)
	if err := r.Run(":" + httpPort); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
