import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { userAPI } from '@/api'
import { TOKEN_KEYS } from '@/api/constants'

export const useUserStore = defineStore('user', () => {
  // 状态
  const userInfo = ref(null)
  const accessToken = ref(localStorage.getItem(TOKEN_KEYS.ACCESS_TOKEN) || '')
  const isLoggedIn = ref(false)
  const signCount = ref(0)

  // 计算属性
  const isLogin = computed(() => !!accessToken.value && isLoggedIn.value)

  // 发送验证码
  async function sendCode(phone) {
    try {
      await userAPI.sendCode(phone)
      return true
    } catch (error) {
      console.error('发送验证码失败:', error)
      return false
    }
  }

  // 登录
  async function login(phone, code) {
    try {
      const data = await userAPI.login(phone, code)
      if (data.accessToken) {
        accessToken.value = data.accessToken
        localStorage.setItem(TOKEN_KEYS.ACCESS_TOKEN, data.accessToken)

        // 获取用户信息
        await fetchUserInfo()
        isLoggedIn.value = true
        return true
      }
      return false
    } catch (error) {
      console.error('登录失败:', error)
      return false
    }
  }

  // 获取用户信息
  async function fetchUserInfo() {
    try {
      const data = await userAPI.getCurrentUser()
      userInfo.value = data
      return data
    } catch (error) {
      console.error('获取用户信息失败:', error)
      // Token可能过期，清除
      if (error.message === '登录已过期') {
        logout()
      }
      return null
    }
  }

  // 签到
  async function sign() {
    try {
      await userAPI.sign()
      // 刷新签到次数
      await fetchSignCount()
      return true
    } catch (error) {
      console.error('签到失败:', error)
      return false
    }
  }

  // 获取签到次数
  async function fetchSignCount() {
    try {
      const data = await userAPI.getSignCount()
      signCount.value = data
    } catch (error) {
      console.error('获取签到次数失败:', error)
    }
  }

  // 检查登录状态
  async function checkLoginStatus() {
    if (accessToken.value) {
      const user = await fetchUserInfo()
      if (user) {
        isLoggedIn.value = true
      }
    }
  }

  // 登出
  function logout() {
    accessToken.value = ''
    userInfo.value = null
    isLoggedIn.value = false
    signCount.value = 0
    localStorage.removeItem(TOKEN_KEYS.ACCESS_TOKEN)
    localStorage.removeItem(TOKEN_KEYS.REFRESH_TOKEN)
  }

  return {
    // 状态
    userInfo,
    accessToken,
    isLoggedIn,
    signCount,
    // 计算属性
    isLogin,
    // 方法
    sendCode,
    login,
    fetchUserInfo,
    sign,
    fetchSignCount,
    checkLoginStatus,
    logout
  }
})