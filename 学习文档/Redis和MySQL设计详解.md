# Redis和MySQL设计详解

## 一、Redis设计详解

### 1. 用户认证模块

#### 1.1 登录验证码

**Key结构**：`login:code:{phone}`
**数据结构**：String
**命令**：
- 存储：`SET login:code:13800138000 "385912" EX 300`
- 获取：`GET login:code:13800138000`
- 过期时间：300秒（5分钟）

**设计原因**：
- 验证码是临时数据，只需要短期存储
- String结构最简单，适合存储简介：以微服务形式搭建的本地生活平台，包括用户服务，购物服务及内容服务，重点攻克了缓存一致性、缓
  存击穿雪崩，高并发秒杀超卖及分布式锁可靠性等问题
  技术栈：Go, Gin, Gorm, MySQL, Redis, Kafka, Etcd, gRPC
  亮点工作：
  鉴权认证优化：设计基于 JWT 与 Refresh Token 的双 Token 安全认证体系，结合 Redis 落地 Token 黑名
  单机制，解决无感刷新与账户强制下线难题
  秒杀系统优化：针对秒杀的超卖风险及数据库压力过大问题，引入Redis Lua脚实现一人一单；通过Kafka
  实现库存扣减与订单创建的解耦并削峰；经压测，支持了5k+ QPS零超卖，订单创建成功率100%，接口延
  时<50ms
  缓存双写一致性：引入布隆过滤器与 Redis 互斥锁防范缓存穿透与击穿。针对双写模式下存在缓存与数据
  库数据不一致风险，采用Cache Aside策略+ 监听Binlog异步更新的方案，确保最终一致性单个值
- 5分钟过期时间足够用户完成登录操作

#### 1.2 双Token认证机制

**核心设计**：
- Access Token：纯JWT，不存Redis
- Refresh Token：存Redis白名单，支持多设备管理

**Refresh Token存储**：
- Key：`token:refresh:{refreshTokenID}`
- 数据结构：String
- Value：`{userID}:{deviceID}`（如"123:device_abc"）
- 命令：`SET token:refresh:abc123 "123:device_abc" EX 604800`
- 过期时间：604800秒（7天）

**多设备Token白名单**：
- Key：`token:whitelist:{userID}:{deviceID}`
- 数据结构：Hash
- Field：refreshTokenID
- Value：过期时间（RFC3339格式）
- 命令：`HSET token:whitelist:123:device_abc "refresh123" "2026-04-23T12:00:00Z"`
- 过期时间：604800秒（7天）

**用户设备列表**：
- Key：`user:devices:{userID}`
- 数据结构：Set
- 命令：`SADD user:devices:123 "device_abc" "device_xyz"`
- 过期时间：604800秒（7天）

**设计原因**：
- Access Token纯JWT：无需查Redis，验证速度快（O(1)）
- Refresh Token白名单：支持多设备登录，方便管理
- Hash结构：同一设备可生成多个refreshToken，便于令牌轮换
- Set结构：快速查询用户登录的所有设备

#### 1.3 用户会话缓存

**Key**：`login:user:{userID}`
**数据结构**：Hash
**命令**：
- 存储：`HSET login:user:123 id 123 phone "13800138000" nickName "用户123" icon "url"`
- 获取：`HGETALL login:user:123`
- 过期时间：1800秒（30分钟）

**设计原因**：
- Hash结构适合存储对象的多个字段
- 可单独获取某个字段（如`HGET login:user:123 nickName`）
- 30分钟过期时间平衡了性能和内存消耗

### 2. 达人探店模块

#### 2.1 博客点赞

**Key**：`blog:liked:{blogId}`
**数据结构**：ZSet
**命令**：
- 点赞：`ZADD blog:liked:100 1713244800 "123"`（score为时间戳）
- 取消点赞：`ZREM blog:liked:100 "123"`
- 判断是否点赞：`ZSCORE blog:liked:100 "123"`
- 获取点赞数：`ZCARD blog:liked:100`
- 获取点赞列表：`ZREVRANGE blog:liked:100 0 9`（按时间倒序）
- 过期时间：永不过期（数据重要，需持久化）

**设计原因**：
- ZSet天然去重：同一用户多次点赞只会更新score
- score为时间戳：可按时间排序展示点赞列表
- ZSCORE判断点赞：O(logN)时间复杂度
- ZCARD获取点赞数：O(1)时间复杂度

