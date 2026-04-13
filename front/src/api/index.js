import request from './request'
import { BASE_URL } from './constants'

// 用户相关API
export const userAPI = {
  // 发送验证码
  sendCode(phone) {
    return request.post(`${BASE_URL.user}/user/code`, { phone })
  },

  // 登录
  login(phone, code) {
    return request.post(`${BASE_URL.user}/user/login`, { phone, code })
  },

  // 刷新Token
  refreshToken() {
    return request.post(`${BASE_URL.user}/user/refresh`)
  },

  // 验证Token
  validateToken() {
    return request.post(`${BASE_URL.user}/user/validate`)
  },

  // 获取当前用户信息
  getCurrentUser() {
    return request.get(`${BASE_URL.user}/user/me`)
  },

  // 获取用户信息
  getUserInfo(userId) {
    return request.get(`${BASE_URL.user}/user/info/${userId}`)
  },

  // 签到
  sign() {
    return request.post(`${BASE_URL.user}/user/sign`)
  },

  // 获取签到次数
  getSignCount() {
    return request.get(`${BASE_URL.user}/user/sign/count`)
  }
}

// 商铺相关API
export const shopAPI = {
  // 获取商铺类型列表
  getShopTypes() {
    return request.get(`${BASE_URL.shop}/shop-type/list`)
  },

  // 获取商铺列表
  getShopList(typeId, current) {
    return request.get(`${BASE_URL.shop}/shop/list`, { typeId, current })
  },

  // 获取商铺详情
  getShopDetail(id) {
    return request.get(`${BASE_URL.shop}/shop/${id}`)
  },

  // 创建商铺
  createShop(shop) {
    return request.post(`${BASE_URL.shop}/shop`, shop)
  },

  // 更新商铺
  updateShop(shop) {
    return request.put(`${BASE_URL.shop}/shop`, shop)
  },

  // 删除商铺
  deleteShop(id) {
    return request.delete(`${BASE_URL.shop}/shop/${id}`)
  }
}

// 优惠券相关API
export const voucherAPI = {
  // 获取优惠券列表
  getVoucherList(shopId) {
    return request.get(`${BASE_URL.shop}/voucher/list`, { shopId })
  },

  // 创建优惠券
  createVoucher(voucher) {
    return request.post(`${BASE_URL.shop}/voucher`, voucher)
  },

  // 创建秒杀优惠券
  createSeckillVoucher(voucher, stock) {
    return request.post(`${BASE_URL.shop}/voucher/seckill`, { voucher, stock })
  },

  // 秒杀下单
  seckillVoucher(voucherId) {
    return request.post(`${BASE_URL.shop}/voucher-order/seckill/${voucherId}`)
  },

  // 获取订单列表
  getOrderList(current) {
    return request.get(`${BASE_URL.shop}/voucher-order/list`, { current })
  }
}

// 博客相关API
export const blogAPI = {
  // 获取热门博客
  getHotBlogs(current = 1, size = 10) {
    return request.get(`${BASE_URL.content}/api/blog/hot`, { current, size })
  },

  // 获取用户博客
  getUserBlogs(userId, current = 1, size = 10) {
    return request.get(`${BASE_URL.content}/api/blog/user`, { userId, current, size })
  },

  // 获取关注 feed
  getFollowBlogs(current = 1, size = 10) {
    return request.get(`${BASE_URL.content}/api/blog/follow`, { current, size })
  },

  // 获取博客详情
  getBlogDetail(id) {
    return request.get(`${BASE_URL.content}/api/blog/${id}`)
  },

  // 点赞博客
  likeBlog(blogId) {
    return request.post(`${BASE_URL.content}/api/blog/like`, null, { blogId })
  },

  // 取消点赞
  unlikeBlog(blogId) {
    return request.post(`${BASE_URL.content}/api/blog/unlike`, null, { blogId })
  },

  // 发布博客
  createBlog(blog) {
    return request.post(`${BASE_URL.content}/api/blog`, blog)
  },

  // 获取博客评论
  getBlogComments(blogId, current = 1, size = 10) {
    return request.get(`${BASE_URL.content}/api/blog/${blogId}/comments`, { current, size })
  },

  // 发表评论
  createComment(blogId, content) {
    return request.post(`${BASE_URL.content}/api/blog/${blogId}/comments`, { content })
  }
}

// 关注相关API
export const followAPI = {
  // 关注用户
  followUser(followUserId, isFollow = true) {
    return request.post(`${BASE_URL.content}/api/follow/user`, { followUserId, isFollow })
  },

  // 获取粉丝列表
  getFollowers(userId, current = 1, size = 10) {
    return request.get(`${BASE_URL.content}/api/follow/followers`, { userId, current, size })
  },

  // 获取关注列表
  getFollowings(userId, current = 1, size = 10) {
    return request.get(`${BASE_URL.content}/api/follow/followings`, { userId, current, size })
  },

  // 获取共同关注
  getCommonFollows(targetUserId) {
    return request.get(`${BASE_URL.content}/api/follow/common`, { targetUserId })
  },

  // 检查是否关注
  checkFollow(targetUserId) {
    return request.get(`${BASE_URL.content}/api/follow/check`, { targetUserId })
  }
}