# 达人探店、好友关注、附近商户、用户签到、UV统计 功能详解

> 本文档基于 hmdp-microservices 项目，深入剖析五大核心功能的代码逻辑、Redis数据结构设计、面试高频问题及场景题答案。

---

## 一、功能讲解与代码逻辑分析

### 1.1 达人探店功能（博客系统）

#### 1.1.1 功能概述

达人探店功能是一个完整的博客系统，支持：
- 博客发布与查询
- 热门博客排序展示
- 关注用户的Feed流
- 博客点赞/取消点赞
- 博客评论

#### 1.1.2 代码逻辑分析

**核心文件：**
- `content-service/controller/blog_controller.go` - 控制器层
- `content-service/service/blog_service.go` - 服务层
- `content-service/repository/blog_repository.go` - 数据访问层

**关键流程：**

1. **博客点赞功能** (`blog_service.go:221-241`)
```go
func (s *BlogService) LikeBlog(ctx context.Context, blogId, userId int64) error {
    key := utils.BlogLikedKey + strconv.FormatInt(blogId, 10)

    // 检查是否已点赞
    _, err := s.rdb.ZScore(ctx, key, strconv.FormatInt(userId, 10)).Result()

    if err == redis.Nil {
        // 未点赞，执行点赞操作
        // 使用ZSet存储，score为时间戳（可用于排序）
        s.rdb.ZAdd(ctx, key, redis.Z{
            Score:  float64(time.Now().Unix()),
            Member: strconv.FormatInt(userId, 10),
        })
    } else if err == nil {
        // 已点赞，返回错误
        return fmt.Errorf("已经点过赞了")
    }
    return nil
}
```

**实现要点：**
- 使用Redis ZSet存储点赞信息，member是用户ID，score是时间戳
- 通过ZScore判断是否已点赞（O(logN)复杂度）
- 点赞数通过ZCard获取

2. **关注Feed流** (`blog_service.go:157-218`)
```go
func (s *BlogService) ListFollowBlogs(ctx context.Context, userId int64, current, size int32) ([]map[string]interface{}, int32, error) {
    key := utils.FeedKey + strconv.FormatInt(userId, 10)
    offset := (current - 1) * size

    // 从Redis收件箱获取关注用户的博客
    results, err := s.rdb.ZRevRange(ctx, key, int64(offset), int64(offset+size-1)).Result()

    // 解析博客ID，查询详情
    var blogIDs []int64
    for _, result := range results {
        blogID, _ := strconv.ParseInt(result, 10, 64)
        blogIDs = append(blogIDs, blogID)
    }

    // 查询博客详情并返回
    // ...
}
```

**待改进点：**
- `CreateBlog`函数（262-282行）中的TODO：博客发布后未实现推送给粉丝的逻辑
- Feed流推送应该是异步的，可以使用消息队列

3. **热门博客查询**
- 从数据库查询热门博客列表
- 循环查询每篇博客的点赞数（N+1问题）
- **问题**：在大数据量下性能较差，应该批量查询或使用Redis缓存

#### 1.1.3 注意事项

1. **缓存一致性**：点赞后没有同步更新数据库的likeCount字段
2. **Feed流实现**：当前只实现了拉取模式，没有实现推模式（写扩散）
3. **点赞去重**：使用ZSet天然去重，但高并发下可能存在并发问题

---

### 1.2 好友关注功能

#### 1.2.1 功能概述

- 关注/取消关注用户
- 获取粉丝列表、关注列表
- 查询共同关注
- 检查关注状态

#### 1.2.2 代码逻辑分析

**核心文件：**
- `content-service/controller/follow_controller.go`
- `content-service/service/follow_service.go`

**关键实现：**

1. **关注/取消关注** (`follow_service.go:28-55`)
```go
func (s *FollowService) FollowUser(ctx context.Context, userId, followUserId int64, isFollow bool) error {
    key := utils.FollowsKey + strconv.FormatInt(userId, 10)

    if isFollow {
        // 关注：写入数据库 + Redis Set
        follow := &model.Follow{
            UserID:       userId,
            FollowUserID: followUserId,
        }
        err := s.blogRepo.CreateFollow(follow)
        if err == nil {
            s.rdb.SAdd(ctx, key, strconv.FormatInt(followUserId, 10))
        }
    } else {
        // 取消关注：删除数据库 + Redis Set
        err := s.blogRepo.DeleteFollow(userId, followUserId)
        if err == nil {
            s.rdb.SRem(ctx, key, strconv.FormatInt(followUserId, 10))
        }
    }
    return err
}
```

2. **共同关注** (`follow_service.go:112-139`)
```go
func (s *FollowService) ListCommonFollows(ctx context.Context, userId, targetUserId int64) ([]map[string]interface{}, error) {
    key1 := utils.FollowsKey + strconv.FormatInt(userId, 10)
    key2 := utils.FollowsKey + strconv.FormatInt(targetUserId, 10)

    // 使用Redis SInter求交集
    intersect, err := s.rdb.SInter(ctx, key1, key2).Result()

    // 解析用户ID并查询用户信息
    // ...
}
```

3. **检查关注状态** (`follow_service.go:141-153`)
```go
func (s *FollowService) CheckFollow(ctx context.Context, userId, targetUserId int64) (bool, error) {
    key := utils.FollowsKey + strconv.FormatInt(userId, 10)

    // 从Redis快速查询
    exists, err := s.rdb.SIsMember(ctx, key, strconv.FormatInt(targetUserId, 10)).Result()
    if err != nil {
        // Redis失败时降级到数据库
        return s.blogRepo.IsFollowed(userId, targetUserId)
    }
    return exists, nil
}
```

#### 1.2.3 注意事项

1. **Redis与数据库一致性**：先操作数据库，成功后同步Redis
2. **共同关注性能**：SInter是O(N)操作，大关注量时需考虑优化
3. **降级策略**：Redis失败时降级到数据库查询

