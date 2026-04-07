package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"hmdp-microservices/shop-service/config"
	"hmdp-microservices/shop-service/model"
	"hmdp-microservices/shop-service/repository"
	"hmdp-microservices/shop-service/utils"
)

// VoucherOrderService 订单服务
type VoucherOrderService struct {
	db               *gorm.DB
	rdb              *redis.Client
	voucherRepo      *repository.VoucherRepository
	voucherOrderRepo *repository.VoucherOrderRepository
	idWorker         *utils.RedisIDWorker
	kafkaWriter      *kafka.Writer
}

// NewVoucherOrderService 创建订单服务
func NewVoucherOrderService(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *VoucherOrderService {
	// 创建Kafka writer
	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Topic:    cfg.Kafka.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	// 启动Kafka消费者
	go startKafkaConsumer(cfg.Kafka.Brokers, cfg.Kafka.Topic, db)

	return &VoucherOrderService{
		db:               db,
		rdb:              rdb,
		voucherRepo:      repository.NewVoucherRepository(db),
		voucherOrderRepo: repository.NewVoucherOrderRepository(db),
		idWorker:         utils.NewRedisIDWorker(rdb, 1),
		kafkaWriter:      writer,
	}
}

// startKafkaConsumer 启动Kafka消费者
func startKafkaConsumer(brokers []string, topic string, db *gorm.DB) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "order-consumer",
	})
	defer reader.Close()

	voucherRepo := repository.NewVoucherRepository(db)
	voucherOrderRepo := repository.NewVoucherOrderRepository(db)

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("Error reading message: %v\n", err)
			continue
		}

		// 解析订单数据
		var order model.VoucherOrder
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			fmt.Printf("Error unmarshaling order: %v\n", err)
			continue
		}

		// 处理订单
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 扣减库存
		err = voucherRepo.UpdateSeckillVoucherStock(order.VoucherID)
		if err != nil {
			tx.Rollback()
			fmt.Printf("Error updating stock: %v\n", err)
			continue
		}

		// 创建订单
		err = voucherOrderRepo.Create(&order)
		if err != nil {
			tx.Rollback()
			fmt.Printf("Error creating order: %v\n", err)
			continue
		}

		// 提交事务
		if err := tx.Commit().Error; err != nil {
			fmt.Printf("Error committing transaction: %v\n", err)
			continue
		}

		fmt.Printf("Order processed successfully: %d\n", order.ID)
	}
}

// SeckillVoucher 秒杀优惠券
func (s *VoucherOrderService) SeckillVoucher(ctx context.Context, voucherID, userID int64) (int64, error) {
	// 生成订单ID，63位=41位+10位机器ID+12位序列号
	orderId, err := s.idWorker.NextId(ctx, "order")
	if err != nil {
		return 0, err
	}

	// 执行秒杀逻辑，构建Redis键值
	stockKey := utils.SeckillVoucherStockKey + strconv.FormatInt(voucherID, 10)
	orderKey := utils.SeckillVoucherOrderKey + strconv.FormatInt(voucherID, 10)

	// 一人一旦检查，检查是否重复下单，使用redis Set集合
	exists, err := s.rdb.SIsMember(ctx, orderKey, userID).Result()
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, fmt.Errorf("禁止重复下单")
	}

	// 扣减库存——原子操作使用redis Decr命令
	stock, err := s.rdb.Decr(ctx, stockKey).Result()
	if err != nil {
		return 0, err
	}
	if stock < 0 {
		// 库存不足，恢复库存
		s.rdb.Incr(ctx, stockKey)
		return 0, fmt.Errorf("库存不足")
	}

	// 添加到已下单集合
	s.rdb.SAdd(ctx, orderKey, userID)

	// 创建订单
	order := &model.VoucherOrder{
		ID:        orderId,
		UserID:    userID,
		VoucherID: voucherID,
		Status:    1,
	}

	// 发送消息到Kafka，创建订单
	orderData, err := json.Marshal(order)
	if err != nil {
		return 0, err
	}

	err = s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Value: orderData,
	})
	if err != nil {
		// 发送失败，恢复库存和订单状态
		s.rdb.Incr(ctx, stockKey)
		s.rdb.SRem(ctx, orderKey, userID)
		return 0, err
	}

	return orderId, nil
}

// CreateVoucherOrder 创建优惠券订单
func (s *VoucherOrderService) CreateVoucherOrder(ctx context.Context, order *model.VoucherOrder) error {
	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 扣减库存
	err := s.voucherRepo.UpdateSeckillVoucherStock(order.VoucherID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 创建订单
	err = s.voucherOrderRepo.Create(order)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

// ListOrders 获取订单列表
func (s *VoucherOrderService) ListOrders(ctx context.Context, userID int64, current, size int32) ([]*model.VoucherOrder, error) {
	return s.voucherOrderRepo.FindByUser(userID, int(current), int(size))
}
