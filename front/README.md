# 黑马点评前端

## 项目介绍

基于 Vue 3 + Vant 4 开发的黑马点评移动端前端应用。

## 技术栈

- Vue 3
- Vant 4 (UI 组件库)
- Pinia (状态管理)
- Vue Router (路由)
- Axios (HTTP 请求)
- Vite (构建工具)
- SCSS (样式)

## 项目结构

```
front/
├── src/
│   ├── api/          # API 接口
│   ├── assets/       # 静态资源
│   ├── router/       # 路由配置
│   ├── store/        # 状态管理
│   ├── views/        # 页面组件
│   ├── main.js       # 入口文件
│   └── App.vue       # 根组件
├── public/           # 公共资源
├── index.html        # HTML 模板
├── vite.config.js    # Vite 配置
└── package.json      # 项目依赖
```

## 快速开始

### 安装依赖

```bash
npm install
```

### 启动开发服务器

```bash
# Windows
start.bat

# 或手动执行
npm run dev
```

然后访问 http://localhost:5173

### 构建生产版本

```bash
npm run build
```

## 功能清单

- [x] 用户登录（手机号 + 验证码）
- [x] 用户签到
- [x] 商铺列表（分类筛选）
- [x] 商铺详情
- [x] 优惠券列表
- [x] 秒杀抢购
- [x] 热门博客
- [x] 关注动态
- [x] 发布博客
- [x] 博客点赞/评论
- [x] 用户关注
- [x] 订单列表
- [x] 个人中心

## API 对应

| 服务 | 端口 | 说明 |
|------|------|------|
| 用户服务 | 8081 | 登录、验证码、签到 |
| 商铺服务 | 8082 | 商铺、优惠券、订单 |
| 内容服务 | 8083 | 博客、关注、评论 |

## 注意事项

1. 确保后端服务已启动
2. 开发模式下直接请求后端服务，无需代理
3. 如需修改 API 地址，修改 `src/api/constants.js`