#### 2.2 关注Feed流

**Key**：`feed:{userId}`
**数据结构**：ZSet
**命令**：
- 推送博客：`ZADD feed:123 1713244800 "100"`（score为发布时间戳）
- 获取Feed：`ZREVRANGE feed:123 0 9`（按时间倒序）
- 分页获取：`ZREVRANGE feed:123 10 19`
- 过期时间：永不过期

**设计原因**：
- ZSet按score排序：保证Feed流按时间顺序
- ZREVRANGE：高效获取最新内容
- 支持分页查询：通过偏移量控制

### 3. 好友关注模块

**Key**：`follows:{userId}`
**数据结构**：Set
**命令**：
- 关注：`SADD follows:123 "456"`
- 取消关注：`SREM follows:123 "456"`
- 判断是否关注：`SISMEMBER follows:123 "456"`
- 获取共同关注：`SINTER follows:123 follows:789`
- 过期时间：永不过期

**设计原因**：
- SISMEMBER判断关注：O(1)时间复杂度
- SADD/SREM：O(1)时间复杂度
- SINTER求交集：O(N)时间复杂度，N为较小集合大小

### 4. 用户签到模块

**Key**：`user:sign:{yyyyMM}:{userId}`
**数据结构**：BitMap
**命令**：
- 签到：`SETBIT user:sign:202604:123 15 1`（16号签到，位从0开始）
- 获取签到情况：`BITFIELD user:sign:202604:123 GET u30 0`（获取前30位）
- 过期时间：32天（保证整月数据可用）

**设计原因**：
- BitMap极致节省内存：31天仅需31位 ≈ 4字节/月
- SETBIT操作：O(1)时间复杂度
- 支持连续签到计算：通过位操作快速统计

### 5. 商铺缓存模块

#### 5.1 商铺缓存

**Key**：`cache:shop:{shopId}`
**数据结构**：String（JSON序列化）
**命令**：
- 存储：`SET cache:shop:1 "{\"id\":1,\"name\":\"商铺1\"}" EX 1800`
- 获取：`GET cache:shop:1`
- 过期时间：1800秒（30分钟）

**空值缓存**：
- Key：`cache:null:{key}`
- 数据结构：String
- 命令：`SET cache:null:shop:999 "" EX 300`
- 过期时间：300秒（5分钟）

**设计原因**：
- 空值缓存防止缓存穿透：查询不存在的商铺时缓存空值
- 30分钟过期时间：平衡缓存命中率和数据一致性
- 5分钟空值过期：避免长期缓存不存在的数据

#### 5.2 分布式锁

**不可重入锁**：
- Key：`lock:shop:{key}`
- 数据结构：String
- 命令：`SET lock:shop:1 "uuid123" NX EX 30`
- 释放：Lua脚本 `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`
- 过期时间：30秒

**可重入锁**：
- Key：`lock:shop:{key}`
- 数据结构：Hash
- 命令：`HSET lock:shop:1 "client123" 1`
- 重入：`HINCRBY lock:shop:1 "client123" 1`
- 释放：`HINCRBY lock:shop:1 "client123" -1`，为0时`DEL`

**设计原因**：
- NX选项：保证互斥性
- EX选项：防止死锁
- Lua脚本：保证释放锁的原子性
- Hash结构：支持可重入性

### 6. 优惠券秒杀模块

#### 6.1 秒杀库存

**Key**：`seckill:stock:{voucherId}`
**数据结构**：String
**命令**：
- 初始化：`SET seckill:stock:100 100 EX 86400`
- 扣减：`DECR seckill:stock:100`
- 过期时间：86400秒（24小时）

**一人一单**：
- Key：`seckill:order:{voucherId}`
- 数据结构：Set
- 命令：`SADD seckill:order:100 "123"`
- 判断：`SISMEMBER seckill:order:100 "123"`
- 过期时间：86400秒（24小时）

**设计原因**：
- DECR原子操作：防止超卖
- Set集合：天然去重，保证一人一单
- 24小时过期：秒杀活动通常持续一天

#### 6.2 订单异步处理

**Redis Stream 订单队列**：
- Key：`stream:orders`
- 数据结构：Stream
- 命令：
  - 生产：`XADD stream:orders * orderData {...}`
  - 消费：`XREADGROUP GROUP order-group consumer-1 STREAMS stream:orders >`
  - 确认：`XACK stream:orders order-group messageId`
