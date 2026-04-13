import { defineStore } from 'pinia'
import { ref } from 'vue'
import { shopAPI, voucherAPI } from '@/api'

export const useShopStore = defineStore('shop', () => {
  // 状态
  const shopTypes = ref([])
  const shops = ref([])
  const currentShop = ref(null)
  const vouchers = ref([])
  const orders = ref([])
  const loading = ref(false)

  // 获取商铺类型
  async function fetchShopTypes() {
    try {
      const data = await shopAPI.getShopTypes()
      shopTypes.value = data || []
      return data
    } catch (error) {
      console.error('获取商铺类型失败:', error)
      return []
    }
  }

  // 获取商铺列表
  async function fetchShops(typeId, current = 1) {
    loading.value = true
    try {
      const data = await shopAPI.getShopList(typeId, current)
      shops.value = data || []
      return data
    } catch (error) {
      console.error('获取商铺列表失败:', error)
      return []
    } finally {
      loading.value = false
    }
  }

  // 获取商铺详情
  async function fetchShopDetail(id) {
    loading.value = true
    try {
      const data = await shopAPI.getShopDetail(id)
      currentShop.value = data
      return data
    } catch (error) {
      console.error('获取商铺详情失败:', error)
      return null
    } finally {
      loading.value = false
    }
  }

  // 获取优惠券列表
  async function fetchVouchers(shopId) {
    try {
      const data = await voucherAPI.getVoucherList(shopId)
      vouchers.value = data || []
      return data
    } catch (error) {
      console.error('获取优惠券列表失败:', error)
      return []
    }
  }

  // 秒杀下单
  async function seckillVoucher(voucherId) {
    try {
      const orderId = await voucherAPI.seckillVoucher(voucherId)
      return orderId
    } catch (error) {
      console.error('秒杀失败:', error)
      throw error
    }
  }

  // 获取订单列表
  async function fetchOrders(current = 1) {
    try {
      const data = await voucherAPI.getOrderList(current)
      orders.value = data || []
      return data
    } catch (error) {
      console.error('获取订单列表失败:', error)
      return []
    }
  }

  return {
    // 状态
    shopTypes,
    shops,
    currentShop,
    vouchers,
    orders,
    loading,
    // 方法
    fetchShopTypes,
    fetchShops,
    fetchShopDetail,
    fetchVouchers,
    seckillVoucher,
    fetchOrders
  }
})