---

### 1.3 附近商户功能

#### 1.3.1 现状分析

当前项目**尚未实现**附近商户的GEO查询功能，仅在Shop模型中定义了经纬度字段：

```go
// shop-service/model/shop.go
type Shop struct {
    // ... 其他字段
    Longitude float64 `json:"longitude"` // 经度
    Latitude  float64 `json:"latitude"`  // 纬度
}
```

#### 1.3.2 预期实现方案

**应该使用Redis GEO数据结构**，核心命令：
- `GEOADD` - 添加商户位置
- `GEODIST` - 计算距离
- `GEORADIUS` / `GEOSEARCH` - 搜索附近商户

**Redis Key设计：**
```
key: "shop:geo:{typeId}"  // 按类型存储
value: 成员(member)是shopId，经纬度是score
```

**实现示例：**
```go
// 添加商户到GEO
func AddShopToGeo(ctx context.Context, rdb *redis.Client, shopTypeId int64, shopId int64, longitude, latitude float64) error {
    key := fmt.Sprintf("shop:geo:%d", shopTypeId)
    return rdb.GeoAdd(ctx, key, &redis.GeoLocation{
        Name:      strconv.FormatInt(shopId, 10),
        Longitude: longitude,
        Latitude:  latitude,
    }).Err()
}

// 查询附近商户
func SearchNearby(ctx context.Context, rdb *redis.Client, shopTypeId int64, longitude, latitude float64, radius float64) ([]redis.GeoLocation, error) {
    key := fmt.Sprintf("shop:geo:%d", shopTypeId)
    return rdb.GeoSearch(ctx, key, &redis.GeoSearchQuery{
        Longitude: longitude,
        Latitude:  latitude,
        Radius:    radius,
        Unit:      "km",
    }).Result()
}
```

#### 1.3.3 待改进点

1. **未实现GEO功能**：需要使用Redis GEO命令实现附近商户查询
2. **距离排序**：GEORADIUS默认按距离排序，可设置多个返回选项
3. **性能考虑**：GEO底层使用ZSet实现，适合亿级数据

---

### 1.4 用户签到功能

#### 1.4.1 功能概述

- 用户每日签到
- 统计本月签到次数
- 计算连续签到天数

#### 1.4.2 代码逻辑分析

**核心文件：**
- `user-service/service/user_service.go` (150-206行)
- `user-service/controller/user_controller.go`

**关键实现：**

1. **签到** (`user_service.go:150-167`)
```go
func (s *UserService) Sign(ctx context.Context, userId int64) error {
    now := time.Now()
    yyyyMM := now.Format("2006:01")
    key := utils.UserSignKey + yyyyMM + ":" + strconv.FormatInt(userId, 10)

    // 获取今天是本月的第几天（1-31）
    dayOfMonth := now.Day()

    // 使用BitMap存储，第 dayOfMonth-1 位表示当天是否签到
    return s.rdb.SetBit(ctx, key, int64(dayOfMonth-1), 1).Err()
}
```

2. **获取签到次数** (`user_service.go:169-206`)
```go
func (s *UserService) GetSignCount(ctx context.Context, userId int64) (int32, error) {
    now := time.Now()
    yyyyMM := now.Format("2006:01")
    key := utils.UserSignKey + yyyyMM + ":" + strconv.FormatInt(userId, 10)
    dayOfMonth := now.Day()

    // 使用BitField获取位图数据
    result, err := s.rdb.BitField(ctx, key, "GET", "u"+strconv.Itoa(dayOfMonth), "0").Result()

    // 解析二进制计算连续签到
    num := result[0]
    binaryStr := strconv.FormatInt(num, 2)
    count := int32(0)
    for i := len(binaryStr) - 1; i >= 0; i-- {
        if binaryStr[i] == '1' {
            count++
        } else {
            break  // 遇到0断开连续
        }
    }
    return count, nil
}
```

#### 1.4.3 注意事项

1. **BitMap优化**：使用BitMap存储签到记录，每月一个Key，每个用户占3.875KB/月
2. **连续签到计算**：当前实现有bug，应该从当往前计算连续天数
3. **Key过期**：每月结束需要设置过期时间或自动创建新Key

---

### 1.5 UV统计功能

#### 1.5.1 现状分析

当前项目**未实现UV统计功能**。

#### 1.5.2 预期实现方案

**应该使用Redis HyperLogLog数据结构**，用于基数统计。

```go
// 使用HyperLogLog统计UV
const UVKey = "uv:page:"

// 记录访问
func RecordUV(ctx context.Context, rdb *redis.Client, pageId string, userId int64) error {
    key := UVKey + pageId
    return rdb.PFAdd(ctx, key, strconv.FormatInt(userId, 10)).Err()
}

// 获取UV统计
func GetUV(ctx context.Context, rdb *redis.Client, pageId string) (int64, error) {
    key := UVKey + pageId
    return rdb.PFCount(ctx, key).Result()
}

// 合并多个页面的UV
func MergeUV(ctx context.Context, rdb *redis.Client, destKey string, sourceKeys ...string) error {
    return rdb.PFMerge(ctx, destKey, sourceKeys...).Err()
}
```

**HyperLogLog特点：**
- 优点：占用内存极小（12KB/键），可统计2^64基数
- 误差率：约0.81%
- 适合：海量数据的UV统计

---

## 二、Redis数据结构总结

### 2.1 达人探店（博客系统）

| 功能 | Redis数据结构 | Key格式 | 用途 |
|------|--------------|---------|------|
| 博客点赞 | ZSet | `blog:liked:{blogId}` | 存储点赞用户ID，score为时间戳 |
| Feed流 | ZSet | `feed:{userId}` | 存储关注的博客ID，按时间倒序 |

**ZSet优势：**
- 天然去重（用户ID不重复）
- 按score排序（点赞时间、博客时间）
- ZCard O(1)获取点赞数
- ZScore O(logN)判断是否点赞