- 过期时间：永不过期

**设计原因**：
- Stream支持消息持久化和回溯
- 支持消费者组，实现负载均衡
- 消息确认机制保证不丢单
- 相比Kafka更轻量，适合单体服务

#### 6.3 分布式ID生成器

**Key**：`icr:{key}`
**数据结构**：String
**命令**：
- 获取ID：`INCR icr:order`
- 格式：41位时间戳 + 10位机器ID + 12位序列号
- 过期时间：永不过期

**设计原因**：
- 41位时间戳：支持约69年
- 10位机器ID：支持1024个节点
- 12位序列号：每毫秒支持4096个ID
- 整体保证全局唯一性和趋势递增

### 7. UV统计模块

**UV统计**：
- Key：`uv:stats:{yyyyMMdd}`
- 数据结构：HyperLogLog
- 命令：
  - 添加：`PFADD uv:stats:20260416 "user1" "user2" "user3"`
  - 统计：`PFCOUNT uv:stats:20260416`
  - 合并：`PFMERGE uv:stats:202604 uv:stats:20260416 uv:stats:20260417`
- 过期时间：90天

**设计原因**：
- HyperLogLog基数统计，12KB内存可统计2.6亿UV
- 误差率约0.81%，满足业务需求
- PFADD/PFCOUNT都是O(N)时间复杂度

### 8. 附近商户地理位置模块

**GeoHash位置存储**：
- Key：`geo:shops`
- 数据结构：Geo
- 命令：
  - 添加：`GEOADD geo:shops 116.397428 39.90923 "shop1"`
  - 搜索：`GEORADIUS geo:shops 116.397428 39.90923 5 km WITHDIST`
- 过期时间：永不过期

**设计原因**：
- Geo数据类型底层使用ZSet，实现地理位置存储
- GeoHash将经纬度转换为字符串，相似字符串表示相近位置
- 支持按距离排序查询附近商户
- 适合LBS应用场景

## 二、MySQL设计详解

### 1. 表结构设计

#### tb_user（用户表）
```sql
CREATE TABLE tb_user (
    id BIGINT PRIMARY KEY,
    phone VARCHAR(20) UNIQUE,
    password VARCHAR(100),
    nick_name VARCHAR(50),
    icon VARCHAR(255),
    create_time DATETIME,
    update_time DATETIME
);
```

#### tb_blog（博客表）
```sql
CREATE TABLE tb_blog (
    id BIGINT PRIMARY KEY,
    user_id BIGINT,
    title VARCHAR(255),
    content TEXT,
    images VARCHAR(1000),
    liked INT DEFAULT 0,
    status INT DEFAULT 1,
    create_time DATETIME,
    update_time DATETIME
);
```

#### tb_blog_comments（博客评论表）
```sql
CREATE TABLE tb_blog_comments (
    id BIGINT PRIMARY KEY,
    blog_id BIGINT,
    user_id BIGINT,
    content VARCHAR(500),
    create_time DATETIME,
    update_time DATETIME
);
```

#### tb_follow（关注表）
```sql
CREATE TABLE tb_follow (
    id BIGINT PRIMARY KEY,
    user_id BIGINT,
    follow_user_id BIGINT,
    create_time DATETIME,
    update_time DATETIME,
    UNIQUE KEY uk_user_follow (user_id, follow_user_id)
);
```

#### tb_shop（商铺表）
```sql
CREATE TABLE tb_shop (
    id BIGINT PRIMARY KEY,
    name VARCHAR(100),
    type_id BIGINT,
    area VARCHAR(50),
    address VARCHAR(200),
    longitude DOUBLE,
    latitude DOUBLE,
    avg_price DECIMAL(10,2),
    sale INT,
    comments INT,
    score DECIMAL(2,1),
    status INT,
    create_time DATETIME,
    update_time DATETIME
);
```

#### tb_voucher（优惠券表）
```sql
CREATE TABLE tb_voucher (
    id BIGINT PRIMARY KEY,
    shop_id BIGINT,
    title VARCHAR(100),
    sub_title VARCHAR(200),
    rules VARCHAR(500),
    pay_value INT,
    actual_value INT,
    type INT,
    status INT,
    create_time DATETIME,
    update_time DATETIME
);
```

