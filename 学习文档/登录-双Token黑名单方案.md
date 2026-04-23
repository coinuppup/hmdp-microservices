# 双Token认证方案设计（JWT + RefreshToken + 黑名单）

## 背景与设计思路

其实最开始的时候，我主要是在时主要是在 **Session + Redis** 和 **JWT** 之间选择。

Session 方案很成熟，但它是**有状态**的。在分布式架构下，如果请求分别打到不同的服务器，就会面临 Session 漂移的问题。为了解决这个问题，必须引入 Redis 做外部存储来实现 Session 共享，这不仅增加了一层网络 IO 依赖，也增加了系统的耦合度。

相比之下，**JWT 是无状态的**。任何一台应用服务器只要持有公钥（或密钥），就可以独立完成令牌的验签和解码，完全解耦了认证服务和业务服务，非常适合微服务和分布式扩展。

但是，我们在实践中发现，如果直接使用单一的 JWT 方案，会陷入一个两难境地：

- **过期时间设置太短**（比如 30 分钟）：用户体验很差，稍微用一会儿就得重新登录。
- **过期时间设置太长**（比如 7 天）：一旦 Token 泄露，攻击者就能长期冒充用户，且纯 JWT 的无状态特性导致我们无法主动让 Token 失效（即无法踢人下线）。

在这种背景下，

- **第一是 Access Token (AT)：**
  - 这是用户的‘临时通行证’，有效期设置得很短，比如 **15 分钟**。
  - 它采用**纯 JWT** 格式，只包含用户 ID、权限等非敏感信息。
  - 每次接口请求都携带它，服务端通过验签即可快速放行，**不需要查询数据库或 Redis**，保证了高频业务的高性能。
- **Refresh Token (RT)**，
  - 这是用户的‘长期凭证’，有效期较长，比如 **7 天**。
  - 它**不直接访问业务接口**，唯一的用途就是用来换取新的 Access Token。用户登录成功后，服务端签发一对AT和RT。

1. 客户端将AT放在内存或`sessionStorage`中，将RT放在`HttpOnly`的Cookie中（防止XSS攻击）。
2. 前端通过拦截器自动为请求头添加 `Authorization: Bearer <access_token>`。
3. 当AT过期，前端捕获到401错误后，会自动调用刷新接口，用RT去换取新的一对AT和RT，整个过程对用户**无感知**。

最后，针对需要主动让用户下线的问题（如用户退出、修改密码、管理员强制下线），我们引入了 **Redis 黑名单**机制。

**核心逻辑：黑名单存储的是 Refresh Token，而不是 Access Token。**

原因：
1. AT 泄露没关系 —— 因为它很快会过期（15分钟），攻击窗口期很短
2. RT 泄露也没关系 —— 因为使用 RT 刷新需要客户端密钥
3. **关键是**：当用户退出时，我们要禁止 RT 刷新新 AT，这样用户就无法继续登录了

**具体做法：**
- 当用户退出时，将 RT 加入黑名单（或直接删除 RT）
- 这样 RT 就无法刷新新 AT 了
- AT 等它自然过期（15分钟）

Key 使用 `token:blacklist:{userId}:{deviceId}` 来实现**多设备维度的隔离**；Value 存储已吊销的 Refresh Token，支持**令牌轮换**；过期时间与 RT 有效期一致（7天）。

在设计登录认证方案之前，我分析了两种传统方案的局限性，并在此基础上设计了这套双Token + 黑名单方案。

### 一、传统Session方案的扩展性问题

传统的Session方案在单机环境下运行良好，但在分布式高并发场景下存在严重的扩展性问题：

