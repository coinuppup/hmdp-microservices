<template>
  <div class="voucher-list-page">
    <!-- 顶部 -->
    <van-sticky>
      <div class="header">
        <van-icon name="arrow-left" @click="goBack" />
        <span class="title">优惠券</span>
      </div>
    </van-sticky>

    <!-- 优惠券列表 -->
    <div class="voucher-list">
      <div v-for="voucher in vouchers" :key="voucher.id" class="voucher-item">
        <div class="voucher-left">
          <div class="voucher-value">
            <span class="unit">¥</span>
            <span class="amount">{{ voucher.payValue || voucher.discount }}</span>
          </div>
          <div class="voucher-condition">
            {{ voucher.type === 1 ? `满${voucher.minPayValue}可用` : '无门槛' }}
          </div>
        </div>
        <div class="voucher-right">
          <div class="voucher-name">{{ voucher.title }}</div>
          <div class="voucher-desc">{{ voucher.desc }}</div>
          <div class="voucher-stock" v-if="voucher.isSkill">
            剩余 {{ voucher.stock }}
          </div>
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

      <van-loading v-if="loading" class="loading" type="spinner" />
      <van-empty v-if="!loading && vouchers.length === 0" description="暂无可用优惠券" />
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
import { showToast, showFailToast } from 'vant'
import { useShopStore } from '@/store/shop'
import { useUserStore } from '@/store/user'

const router = useRouter()
const shopStore = useShopStore()
const userStore = useUserStore()

const activeTab = ref(1)
const vouchers = ref([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    // 获取优惠券列表（使用默认店铺ID 1）
    const data = await shopStore.fetchVouchers(1)
    vouchers.value = data || []
  } finally {
    loading.value = false
  }
})

// 返回
function goBack() {
  router.back()
}

// 秒杀抢购
async function handleSeckill(voucherId) {
  if (!userStore.isLogin) {
    router.push('/login')
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
    router.push('/login')
    return
  }
  showToast('领取成功')
}
</script>

<style lang="scss" scoped>
.voucher-list-page {
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

.voucher-list {
  padding: 16px;
}

.voucher-item {
  display: flex;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 12px;
}

.voucher-left {
  width: 100px;
  background: linear-gradient(135deg, #ff6b00 0%, #ff8f00 100%);
  color: #fff;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.voucher-value {
  display: flex;
  align-items: baseline;

  .unit {
    font-size: 14px;
  }

  .amount {
    font-size: 28px;
    font-weight: 600;
  }
}

.voucher-condition {
  font-size: 12px;
  margin-top: 4px;
  opacity: 0.8;
}

.voucher-right {
  flex: 1;
  padding: 16px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.voucher-name {
  font-size: 15px;
  font-weight: 600;
  color: #333;
  margin-bottom: 4px;
}

.voucher-desc {
  font-size: 12px;
  color: #999;
  margin-bottom: 8px;
}

.voucher-stock {
  font-size: 12px;
  color: #ff6b00;
  margin-bottom: 8px;
}

.loading {
  padding: 40px 0;
  text-align: center;
}
</style>