### 2.2 好友关注

| 功能 | Redis数据结构 | Key格式 | 用途 |
|------|--------------|---------|------|
| 关注列表 | Set | `follows:{userId}` | 存储关注用户的ID |
| 共同关注 | - | SInter(key1, key2) | Set求交集 |

**Set优势：**
- SIsMember O(1)判断是否关注
- SInter求共同关注
- SAdd/SRem O(1)添加/删除关注

### 2.3 用户签到

| 功能 | Redis数据结构 | Key格式 | 用途 |
|------|--------------|---------|------|
| 签到记录 | BitMap | `user:sign:{yyyyMM}:{userId}` | 31位，每月一个Key |

**BitMap优势：**
- 极致节省内存：31位≈4字节/月
- 位操作效率高
- 适合稀疏数据

### 2.4 附近商户（应使用GEO）

| 功能 | Redis数据结构 | Key格式 | 用途 |
|------|--------------|---------|------|
| 商户位置 | GEO (底层ZSet) | `shop:geo:{typeId}` | 存储商户经纬度 |

### 2.5 UV统计（应使用HyperLogLog）

| 功能 | Redis数据结构 | Key格式 | 用途 |
|------|--------------|---------|------|
| 页面UV | HyperLogLog | `uv:page:{pageId}` | 存储访问用户ID集合 |

---

## 三、面试高频问题

### 3.1 达人探店相关

**Q1: 点赞功能如何实现防重复？**

回答要点：
- 使用Redis ZSet天然去重，member是用户ID
- ZAdd会自动去重，多次添加相同用户ID会更新时间戳
- 数据库层面也可以加唯一索引防重

**Q2: Feed流如何实现？推模式和拉模式区别？**

回答要点：
- 拉模式（Pull）：用户请求时从关注列表中拉取博客
- 推模式（Push）：博客发布时主动推送到粉丝的收件箱
- 当前项目使用拉模式，但推模式代码有TODO未实现
- 推模式适合粉丝少、博客多的场景
- 拉模式适合粉丝多、博客少的场景

**Q3: 如何实现热门博客排序？**

回答要点：
- 可以用Redis ZSet，score为热度分数（点赞数+时间衰减）
- 定时计算热度分数并更新ZSet
- 也可以用数据库的ORDER BY + LIMIT

### 3.2 好友关注相关

**Q4: 如何实现共同关注？**

回答要点：
- 使用Redis SInter命令对两个Set求交集
- 时间复杂度O(N)，N为较小集合的大小
- 大数据量时可考虑预计算或降级

**Q5: 关注关系如何存储？**

回答要点：
- Redis Set存关注列表，用于快速查询
- MySQL存关系表，用于持久化
- 双向同步，Redis作缓存

### 3.3 签到功能相关

**Q6: BitMap签到如何实现连续签到计算？**

回答要点：
- 使用BitField获取位图
- 从当前天往前遍历，统计连续1的个数
- 注意bit位置：第0位对应1号

### 3.4 Redis数据结构相关

**Q7: String、Hash、List、Set、ZSet区别？**

回答要点：
- String：字符串，适用于缓存简单值
- Hash：哈希，适用于对象存储
- List：列表，适用于消息队列、列表
- Set：无序集合，适用于去重、标签
- ZSet：有序集合，适用于排序场景

**Q8: 如何选择合适的数据结构？**

回答要点：
- 需要排序 → ZSet
- 需要去重 → Set
- 需要位操作 → BitMap
- 需要地理位置 → GEO
- 需要基数统计 → HyperLogLog

---

## 四、场景问题及口述回答（20+）

### 场景1：如何实现王者荣耀的实时在线排行榜？

**口述回答：**

面试官您好，我来详细讲解一下王者荣耀实时在线排行榜的实现方案。

首先，我们采用的是**冷热数据分离**的方案。为什么需要分离？因为王者荣耀的玩家数量巨大，榜单需要实时更新，如果直接对所有玩家排序，数据库压力会非常大。

**热数据层**：我们使用Redis ZSet存储前1000名的玩家数据。ZSet的score就是玩家的荣耀积分，ZRevRange可以O(logN+M)快速获取Top N。热数据更新非常频繁，每一局游戏结束都会更新相关玩家的分数，所以我们用Redis保证高性能。

**冷数据层**：对于1000名之外的玩家，数据存储在MySQL数据库中。每隔一定时间（比如5分钟），我们会从Redis中获取最新的Top 1000，然后与数据库中的数据进行合并。同时，玩家分数变化时，我们会异步写入消息队列，最终落地到数据库。

**查询逻辑**：当玩家请求排行榜时，首先查询Redis热数据，如果排名在1000以内，直接返回。如果排名在1000以外，则需要查询数据库，但这种情况我们会返回"未上榜"或者"排名1000+"。

**进阶优化**：
1. 使用Lua脚本保证原子性，避免并发问题
2. 考虑数据分片，减轻单节点压力
3. 热点玩家数据缓存到本地，减少Redis请求
4. 异步写入数据库，避免影响主流程

这样设计，既保证了Top玩家的实时性，又通过分层减轻了数据库压力，整体延迟可以控制在10ms以内。

---

### 场景2：如何实现朋友圈功能？

**口述回答：**

面试官您好，朋友圈是一个比较复杂的系统，我分几个核心模块来讲解。

**1. _feed流实现_**
我们采用**推拉结合**的模式。对于普通用户，采用拉取模式 - 关注的人发朋友圈时，仅仅是写入自己的朋友圈列表（一种特殊的收件箱），好友在查看时拉取。对于粉丝量大的大V，采用推模式 - 大V发朋友圈时，主动推送到所有粉丝的收件箱。

朋友圈的收件箱我们使用Redis ZSet存储，score是发布时间戳，这样天然保证时间顺序。Key设计为`timeline:user:{userId}`。