#### tb_seckill_voucher（秒杀优惠券表）
```sql
CREATE TABLE tb_seckill_voucher (
    id BIGINT PRIMARY KEY,
    voucher_id BIGINT,
    stock INT,
    begin_time DATETIME,
    end_time DATETIME,
    create_time DATETIME,
    update_time DATETIME
);
```

#### tb_voucher_order（优惠券订单表）
```sql
CREATE TABLE tb_voucher_order (
    id BIGINT PRIMARY KEY,
    user_id BIGINT,
    voucher_id BIGINT,
    status INT,
    create_time DATETIME,
    update_time DATETIME
);
```

### 2. 索引设计与使用原因

#### 2.1 用户表索引

**`phone` 唯一索引**：
- 用途：登录时通过手机号查询用户
- 原因：登录是高频操作，需要快速定位用户
- 时间复杂度：O(logN)

#### 2.2 博客表索引

**`user_id` 普通索引**：
- 用途：查询用户的所有博客
- 原因：用户个人主页需要展示自己的博客列表
- 时间复杂度：O(logN)

**`liked DESC, create_time DESC` 联合索引**：
- 用途：热门博客排序
- 原因：避免 `ORDER BY liked DESC` 导致的 filesort
- 时间复杂度：O(logN)

#### 2.3 博客评论表索引

**`blog_id` 普通索引**：
- 用途：查询博客的所有评论
- 原因：博客详情页需要展示评论列表
- 时间复杂度：O(logN)

#### 2.4 关注表索引

**`uk_user_follow` 联合唯一索引**：
- 用途：防止重复关注，查询关注列表
- 原因：
  1. 保证数据唯一性（一个用户不能重复关注同一人）
  2. 支持 `WHERE user_id = ?` 查询关注列表
  3. 支持 `WHERE follow_user_id = ?` 查询粉丝列表
- 时间复杂度：O(logN)

#### 2.5 商铺表索引

**`type_id` 普通索引**：
- 用途：查询某类型的所有商铺
- 原因：分类页需要按类型展示商铺
- 时间复杂度：O(logN)

#### 2.6 优惠券表索引

**`shop_id` 普通索引**：
- 用途：查询商铺的所有优惠券
- 原因：商铺详情页需要展示可用优惠券
- 时间复杂度：O(logN)

#### 2.7 秒杀优惠券表索引

**`voucher_id` 唯一索引**：
- 用途：通过优惠券ID查询秒杀信息
- 原因：秒杀活动需要快速获取库存信息
- 时间复杂度：O(logN)

#### 2.8 订单表索引

**`user_id` 普通索引**：
- 用途：查询用户的所有订单
- 原因：用户个人中心需要展示订单列表
- 时间复杂度：O(logN)

**`voucher_id` 普通索引**：
- 用途：查询优惠券的使用情况
- 原因：统计优惠券使用次数
- 时间复杂度：O(logN)

## 三、性能优化要点

### 1. Redis优化

1. **数据结构选择**：根据场景选择合适的数据结构
   - 点赞：ZSet（天然去重+排序）
   - 关注：Set（快速判断）
   - 签到：BitMap（节省内存）
   - 缓存：String（简单高效）

2. **过期时间设置**：
   - 验证码：5分钟
   - Token：7天
   - 缓存：30分钟
   - 空值：5分钟

3. **批量操作**：
   - 使用Pipeline减少网络往返
   - 使用Lua脚本保证原子性

4. **缓存策略**：
   - Cache-Aside：先查缓存，再查DB
   - 延迟双删：更新后删除缓存，sleep后再次删除

### 2. MySQL优化

1. **索引设计**：
   - 高频查询字段建索引
   - 避免全表扫描
   - 合理使用联合索引

2. **SQL优化**：
   - 使用`LIMIT`分页
   - 避免`SELECT *`
   - 使用`EXPLAIN`分析执行计划

3. **分库分表**：
   - 订单表：按用户ID分表
   - 评论表：按博客ID分表

4. **读写分离**：
   - 主库写，从库读
   - 减轻主库压力

## 四、代码对应关系

### Redis常量定义

**user-service/utils/redis_constants.go**：
- `LoginCodeKey` = "login:code:"
- `LoginCodeTTL` = 5 // 5分钟
- `RefreshTokenKey` = "token:refresh:"
- `RefreshTokenTTL` = 7 * 24 * time.Hour
- `UserSignKey` = "user:sign:"

