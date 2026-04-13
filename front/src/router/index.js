import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/store/user'

// 路由懒加载
const Home = () => import('@/views/home/index.vue')
const Login = () => import('@/views/login/index.vue')
const ShopList = () => import('@/views/shop/list.vue')
const ShopDetail = () => import('@/views/shop/detail.vue')
const BlogList = () => import('@/views/blog/list.vue')
const BlogDetail = () => import('@/views/blog/detail.vue')
const BlogEdit = () => import('@/views/blog/edit.vue')
const UserProfile = () => import('@/views/user/profile.vue')
const VoucherList = () => import('@/views/voucher/list.vue')
const OrderList = () => import('@/views/order/list.vue')

const routes = [
  {
    path: '/',
    name: 'home',
    component: Home,
    meta: { title: '首页' }
  },
  {
    path: '/login',
    name: 'login',
    component: Login,
    meta: { title: '登录' }
  },
  {
    path: '/shop',
    name: 'shop-list',
    component: ShopList,
    meta: { title: '商铺列表' }
  },
  {
    path: '/shop/:id',
    name: 'shop-detail',
    component: ShopDetail,
    meta: { title: '商铺详情' }
  },
  {
    path: '/blog',
    name: 'blog-list',
    component: BlogList,
    meta: { title: '博客' }
  },
  {
    path: '/blog/:id',
    name: 'blog-detail',
    component: BlogDetail,
    meta: { title: '博客详情' }
  },
  {
    path: '/blog/edit',
    name: 'blog-edit',
    component: BlogEdit,
    meta: { title: '发布博客', requiresAuth: true }
  },
  {
    path: '/user',
    name: 'user-profile',
    component: UserProfile,
    meta: { title: '个人中心', requiresAuth: true }
  },
  {
    path: '/voucher',
    name: 'voucher-list',
    component: VoucherList,
    meta: { title: '优惠券' }
  },
  {
    path: '/order',
    name: 'order-list',
    component: OrderList,
    meta: { title: '订单', requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const userStore = useUserStore()

  // 设置页面标题
  document.title = to.meta.title ? `${to.meta.title} - 黑马点评` : '黑马点评'

  // 检查是否需要登录
  if (to.meta.requiresAuth && !userStore.isLogin) {
    // 未登录，跳转登录页
    next({ name: 'login', query: { redirect: to.fullPath } })
  } else {
    next()
  }
})

export default router