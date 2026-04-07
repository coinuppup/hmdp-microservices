# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HMDP (HeiMa DianPing) is a Go-based microservices implementation of a DianPing (restaurant review platform) clone. This is a learning project demonstrating microservices architecture with Redis patterns.

**Services:**
- `user-service`: User authentication, SMS-based login, user sign-in tracking (port 8081, gRPC 50051)
- `shop-service`: Shop management, voucher system, seckill orders (port 8082, gRPC 50052)
- `content-service`: Blog posts, likes, comments, follow relationships (port 8083, gRPC 50053)
- `common/`: Shared protobuf definitions (proto files only, no Go code)

## Tech Stack

- Go 1.25
- Gin (HTTP framework)
- gRPC + Protocol Buffers
- GORM + MySQL driver
- go-redis/v9
- Kafka (segmentio/kafka-go) - for async order processing
- Viper (configuration)

## Development Commands

```bash
# Run a specific service
cd user-service && go run .
cd shop-service && go run .
cd content-service && go run .

# Build a service
cd user-service && go build -o user-service.exe .
cd shop-service && go build -o shop-service.exe .
cd content-service && go build -o content-service.exe .

# Generate protobuf code (when proto files change)
protoc --go_out=. --go-grpc_out=. common/proto/*.proto

# Download dependencies
cd user-service && go mod download
cd shop-service && go mod download
cd content-service && go mod download
```

## Architecture

### Service Structure
Each service follows this pattern:
```
service-name/
├── main.go              # Entry point - starts HTTP + gRPC servers
├── config/
│   ├── config.go        # Config struct and Viper loader
│   ├── config.yaml      # Service-specific config (ports, DB credentials)
│   ├── db.go            # MySQL/GORM initialization
│   └── redis.go         # Redis client initialization
├── controller/          # HTTP handlers (Gin)
├── service/             # Business logic
├── repository/          # Data access (GORM)
├── model/               # GORM entities
├── utils/               # Helpers (auth, Redis constants, cache client)
└── proto/               # Service-specific proto stubs
```

### Dual Server Pattern
Each service starts two servers concurrently:
1. **HTTP Server** (Gin): External API on ports 8081-8083
2. **gRPC Server**: Internal service communication on ports 50051-50053

See `main.go` in each service for the startup pattern.

### Authentication Flow
Token-based authentication implemented in `content-service/utils/auth.go`:
- `AuthMiddleware()` extracts Bearer token from Authorization header
- Currently mocked - sets a test user (ID: 1) for any valid token format
- User context stored via `SetUser()` / retrieved via `GetUser()` in user_holder.go

### Database Configuration
MySQL and Redis credentials are in each service's `config/config.yaml`:
- Database: `hmdp`
- Default MySQL: `root:001020@127.0.0.1:3306`
- Default Redis: `localhost:6379`, password `001020`, DB 0

## Redis Usage Patterns

**CacheClient** (`shop-service/utils/cache_client.go`): Custom caching utility implementing:
- `QueryWithPassThrough`: Cache penetration protection with null value caching
- `QueryWithMutex`: Cache breakdown protection with distributed locks (SetNX)

**RedisIDWorker** (`shop-service/utils/redis_id_worker.go`): Snowflake-style distributed ID generator using timestamp + serverID + sequence

**Key Constants** (`shop-service/utils/redis_constants.go`):
- `CacheShopKey`: Shop cache prefix
- `CacheNullKey`: Null value cache for penetration protection
- `LockShopKey`: Distributed lock prefix
- `SeckillVoucherStockKey`: Seckill stock in Redis
- `SeckillVoucherOrderKey`: Track users who already ordered (Set)
- `StreamOrdersKey`: Redis Stream for orders

**Content Service Constants** (`content-service/utils/redis_constants.go`):
- `BlogLikedKey`: Blog like status (ZSet by timestamp)
- `FollowsKey`: Follow relationships
- `FeedKey`: User feed inbox (ZSet by timestamp)

## Seckill (Flash Sale) Implementation

**Shop-service** implements high-concurrency seckill:
1. Stock stored in Redis (`SeckillVoucherStockKey`)
2. Atomic stock deduction via `DECR`
3. Duplicate order prevention via Redis Set (`SeckillVoucherOrderKey`)
4. Order async processing via Kafka topic `order-create`
5. Kafka consumer handles DB stock deduction and order creation transactionally

See `shop-service/service/voucher_order_service.go` for implementation.

## Proto Definitions

Service contracts defined in `common/proto/`:
- `user.proto`: UserService (Login, GetUserInfo, Sign, GetSignCount)
- `shop.proto`: ShopService (GetShop, ListShops, SeckillVoucher, ListOrders)
- `content.proto`: ContentService (Blog CRUD, Like/Unlike, Follow/Unfollow, Feed)

**Note:** Proto Go stubs are not yet generated - gRPC handlers return "TODO" comments.

## Prerequisites

Before running services:
1. MySQL 5.6+ running with `hmdp` database
2. Redis server running with password `001020`
3. Kafka broker at `localhost:9092` (for shop-service order processing)

## Service Ports

| Service | HTTP | gRPC |
|---------|------|------|
| user-service | 8081 | 50051 |
| shop-service | 8082 | 50052 |
| content-service | 8083 | 50053 |