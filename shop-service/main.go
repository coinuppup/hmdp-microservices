package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"hmdp-microservices/common/etcd"
	"hmdp-microservices/shop-service/config"
	"hmdp-microservices/shop-service/controller"
	"hmdp-microservices/shop-service/service"

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
	shopService := service.NewShopService(db, rdb)
	voucherService := service.NewVoucherService(db, rdb)
	voucherOrderService := service.NewVoucherOrderService(db, rdb, cfg)

	// 启动gRPC服务
	go startGRPCServer(shopService, voucherService, voucherOrderService, cfg.GRPC.Port)

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

	// 启动HTTP服务
	startHTTPServer(shopService, voucherService, voucherOrderService, cfg.Server.Port)
}

// startGRPCServer 启动gRPC服务
func startGRPCServer(shopService *service.ShopService, voucherService *service.VoucherService, voucherOrderService *service.VoucherOrderService, grpcPort string) {
	// 注册gRPC服务
	server := grpc.NewServer()
	// TODO: 注册服务的gRPC实现

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
func startHTTPServer(shopService *service.ShopService, voucherService *service.VoucherService, voucherOrderService *service.VoucherOrderService, httpPort string) {
	// 初始化Gin
	r := gin.New()
	r.Use(gin.Recovery())

	// 创建控制器
	shopController := controller.NewShopController(shopService)
	voucherController := controller.NewVoucherController(voucherService)
	voucherOrderController := controller.NewVoucherOrderController(voucherOrderService)

	// 注册路由
	// 商铺相关路由
	r.GET("/shop/:id", shopController.GetShop)
	r.GET("/shop/list", shopController.ListShops)
	r.POST("/shop", shopController.CreateShop)
	r.PUT("/shop", shopController.UpdateShop)
	r.DELETE("/shop/:id", shopController.DeleteShop)
	r.GET("/shop-type/list", shopController.ListShopTypes)

	// 优惠券相关路由
	r.GET("/voucher/list", voucherController.ListVouchers)
	r.POST("/voucher", voucherController.CreateVoucher)
	r.POST("/voucher/seckill", voucherController.CreateSeckillVoucher)

	// 订单相关路由
	r.POST("/voucher-order/seckill/:id", voucherOrderController.SeckillVoucher)
	r.GET("/voucher-order/list", voucherOrderController.ListOrders)

	// 启动服务器
	log.Printf("HTTP server starting on :%s", httpPort)
	if err := r.Run(":" + httpPort); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