**2. 相册存储**
朋友圈的图片需要单独存储。我们使用对象存储服务（如OSS），图片上传后会返回URL。原图和缩略图分别存储，缩略图用于朋友圈列表展示，原图点击查看。

图片表设计：id, user_id, url, width, height, size, created_at。每条朋友圈最多9张图片。

**3. 评论与点赞**
朋友圈的点赞和评论也需要实时展示。这部分和微博类似，使用Redis ZSet存储点赞列表，List存储评论。为了减轻数据库压力，点赞数会异步批量写入数据库。

**4. 缓存策略**
朋友圈的feed流需要缓存。我们会缓存用户最近访问的页面数据，使用LRU淘汰策略。同时，热门朋友圈会预加载到CDN。

**5. 跨天数据处理**
朋友圈按天展示，每天一个分组。这个分组在用户滑动到对应位置时异步加载，使用无限滚动。

整体架构上，我们使用了微服务拆分，Feed服务、互动服务、相册服务独立部署，通过消息队列解耦。核心接口的TP99可以控制在200ms以内。

---

### 场景3：如何设计一个高并发的点赞系统？

**口述回答：**

面试官您好，高并发点赞系统的核心挑战在于：如何保证性能的同时实现去重，以及如何应对热点数据。

**1. Redis缓存层**
我们使用Redis ZSet存储点赞列表，Key设计为`like:blog:{blogId}`，member是用户ID，score是点赞时间。这样做有几个好处：ZAdd天然去重、ZScore可以O(logN)判断是否点过赞、ZCard可以O(1)获取点赞数。

但是这样会有问题：每次获取点赞列表都要查一次Redis。我们可以做读写分离：点赞只写Redis，然后异步批量同步到数据库。读取时优先读Redis，Redis没有才查数据库。

**2. 热点数据处理**
如果某条内容突然爆火，点赞请求会大量打到同一台Redis。我们可以：
- 使用本地缓存做三级缓存：本地缓存 → Redis → 数据库
- 对热点Key做哈希分片，分散到不同Redis实例
- 限流熔断，保护后端服务

**3. 分布式锁**
高并发下需要防止重复点赞。我们可以使用Redis分布式锁，或者利用ZSet的天然特性 - ZAdd返回的响应中可以知道是更新还是新增。

**4. 计数器的优化**
点赞数如果每次都ZCard，压力大。我们可以在Redis中额外维护一个计数器`like:count:{blogId}`，每次点赞+1，取消点赞-1。使用Lua脚本保证原子性。

**5. 异步落库**
为了提高吞吐量，点赞请求直接返回成功，然后通过消息队列异步写入数据库。这样可以把突发的流量削峰填谷。

整体设计思路就是这样，通过多级缓存、读写分离、异步处理来应对高并发。

---

### 场景4：如何实现附近的人功能？

**口述回答：**

面试官您好，附近的人功能核心是地理位置检索，我详细介绍一下。

**1. Redis GEO方案**
我们使用Redis GEO数据结构来存储用户位置。GEO底层使用ZSet实现，member是用户ID，score是经纬度哈希值。

Key设计为`geo:nearby:{城市ID}`，因为按城市分片可以减少数据量。添加位置使用GEOADD，查询附近使用GEORADIUS或GEOSEARCH。

GEORADIUS的参数：
- 半径大小（单位可选km/m）
- 排序方式（DESC/ASC）
- 返回数量和坐标

**2. 性能优化**
GEO查询的复杂度是O(N)，数据量大时会慢。我们的优化策略：
- 按城市分片，减少单次搜索范围
- 只返回用户ID，不返回具体距离（减少计算）
- 使用GeoHash优化，附近的人可以先用粗糙的GeoHash过滤

**3. 在线状态**
附近的人只显示在线用户。我们维护一个Set`online:users`，用户登录时加入，下线时移除。查询时先获取在线用户列表，再GEO查询。

**4. 隐私保护**
位置信息是敏感数据，我们做了多重保护：
- 位置信息加密存储
- 用户可以设置可见范围（仅好友可见/附近可见/完全公开）
- 模糊化处理，显示在某个小区而非精确位置

**5. 其他方案对比**
除了Redis GEO，还可以使用：
- MongoDB的2dsphere索引
- MySQL的空间索引
- ElasticSearch的geo_point类型

我们选择Redis GEO主要是因为性能好、API简单、集成成本低。

---

### 场景5：如何设计一个抢票系统？

**口述回答：**

面试官您好，抢票系统的核心是保证库存不超卖，以及高并发下的性能。

**1. 库存超卖问题**
这是最经典的问题。简单的数据库update库存会存在并发问题：
```sql
UPDATE ticket SET stock = stock - 1 WHERE id = ? AND stock > 0
```
我们使用乐观锁，在表中添加version字段，每次更新时version+1。或者使用Redis原子操作：
```lua
local stock = redis.call('GET', KEYS[1])
if stock > 0 then
    return redis.call('DECR', KEYS[1])
else
    return -1
end
```
使用Lua脚本保证查询和减库存的原子性。

**2. 请求削峰**
抢票开始时流量巨大，直接打穿数据库会崩溃。我们使用多层限流：
- 验证码过滤：答题或图形验证码
- 预约机制：提前预约抢票资格
- 消息队列：抢票请求先进入MQ，异步处理
- 限流熔断：超出处理能力时返回排队中

**3. 缓存策略**
票务信息缓存在Redis，Key设计为`ticket:{showId}:{date}`。库存变化时及时更新缓存，并设置合理的过期时间。

**4. 分布式锁**
防止同一用户重复抢购。使用Redis分布式锁，Key为`lock:buy:{userId}:{ticketId}`。

**5. 消息通知**
抢票成功后，通过消息队列发送通知（短信、APP推送、邮件）。失败的用户进入候补队列。

**6. 对账与补录**
最终以数据库数据为准，定时对账发现异常及时补录。

整体思路就是这样，通过Redis+Lua保证原子性、MQ削峰、多层限流保证系统稳定性。