```
┌─────────────────────────────────────────────────────────────────┐
│                 Session方案的分布式困境                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  单机Session架构：                                              │
│  ┌──────────┐     ┌──────────┐                                │
│  │  用户    │────▶│  Server  │────▶│ Session│                │
│  └──────────┘     └──────────┘     └────────┘                 │
│                                                                 │
│  问题来了：                                                     │
│  当增加服务器到3台时：                                          │
│                                                                 │
│  分布式Session架构：                                             │
│  ┌──────────┐                                                  │
│  │  用户A   │──▶ Server1 ──✗──▶ Session不存在！             │
│  └──────────┘     ┌──────────┐                                │
│  ┌──────────┐     │  用户B   │──▶ Server2 ──✓──▶ Session OK  │
│  └──────────┘     └──────────┘                                │
│  ┌──────────┐     ┌──────────┐                                │
│  │  用户C   │──▶ Server3 ──✗──▶ Session不存在！             │
│  └──────────┘                                                  │
│                                                                 │
│  核心问题：Session存储在服务器内存中，不同服务器不共享           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Session方案扩展性差的三个原因：**

1. **Session无法共享**
   - 每个服务器只维护自己的Session
   - 用户请求可能被负载均衡到不同服务器
   - 结果：用户在Server1登录，Server2不认识他

2. **解决方案的代价**
   - 方案A：Session粘性（Sticky Session）—— 绑死服务器，无法真正扩展
   - 方案B：Session集中存储（Redis）—— 每次请求都要查Redis，网络开销大
   - 方案C：Session复制 —— 同步延迟高，不适合大规模

3. **性能瓶颈**
   - 每个请求都要验证Session
   - 集中式Session存储：每次请求多一次Redis网络调用
   - 高并发下，Redis成为瓶颈

---

### 二、纯JWT方案的死局

为了解决Session的扩展性问题，我首先考虑使用纯JWT方案，但很快发现陷入了两难境地：

```
┌─────────────────────────────────────────────────────────────────┐
│                     JWT方案的死局                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 场景：设置Token有效期为 1小时                           │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  问题A：Token泄露后，风险极大                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 攻击者拿到Token                                         │   │
│  │   ↓                                                     │   │
│  │ 在1小时内可以冒充用户做任何操作                         │   │
│  │   ↓                                                     │   │
│  │ 用户改密码也没用！Token仍然有效！                       │   │
│  │   ↓                                                     │   │
│  │ 只能等1小时后Token自然过期                             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 场景：设置Token有效期为 5分钟                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  问题B：用户体验极差                                           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 用户看文章看到一半                                      │   │
│  │   ↓                                                     │   │
│  │ 突然弹出：请重新登录                                   │   │
│  │   ↓                                                     │   │
│  │ 用户：这是什么垃圾体验？                               │   │
│  │   ↓                                                     │   │
│  │ 5分钟登录一次，谁能受得了？                           │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 场景：用户点击退出登录                                 │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  问题C：无法主动下线                                           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 用户在网吧登录后离开                                    │   │
│  │   ↓                                                     │   │
│  │ 点击退出                                                │   │
│  │   ↓                                                     │   │
│  │ Token仍然有效！任何人坐那台电脑都能继续操作！          │   │
│  │   ↓                                                     │   │
│  │ 因为JWT是无状态的，服务器无法撤销它！                   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**JWT死局的本质：**

| 有效期设置 | 问题 | 后果 |
|------------|------|------|
| 长（1-7天） | 泄露风险极大 | 攻击者有充足时间冒充用户 |
| 短（5-30分钟） | 体验极差 | 用户频繁掉线，无法接受 |

**为什么无法破解？**

因为JWT的**无状态特性**：
- 签发后，服务器不保存任何信息
- 验证时只需要解析Token本身
- **无法让它失效** —— 这是基因决定的

---

### 三、双Token + 黑名单方案设计

为了彻底解决上述问题，我设计了这套双Token + 黑名单方案：

