package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"hmdp-microservices/common/etcd"
	"hmdp-microservices/content-service/config"
	"hmdp-microservices/content-service/controller"
	"hmdp-microservices/content-service/repository"
	"hmdp-microservices/content-service/service"
	"hmdp-microservices/content-service/utils"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 初始化配置 → 初始化数据库 → 初始化Redis → 创建Service层 → 启动gRPC服务 → 启动HTTP服务
func main() {
	cfg := config.Load()

	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	rdb := config.InitRedis(cfg)

	// 初始化gRPC客户端配置
	utils.InitUserGrpcClient(cfg.Etcd.Endpoints, cfg.Etcd.UserService.Name)

	blogRepo := repository.NewBlogRepository(db)

	blogService := service.NewBlogService(blogRepo, rdb)
	followService := service.NewFollowService(blogRepo, rdb)

	go startGRPCServer(blogService, followService, cfg.GRPC.Port)

	// 注册服务到etcd
	serviceAddr := cfg.Etcd.Service.Host + ":" + cfg.GRPC.Port
	serviceRegister, err := etcd.NewServiceRegister(cfg.Etcd.Endpoints, cfg.Etcd.Service.Name, serviceAddr, cfg.Etcd.Service.TTL)
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

	startHTTPServer(blogService, followService)
}

// startGRPCServer 启动gRPC服务
func startGRPCServer(blogService *service.BlogService, followService *service.FollowService, grpcPort string) {
	// 注册gRPC服务
	server := grpc.NewServer()
	// TODO: 注册内容服务的gRPC实现

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
// 客户端请求
// ↓
// Gin Router 路由匹配
// ↓
// AuthMiddleware 认证中间件
// ↓
// Controller 控制器层
// ↓
// Service 服务层（业务逻辑）
// ↓
// Repository/Redis 数据访问层
// ↓
// MySQL Database
// ↓
// 返回响应

func startHTTPServer(blogService *service.BlogService, followService *service.FollowService) {
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

	// 注册中间件
	r.Use(utils.AuthMiddleware())

	// 初始化控制器
	blogController := controller.NewBlogController(blogService)
	followController := controller.NewFollowController(followService)

	// 注册路由
	api := r.Group("/api")
	{
		blogController.Register(api)
		followController.Register(api)
	}

	// 启动服务器
	log.Printf("HTTP server starting on :8083")
	if err := r.Run(":8083"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
