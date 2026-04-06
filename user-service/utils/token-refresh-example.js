// 前端无感刷新实现示例
// 处理并发请求的挑战

class AuthService {
  constructor() {
    this.accessToken = localStorage.getItem('accessToken');
    this.isRefreshing = false; // 标记是否正在刷新token
    this.refreshSubscribers = []; // 存储等待刷新token的请求
  }

  // 获取access token
  getAccessToken() {
    return this.accessToken;
  }

  // 存储token
  setTokens(tokens) {
    this.accessToken = tokens.accessToken;
    localStorage.setItem('accessToken', tokens.accessToken);
    // Refresh Token由后端存储在HttpOnly Cookie中，前端不需要存储
  }

  // 订阅刷新token的事件
  subscribeToRefresh(cb) {
    this.refreshSubscribers.push(cb);
  }

  // 通知所有订阅者刷新token完成
  notifyRefreshSubscribers(tokens) {
    this.refreshSubscribers.forEach(cb => cb(tokens));
    this.refreshSubscribers = [];
  }

  // 刷新token
  async refreshTokens() {
    // 如果已经有请求正在刷新token，返回一个Promise等待刷新完成
    if (this.isRefreshing) {
      return new Promise(resolve => {
        this.subscribeToRefresh(resolve);
      });
    }

    // 标记开始刷新token
    this.isRefreshing = true;

    try {
      // 调用刷新token接口
      const response = await fetch('/api/auth/refresh', {
        method: 'POST',
        credentials: 'include', // 自动携带HttpOnly Cookie中的refresh token
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          deviceId: this.getDeviceId() // 设备ID，用于多端识别
        })
      });

      if (!response.ok) {
        throw new Error('刷新token失败');
      }

      const tokens = await response.json();
      this.setTokens(tokens);
      this.isRefreshing = false;
      this.notifyRefreshSubscribers(tokens);
      return tokens;
    } catch (error) {
      this.isRefreshing = false;
      this.refreshSubscribers = [];
      // 刷新失败，跳转到登录页面
      this.redirectToLogin();
      throw error;
    }
  }

  // 获取设备ID
  getDeviceId() {
    let deviceId = localStorage.getItem('deviceId');
    if (!deviceId) {
      deviceId = Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15);
      localStorage.setItem('deviceId', deviceId);
    }
    return deviceId;
  }

  // 跳转到登录页面
  redirectToLogin() {
    localStorage.removeItem('accessToken');
    // Refresh Token由后端存储在HttpOnly Cookie中，前端不需要处理
    window.location.href = '/login';
  }
}

// 创建AuthService实例
const authService = new AuthService();

// 封装fetch请求，处理token过期和并发请求
async function authenticatedFetch(url, options = {}) {
  // 添加Authorization头
  options.headers = {
    ...options.headers,
    'Authorization': `Bearer ${authService.getAccessToken()}`
  };

  try {
    // 发起请求
    const response = await fetch(url, options);

    // 处理401错误
    if (response.status === 401) {
      // 刷新token
      const tokens = await authService.refreshTokens();
      
      // 更新请求头中的token
      options.headers['Authorization'] = `Bearer ${tokens.accessToken}`;
      
      // 重试原请求
      return await fetch(url, options);
    }

    return response;
  } catch (error) {
    // 处理网络错误等
    console.error('请求失败:', error);
    throw error;
  }
}

// 示例：同时发起多个请求
async function fetchMultipleResources() {
  try {
    // 同时发起多个请求
    const [userInfo, userList, userSettings] = await Promise.all([
      authenticatedFetch('/api/user/info'),
      authenticatedFetch('/api/user/list'),
      authenticatedFetch('/api/user/settings')
    ]);

    // 处理响应
    const userInfoData = await userInfo.json();
    const userListData = await userList.json();
    const userSettingsData = await userSettings.json();

    console.log('用户信息:', userInfoData);
    console.log('用户列表:', userListData);
    console.log('用户设置:', userSettingsData);
  } catch (error) {
    console.error('请求失败:', error);
  }
}

// 导出
export { authService, authenticatedFetch, fetchMultipleResources };