```
┌─────────────────────────────────────────────────────────────────┐
│                    双Token + 黑名单 核心思路                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  核心思想：分离"高频验证"和"低频管理"                           │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Access Token（访问凭证）                                  │   │
│  │ - 有效期：短（15分钟）                                  │   │
│  │ - 用途：每次接口请求携带                               │   │
│  │ - 验证：纯JWT，不查Redis                               │   │
│  │ - 特点：性能极高，即使泄露风险仅15分钟                 │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          ↑                                     │
│                          │ 换发                                 │
│                          ↓                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Refresh Token（刷新凭证）                                │   │
│  │ - 有效期：长（7天）                                    │   │
│  │ - 用途：换发Access Token                               │   │
│  │ - 验证：Redis白名单                                    │   │
│  │ - 特点：可吊销、可管理，支持多设备                     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          ↑                                     │
│                          │ 吊销                                │
│                          ↓                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 黑名单机制                                              │   │
│  │ - 触发：登出、修改密码、异常检测                       │   │
│  │ - 效果：让指定Token立即失效                            │   │
│  │ - 实现：Redis Set存储已吊销的TokenID                   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 概述

本项目采用**基于JWT的双Token机制 + 黑名单**实现用户认证，核心设计思路：

- **Access Token**：纯JWT，用于接口鉴权，15分钟有效期
- **Refresh Token**：Redis存储，用于换发新Token，7天有效期
- **黑名单机制**：用于Token吊销，支持单设备登出、全设备登出、修改密码时吊销

---

## 一、为什么选择双Token + 黑名单方案？

### 1.1 单Token问题

```
┌─────────────────────────────────────────────────────────────────┐
│                     单Token方案的问题                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  问题1：无法主动吊销                                            │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ JWT是无状态的，签发后无法撤销                            │   │
│  │ 用户点击退出后，Token仍然有效，直到自然过期              │   │
│  │ 风险：用户在别的设备登录，原设备还能正常使用             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  问题2：无法多设备管理                                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 一个Token所有设备通用，无法区分设备                      │   │
│  │ 用户无法查看有哪些设备在线                               │   │
│  │ 无法单独踢掉某个设备                                    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  问题3：有效期设置尴尬                                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 有效期长 → 风险大（泄露后长期可用）                    │   │
│  │ 有效期短 → 体验差（频繁登录）                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 双Token + 黑名单方案优势

