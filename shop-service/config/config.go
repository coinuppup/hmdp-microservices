package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	GRPC   GRPCConfig   `mapstructure:"grpc"`
	Kafka  KafkaConfig  `mapstructure:"kafka"`
	Etcd   EtcdConfig   `mapstructure:"etcd"`
	Cache  CacheConfig  `mapstructure:"cache"`
	Canal  CanalConfig  `mapstructure:"canal"`
}

// EtcdConfig Etcd配置
type EtcdConfig struct {
	Endpoints []string          `mapstructure:"endpoints"`
	Service   EtcdServiceConfig `mapstructure:"service"`
}

// EtcdServiceConfig Etcd服务配置
type EtcdServiceConfig struct {
	Host string `mapstructure:"host"`
	Name string `mapstructure:"name"`
	TTL  int64  `mapstructure:"ttl"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// MySQLConfig 数据库配置
type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	InvalidateTopic  string `mapstructure:"invalidate-topic"`   // 缓存失效消息主题
	BinlogTopic      string `mapstructure:"binlog-topic"`       // 数据库binlog消息主题
	EnableLocalCache bool   `mapstructure:"enable-local-cache"` // 启用本地缓存
	LocalCacheSize   int    `mapstructure:"local-cache-size"`   // 本地缓存大小
	LocalCacheTTL    string `mapstructure:"local-cache-ttl"`    // 本地缓存TTL
}

// CanalConfig Canal配置
type CanalConfig struct {
	Server     string `mapstructure:"server"`       // Canal服务端地址
	Destination string `mapstructure:"destination"` // Canal实例名
	Username   string `mapstructure:"username"`     // Canal用户名
	Password   string `mapstructure:"password"`     // Canal密码
	Filter     string `mapstructure:"filter"`       // 过滤规则
	BatchSize  int    `mapstructure:"batch-size"`  // 批量获取大小
	Timeout    int    `mapstructure:"timeout"`     // 超时时间(秒)
	Enabled    bool   `mapstructure:"enabled"`      // 是否启用Canal
}

// Load 加载配置
func Load() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Error reading config file: %v", err)
		log.Println("Using default configuration")
		return getDefaultConfig()
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Printf("Error unmarshaling config: %v", err)
		log.Println("Using default configuration")
		return getDefaultConfig()
	}

	return &config
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: "8082",
		},
		MySQL: MySQLConfig{
			Host:     "127.0.0.1",
			Port:     "3306",
			User:     "root",
			Password: "001020",
			DBName:   "hmdp",
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "001020",
			DB:       0,
		},
		GRPC: GRPCConfig{
			Port: "50052",
		},
		Kafka: KafkaConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   "order-create",
		},
		Cache: CacheConfig{
			InvalidateTopic:  "cache-invalidate",
			BinlogTopic:      "db-binlog",
			EnableLocalCache: true,
			LocalCacheSize:   10000,
			LocalCacheTTL:    "5m",
		},
		Canal: CanalConfig{
			Server:      "127.0.0.1:11111",
			Destination: "example",
			Username:    "canal",
			Password:    "canal",
			Filter:      "hmdp\\..*",
			BatchSize:   1000,
			Timeout:     60,
			Enabled:     false,
		},
		Etcd: EtcdConfig{
			Endpoints: []string{"localhost:2379"},
			Service: EtcdServiceConfig{
				Host: "127.0.0.1",
				Name: "shop-service",
				TTL:  10,
			},
		},
	}
}

// GetDSN 获取数据库连接字符串
func (c *MySQLConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

// GetRedisAddr 获取Redis连接地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// GetKafkaBrokers 获取Kafka broker地址
func (c *Config) GetKafkaBrokers() []string {
	return c.Kafka.Brokers
}

// GetCacheInvalidateTopic 获取缓存失效消息主题
func (c *Config) GetCacheInvalidateTopic() string {
	return c.Cache.InvalidateTopic
}

// GetCacheBinlogTopic 获取Binlog消息主题
func (c *Config) GetCacheBinlogTopic() string {
	return c.Cache.BinlogTopic
}

// IsCanalEnabled 检查Canal是否启用
func (c *Config) IsCanalEnabled() bool {
	return c.Canal.Enabled
}

// GetCanalConfig 获取Canal配置
func (c *Config) GetCanalConfig() CanalConfig {
	return c.Canal
}