---

### 场景6：如何实现延迟任务系统？

**口述回答：**

面试官您好，延迟任务在订单超时取消、消息重试等场景很常见。

**1. Redis ZSet方案**
我们使用Redis ZSet实现延迟队列。Key设计为`delay:task:{type}`，member是任务ID，score是执行时间戳。

工作流程：
1. 添加任务时，ZAdd到Redis，score为执行时间
2. 定时任务每秒扫描ZSet，使用ZRangeByScore获取到期的任务
3. 取出任务执行，执行成功从ZSet中删除
4. 执行失败可以重新ZAdd，延迟重试

**2. 消息队列方案**
更主流的方案是使用消息队列的延迟消息功能。比如RabbitMQ的TTL+死信队列，或者Kafka的时间轮。

Kafka时间轮的原理：
- 多个时间轮分层，每层时间精度不同
- 任务根据延迟时间放入对应层级
- 精确度高，性能好

**3. 持久化问题**
Redis方案在Redis挂掉时会丢任务。我们会：
- 定时将ZSet数据持久化到数据库
- Redis启动时从数据库恢复
- 或者使用Redis Cluster + 哨兵保证高可用

**4. 任务幂等**
任务可能被重复执行，需要幂等处理。我们使用数据库唯一键或者分布式锁。

**5. 监控告警**
任务堆积、任务失败都需要告警。我们接入Prometheus监控任务执行情况。

---

### 场景7：如何设计一个_feed流系统？

**口述回答：**

面试官您好，Feed流系统是社交平台的核心，我来详细讲解。

**1. 推拉模式选择**
Feed流有两种模式：

**拉模式（Fan-out-on-load）**：用户请求时，从关注列表中拉取内容。好处是简单，坏处是请求时延迟高。

**推模式（Fan-out-on-write）**：内容发布时，主动推送到粉丝的收件箱。好处是读取快，坏处是写入压力大。

我们采用混合策略：
- 普通用户：拉模式（粉丝少，内容少）
- 大V用户：推模式（粉丝多，写一次推给所有粉丝）
- 关键：识别大V，设置阈值比如粉丝>10万

**2. 收件箱设计**
使用Redis ZSet，Key为`inbox:{userId}`，score是发布时间戳。ZRevRange按时间倒序获取。

收件箱需要限制大小，比如只保留最近1000条。更早的内容通过"查看更多"从数据库加载。

**3. 内容存储**
Feed内容存储在独立的Feed库，按用户ID分表。使用MySQL + 分库分表中间件。

**4. Cache缓存**
热门内容需要缓存。使用本地缓存+Redis二级缓存。本地缓存用Caffeine或Guava Cache。

**5. 分页实现**
Feed流分页不能用简单的offset，因为数据可能变化。常见方案：
- 记录上一页最后一条的时间戳，下一页用时间戳分页
- 使用游标分页，返回下一页的cursor

**6. 点赞评论计数**
这些信息需要实时显示。使用Redis缓存计数值，异步批量更新数据库。

整体架构就是这样的，可以支撑亿级Feed流服务。

---

### 场景8：如何实现一个搜索建议功能？

**口述回答：**

面试官您好，搜索建议功能类似搜索引擎的联想词，我来讲解实现方案。

**1. 前缀匹配**
搜索建议的核心是前缀匹配。最简单的方式是Trie树（前缀树），但数据量大时内存占用高。

**2. Redis方案**
使用Redis ZSet实现：
- Key: `suggest:{prefix}`，存储以该前缀开头的热词
- score: 搜索热度
- member: 完整关键词

添加热词时，需要为每个前缀都添加索引。比如"手机"，需要添加：
- `suggest:手` -> 手机
- `suggest:手机` -> 手机

查询时：ZRevRange获取top 10。

**3. 自动补全优化**
- 考虑到性能，前端可以加防抖，300ms发送一次请求
- 使用本地缓存，相同前缀直接返回
- 支持拼音模糊匹配（如输入"sj"匹配"手机"）

**4. 特殊处理**
- 敏感词过滤
- 搜索结果为空的建议
- 新词热词自动加入

**5. 数据更新**
热词数据需要定时更新。可以：
- 实时统计搜索日志
- 每天凌晨计算热词
- 运营人工干预

---

### 场景9：如何设计一个短链接系统？

**口述回答：**

面试官您好，短链接系统核心是将长URL转换成短字符串。

**1. 短链接生成**
两种方案：

**自增ID方案**：数据库自增主键，转成62进制（数字+大小写字母）。比如ID=10000，转成"2Bi"。优点是确定性，缺点是泄密。

**哈希方案**：对长URL做MD5或SHA256，然后Base62编码取前8位。优点是URL相同生成相同短链接，缺点是有哈希碰撞可能。

我们使用自增ID方案，用Redis INCR生成ID。

**2. 跳转流程**
```
用户访问短链接 -> 302重定向到长链接
```
需要记录短链接到长链接的映射，存储在Redis和MySQL。

**3. 缓存设计**
热点短链接缓存到Redis，Key为`short:{code}`，value是长链接。设置较长的过期时间，因为短链接几乎不会变。

**4. 防攻击**
- 限制单IP请求频率
- 验证码保护
- 监控异常访问

**5. 有效期与回收**
- 永久短链接不删除
- 临时短链接设置过期时间
- 定期清理过期数据

---

### 场景10：如何实现接口限流？

**口述回答：**

面试官您好，接口限流是保障系统稳定性的重要手段，我来介绍几种常用方案。

**1. 计数器限流**
最简单的方式。使用Redis INCR，每请求+1，设置过期时间（如1分钟）。问题是临界时刻可能翻倍。

改进：使用滑动窗口，用Redis ZSet记录请求时间戳，统计窗口内请求数。

**2. 令牌桶算法**
桶里有令牌，每个请求消耗一个令牌，没令牌则拒绝。Redis实现：
```lua
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local allowed = redis.call('INCR', key)
if allowed == 1 then
    redis.call('EXPIRE', key, 60)
end
return allowed <= rate
```

