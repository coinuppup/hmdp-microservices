import axios from 'axios'
import { showToast, showFailToast } from 'vant'
import { TOKEN_KEYS, BASE_URL } from './constants'

// 创建axios实例
const service = axios.create({
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
service.interceptors.request.use(
  (config) => {
    // 添加Token
    const accessToken = localStorage.getItem(TOKEN_KEYS.ACCESS_TOKEN)
    if (accessToken) {
      config.headers['Authorization'] = `Bearer ${accessToken}`
    }

    // 添加设备ID
    let deviceId = localStorage.getItem('deviceId')
    if (!deviceId) {
      deviceId = generateDeviceId()
      localStorage.setItem('deviceId', deviceId)
    }
    config.headers['X-Device-ID'] = deviceId

    return config
  },
  (error) => {
    console.error('请求错误:', error)
    return Promise.reject(error)
  }
)

// 响应拦截器
service.interceptors.response.use(
  (response) => {
    const res = response.data

    // 根据后端返回的响应格式处理
    // 后端格式: { code: 200, message: "OK", data: ... }
    // content-service使用 { success: true, data: ... }
    if (res.code === 200 || res.success === true || res.code === undefined) {
      return res.data !== undefined ? res.data : res
    }

    // 处理业务错误
    if (res.code === 401) {
      // Token过期，尝试刷新
      return handleUnauthorized()
    }

    showFailToast(res.message || res.msg || '请求失败')
    return Promise.reject(new Error(res.message || res.msg || '请求失败'))
  },
  async (error) => {
    console.error('响应错误:', error)

    if (error.response) {
      if (error.response.status === 401) {
        return handleUnauthorized()
      }
      showFailToast(error.response.data?.message || '网络错误')
    } else {
      showToast('网络连接失败')
    }

    return Promise.reject(error)
  }
)

// 处理未授权情况
async function handleUnauthorized() {
  // 尝试刷新Token
  try {
    const refreshToken = localStorage.getItem(TOKEN_KEYS.REFRESH_TOKEN)
    if (!refreshToken) {
      // 没有refreshToken，跳转登录
      return Promise.reject(new Error('未登录'))
    }

    // 调用刷新Token接口
    const response = await axios.post(`${BASE_URL.user}/user/refresh`, {}, {
      withCredentials: true
    })

    if (response.data.code === 200) {
      const { accessToken } = response.data.data
      localStorage.setItem(TOKEN_KEYS.ACCESS_TOKEN, accessToken)
      // 重新发起请求 - 这里需要特殊处理，暂时简化
      return Promise.reject(new Error('token_refreshed'))
    }
  } catch (e) {
    // 刷新失败，清除Token
    localStorage.removeItem(TOKEN_KEYS.ACCESS_TOKEN)
    localStorage.removeItem(TOKEN_KEYS.REFRESH_TOKEN)
  }

  return Promise.reject(new Error('登录已过期'))
}

// 生成设备ID
function generateDeviceId() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
}

// 封装请求方法
const request = {
  get(url, params = {}, config = {}) {
    return service.get(url, { params, ...config })
  },
  post(url, data = {}, config = {}) {
    return service.post(url, data, config)
  },
  put(url, data = {}, config = {}) {
    return service.put(url, data, config)
  },
  delete(url, params = {}, config = {}) {
    return service.delete(url, { params, ...config })
  }
}

export default request