```
┌─────────────────────────────────────────────────────────────────┐
│                   双Token + 黑名单方案优势                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Access Token（短期凭证）：                                      │
│  - 有效期15分钟                                                │
│  - 纯JWT，无需查库，性能高                                      │
│  - 即使泄露，窗口期短                                          │
│                                                                 │
│  Refresh Token（长期凭证）：                                    │
│  - 有效期7天                                                   │
│  - 用于换发新Access Token                                       │
│  - 存储在Redis，支持吊销                                       │
│                                                                 │
│  黑名单机制：                                                   │
│  - 登出时加入黑名单                                            │
│  - 修改密码时加入黑名单                                        │
│  - 支持单设备/全设备吊销                                       │
│                                                                 │
│  效果：                                                         │
│  ✓ 安全性高：短期AccessToken + 可吊销RefreshToken              │
│  ✓ 体验好：7天无需登录                                         │
│  ✓ 可管理：支持多设备、单设备登出                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 二、整体架构设计

### 2.1 技术架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                      用户认证系统架构                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌─────────────┐                              ┌─────────────┐ │
│   │   前端      │                              │   后端服务   │ │
│   │ (浏览器)   │                              │             │ │
│   └──────┬──────┘                              └──────┬──────┘ │
│          │                                            │         │
│          │ 1. 登录请求                                │         │
│          │ POST /user/login                          │         │
│          │ {phone, code, deviceId}                  │         │
│          │─────────────────────────────────────────>│         │
│          │                                            │         │
│          │  2. 验证+生成Token                       │         │
│          │    ┌─────────────────────────────────┐   │         │
│          │    │ Access Token (JWT)              │   │         │
│          │    │ - userID, deviceID             │   │         │
│          │    │ - 签名加密                      │   │         │
│          │    │ - 15分钟有效期                  │   │         │
│          │    └─────────────────────────────────┘   │         │
│          │    ┌─────────────────────────────────┐   │         │
│          │    │ Refresh Token (Redis存储)       │   │         │
│          │    │ - 随机字符串                    │   │         │
│          │    │ - 7天有效期                    │   │         │
│          │    │ - 支持吊销                     │   │         │
│          │    └─────────────────────────────────┘   │         │
│          │                                            │         │
│          │  返回: accessToken + refreshToken         │         │
│          │<─────────────────────────────────────────│         │
│          │                                            │         │
│          │  3. 后续请求                              │         │
│          │  GET /api/xxx                            │         │
│          │  Authorization: Bearer <jwt>             │         │
│          │─────────────────────────────────────────>│         │
│          │                                            │         │
│          │  4. 验证JWT（纯内存，不查Redis）         │         │
│          │  ┌─────────────────────────────────┐   │         │
│          │  │ 1. 解析JWT                     │   │         │
│          │  │ 2. 验证签名                    │   │         │
│          │  │ 3. 检查过期时间                 │   │         │
│          │  │ 4. 检查黑名单                   │   │         │
│          │  └─────────────────────────────────┘   │         │
│          │                                            │         │
│          │  返回业务数据                            │         │
│          │<─────────────────────────────────────────│         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 核心流程

```
┌─────────────────────────────────────────────────────────────────┐
│                        三大核心流程                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  【流程1】登录流程                                              │
│  ┌──────────┐     ┌──────────────┐     ┌─────────┐          │
│  │  前端    │     │ user-service │     │  Redis  │          │
│  └────┬─────┘     └──────┬───────┘     └────┬────┘          │
│       │                    │                  │                 │
│       │ POST /user/login │                  │                 │
│       │ {phone,code,deviceId}              │                 │
│       │─────────────────>│                  │                 │
│       │                    │                  │                 │
│       │                    │ 验证码校验       │                 │
│       │                    │────────────────>│                 │
│       │                    │<────────────────│                 │
│       │                    │                  │                 │
│       │                    │ 生成双Token     │                 │
│       │                    │ - AccessToken   │                 │
│       │                    │ - RefreshToken  │                 │
│       │                    │                  │                 │
│       │                    │ 写入Redis      │                 │
│       │                    │ SET token:refresh:xxx userId:deviceId EX 604800 │
│       │                    │────────────────>│                 │
│       │                    │<────────────────│                 │
│       │                    │                  │                 │
│       │ 返回Token          │                  │                 │
│       │<─────────────────│                  │                 │
│                                                                 │
│  【流程2】接口鉴权流程                                          │
│  ┌──────────┐     ┌──────────────┐                           │
│  │  前端    │     │ content-service │                         │
│  └────┬─────┘     └──────┬───────┘                           │
│       │                    │                                     │
│       │ GET /api/xxx      │                                     │
│       │ Auth: Bearer jwt  │                                     │
│       │─────────────────>│                                     │
│       │                    │                                     │
│       │                    │ 验证JWT     │                      │
│       │                    │ 1.解析      │                      │
│       │                    │ 2.验签      │                      │
│       │                    │ 3.检查过期  │                      │
│       │                    │ 4.检查黑名单│ ← 关键步骤         │
│       │                    │                                     │
│       │                    │ 执行业务逻辑 │                     │
│       │                    │                                     │
│       │ 返回数据           │                                     │
│       │<─────────────────│                                     │
│                                                                 │
│  【流程3】Token刷新流程                                         │
│  ┌──────────┐     ┌──────────────┐     ┌─────────┐          │
│  │  前端    │     │ user-service │     │  Redis  │          │
│  └────┬─────┘     └──────┬───────┘     └────┬────┘          │
│       │                    │                  │                 │
│       │ POST /user/refresh                   │                 │
│       │ {refreshToken}  │                  │                 │
│       │─────────────────>│                  │                 │
│       │                    │                  │                 │
│       │                    │ 查Redis验证    │                 │
│       │                    │ GET token:refresh:xxx            │
│       │                    │────────────────>│                 │
│       │                    │<────────────────│                 │
│       │                    │                  │                 │
│       │                    │ 检查黑名单      │                 │
│       │                    │ EXISTS token:blacklist:userId:deviceId │
│       │                    │────────────────>│                 │
│       │                    │<────────────────│                 │
│       │                    │                  │                 │
│       │                    │ 生成新Token    │                 │
│       │                    │                  │                 │
│       │                    │ 吊销旧Token    │                 │
│       │                    │ DEL token:refresh:old             │
│       │                    │ SET token:refresh:new ...         │
│       │                    │────────────────>│                 │
│       │                    │<────────────────│                 │
│       │                    │                  │                 │
│       │ 返回新Token        │                  │                 │
│       │<─────────────────│                  │                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 三、Redis存储设计

