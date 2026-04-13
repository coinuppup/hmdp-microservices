<template>
  <div class="blog-detail-page">
    <!-- 头部返回 -->
    <div class="header">
      <van-icon name="arrow-left" @click="goBack" />
    </div>

    <!-- 博客内容 -->
    <div class="blog-content" v-if="blog.id">
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
          @click="handleFollow"
        >
          关注
        </van-button>
      </div>

      <!-- 博客标题和内容 -->
      <h1 class="blog-title">{{ blog.title }}</h1>
      <div class="blog-text">{{ blog.content }}</div>

      <!-- 图片 -->
      <div class="blog-images" v-if="blog.images">
        <img
          v-for="(img, index) in blog.images.split(',')"
          :key="index"
          :src="img"
          alt=""
          @click="previewImage(index)"
        >
      </div>

      <!-- 底部操作 -->
      <div class="blog-actions">
        <div class="action-item" @click="handleLike">
          <van-icon :name="blog.isLike ? 'good-job' : 'good-job-o'" />
          <span>{{ blog.likes || 0 }}</span>
        </div>
        <div class="action-item">
          <van-icon name="chat-o" />
          <span>{{ comments.length || 0 }}</span>
        </div>
        <div class="action-item">
          <van-icon name="share-o" />
          <span>分享</span>
        </div>
      </div>
    </div>

    <!-- 评论区域 -->
    <div class="comment-section">
      <div class="section-title">评论 {{ comments.length }}</div>
      <div class="comment-list">
        <div v-for="comment in comments" :key="comment.id" class="comment-item">
          <div class="comment-avatar">
            <img v-if="comment.icon" :src="comment.icon" alt="">
            <img v-else src="https://via.placeholder.com/36" alt="">
          </div>
          <div class="comment-body">
            <div class="comment-header">
              <span class="comment-name">{{ comment.nickname || '用户' }}</span>
              <span class="comment-time">{{ formatTime(comment.createTime) }}</span>
            </div>
            <div class="comment-content">{{ comment.content }}</div>
          </div>
        </div>
        <van-empty v-if="comments.length === 0" description="暂无评论" />
      </div>
    </div>

    <!-- 评论输入框 -->
    <div class="comment-input" v-if="userStore.isLogin">
      <van-field
        v-model="commentContent"
        placeholder="说点什么..."
        :border="false"
        @keyup.enter="submitComment"
      >
        <template #button>
          <van-button size="small" type="primary" @click="submitComment">
            发送
          </van-button>
        </template>
      </van-field>
    </div>

    <!-- 未登录提示 -->
    <div class="login-tip" v-else>
      <span @click="goLogin">登录</span>后参与评论
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
import { showToast, showFailToast, ImagePreview } from 'vant'
import { useBlogStore } from '@/store/blog'
import { useUserStore } from '@/store/user'
import dayjs from 'dayjs'

const router = useRouter()
const route = useRoute()
const blogStore = useBlogStore()
const userStore = useUserStore()

const activeTab = ref(2)
const blog = ref({})
const comments = ref([])
const commentContent = ref('')
const loading = ref(false)

onMounted(async () => {
  const blogId = parseInt(route.params.id)
  loading.value = true

  try {
    // 获取博客详情
    const blogData = await blogStore.fetchBlogDetail(blogId)
    if (blogData) {
      blog.value = blogData
    }

    // 获取评论
    const commentData = await blogStore.fetchBlogComments(blogId)
    comments.value = commentData || []
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

// 点赞
async function handleLike() {
  if (!userStore.isLogin) {
    router.push('/login')
    return
  }

  try {
    if (blog.value.isLike) {
      await blogStore.unlikeBlog(blog.value.id)
      blog.value.isLike = false
      blog.value.likes = Math.max((blog.value.likes || 1) - 1, 0)
    } else {
      await blogStore.likeBlog(blog.value.id)
      blog.value.isLike = true
      blog.value.likes = (blog.value.likes || 0) + 1
    }
  } catch (error) {
    showFailToast(error.message || '操作失败')
  }
}

// 关注
async function handleFollow() {
  if (!userStore.isLogin) {
    router.push('/login')
    return
  }

  try {
    await blogStore.followUser(blog.value.userId, true)
    blog.value.isFollow = true
    showToast('关注成功')
  } catch (error) {
    showFailToast(error.message || '关注失败')
  }
}

// 预览图片
function previewImage(index) {
  const images = blog.value.images.split(',')
  ImagePreview({
    images,
    startPosition: index
  })
}

// 发表评论
async function submitComment() {
  if (!commentContent.value.trim()) {
    showToast('请输入评论内容')
    return
  }

  try {
    await blogStore.createComment(blog.value.id, commentContent.value)
    commentContent.value = ''
    showToast('评论成功')
    // 刷新评论
    const commentData = await blogStore.fetchBlogComments(blog.value.id)
    comments.value = commentData || []
  } catch (error) {
    showFailToast(error.message || '评论失败')
  }
}

// 去登录
function goLogin() {
  router.push({ name: 'login', query: { redirect: route.fullPath } })
}
</script>

<style lang="scss" scoped>
.blog-detail-page {
  min-height: 100vh;
  background: #f5f5f5;
  padding-bottom: 50px;
}

.header {
  position: sticky;
  top: 0;
  z-index: 100;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(10px);

  .van-icon {
    font-size: 20px;
  }
}

.blog-content {
  background: #fff;
  padding: 16px;
  margin-bottom: 12px;
}

.blog-user {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}

.user-avatar {
  width: 44px;
  height: 44px;
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
  font-size: 15px;
  color: #333;
  font-weight: 500;
}

.blog-time {
  font-size: 12px;
  color: #999;
}

.blog-title {
  font-size: 20px;
  font-weight: 600;
  color: #333;
  margin-bottom: 16px;
  line-height: 1.4;
}

.blog-text {
  font-size: 15px;
  color: #333;
  line-height: 1.8;
  margin-bottom: 16px;
}

.blog-images {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;

  img {
    width: calc(50% - 4px);
    border-radius: 8px;
    cursor: pointer;
  }
}

.blog-actions {
  display: flex;
  justify-content: space-around;
  padding-top: 16px;
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

.comment-section {
  background: #fff;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  padding: 16px;
  border-bottom: 1px solid #f5f5f5;
}

.comment-list {
  padding: 0 16px;
}

.comment-item {
  display: flex;
  padding: 16px 0;
  border-bottom: 1px solid #f5f5f5;
}

.comment-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  overflow: hidden;
  flex-shrink: 0;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.comment-body {
  flex: 1;
  margin-left: 12px;
}

.comment-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 6px;
}

.comment-name {
  font-size: 14px;
  color: #333;
  font-weight: 500;
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

.comment-input {
  position: fixed;
  bottom: 50px;
  left: 0;
  right: 0;
  background: #fff;
  padding: 8px 16px;
  box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.05);

  :deep(.van-field) {
    background: #f5f5f5;
    border-radius: 20px;
    padding: 6px 12px;
  }
}

.login-tip {
  position: fixed;
  bottom: 50px;
  left: 0;
  right: 0;
  background: #fff;
  padding: 16px;
  text-align: center;
  color: #999;
  font-size: 14px;
  box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.05);

  span {
    color: #07c160;
    cursor: pointer;
  }
}
</style>