**3. 漏桶算法**
请求像水一样流入漏桶，漏出速率固定。超出容量则拒绝。

**4. 分布式限流**
单机限流不够，需要分布式限流。用Redis Centralized Counter，所有服务实例共享计数。

**5. 本地限流**
Redis限流有网络开销，高性能场景可以用本地限流（如Go的ratelimit库）+ Redis分布式锁。

**6. 分层限流**
- 接口级别限流
- 用户级别限流
- IP级别限流

我们一般配置多个限流规则，逐层生效。

---

### 场景11：如何实现一个分布式ID生成器？

**口述回答：**

面试官您好，分布式ID需要满足：唯一性、趋势递增、高性能。

**1. UUID**
简单，但无序、长度太长、字符串存储占用空间大。

**2. 数据库自增**
MySQL主键自增，单机没问题。分布式下可以：
- 不同实例设置不同起始值和步长（如A：1,3,5... B：2,4,6...）
- 缺点是扩展麻烦

**3. Redis INCR**
使用Redis原子操作INCR，天然趋势递增，性能高。需要注意：
- Redis持久化问题（可配置RDB+AOF）
- Redis高可用（集群+哨兵）

**4. 雪花算法（Snowflake）**
Twitter开源的算法，64位Long：
- 1位：符号位（0）
- 41位：时间戳（毫秒），可支撑69年
- 10位：机器ID（5位数据中心+5位机器ID）
- 12位：序列号，每毫秒可生成4096个ID

优点：高性能、无依赖。缺点：依赖时钟。

**5.  Leaf算法**
美团开源的方案，是雪花算法的改进：
- 支持号段模式，每次从数据库获取一段ID
- 支持雪花模式，支持时钟回拨

我们一般用雪花算法或Leaf，根据业务场景选择。

---

### 场景12：如何实现去重功能？

**口述回答：**

面试官您好，去重在很多场景都需要，我来介绍不同方案。

**1. 布隆过滤器（Bloom Filter）**
原理：使用多个哈希函数，将数据映射到位数组。查询时，所有哈希位置都为1才认为存在。

优点：内存占用极低，查询O(k)，k是哈希函数个数。缺点：有误判率（假阳性），无法删除。

适用场景：缓存穿透防护、黑名单过滤。

Redis实现：使用BF.ADD、BF.EXISTS命令，或直接用位数组。

**2. Cuckoo Filter**
布隆过滤器的改进，支持删除，误判率更低。

**3. HashSet/Redis Set**
精确去重。HashSet内存占用高，Redis Set适合海量去重。

**4. 数据库去重**
唯一索引去重。需要考虑并发插入时的重复处理。

**5. 布谷鸟过滤器**
新一代过滤器，支持计数删除，误判率比Cuckoo Filter更低。

选择建议：
- 允许误判 → 布隆过滤器
- 需要删除 → Cuckoo Filter
- 精确去重 → HashSet/Set

---

### 场景13：如何设计一个排行榜系统？

**口述回答：**

面试官您好，排行榜是常见需求，我来详细介绍。

**1. Redis ZSet实现**
最常用的方案。ZSet的score就是排行榜分数，ZRevRange获取Top N。

优点：天然排序、O(logN+M)获取前N名、O(logN)更新分数。

Key设计：按业务维度分，如`rank:score:weekly`（周榜）。

**2. 分榜设计**
- 日榜、周榜、月榜、年榜
- 每次更新分数时，同步更新各个榜单
- 定时任务生成各榜单数据

**3. 实时与历史**
- 实时排名用Redis
- 历史排名存MySQL，按天/月归档

**4. 复杂排名查询**
- 获取某用户的排名：ZRevRank O(logN)
- 获取排名区间的用户：ZRevRangeByScore

**5. 性能优化**
- 热点数据本地缓存
- 分片降低单实例压力
- 读写分离，读请求打到从节点

**6. 防刷**
- 分数变化需要校验
- 异常监控告警

---

### 场景14：如何实现CAS乐观锁？

**口述回答：**

面试官您好，乐观锁用于并发控制，我来详细讲解。

**1. 版本号机制**
数据库添加version字段：
```sql
UPDATE table SET count = count + 1, version = version + 1
WHERE id = ? AND version = ?
```
如果version不匹配，说明被其他事务修改，事务回滚。

**2. CAS操作**
Compare And Swap，原子操作。Redis实现：
```lua
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then
    return redis.call('SET', KEYS[1], ARGV[2])
else
    return false
end
```

**3. 乐观锁应用场景**
- 库存扣减
- 账户余额修改
- 分布式事务协调

**4. ABA问题**
如果线程A读取数据，线程B修改并提交，线程C又修改回原值，线程A无法感知。

解决：加版本号，或者使用带时间戳的CAS。

**5. 重试机制**
乐观锁冲突时需要重试。使用指数退避，避免频繁重试。

---

### 场景15：如何实现缓存策略？

**口述回答：**

面试官您好，缓存策略是后端开发的核心技能，我来系统讲解。

**1. Cache-Aside（旁路缓存）**
最常用的策略：
- 读：先查缓存，缓存没有查数据库，然后写入缓存
- 写：先更新数据库，然后删除缓存

问题：
- 缓存删除失败：可以重试，或通过消息队列异步删除
- 缓存击穿：使用互斥锁或本地缓存
- 缓存穿透：布隆过滤器

**2. Write-Through**
读写都穿透到缓存，由缓存同步写数据库。适合数据一致性要求高的场景。

**3. Write-Behind**
写先写缓存，异步批量写数据库。性能高但有丢数据风险。

**4. 缓存过期策略**
- 固定过期时间：如30分钟
- 懒加载：访问时刷新
- 定期过期：定时清理

