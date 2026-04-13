<template>
  <div class="shop-list-page">
    <!-- 顶部搜索栏 -->
    <van-sticky>
      <div class="search-header">
        <van-icon name="arrow-left" @click="goBack" />
        <div class="search-box">
          <van-icon name="search" />
          <input type="text" placeholder="搜索商家" v-model="keyword">
        </div>
      </div>

      <!-- 分类筛选 -->
      <div class="filter-bar">
        <div
          class="filter-item"
          :class="{ active: currentTypeId === 0 }"
          @click="changeType(0)"
        >
          全部
        </div>
        <div
          v-for="type in shopTypes"
          :key="type.id"
          class="filter-item"
          :class="{ active: currentTypeId === type.id }"
          @click="changeType(type.id)"
        >
          {{ type.name }}
        </div>
      </div>
    </van-sticky>

    <!-- 商铺列表 -->
    <div class="shop-list">
      <div
        v-for="shop in shops"
        :key="shop.id"
        class="shop-item"
        @click="goShopDetail(shop.id)"
      >
        <div class="shop-img">
          <img v-if="shop.images" :src="shop.images.split(',')[0]" alt="">
          <img v-else src="https://via.placeholder.com/150" alt="">
        </div>
        <div class="shop-info">
          <h3 class="shop-name">{{ shop.name }}</h3>
          <div class="shop-meta">
            <div class="shop-rating">
              <van-rate v-model="shop.score" readonly :count="5" size="12" />
              <span class="score">{{ shop.score }}分</span>
            </div>
            <span class="sales">月售{{ shop.sales || 0 }}</span>
          </div>
          <div class="shop-location">
            <van-icon name="location-o" size="12" />
            <span>{{ shop.address }}</span>
          </div>
        </div>
      </div>

      <van-loading v-if="loading" class="loading" type="spinner" />
      <van-empty v-if="!loading && shops.length === 0" description="暂无商铺" />
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
import { ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useShopStore } from '@/store/shop'

const router = useRouter()
const route = useRoute()
const shopStore = useShopStore()

const activeTab = ref(1)
const shopTypes = ref([])
const shops = ref([])
const loading = ref(false)
const currentTypeId = ref(0)
const currentPage = ref(1)
const keyword = ref('')

// 获取数据
async function fetchData() {
  loading.value = true
  try {
    // 获取类型列表
    if (shopTypes.value.length === 0) {
      shopTypes.value = await shopStore.fetchShopTypes()
    }
    // 获取商铺列表
    const data = await shopStore.fetchShops(currentTypeId.value, currentPage.value)
    shops.value = data || []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  // 从路由获取分类
  if (route.query.typeId) {
    currentTypeId.value = parseInt(route.query.typeId)
  }
  fetchData()
})

// 切换分类
function changeType(typeId) {
  currentTypeId.value = typeId
  currentPage.value = 1
  fetchData()
}

// 返回
function goBack() {
  router.back()
}

// 跳转到详情
function goShopDetail(id) {
  router.push(`/shop/${id}`)
}
</script>

<style lang="scss" scoped>
.shop-list-page {
  min-height: 100vh;
  background: #f5f5f5;
}

.search-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: #fff;

  .van-icon {
    font-size: 20px;
  }

  .search-box {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    background: #f5f5f5;
    border-radius: 20px;
    padding: 8px 16px;
    color: #999;
    font-size: 14px;

    input {
      flex: 1;
      border: none;
      background: transparent;
      font-size: 14px;
      outline: none;
    }
  }
}

.filter-bar {
  display: flex;
  gap: 12px;
  padding: 12px 16px;
  background: #fff;
  overflow-x: auto;

  &::-webkit-scrollbar {
    display: none;
  }
}

.filter-item {
  flex-shrink: 0;
  padding: 6px 16px;
  font-size: 14px;
  color: #666;
  background: #f5f5f5;
  border-radius: 16px;
  cursor: pointer;

  &.active {
    background: #07c160;
    color: #fff;
  }
}

.shop-list {
  padding: 12px;
}

.shop-item {
  display: flex;
  background: #fff;
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 12px;
  cursor: pointer;
}

.shop-img {
  width: 80px;
  height: 80px;
  border-radius: 8px;
  overflow: hidden;
  flex-shrink: 0;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.shop-info {
  flex: 1;
  margin-left: 12px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.shop-name {
  font-size: 15px;
  font-weight: 600;
  color: #333;
}

.shop-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.shop-rating {
  display: flex;
  align-items: center;
  gap: 4px;

  .score {
    font-size: 12px;
    color: #ff6b00;
  }
}

.sales {
  font-size: 12px;
  color: #999;
}

.shop-location {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #999;
}

.loading {
  padding: 40px 0;
  text-align: center;
}
</style>