**shop-service/utils/redis_constants.go**：
- `CacheShopKey` = "cache:shop:"
- `CacheShopTTL` = 30 // 30分钟
- `CacheNullKey` = "cache:null:"
- `CacheNullTTL` = 5 // 5分钟
- `LockShopKey` = "lock:shop:"
- `LockShopTTL` = 30 // 30秒
- `SeckillVoucherStockKey` = "seckill:stock:"
- `SeckillVoucherOrderKey` = "seckill:order:"

**content-service/utils/redis_constants.go**：
- `BlogLikedKey` = "blog:liked:"
- `FollowsKey` = "follows:"
- `FeedKey` = "feed:"

### MySQL索引使用

**user-service/repository/user_repository.go**：
- `FindByPhone`：使用`phone`唯一索引
- `FindByID`：使用主键索引

**content-service/repository/blog_repository.go**：
- `FindHot`：使用`liked`索引
- `FindByUser`：使用`user_id`索引
- `FindCommentsByBlog`：使用`blog_id`索引
- `IsFollowed`：使用`uk_user_follow`联合索引

**shop-service/repository/shop_repository.go**：
- `FindByType`：使用`type_id`索引

**shop-service/repository/voucher_repository.go**：
- `FindByShopID`：使用`shop_id`索引
- `FindSeckillVoucherByVoucherID`：使用`voucher_id`索引
- `UpdateSeckillVoucherStock`：使用`voucher_id`索引 + 乐观锁

## 五、面试高频问题

### 1. Redis相关

**Q1：为什么点赞使用ZSet而不是Set？**
A：ZSet可以存储score，用于记录点赞时间，支持按时间排序展示点赞列表。Set只能存储成员，无法排序。

**Q2：如何防止缓存穿透？**
A：使用布隆过滤器判断key是否存在，结合空值缓存（缓存不存在的数据5分钟）。

**Q3：如何防止缓存击穿？**
A：使用分布式互斥锁，只有一个请求去查询数据库并更新缓存。

**Q4：如何防止缓存雪崩？**
A：设置随机过期时间，使用多级缓存，Redis高可用（主从+哨兵）。

**Q5：秒杀超卖如何解决？**
A：使用Redis DECR原子操作预扣减库存，结合Lua脚本保证原子性，最后异步落库。

### 2. MySQL相关

**Q1：为什么关注表使用联合唯一索引？**
A：既保证数据唯一性（防止重复关注），又支持快速查询关注列表和粉丝列表。

**Q2：如何优化`ORDER BY liked DESC`？**
A：创建`(liked DESC, create_time DESC)`联合索引，避免filesort。

**Q3：秒杀场景如何优化库存扣减？**
A：使用`UPDATE tb_seckill_voucher SET stock = stock - 1 WHERE voucher_id = ? AND stock > 0`，利用数据库行级锁和条件判断。

**Q4：如何优化分页查询？**
A：使用`LIMIT`和`OFFSET`，结合索引覆盖，避免全表扫描。

**Q5：如何设计分布式ID？**
A：使用Redis ID Worker，格式：41位时间戳 + 10位机器ID + 12位序列号。

## 六、总结

本项目的Redis和MySQL设计充分考虑了：

1. **性能优化**：选择合适的数据结构和索引
2. **数据一致性**：缓存与数据库的同步策略
3. **高并发处理**：分布式锁、原子操作
4. **内存优化**：BitMap、HyperLogLog等特殊数据结构
5. **可扩展性**：分库分表、读写分离

通过合理的设计，系统能够在高并发场景下保持良好的性能和可靠性。

---

## 七、面试问题口述回答

### 问题1：项目中使用了什么Redis的数据结构？

**口述回答**：

我们项目使用了Redis的多种数据结构，根据不同业务场景选择最合适的：

1. **String（字符串）**：用于验证码存储、缓存Value、分布式ID生成器
   - 示例：`login:code:13800138000` 存储登录验证码

2. **Hash（哈希）**：用于存储用户会话信息、Token白名单
   - 示例：`login:user:123` 存储用户信息，字段包括id、phone、nickName等

3. **Set（集合）**：用于关注列表、秒杀一人一单、用户设备列表
   - 示例：`follows:123` 存储用户123关注的人
   - 示例：`seckill:order:100` 存储秒杀100的已下单用户

