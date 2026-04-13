<template>
  <div class="home">
    <!-- 顶部搜索栏 -->
    <div class="home-header">
      <div class="search-bar" @click="goSearch">
        <van-icon name="search" />
        <span>搜索商家、店铺</span>
      </div>
    </div>

    <!-- 分类导航 -->
    <div class="category-nav">
      <div
        v-for="type in shopTypes"
        :key="type.id"
        class="category-item"
        @click="goShopList(type.id)"
      >
        <div class="category-icon">
          <img v-if="type.icon" :src="type.icon" alt="">
          <van-icon v-else name="shop-o" size="24" />
        </div>
        <span class="category-name">{{ type.name }}</span>
      </div>
    </div>

    <!-- 附近商铺 -->
    <div class="section-title">
      <span>附近商铺</span>
      <van-icon name="arrow-right" />
    </div>

    <div class="shop-list">
      <div
        v-for="shop in shops"
        :key="shop.id"
        class="shop-card"
        @click="goShopDetail(shop.id)"
      >
        <div class="shop-img">
          <img v-if="shop.images" :src="shop.images.split(',')[0]" alt="">
          <img v-else src="https://via.placeholder.com/150" alt="">
        </div>
        <div class="shop-info">
          <h3 class="shop-name">{{ shop.name }}</h3>
          <div class="shop-meta">
            <span class="shop-rate">
              <van-rate v-model="shop.score" readonly :count="5" size="12" />
              {{ shop.score }}分
            </span>
            <span class="shop-sales">月售{{ shop.sales || 0 }}</span>
          </div>
          <div class="shop-address">
            <van-icon name="location-o" size="12" />
            {{ shop.address }}
          </div>
        </div>
      </div>

      <!-- 加载中 -->
      <van-loading v-if="loading" class="loading" type="spinner" />
      <!-- 空状态 -->
      <div v-if="!loading && shops.length === 0" class="empty">
        <van-empty description="暂无商铺" />
      </div>
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
import { useRouter } from 'vue-router'
import { useShopStore } from '@/store/shop'

const router = useRouter()
const shopStore = useShopStore()

const activeTab = ref(0)
const shopTypes = ref([])
const shops = ref([])
const loading = ref(false)
const currentType = ref(1)
const currentPage = ref(1)

onMounted(async () => {
  // 获取商铺类型
  shopTypes.value = await shopStore.fetchShopTypes()

  // 获取商铺列表
  loading.value = true
  shops.value = await shopStore.fetchShops(currentType.value, currentPage.value)
  loading.value = false
})

// 跳转到搜索页
function goSearch() {
  router.push('/shop')
}

// 跳转到商铺列表
function goShopList(typeId) {
  router.push({ path: '/shop', query: { typeId } })
}

// 跳转到商铺详情
function goShopDetail(shopId) {
  router.push(`/shop/${shopId}`)
}
</script>

<style lang="scss" scoped>
.home {
  min-height: 100vh;
  background-color: #f5f5f5;
}

.home-header {
  position: sticky;
  top: 0;
  z-index: 100;
  background: linear-gradient(135deg, #07c160 0%, #06ad56 100%);
  padding: 12px 16px;
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.9);
  border-radius: 20px;
  padding: 8px 16px;
  color: #999;
  font-size: 14px;
}

.category-nav {
  display: flex;
  flex-wrap: wrap;
  background: #fff;
  padding: 16px 0;
  margin-bottom: 8px;
}

.category-item {
  width: 20%;
  display: flex;
  flex-direction: column;
  align-items: center;
  cursor: pointer;

  &:active {
    opacity: 0.7;
  }
}

.category-icon {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f5f5;
  border-radius: 12px;
  margin-bottom: 6px;

  img {
    width: 28px;
    height: 28px;
  }
}

.category-name {
  font-size: 12px;
  color: #333;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  font-size: 16px;
  font-weight: 600;
  background: #fff;
  margin-bottom: 8px;
}

.shop-list {
  padding: 0 12px;
}

.shop-card {
  display: flex;
  background: #fff;
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
  cursor: pointer;

  &:active {
    opacity: 0.9;
  }
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
  overflow: hidden;
}

.shop-name {
  font-size: 15px;
  font-weight: 600;
  color: #333;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.shop-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.shop-rate {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #ff6b00;
}

.shop-sales {
  font-size: 12px;
  color: #999;
}

.shop-address {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #999;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.loading {
  padding: 40px 0;
  text-align: center;
}
</style>