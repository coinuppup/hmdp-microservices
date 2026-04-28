# CODEBUDDY.md This file provides guidance to CodeBuddy when working with code in this repository.

## 常用命令

### 基础设施
```bash
# 启动所有中间件 (MySQL, Redis, etcd, Zookeeper, Kafka)
docker-compose up -d

# 查看中间件运行状态
docker ps | findstr hmdp-
```

### Go微服务开发
```bash
# 启动单个服务
cd <service-dir> && go run main.go

# 拉取Go依赖
go mod tidy

# 编译单个服务
go build -o <service-name> main.go
```

### 前端开发
```bash
cd front
npm install
npm run dev      # 开发模式
npm run build    # 生产构建
```

## 项目架构

### 微服务结构
项目包含3个独立的Go微服务，每个服务都包含完整的HTTP API和gRPC接口：
- **user-service** (端口8081/50051): 用户认证、登录、签到
- **shop-service** (端口8082/50052): 商铺、优惠券、订单管理
- **content-service** (端口8083/50053): 博客、关注功能

### 服务注册与发现
使用Etcd实现服务治理：
- 服务启动时向etcd注册 (`/services/<service-name>/<addr>`)
- 通过租约(TTL)机制保持心跳
- 客户端通过`common/etcd/discovery.go`发现服务

### 数据层
- **MySQL**: 主数据存储，通过GORM操作
- **Redis**: 缓存层，用于会话、分布式锁、热点数据缓存
- **Kafka**: 消息队列(已配置但部分功能待实现)

### 各服务技术栈
| 服务 | HTTP框架 | ORM | 通信协议 |
|------|----------|-----|----------|
| user-service | Gin | GORM | HTTP+gRPC |
| shop-service | Gin | GORM | HTTP+gRPC |
| content-service | Gin | GORM | HTTP+gRPC |

### 典型请求链路
```
请求 → Gin中间件(认证/CORS) → Controller → Service(业务逻辑) → Repository/Redis → MySQL
```

### 前端技术栈
Vue 3 + Vite + Pinia + Vant(移动端UI) + Axios

### 启动脚本
`start.bat`一键启动：启动docker中间件 → 启动3个Go服务 → 打开浏览器访问

### 通用配置模式
每个服务`config/config.go`使用Viper加载YAML配置，包含Server、MySQL、Redis、GRPC、Etcd五个配置块。配置失败时使用硬编码默认值。
