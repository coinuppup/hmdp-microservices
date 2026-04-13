<template>
  <div class="order-list-page">
    <!-- 顶部 -->
    <van-sticky>
      <div class="header">
        <van-icon name="arrow-left" @click="goBack" />
        <span class="title">我的订单</span>
      </div>
    </van-sticky>

    <!-- 订单列表 -->
    <div class="order-list">
      <div v-for="order in orders" :key="order.id" class="order-item">
        <div class="order-header">
          <span class="order-no">订单号: {{ order.id }}</span>
          <span class="order-status">{{ getStatusText(order.status) }}</span>
        </div>
        <div class="order-content">
          <div class="order-voucher-info">
            <div class="voucher-name">{{ order.voucher?.title || '优惠券' }}</div>
            <div class="voucher-value">
              {{ order.voucher?.payValue ? `¥${order.voucher.payValue}` : '折扣券' }}
            </div>
          </div>
          <div class="order-price">
            实付: <span>¥{{ order.payPrice || 0 }}</span>
          </div>
        </div>
        <div class="order-footer">
          <span class="order-time">{{ formatTime(order.createTime) }}</span>
        </div>
      </div>

      <van-loading v-if="loading" class="loading" type="spinner" />
      <van-empty v-if="!loading && orders.length === 0" description="暂无订单" />
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
import { useRouter } from 'vue-router'
import { useShopStore } from '@/store/shop'
import { useUserStore } from '@/store/user'
import dayjs from 'dayjs'

const router = useRouter()
const shopStore = useShopStore()
const userStore = useUserStore()

const activeTab = ref(1)
const orders = ref([])
const loading = ref(false)

onMounted(async () => {
  if (!userStore.isLogin) {
    router.push('/login')
    return
  }

  loading.value = true
  try {
    const data = await shopStore.fetchOrders()
    orders.value = data || []
  } finally {
    loading.value = false
  }
})

// 返回
function goBack() {
  router.back()
}

// 获取状态文本
function getStatusText(status) {
  const statusMap = {
    1: '待支付',
    2: '已支付',
    3: '已使用',
    4: '已取消'
  }
  return statusMap[status] || '未知'
}

// 格式化时间
function formatTime(time) {
  if (!time) return ''
  return dayjs(time).format('YYYY-MM-DD HH:mm:ss')
}
</script>

<style lang="scss" scoped>
.order-list-page {
  min-height: 100vh;
  background: #f5f5f5;
  padding-bottom: 50px;
}

.header {
  display: flex;
  align-items: center;
  padding: 16px;
  background: #fff;

  .van-icon {
    font-size: 20px;
  }

  .title {
    flex: 1;
    text-align: center;
    font-size: 16px;
    font-weight: 600;
    margin-right: 24px;
  }
}

.order-list {
  padding: 16px;
}

.order-item {
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
}

.order-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid #f5f5f5;
}

.order-no {
  font-size: 12px;
  color: #999;
}

.order-status {
  font-size: 14px;
  color: #07c160;
}

.order-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.order-voucher-info {
  flex: 1;
}

.voucher-name {
  font-size: 15px;
  color: #333;
  font-weight: 500;
  margin-bottom: 4px;
}

.voucher-value {
  font-size: 14px;
  color: #ff6b00;
}

.order-price {
  font-size: 14px;
  color: #666;

  span {
    color: #333;
    font-weight: 600;
    font-size: 16px;
  }
}

.order-footer {
  text-align: right;
}

.order-time {
  font-size: 12px;
  color: #999;
}

.loading {
  padding: 40px 0;
  text-align: center;
}
</style>