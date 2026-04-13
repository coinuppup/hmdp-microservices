<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useUserStore } from '@/store/user'

const userStore = useUserStore()

onMounted(() => {
  // 尝试恢复登录状态
  userStore.checkLoginStatus()
})
</script>

<style lang="scss">
#app {
  min-height: 100vh;
  background-color: #f5f5f5;
}
</style>