<template>
  <div class="blog-edit-page">
    <!-- 头部 -->
    <div class="header">
      <van-icon name="cross" @click="goBack" />
      <span class="title">发布博客</span>
      <van-button size="small" type="primary" :loading="publishing" @click="handlePublish">
        发布
      </van-button>
    </div>

    <!-- 表单 -->
    <div class="edit-form">
      <van-field
        v-model="title"
        placeholder="标题"
        :border="false"
        maxlength="50"
        show-word-limit
      />

      <van-divider />

      <van-field
        v-model="content"
        type="textarea"
        placeholder="分享你的想法..."
        :border="false"
        maxlength="500"
        show-word-limit
        autosize
        rows="8"
      />

      <!-- 图片上传 -->
      <div class="image-upload">
        <van-uploader
          v-model="fileList"
          :max-count="9"
          :after-read="afterRead"
          @delete="deleteImage"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showFailToast } from 'vant'
import { useBlogStore } from '@/store/blog'
import { useUserStore } from '@/store/user'

const router = useRouter()
const blogStore = useBlogStore()
const userStore = useUserStore()

const title = ref('')
const content = ref('')
const fileList = ref([])
const publishing = ref(false)

// 上传图片
function afterRead(file) {
  // 这里应该上传到服务器，这里简单处理为base64
  file.status = 'uploading'
  file.message = '上传中...'

  // 模拟上传
  setTimeout(() => {
    file.status = 'done'
    file.message = '上传成功'
  }, 1000)
}

// 删除图片
function deleteImage(file) {
  const index = fileList.value.findIndex(item => item.file === file.file)
  if (index > -1) {
    fileList.value.splice(index, 1)
  }
}

// 返回
function goBack() {
  router.back()
}

// 发布
async function handlePublish() {
  if (!title.value.trim()) {
    showToast('请输入标题')
    return
  }

  if (!content.value.trim()) {
    showToast('请输入内容')
    return
  }

  publishing.value = true

  try {
    // 组合图片
    const images = fileList.value.map(item => item.content).join(',')

    await blogStore.createBlog({
      title: title.value,
      content: content.value,
      images
    })

    showToast('发布成功')
    router.back()
  } catch (error) {
    showFailToast(error.message || '发布失败')
  } finally {
    publishing.value = false
  }
}
</script>

<style lang="scss" scoped>
.blog-edit-page {
  min-height: 100vh;
  background: #fff;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #f5f5f5;

  .van-icon {
    font-size: 20px;
  }

  .title {
    font-size: 16px;
    font-weight: 600;
  }
}

.edit-form {
  padding: 16px;

  :deep(.van-field__body) {
    font-size: 16px;
  }

  :deep(.van-field__control) {
    min-height: 100px !important;
  }
}

.image-upload {
  margin-top: 16px;
}
</style>