<template>
  <div class="user-profile-page">
    <!-- 未登录状态 -->
    <div class="not-login" v-if="!userStore.isLogin">
      <div class="avatar-large">
        <img src="https://via.placeholder.com/80" alt="">
      </div>
      <van-button type="primary" size="large" class="login-btn" @click="goLogin">
        登录
      </van-button>
    </div>

    <!-- 已登录状态 -->
    <div class="user-info" v-else>
      <!-- 用户头部信息 -->
      <div class="user-header">
        <div class="user-avatar">
          <img v-if="userStore.userInfo?.icon" :src="userStore.userInfo.icon" alt="">
          <img v-else src="https://via.placeholder.com/80" alt="">
        </div>
        <div class="user-detail">
          <h2 class="nickname">{{ userStore.userInfo?.nickname || '用户' }}</h2>
          <p class="phone">{{ formatPhone(userStore.userInfo?.phone) }}</p>
        </div>
        <van-button size="small" type="default" @click="handleSign">
          签到
        </van-button>
      </div>

      <!-- 签到统计 -->
      <div class="sign-stats">
        <div class="stat-item">
          <span class="stat-value">{{ userStore.signCount }}</span>
          <span class="stat-label">签到天数</span>
        </div>
      </div>

      <!-- 功能菜单 -->
      <div class="menu-section">
        <van-cell-group>
          <van-cell title="我的订单" is-link to="/order" icon="orders-o" />
          <van-cell title="我的优惠券" is-link to="/voucher" icon="coupon-o" />
          <van-cell title="我的收藏" is-link icon="star-o" />
          <van-cell title="我的关注" is-link icon="friends-o" />
        </van-cell-group>
      </div>

      <!-- 其他设置 -->
      <div class="menu-section">
        <van-cell-group>
          <van-cell title="设置" is-link icon="setting-o" />
          <van-cell title="关于我们" is-link icon="info-o" />
        </van-cell-group>
      </div>

      <!-- 退出登录 -->
      <div class="logout-btn" v-if="userStore.isLogin">
        <van-button type="danger" size="large" block @click="handleLogout">
          退出登录
        </van-button>
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
import { useRouter } from 'vue-router'
import { showToast, showFailToast, Dialog } from 'vant'
import { useUserStore } from '@/store/user'

const router = useRouter()
const userStore = useUserStore()

const activeTab = ref(3)

onMounted(async () => {
  if (userStore.isLogin) {
    // 获取签到次数
    await userStore.fetchSignCount()
  }
})

// 格式化手机号
function formatPhone(phone) {
  if (!phone) return ''
  return phone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2')
}

// 去登录
function goLogin() {
  router.push({ name: 'login', query: { redirect: '/user' } })
}

// 签到
async function handleSign() {
  try {
    const success = await userStore.sign()
    if (success) {
      showToast('签到成功')
    }
  } catch (error) {
    showFailToast(error.message || '签到失败')
  }
}

// 退出登录
function handleLogout() {
  Dialog.confirm({
    title: '提示',
    message: '确定要退出登录吗？'
  }).then(() => {
    userStore.logout()
    showToast('已退出登录')
  }).catch(() => {})
}
</script>

<style lang="scss" scoped>
.user-profile-page {
  min-height: 100vh;
  background: #f5f5f5;
  padding-bottom: 50px;
}

.not-login {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 0;

  .avatar-large {
    width: 80px;
    height: 80px;
    border-radius: 50%;
    overflow: hidden;
    margin-bottom: 20px;

    img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
  }

  .login-btn {
    width: 200px;
  }
}

.user-info {
  .user-header {
    display: flex;
    align-items: center;
    padding: 24px 16px;
    background: linear-gradient(135deg, #07c160 0%, #06ad56 100%);
    color: #fff;
  }

  .user-avatar {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    overflow: hidden;
    border: 2px solid rgba(255, 255, 255, 0.3);

    img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
  }

  .user-detail {
    flex: 1;
    margin-left: 16px;

    .nickname {
      font-size: 18px;
      font-weight: 600;
      margin-bottom: 4px;
    }

    .phone {
      font-size: 14px;
      opacity: 0.8;
    }
  }
}

.sign-stats {
  display: flex;
  justify-content: center;
  gap: 40px;
  padding: 20px;
  background: #fff;
  margin-bottom: 12px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #07c160;
}

.stat-label {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.menu-section {
  margin-bottom: 12px;
}

.logout-btn {
  padding: 16px;
}
</style>