### 3.1 存储结构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Redis存储结构                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  【1】Refresh Token存储                                          │
│  Key:    token:refresh:<tokenId>                               │
│  Type:   String                                                │
│  Value:  userId:deviceId                                       │
│  TTL:    7天 (604800秒)                                        │
│                                                                 │
│  示例：                                                          │
│  > SET token:refresh:abc123def456 "12345:iphone15" EX 604800  │
│  > GET token:refresh:abc123def456                             │
│  "12345:iphone15"                                              │
│                                                                 │
│  ─────────────────────────────────────────────────────────     │
│                                                                 │
│  【2】Token黑名单（存的是Refresh Token）                         │
│  Key:    token:blacklist:<userId>:<deviceId>                  │
│  Type:   Set                                                   │
│  Members: 已吊销的Refresh Token列表                            │
│  TTL:    7天（与RefreshToken一致）                           │
│                                                                 │
│  示例：                                                          │
│  > SADD token:blacklist:12345:iphone15 refreshTokenId1 refreshTokenId2│
│  > SISMEMBER token:blacklist:12345:iphone15 refreshTokenId1│
│  (integer) 1  ← 在黑名单中，已吊销                            │
│                                                                 │
│  说明：                                                          │
│  - 黑名单存的是RT，不是AT                                        │
│  - 退出时将RT加入黑名单，RT就无法刷新新AT                       │
│  - AT等它自然过期（15分钟）                                     │
│                                                                 │
│  ─────────────────────────────────────────────────────────     │
│                                                                 │
│  【3】用户设备列表                                              │
│  Key:    user:devices:<userId>                                │
│  Type:   Set                                                   │
│  Members: deviceId列表                                          │
│  TTL:    7天                                                   │
│                                                                 │
│  示例：                                                          │
│  > SADD user:devices:12345 iphone15 huawei-p50 xiaomi13      │
│  > SMEMBERS user:devices:12345                                │
│  1) "iphone15"                                                 │
│  2) "huawei-p50"                                               │
│  3) "xiaomi13"                                                 │
│                                                                 │
│  ─────────────────────────────────────────────────────────     │
│                                                                 │
│  【4】验证码存储                                                 │
│  Key:    login:code:<phone>                                    │
│  Type:   String                                                │
│  Value:  6位验证码                                              │
│  TTL:    5分钟                                                 │
│                                                                 │
│  示例：                                                          │
│  > SET login:code:13800138000 123456 EX 300                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 四、核心代码实现

### 4.1 Token生成

```go
// user-service/utils/token.go

const (
    AccessTokenTTL  = 15 * time.Minute   // Access Token有效期
    RefreshTokenTTL = 7 * 24 * time.Hour // Refresh Token有效期
    TokenSecret     = "your-secret-key"  // JWT签名密钥
)

// Token结构体
type TokenPair struct {
    AccessToken  string `json:"accessToken"`
    RefreshToken string `json:"refreshToken"`
    ExpiresIn   int    `json:"expiresIn"` // 秒
}

// 生成双Token
func GenerateTokenPair(userID int64, deviceID string) (*TokenPair, error) {
    // 1. 生成随机TokenId
    tokenID := generateRandomString(32)

    // 2. 生成Access Token (JWT)
    accessToken, err := generateJWT(userID, deviceID, tokenID, AccessTokenTTL)
    if err != nil {
        return nil, err
    }

    // 3. 生成Refresh Token
    refreshToken := generateRandomString(64)

    // 4. 写入Redis
    ctx := context.Background()
    pipe := rdb.Pipeline()

    // 存储Refresh Token: token:refresh:<tokenId> -> userId:deviceId
    refreshKey := fmt.Sprintf("token:refresh:%s", refreshToken)
    pipe.Set(ctx, refreshKey, fmt.Sprintf("%d:%s", userID, deviceID), RefreshTokenTTL)

    // 记录用户设备列表
    devicesKey := fmt.Sprintf("user:devices:%d", userID)
    pipe.SAdd(ctx, devicesKey, deviceID)
    pipe.Expire(ctx, devicesKey, RefreshTokenTTL)

    _, err = pipe.Exec(ctx)
    if err != nil {
        return nil, err
    }

    return &TokenPair{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    int(AccessTokenTTL.Seconds()),
    }, nil
}

// 生成JWT
func generateJWT(userID int64, deviceID string, tokenID string, ttl time.Duration) (string, error) {
    now := time.Now()
    expire := now.Add(ttl)

    claims := jwt.MapClaims{
        "userId":   userID,
        "deviceId": deviceID,
        "tokenId":  tokenID,
        "iat":      now.Unix(),
        "exp":      expire.Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(TokenSecret))
}
```