4. **ZSet（有序集合）**：用于博客点赞（按时间排序）、Feed流推送
   - 示例：`blog:liked:100` 存储博客100的点赞用户，score为时间戳

5. **BitMap（位图）**：用于用户签到
   - 示例：`user:sign:202604:123` 存储用户123的2024年4月签到情况

6. **HyperLogLog**：用于UV统计
   - 示例：`uv:stats:20260416` 统计当天的独立访客数

7. **Geo（地理位置）**：用于附近商户查询
   - 示例：`geo:shops` 存储商户的经纬度坐标

8. **Stream**：用于订单异步处理
   - 示例：`stream:orders` 存储订单消息队列

**选择依据**：主要是看业务场景，比如需要排序用ZSet，需要去重用Set，需要存储大量计数用HyperLogLog节省内存。

---

### 问题2：项目中MySQL使用了哪些索引？

**口述回答**：

我们项目主要使用了以下索引：

1. **主键索引**（PRIMARY KEY）：
   - 所有表都有id作为主键
   - 物理存储按照主键顺序，查询效率最高

2. **唯一索引**（UNIQUE）：
   - `tb_user.phone`：手机号唯一，用于登录查询
   - `tb_follow(user_id, follow_user_id)`：联合唯一索引，防止重复关注

3. **普通索引**（INDEX）：
   - `tb_blog.user_id`：查询用户发布的博客
   - `tb_blog_comments.blog_id`：查询博客的评论
   - `tb_shop.type_id`：按类型查询商铺
   - `tb_voucher.shop_id`：查询商铺的优惠券
   - `tb_voucher_order.user_id`：查询用户的订单
   - `tb_voucher_order.voucher_id`：查询优惠券的使用情况

4. **联合索引**：
   - `tb_blog(liked DESC, create_time DESC)`：优化热门博客排序，避免filesort

**设计原则**：
- 索引建立在高频查询字段上
- 避免过多索引增加写入开销
- 遵循最左前缀原则设计联合索引

---

### 问题3：项目中MySQL有哪些表格，字段有哪些？

**口述回答**：

我们项目共有8张核心表：

1. **tb_user（用户表）**
   - id（BIGINT 主键）、phone（VARCHAR 唯一）、password（VARCHAR）、nick_name（VARCHAR）、icon（VARCHAR 头像）、create_time、update_time

2. **tb_blog（博客表）**
   - id（BIGINT 主键）、user_id（BIGINT）、title（VARCHAR）、content（TEXT）、images（VARCHAR）、liked（INT 点赞数）、status（INT 状态）、create_time、update_time

3. **tb_blog_comments（博客评论表）**
   - id（BIGINT 主键）、blog_id（BIGINT）、user_id（BIGINT）、content（VARCHAR）、create_time、update_time

4. **tb_follow（关注表）**
   - id（BIGINT 主键）、user_id（BIGINT 当前用户）、follow_user_id（BIGINT 被关注的用户）、create_time、update_time

5. **tb_shop（商铺表）**
   - id（BIGINT 主键）、name（VARCHAR）、type_id（BIGINT 分类ID）、area（VARCHAR 区域）、address（VARCHAR 地址）、longitude（DOUBLE 经度）、latitude（DOUBLE 纬度）、avg_price（DECIMAL 均价）、sale（INT 销量）、comments（INT 评论数）、score（DECIMAL 评分）、status（INT 状态）、create_time、update_time

6. **tb_voucher（优惠券表）**
   - id（BIGINT 主键）、shop_id（BIGINT）、title（VARCHAR 标题）、sub_title（VARCHAR 子标题）、rules（VARCHAR 规则）、pay_value（INT 支付金额）、actual_value（INT 实际价值）、type（INT 类型）、status（INT 状态）、create_time、update_time

7. **tb_seckill_voucher（秒杀优惠券表）**
   - id（BIGINT 主键）、voucher_id（BIGINT 关联优惠券ID）、stock（INT 库存）、begin_time（DATETIME 开始时间）、end_time（DATETIME 结束时间）、create_time、update_time

8. **tb_voucher_order（优惠券订单表）**
   - id（BIGINT 主键）、user_id（BIGINT）、voucher_id（BIGINT）、status（INT 状态）、create_time、update_time

**额外说明**：
- 还有tb_shop_type（商铺类型表）等辅助表
- 所有表都有create_time和update_time字段，便于审计和缓存一致性处理