<template>
  <div class="shop-detail-page">
    <!-- 头部图片 -->
    <div class="shop-header">
      <img v-if="shop.images" :src="shop.images.split(',')[0]" alt="">
      <img v-else src="https://via.placeholder.com/400x200" alt="">
      <div class="back-btn" @click="goBack">
        <van-icon name="arrow-left" />
      </div>
    </div>

    <!-- 商铺信息 -->
    <div class="shop-info-card">
      <h2 class="shop-name">{{ shop.name }}</h2>
      <div class="shop-meta">
        <van-rate v-model="shop.score" readonly :count="5" size="16" />
        <span class="score">{{ shop.score }}分</span>
        <span class="sales">月售{{ shop.sales || 0 }}</span>
      </div>
      <div class="shop-address">
        <van-icon name="location-o" />
        <span>{{ shop.address }}</span>
      </div>
      <div class="shop-open-time" v-if="shop.openHours">
        <van-icon name="clock-o" />
        <span>{{ shop.openHours }}</span>
      </div>
    </div>

    <!-- 优惠券 -->
    <div class="voucher-section" v-if="vouchers.length > 0">
      <div class="section-title">优惠活动</div>
      <div class="voucher-list">
        <div v-for="voucher in vouchers" :key="voucher.id" class="voucher-item">
          <div class="voucher-left">
            <span class="voucher-value">{{ voucher.payValue || voucher.discount }}</span>
            <span class="voucher-type">
              {{ voucher.type === 1 ? '满减券' : '折扣券' }}
            </span>
          </div>
          <div class="voucher-right">
            <van-button
              v-if="voucher.isSkill"
              size="small"
              type="danger"
              :disabled="voucher.stock <= 0"
              @click="handleSeckill(voucher.id)"
            >
              {{ voucher.stock > 0 ? '立即抢购' : '已抢光' }}
            </van-button>
            <van-button
              v-else
              size="small"
              type="primary"
              @click="handleClaim(voucher.id)"
            >
              立即领取
            </van-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 店铺评价 -->
    <div class="comment-section" v-if="comments.length > 0">
      <div class="section-title">用户评价</div>
      <div class="comment-list">
        <div v-for="comment in comments" :key="comment.id" class="comment-item">
          <div class="comment-header">
            <div class="comment-avatar">
              <img v-if="comment.userIcon" :src="comment.userIcon" alt="">
              <img v-else src="https://via.placeholder.com/40" alt="">
            </div>
            <div class="comment-user">
              <div class="user-name">{{ comment.userName }}</div>
              <div class="comment-time">{{ formatTime(comment.createTime) }}</div>
            </div>
          </div>
          <div class="comment-content">{{ comment.content }}</div>
        </div>
      </div>
    </div>

    <!-- 底部导航 -->
    <van-tabbar v-model="activeTab" route>
      <van-tabbar-item to="/" icon="home-o">首页</van-tabbar-item>
      <van-tabbar-item to="/shop" icon="shop-o">商铺</van-tabbar-item>
      <van-tabbar-item to="/blog" icon="chat-o">博客</van-tabbar-item>
      <van-tabbar-item to="/user" icon="user-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast, showFailToast } from 'vant'
import { useShopStore } from '@/store/shop'
import { useUserStore } from '@/store/user'
import dayjs from 'dayjs'

const router = useRouter()
const route = useRoute()
const shopStore = useShopStore()
const userStore = useUserStore()

const activeTab = ref(1)
const shop = ref({})
const vouchers = ref([])
const comments = ref([])
const loading = ref(false)

onMounted(async () => {
  const shopId = parseInt(route.params.id)
  loading.value = true

  try {
    // 获取商铺详情
    const shopData = await shopStore.fetchShopDetail(shopId)
    if (shopData) {
      shop.value = shopData
    }

    // 获取优惠券
    const voucherData = await shopStore.fetchVouchers(shopId)
    vouchers.value = voucherData || []
  } finally {
    loading.value = false
  }
})

// 格式化时间
function formatTime(time) {
  if (!time) return ''
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

// 返回
function goBack() {
  router.back()
}

// 秒杀抢购
async function handleSeckill(voucherId) {
  if (!userStore.isLogin) {
    router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }

  try {
    const orderId = await shopStore.seckillVoucher(voucherId)
    showToast('抢购成功')
  } catch (error) {
    showFailToast(error.message || '抢购失败')
  }
}

// 领取优惠券
function handleClaim(voucherId) {
  if (!userStore.isLogin) {
    router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  showToast('领取成功')
}
</script>

<style lang="scss" scoped>
.shop-detail-page {
  min-height: 100vh;
  background: #f5f5f5;
  padding-bottom: 50px;
}

.shop-header {
  position: relative;
  height: 200px;
  overflow: hidden;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .back-btn {
    position: absolute;
    top: 16px;
    left: 16px;
    width: 36px;
    height: 36px;
    background: rgba(0, 0, 0, 0.5);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    cursor: pointer;
  }
}

.shop-info-card {
  background: #fff;
  padding: 16px;
  margin-bottom: 12px;
}

.shop-name {
  font-size: 20px;
  font-weight: 600;
  color: #333;
  margin-bottom: 12px;
}

.shop-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;

  .score {
    color: #ff6b00;
    font-size: 14px;
  }

  .sales {
    color: #999;
    font-size: 12px;
  }
}

.shop-address,
.shop-open-time {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #666;
  font-size: 14px;
  margin-top: 8px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  padding: 16px;
  background: #fff;
}

.voucher-section {
  background: #f5f5f5;
}

.voucher-list {
  background: #fff;
}

.voucher-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid #f5f5f5;
}

.voucher-left {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.voucher-value {
  font-size: 24px;
  color: #ee0a24;
  font-weight: 600;
}

.voucher-type {
  font-size: 12px;
  color: #999;
}

.comment-section {
  background: #f5f5f5;
}

.comment-list {
  background: #fff;
}

.comment-item {
  padding: 16px;
  border-bottom: 1px solid #f5f5f5;
}

.comment-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.comment-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  overflow: hidden;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.comment-user {
  flex: 1;
}

.user-name {
  font-size: 14px;
  color: #333;
}

.comment-time {
  font-size: 12px;
  color: #999;
}

.comment-content {
  font-size: 14px;
  color: #666;
  line-height: 1.6;
}
</style>