### 4.2 Access Token验证（不检查黑名单）

```go
// 验证Access Token
// 注意：AT验证不检查黑名单！因为AT很快会过期（15分钟）
// 即使泄露，攻击窗口期也很短
func ValidateAccessToken(tokenString string) (*AccessTokenInfo, error) {
    if tokenString == "" {
        return nil, errors.New("token不能为空")
    }

    // 1. 解析JWT（纯内存，不查Redis）
    token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(TokenSecret), nil
    })

    if err != nil {
        return nil, fmt.Errorf("token解析失败: %w", err)
    }

    if !token.Valid {
        return nil, errors.New("token无效")
    }

    // 2. 提取claims
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return nil, errors.New("token claims无效")
    }

    userID := int64(claims["userId"].(float64))
    deviceID := claims["deviceId"].(string)
    tokenID := claims["tokenId"].(string)

    // 注意：这里不检查黑名单！
    // 因为AT很快会过期，没必须每次请求都查Redis
    // 黑名单在Refresh时检查

    return &AccessTokenInfo{
        UserID:   userID,
        DeviceID: deviceID,
        TokenID:  tokenID,
    }, nil
}

// 检查Token是否在黑名单中
func isInBlacklist(userID int64, deviceID string, tokenID string) bool {
    blacklistKey := fmt.Sprintf("token:blacklist:%d:%s", userID, deviceID)
    result, err := rdb.SIsMember(context.Background(), blacklistKey, tokenID).Result()
    return err == nil && result
}
```

### 4.3 Token刷新

```go
// 刷新Token
func RefreshToken(refreshToken string) (*TokenPair, error) {
    ctx := context.Background()

    // 1. 验证Refresh Token存在
    refreshKey := fmt.Sprintf("token:refresh:%s", refreshToken)
    value, err := rdb.Get(ctx, refreshKey).Result()
    if err == redis.Nil {
        return nil, errors.New("refresh token已过期或无效")
    }
    if err != nil {
        return nil, fmt.Errorf("查询refresh token失败: %w", err)
    }

    // 2. 解析 userId:deviceId
    parts := strings.Split(value, ":")
    if len(parts) != 2 {
        return nil, errors.New("token数据格式错误")
    }
    userID, _ := strconv.ParseInt(parts[0], 10, 64)
    deviceID := parts[1]

    // 3. 检查黑名单
    // 注意：刷新时也需要检查，防止被吊销的token继续刷新
    blacklistKey := fmt.Sprintf("token:blacklist:%d:%s", userID, deviceID)
    isBlacklisted, _ := rdb.Exists(ctx, blacklistKey).Result()
    if isBlacklisted > 0 {
        return nil, errors.New("token已吊销，请重新登录")
    }

    // 4. 吊销旧的Refresh Token
    rdb.Del(ctx, refreshKey)

    // 5. 生成新的双Token
    return GenerateTokenPair(userID, deviceID)
}
```

### 4.4 吊销Token（登出）

