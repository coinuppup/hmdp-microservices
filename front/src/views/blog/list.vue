<template>
  <div class="blog-list-page">
    <!-- 顶部标签栏 -->
    <van-sticky>
      <div class="tab-bar">
        <div
          class="tab-item"
          :class="{ active: currentTab === 'hot' }"
          @click="changeTab('hot')"
        >
          热门
        </div>
        <div
          class="tab-item"
          :class="{ active: currentTab === 'follow' }"
          @click="changeTab('follow')"
        >
          关注
        </div>
      </div>
    </van-sticky>

    <!-- 博客列表 -->
    <div class="blog-list">
      <div
        v-for="blog in blogList"
        :key="blog.id"
        class="blog-item"
        @click="goBlogDetail(blog.id)"
      >
        <!-- 用户信息 -->
        <div class="blog-user">
          <div class="user-avatar">
            <img v-if="blog.icon" :src="blog.icon" alt="">
            <img v-else src="https://via.placeholder.com/40" alt="">
          </div>
          <div class="user-info">
            <div class="user-name">{{ blog.nickname || '用户' }}</div>
            <div class="blog-time">{{ formatTime(blog.createTime) }}</div>
          </div>
          <van-button
            v-if="!blog.isFollow && blog.userId !== userStore.userInfo?.id"
            size="small"
            type="primary"
            plain
            @click.stop="handleFollow(blog)"
          >
            关注
          </van-button>
        </div>

        <!-- 博客内容 -->
        <div class="blog-content">
          <h3 class="blog-title">{{ blog.title }}</h3>
          <p class="blog-text">{{ blog.content }}</p>
        </div>

        <!-- 图片 -->
        <div class="blog-images" v-if="blog.images">
          <img
            v-for="(img, index) in blog.images.split(',').slice(0, 3)"
            :key="index"
            :src="img"
            alt=""
          >
        </div>

        <!-- 底部操作 -->
        <div class="blog-actions">
          <div class="action-item" @click.stop="handleLike(blog)">
            <van-icon :name="blog.isLike ? 'good-job' : 'good-job-o'" />
            <span>{{ blog.likes || 0 }}</span>
          </div>
          <div class="action-item">
            <van-icon name="chat-o" />
            <span>{{ blog.comments || 0 }}</span>
          </div>
          <div class="action-item">
            <van-icon name="share-o" />
            <span>分享</span>
          </div>
        </div>
      </div>

      <van-loading v-if="loading" class="loading" type="spinner" />
      <van-empty v-if="!loading && blogList.length === 0" description="暂无博客" />
    </div>

    <!-- 发布按钮 -->
    <van-button
      v-if="userStore.isLogin"
      type="primary"
      size="large"
      class="publish-btn"
      @click="goPublish"
    >
      <van-icon name="plus" />
      发布博客
    </van-button>

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
import { useBlogStore } from '@/store/blog'
import { useUserStore } from '@/store/user'
import dayjs from 'dayjs'

const router = useRouter()
const blogStore = useBlogStore()
const userStore = useUserStore()

const activeTab = ref(2)
const currentTab = ref('hot')
const blogList = ref([])
const loading = ref(false)

onMounted(() => {
  fetchData()
})

// 获取数据
async function fetchData() {
  loading.value = true
  try {
    if (currentTab.value === 'hot') {
      const data = await blogStore.fetchHotBlogs()
      blogList.value = data || []
    } else {
      if (!userStore.isLogin) {
        showToast('请先登录')
        return
      }
      const data = await blogStore.fetchFollowBlogs()
      blogList.value = data || []
    }
  } finally {
    loading.value = false
  }
}

// 切换标签
function changeTab(tab) {
  currentTab.value = tab
  fetchData()
}

// 格式化时间
function formatTime(time) {
  if (!time) return ''
  return dayjs(time).format('MM-DD HH:mm')
}

// 跳转详情
function goBlogDetail(id) {
  router.push(`/blog/${id}`)
}

// 点赞
async function handleLike(blog) {
  if (!userStore.isLogin) {
    router.push('/login')
    return
  }

  try {
    if (blog.isLike) {
      await blogStore.unlikeBlog(blog.id)
      blog.isLike = false
      blog.likes = Math.max((blog.likes || 1) - 1, 0)
    } else {
      await blogStore.likeBlog(blog.id)
      blog.isLike = true
      blog.likes = (blog.likes || 0) + 1
    }
  } catch (error) {
    showFailToast(error.message || '操作失败')
  }
}

// 关注
async function handleFollow(blog) {
  if (!userStore.isLogin) {
    router.push('/login')
    return
  }

  try {
    await blogStore.followUser(blog.userId, true)
    blog.isFollow = true
    showToast('关注成功')
  } catch (error) {
    showFailToast(error.message || '关注失败')
  }
}

// 发布博客
function goPublish() {
  router.push('/blog/edit')
}
</script>

<style lang="scss" scoped>
.blog-list-page {
  min-height: 100vh;
  background: #f5f5f5;
  padding-bottom: 100px;
}

.tab-bar {
  display: flex;
  background: #fff;
  padding: 0 16px;

  .tab-item {
    flex: 1;
    text-align: center;
    padding: 16px 0;
    font-size: 16px;
    color: #666;
    cursor: pointer;
    position: relative;

    &.active {
      color: #07c160;
      font-weight: 600;

      &::after {
        content: '';
        position: absolute;
        bottom: 0;
        left: 50%;
        transform: translateX(-50%);
        width: 30px;
        height: 3px;
        background: #07c160;
        border-radius: 2px;
      }
    }
  }
}

.blog-list {
  padding: 12px;
}

.blog-item {
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
}

.blog-user {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.user-avatar {
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

.user-info {
  flex: 1;
  margin-left: 12px;
}

.user-name {
  font-size: 14px;
  color: #333;
  font-weight: 500;
}

.blog-time {
  font-size: 12px;
  color: #999;
}

.blog-content {
  margin-bottom: 12px;
}

.blog-title {
  font-size: 16px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}

.blog-text {
  font-size: 14px;
  color: #666;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}

.blog-images {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;

  img {
    width: 100px;
    height: 100px;
    object-fit: cover;
    border-radius: 8px;
  }
}

.blog-actions {
  display: flex;
  justify-content: space-around;
  padding-top: 12px;
  border-top: 1px solid #f5f5f5;
}

.action-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #666;
  font-size: 14px;
  cursor: pointer;
}

.publish-btn {
  position: fixed;
  bottom: 70px;
  left: 50%;
  transform: translateX(-50%);
  width: 120px;
  height: 40px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  box-shadow: 0 4px 12px rgba(7, 193, 96, 0.3);
}

.loading {
  padding: 40px 0;
  text-align: center;
}
</style>