**5. 缓存更新策略**
- 主动更新：数据变化时立即更新缓存
- 被动过期：依靠TTL自然过期
- 延迟双删：更新后删除缓存，sleep后再次删除

**6. 多级缓存**
本地缓存（Guava/Caffeine）→ Redis → 数据库。减少Redis请求。

选择合适的策略需要权衡：一致性、性能、开发复杂度。

---

### 场景16：如何处理缓存击穿、穿透、雪崩？

**口述回答：**

面试官您好，这是缓存的三大经典问题，我来逐一讲解。

**1. 缓存击穿（Cache Breakdown）**
热点Key过期瞬间，大量请求击穿到数据库。

解决方案：
- 互斥锁：只有一个请求去加载缓存
- 永不过期：物理不过期，逻辑过期
- 预热：提前加载热点数据

Redis实现互斥锁：
```lua
if redis.call('SET', 'lock:key', '1', 'NX', 'EX', '10') then
    -- 加载数据
    redis.call('DEL', 'lock:key')
end
```

**2. 缓存穿透（Cache Penetration）**
查询不存在的数据，每次都打到数据库。

解决方案：
- 布隆过滤器：判断Key是否存在
- 空值缓存：查询结果为空也缓存一个空值
- 参数校验：过滤非法请求

**3. 缓存雪崩（Cache Avalanche）**
大量缓存同时过期，或Redis宕机。

解决方案：
- 随机过期时间：避免同时过期
- 永不过期：核心数据
- Redis高可用：主从+哨兵/集群
- 限流降级：保护数据库

**4. 综合方案**
实际项目中综合使用：
- 布隆过滤器防穿透
- 互斥锁防击穿
- 随机TTL + 高可用防雪崩

---

### 场景17：如何实现分布式锁？

**口述回答：**

面试官您好，分布式锁是分布式系统的核心组件，我来详细讲解。

**1. Redis分布式锁**
基础实现：
```lua
SET lock:key value NX EX 30
```
- NX：不存在才设置
- EX：过期时间

释放锁需要用Lua脚本，保证原子性：
```lua
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
else
    return 0
end
```

**2. Redisson封装**
Java项目推荐使用Redisson，封装了完整的分布式锁：
- 可重入锁
- 公平锁
- 读写锁
- 联锁

**3. 锁超时问题**
如果业务执行时间超过锁过期时间，锁会自动释放。解决：
- 合理设置过期时间
- 续期机制（看门狗）
- 业务层面幂等

**4. Redis主从问题**
主节点加锁后，还未同步到从节点，主节点宕机，从节点成为主节点，锁丢失。

解决：RedLock（红锁），需要多数节点加锁成功。但业界有争议，很多公司不用。

**5. ZK分布式锁**
ZK的临时顺序节点 + Watch机制。优点是不用考虑过期时间，缺点是性能不如Redis。

**6. 数据库分布式锁**
表锁或乐观锁，适合简单场景。

---

### 场景18：如何实现消息队列？

**口述回答：**

面试官您好，消息队列是系统解耦的核心，我来介绍选型和实现。

**1. 消息队列对比**
- RabbitMQ：功能丰富，支持复杂路由
- Kafka：高性能，日志系统首选
- RocketMQ：阿里开源，金融级可靠
- Pulsar：云原生，多租户

选择依据：吞吐量、可靠性、功能需求、团队熟悉度。

**2. 消息可靠投递**
生产者确认：
- 同步确认：Broker收到消息后确认
- 异步确认：回调通知

消费者确认：
- 手动ACK：处理成功后确认
- 自动ACK：默认方式，可能丢消息

**3. 消息顺序性**
- 全局顺序：一个队列一个消费者
- 局部顺序：同一业务KEY放入同一队列

**4. 消息幂等**
消费者可能重复消费，需要幂等处理：
- 业务层面：唯一ID + 状态机
- 去重表：记录已处理消息

**5. 消息积压**
消费者能力不足，消息堆积：
- 增加消费者
- 临时扩容
- 消息过期/丢弃策略

**6. 延迟消息**
订单超时取消、延迟处理：
- RabbitMQ：TTL + 死信队列
- RocketMQ：延迟消息
- Redis：延迟队列

---

### 场景19：如何设计一个秒杀系统？

**口述回答：**

面试官您好，秒杀是电商最典型的场景，我来系统讲解。

**1. 整体架构**
```
CDN → 负载均衡 → 限流 → 消息队列 → 异步处理 → 数据库
```

**2. 库存预扣减**
秒杀开始前，库存加载到Redis：
```lua
local stock = redis.call('GET', 'seckill:stock')
if stock > 0 then
    return redis.call('DECR', 'seckill:stock')
else
    return -1
end
```
使用Lua保证原子性。

**3. 限流**
多层次限流：
- 验证码过滤
- 限流（令牌桶/漏桶）
- 排队机制

**4. 削峰填谷**
请求先进入消息队列：
```go
// 生产者
mq.Send("seckill:order", orderInfo)

// 消费者
for {
    msg := mq.Consume("seckill:order")
    // 扣库存、下单
}
```

**5. 订单隔离**
秒杀订单单独数据库分片，避免影响普通订单。

**6. 防刷**
- 限IP
- 限用户
- 设备指纹
- 行为检测

**7. 超时处理**
未支付订单超时取消，释放库存。使用延迟队列。

**8. 监控**
秒杀的各项指标需要实时监控：QPS、响应时间、库存、订单量。

---

### 场景20：如何实现系统监控与告警？

**口述回答：**

面试官您好，系统监控是保障服务稳定的关键，我来介绍完整方案。

**1. 监控指标**
- 基础设施：CPU、内存、磁盘、网络
- 应用：QPS、响应时间、错误率
- 业务：订单量、转化率、活跃用户

**2. 采集方案**
- Metrics：Prometheus + Grafana
- Logging：ELK / Loki
- Tracing：Jaeger / SkyWalking
- Profiling：Arthas