```go
// 吊销当前设备Token
// 核心：将Refresh Token加入黑名单，禁止刷新新AT
func RevokeToken(userID int64, deviceID string, refreshToken string) error {
    ctx := context.Background()
    pipe := rdb.Pipeline()

    // 1. 将Refresh Token加入黑名单（禁止刷新新AT）
    blacklistKey := fmt.Sprintf("token:blacklist:%d:%s", userID, deviceID)
    pipe.SAdd(ctx, blacklistKey, refreshToken)
    // 黑名单有效期 = RT有效期（7天）
    pipe.Expire(ctx, blacklistKey, RefreshTokenTTL)

    // 2. 删除Refresh Token（从白名单中移除）
    refreshKey := fmt.Sprintf("token:refresh:%s", refreshToken)
    pipe.Del(ctx, refreshKey)

    // 3. 从设备列表中移除
    devicesKey := fmt.Sprintf("user:devices:%d", userID)
    pipe.SRem(ctx, devicesKey, deviceID)

    _, err := pipe.Exec(ctx)
    return err
}

// 吊销所有设备Token（修改密码、管理员强制下线时调用）
func RevokeAllTokens(userID int64) error {
    ctx := context.Background()

    // 1. 获取用户所有设备
    devicesKey := fmt.Sprintf("user:devices:%d", userID)
    devices, err := rdb.SMembers(ctx, devicesKey).Result()
    if err != nil {
        return err
    }

    // 2. 将所有设备的RT加入黑名单
    pipe := rdb.Pipeline()
    for _, deviceID := range devices {
        blacklistKey := fmt.Sprintf("token:blacklist:%d:%s", userID, deviceID)
        pipe.SAdd(ctx, blacklistKey, "all") // 标记为全部吊销
        pipe.Expire(ctx, blacklistKey, RefreshTokenTTL) // 7天
    }

    // 3. 删除所有Refresh Token
    // 实际实现需要遍历或记录映射关系

    // 4. 清空设备列表
    pipe.Del(ctx, devicesKey)

    _, err = pipe.Exec(ctx)
    return err
}
```

---

## 五、接口设计

### 5.1 登录接口

```
请求：
POST /user/login
Content-Type: application/json

{
    "phone": "13800138000",
    "code": "123456",
    "deviceId": "iphone15-pro"
}

响应：
{
    "code": 200,
    "message": "OK",
    "data": {
        "accessToken": "eyJhbGciOiJIUzI1NiIs...",
        "refreshToken": "a1b2c3d4e5f6...",
        "expiresIn": 900,
        "tokenType": "Bearer",
        "user": {
            "id": 12345,
            "phone": "13800138000",
            "nickName": "用户12345"
        }
    }
}
```

### 5.2 刷新Token接口

```
请求：
POST /user/refresh
Content-Type: application/json

{
    "refreshToken": "a1b2c3d4e5f6..."
}

响应：
{
    "code": 200,
    "message": "OK",
    "data": {
        "accessToken": "eyJhbGciOiJIUzI1NiIs...",
        "refreshToken": "new-refresh-token...",
        "expiresIn": 900
    }
}
```

### 5.3 登出接口

```
请求：
POST /user/logout
Authorization: Bearer <accessToken>

响应：
{
    "code": 200,
    "message": "OK"
}
```

### 5.4 获取用户设备列表

```
请求：
GET /user/devices
Authorization: Bearer <accessToken>

响应：
{
    "code": 200,
    "data": [
        {"deviceId": "iphone15-pro", "loginTime": "2024-01-01 10:00:00"},
        {"deviceId": "huawei-p50", "loginTime": "2024-01-02 15:30:00"}
    ]
}
```

### 5.5 吊销指定设备

```
请求：
POST /user/revoke-device
Authorization: Bearer <accessToken>
Content-Type: application/json

{
    "deviceId": "huawei-p50"
}

响应：
{
    "code": 200,
    "message": "OK"
}
```

---

## 六、面试回答模板

### 6.1 完整口述回答

