<template>
  <div class="login-page">
    <div class="login-header">
      <h1 class="logo">黑马点评</h1>
      <p class="slogan">，发现美好生活</p>
    </div>

    <div class="login-form">
      <div class="form-item">
        <van-field
          v-model="phone"
          type="tel"
          maxlength="11"
          placeholder="请输入手机号"
          :border="false"
          clearable
        >
          <template #left-icon>
            <van-icon name="phone-o" />
          </template>
        </van-field>
      </div>

      <div class="form-item code-item">
        <van-field
          v-model="code"
          type="digit"
          maxlength="6"
          placeholder="请输入验证码"
          :border="false"
          clearable
        >
          <template #left-icon>
            <van-icon name="envelop-o" />
          </template>
        </van-field>
        <van-button
          size="small"
          type="primary"
          :disabled="countdown > 0"
          @click="sendCode"
          class="send-btn"
        >
          {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
        </van-button>
      </div>

      <div class="login-btn">
        <van-button
          type="primary"
          size="large"
          :loading="loading"
          @click="handleLogin"
        >
          登录
        </van-button>
      </div>

      <div class="login-tip">
        未注册的手机号将自动创建账号
      </div>
    </div>

    <div class="login-footer">
      <p class="agreement">
        登录即表示同意
        <a href="#">《用户协议》</a>
        和
        <a href="#">《隐私政策》</a>
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast, showFailToast } from 'vant'
import { useUserStore } from '@/store/user'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const phone = ref('')
const code = ref('')
const loading = ref(false)
const countdown = ref(0)

// 发送验证码
async function sendCode() {
  if (!phone.value || phone.value.length !== 11) {
    showFailToast('请输入正确的手机号')
    return
  }

  const success = await userStore.sendCode(phone.value)
  if (success) {
    showToast('验证码已发送')
    // 开始倒计时
    countdown.value = 60
    const timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0) {
        clearInterval(timer)
      }
    }, 1000)
  }
}

// 登录
async function handleLogin() {
  if (!phone.value || phone.value.length !== 11) {
    showFailToast('请输入正确的手机号')
    return
  }

  if (!code.value || code.value.length < 4) {
    showFailToast('请输入验证码')
    return
  }

  loading.value = true

  try {
    const success = await userStore.login(phone.value, code.value)

    if (success) {
      showToast('登录成功')

      // 跳转到之前的页面或首页
      const redirect = route.query.redirect || '/'
      router.replace(redirect)
    } else {
      showFailToast('登录失败，请检查验证码')
    }
  } catch (error) {
    showFailToast(error.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style lang="scss" scoped>
.login-page {
  min-height: 100vh;
  background: #fff;
  display: flex;
  flex-direction: column;
}

.login-header {
  padding: 60px 30px 40px;
  text-align: center;

  .logo {
    font-size: 32px;
    color: #07c160;
    font-weight: bold;
    margin-bottom: 8px;
  }

  .slogan {
    font-size: 14px;
    color: #999;
  }
}

.login-form {
  flex: 1;
  padding: 0 30px;

  .form-item {
    background: #f5f5f5;
    border-radius: 24px;
    margin-bottom: 16px;
    overflow: hidden;

    :deep(.van-field__body) {
      height: 50px;
    }
  }

  .code-item {
    display: flex;
    align-items: center;

    .van-field {
      flex: 1;
    }

    .send-btn {
      margin-right: 8px;
      background: #07c160;
      border: none;
      border-radius: 18px;

      &:disabled {
        background: #ccc;
      }
    }
  }

  .login-btn {
    margin-top: 32px;

    :deep(.van-button) {
      height: 50px;
      border-radius: 25px;
      font-size: 16px;
      background: #07c160;
      border: none;
    }
  }

  .login-tip {
    text-align: center;
    margin-top: 16px;
    font-size: 12px;
    color: #999;
  }
}

.login-footer {
  padding: 30px;

  .agreement {
    text-align: center;
    font-size: 12px;
    color: #999;

    a {
      color: #07c160;
      text-decoration: none;
    }
  }
}
</style>