**3. 告警规则**
- 阈值告警：CPU > 80%
- 同比/环比：流量下降30%
- 异常检测：异常值自动识别

**4. 告警通道**
- 邮件
- 短信
- 电话（严重告警）
- 即时通讯（钉钉/飞书/Slack）

**5. 告警收敛**
避免告警风暴：
- 分组：相同问题只发一条
- 抑制：严重告警抑制次要告警
- 静默：维护窗口期

**6. 根因分析**
告警触发后，需要快速定位问题：
- 关联日志
- 调用链追踪
- 异常堆栈

---

### 场景21：如何设计一个评论系统？

**口述回答：**

面试官您好，评论系统是社交平台的核心功能，我来介绍实现方案。

**1. 评论存储**
评论需要支持分页、按时间排序。存储方案：
- MySQL：按文章ID分表
- MongoDB：天然支持嵌套评论
- ES：支持全文检索

**2. 嵌套评论**
- 方案一：父评论ID关联，递归查询
- 方案二：Path路径存储，如"0-1-5"，查询快但更新麻烦

**3. 分页**
评论分页不能用简单offset，因为数据可能变化。使用：
- 记录上一页最后一条的ID
- 游标分页，返回cursor

**4. 点赞**
和微博点赞类似，使用Redis ZSet。

**5. 敏感词过滤**
- 本地词典：AC自动机，性能好
- 第三方服务：阿里云、网易易盾

**6. 评论审核**
- 先发后审：机器审核 + 人工抽审
- 先审后发：审核通过才展示

---

### 场景22：如何实现一个反垃圾系统？

**口述回答：**

面试官您好，内容安全是平台的生命线，我来介绍反垃圾方案。

**1. 多层防御**
```
客户端 → 网关 → 业务服务 → 消息队列 → 审核服务 → 数据库
```

**2. 文本审核**
- 关键词过滤：敏感词库匹配
- 语义分析：AI模型判断意图
- 规则引擎：组合规则判断

**3. 图片审核**
- 图像指纹：MD5去重
- 色情检测：AI识别
- OCR识别：文字提取后文本审核

**4. 用户画像**
- 新用户：严格审核
- 老用户：信用分高放行
- 异常用户：人工复审

**5. 实时与离线**
- 实时：轻量级规则，快速拦截
- 离线：AI模型，批量处理

**6. 反馈学习**
用户举报 → 模型迭代 → 效果提升

---

### 场景23：如何实现数据同步？

**口述回答：**

面试官您好，数据同步是架构中的常见需求，我来介绍方案。

**1. 同步策略**
- 实时同步：CDC、消息队列
- 准实时：定时任务
- 批量：ETL

**2. Binlog同步**
MySQL Binlog → 解析 → 目标存储：
- Canal：阿里开源
- Debezium：RedHat开源

**3. 双写方案**
写操作同时写两个数据源：
- 先写MySQL，成功后写Redis
- 失败重试或补偿

**4. 异步方案**
- 写MySQL → 消息队列 → 消费 → 写Redis

**5. 数据一致性**
- 最终一致性：允许短暂不一致
- 强一致性：分布式事务（Seata）

**6. 注意事项**
- 顺序性：需要保证
- 幂等性：可能重复消费
- 延迟监控：同步延迟告警

---

### 场景24：如何设计一个长链接推送系统？

**口述回答：**

面试官您好，实时推送在很多场景需要，我来介绍实现方案。

**1. WebSocket**
最常用的方案：
```javascript
// 前端
const ws = new WebSocket('ws://api.example.com/push');
ws.onmessage = (event) => { console.log(event.data); };
```

后端可以使用Gorilla WebSocket或Melody框架。

**2. 消息路由**
- 用户ID → 连接映射
- 消息路由到对应连接

**3. 心跳保活**
WebSocket长时间空闲会断开，需要心跳：
```javascript
// 客户端每30秒发送心跳
setInterval(() => ws.send('ping'), 30000);
```

**4. 消息存储**
离线消息需要存储：
- Redis List：临时存储
- MySQL：持久化

**5. 百万连接**
单机无法支撑，需要：
- 消息网关：Kafka
- 消费者：多节点
- 路由：维护用户与节点映射

**6. 其他方案**
- SSE：Server-Sent Events，单向
- MQTT：物联网场景
- gRPC流：内部服务

---

### 场景25：如何实现一个APP推送系统？

**口述回答：**

面试官您好，APP推送是运营的重要手段，我来介绍完整方案。

**1. 推送通道**
- 厂商通道：小米、华为、OPPO、VIVO、苹果APNs
- 第三方：极光、个推
- 自建：WebSocket长连接

**2. 推送流程**
```
运营后台 → 推送服务 → 厂商/长连接 → APP
```

**3. 消息类型**
- 通知栏消息：系统通道
- 应用内消息：长连接
- 透传消息：APP处理

**4. 离线消息**
用户不在线时：
- 存储到离线库
- 用户上线后推送
- 设置过期时间

**5. 送达率优化**
- 定期清理无效Token
- 厂商通道配置
- 多通道互补

**6. 点击跳转**
消息带自定义数据，APP点击后跳转对应页面。

---

## 五、总结

本文档详细分析了hmdp-microservices项目中五大核心功能的实现：

1. **达人探店**：使用Redis ZSet实现点赞和Feed流
2. **好友关注**：使用Redis Set实现关注关系和共同关注
3. **附近商户**：应使用Redis GEO实现（当前未完成）
4. **用户签到**：使用Redis BitMap实现
5. **UV统计**：应使用Redis HyperLogLog实现（当前未完成）

这些功能涵盖了Redis的多种数据结构：String、Hash、List、Set、ZSet、BitMap、GEO、HyperLogLog，是非常好的Redis实践案例。

面试过程中，这些功能的实现细节、Redis数据结构选择、性能优化、场景设计方案都是高频考点。建议读者深入理解每个功能的实现原理，并能够举一反三应用到其他场景中。