> 面试官您好，我来介绍一下项目中实现的登录认证方案。
>
> 我们项目采用**双Token + 黑名单**机制实现用户认证，主要解决三个问题：
> 1. 安全性：短期AccessToken即使泄露风险也小
> 2. 体验好：7天无需登录
> 3. 可管理：支持多设备、单设备登出
>
> **核心设计：**
> - Access Token：纯JWT，15分钟有效期，用于接口鉴权，验证时只解析JWT不查Redis，性能很高
> - Refresh Token：64位随机字符串，7天有效期，存Redis白名单，用于换发新Token
> - 黑名单机制：用户登出或修改密码时，将Token加入黑名单，下次验证时检查黑名单
>
> **为什么这样设计？**
> 主要是为了平衡安全性、性能和用户体验。如果只用JWT，无法实现主动吊销（比如用户改密码后需要强制下线所有设备）；如果每次验证都查Redis，QPS会受限制。我们采用AccessToken纯JWT验证（不查库），只有刷新时才查Redis，这样既保证了高性能，又能实现Token吊销。
>
> **黑名单实现：**
> 黑名单key是 `token:blacklist:用户ID:设备ID`，value是Set结构存储已吊销的tokenId列表，过期时间与AccessToken一致（15分钟）。用户登出时把当前tokenId加入黑名单，下次验证JWT时会检查黑名单，如果存在则拒绝请求。

### 6.2 关键技术点

```
┌─────────────────────────────────────────────────────────────────┐
│                     面试必问技术点                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Q1: 为什么AccessToken验证不查Redis？                           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 答：JWT本身包含过期时间和签名，验证签名和过期时间        │   │
│  │     已经是完整的安全验证，不需要查Redis。               │   │
│  │     查Redis会增加网络延迟，高并发下是瓶颈。             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  Q2: 黑名单为什么存在？                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 答：JWT无法主动吊销，通过黑名单实现主动失效。           │   │
│  │     登出/改密码时将tokenId加入黑名单。                 │   │
│  │     验证时检查黑名单，存在则拒绝。                      │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  Q3: 黑名单的过期时间为什么是7天？                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 答：与RefreshToken有效期一致。                         │   │
│  │     因为黑名单存的是RT，不是AT。                       │   │
│  │     RT有效期7天，7天后黑名单自动清理。                 │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  Q4: 如何实现单设备登出？                                      │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 答：每个Token绑定deviceId，登出时                      │   │
│  │     将该用户的该设备token加入黑名单即可。               │   │
│  │     不影响其他设备。                                    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  Q5: 双Token相比单Token的优势？                                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 答：1. AccessToken短期，泄露风险小                      │   │
│  │     2. RefreshToken可吊销，支持主动失效                 │   │
│  │     3. 支持多设备管理                                   │   │
│  │     4. 验证性能高（纯JWT不查库）                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 七、与单Token方案对比

| 特性 | 单JWT方案 | 双Token+黑名单方案 |
|------|-----------|-------------------|
| AccessToken有效期 | 短（需频繁登录） | 短（15分钟） |
| 能否主动吊销 | 不能 | 能 |
| 能否多设备管理 | 不能 | 能 |
| 能否单设备登出 | 不能 | 能 |
| 验证性能 | 高（纯JWT） | 高（纯JWT） |
| 刷新性能 | 高 | 中等（需查Redis） |
| 实现复杂度 | 低 | 中 |

---

## 八、方案总结

```
┌─────────────────────────────────────────────────────────────────┐
│                       方案核心要点                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. 双Token设计：                                              │
│     AccessToken = JWT（15分钟，纯内存验证）                    │
│     RefreshToken = Redis存储（7天，支持吊销）                  │
│                                                                 │
│  2. 黑名单机制：                                               │
│     - 登出时加入黑名单                                         │
│     - 改密码时加入黑名单                                       │
│     - 有效期=AccessToken有效期                                 │
│                                                                 │
│  3. 多设备支持：                                               │
│     - Token绑定deviceId                                        │
│     - 支持单设备登出                                            │
│     - 支持查看设备列表                                          │
│                                                                 │
│  4. 性能优化：                                                 │
│     - AccessToken验证不查Redis                                 │
│     - 使用Pipeline批量操作                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```