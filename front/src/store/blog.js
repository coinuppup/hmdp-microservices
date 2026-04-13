import { defineStore } from 'pinia'
import { ref } from 'vue'
import { blogAPI, followAPI } from '@/api'

export const useBlogStore = defineStore('blog', () => {
  // 状态
  const hotBlogs = ref([])
  const userBlogs = ref([])
  const followBlogs = ref([])
  const currentBlog = ref(null)
  const comments = ref([])
  const followers = ref([])
  const followings = ref([])
  const loading = ref(false)

  // 获取热门博客
  async function fetchHotBlogs(current = 1, size = 10) {
    loading.value = true
    try {
      const data = await blogAPI.getHotBlogs(current, size)
      hotBlogs.value = data || []
      return data
    } catch (error) {
      console.error('获取热门博客失败:', error)
      return []
    } finally {
      loading.value = false
    }
  }

  // 获取用户博客
  async function fetchUserBlogs(userId, current = 1, size = 10) {
    loading.value = true
    try {
      const data = await blogAPI.getUserBlogs(userId, current, size)
      userBlogs.value = data || []
      return data
    } catch (error) {
      console.error('获取用户博客失败:', error)
      return []
    } finally {
      loading.value = false
    }
  }

  // 获取关注 feed
  async function fetchFollowBlogs(current = 1, size = 10) {
    loading.value = true
    try {
      const data = await blogAPI.getFollowBlogs(current, size)
      followBlogs.value = data || []
      return data
    } catch (error) {
      console.error('获取关注动态失败:', error)
      return []
    } finally {
      loading.value = false
    }
  }

  // 获取博客详情
  async function fetchBlogDetail(id) {
    loading.value = true
    try {
      const data = await blogAPI.getBlogDetail(id)
      currentBlog.value = data
      return data
    } catch (error) {
      console.error('获取博客详情失败:', error)
      return null
    } finally {
      loading.value = false
    }
  }

  // 点赞博客
  async function likeBlog(blogId) {
    try {
      await blogAPI.likeBlog(blogId)
      // 更新本地状态
      const blog = findBlogById(blogId)
      if (blog) {
        blog.isLike = true
        blog.liked = true
        blog.likes = (blog.likes || 0) + 1
      }
      return true
    } catch (error) {
      console.error('点赞失败:', error)
      throw error
    }
  }

  // 取消点赞
  async function unlikeBlog(blogId) {
    try {
      await blogAPI.unlikeBlog(blogId)
      // 更新本地状态
      const blog = findBlogById(blogId)
      if (blog) {
        blog.isLike = false
        blog.liked = false
        blog.likes = Math.max((blog.likes || 1) - 1, 0)
      }
      return true
    } catch (error) {
      console.error('取消点赞失败:', error)
      throw error
    }
  }

  // 发布博客
  async function createBlog(blog) {
    try {
      const blogId = await blogAPI.createBlog(blog)
      // 刷新博客列表
      await fetchHotBlogs()
      return blogId
    } catch (error) {
      console.error('发布博客失败:', error)
      throw error
    }
  }

  // 获取博客评论
  async function fetchBlogComments(blogId, current = 1, size = 10) {
    try {
      const data = await blogAPI.getBlogComments(blogId, current, size)
      comments.value = data || []
      return data
    } catch (error) {
      console.error('获取评论失败:', error)
      return []
    }
  }

  // 发表评论
  async function createComment(blogId, content) {
    try {
      const commentId = await blogAPI.createComment(blogId, content)
      // 刷新评论
      await fetchBlogComments(blogId)
      return commentId
    } catch (error) {
      console.error('发表评论失败:', error)
      throw error
    }
  }

  // 关注用户
  async function followUser(followUserId, isFollow = true) {
    try {
      await followAPI.followUser(followUserId, isFollow)
      return true
    } catch (error) {
      console.error('关注失败:', error)
      throw error
    }
  }

  // 获取粉丝列表
  async function fetchFollowers(userId, current = 1, size = 10) {
    try {
      const data = await followAPI.getFollowers(userId, current, size)
      followers.value = data || []
      return data
    } catch (error) {
      console.error('获取粉丝列表失败:', error)
      return []
    }
  }

  // 获取关注列表
  async function fetchFollowings(userId, current = 1, size = 10) {
    try {
      const data = await followAPI.getFollowings(userId, current, size)
      followings.value = data || []
      return data
    } catch (error) {
      console.error('获取关注列表失败:', error)
      return []
    }
  }

  // 检查是否关注
  async function checkFollow(targetUserId) {
    try {
      const isFollow = await followAPI.checkFollow(targetUserId)
      return isFollow
    } catch (error) {
      console.error('检查关注状态失败:', error)
      return false
    }
  }

  // 查找博客
  function findBlogById(id) {
    return hotBlogs.value.find(b => b.id === id) ||
           userBlogs.value.find(b => b.id === id) ||
           followBlogs.value.find(b => b.id === id)
  }

  return {
    // 状态
    hotBlogs,
    userBlogs,
    followBlogs,
    currentBlog,
    comments,
    followers,
    followings,
    loading,
    // 方法
    fetchHotBlogs,
    fetchUserBlogs,
    fetchFollowBlogs,
    fetchBlogDetail,
    likeBlog,
    unlikeBlog,
    createBlog,
    fetchBlogComments,
    createComment,
    followUser,
    fetchFollowers,
    fetchFollowings,
    checkFollow
  }
})