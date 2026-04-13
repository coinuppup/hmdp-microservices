import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import 'vant/lib/index.css'
import './assets/main.scss'

// Vant 全量导入
import {
  Button,
  Tabbar,
  TabbarItem,
  Sticky,
  Field,
  Icon,
  Rate,
  Empty,
  Loading,
  Divider,
  Image as VanImage,
  ImagePreview,
  Uploader,
  Cell,
  CellGroup,
  Dialog,
  Toast,
  showToast,
  showFailToast
} from 'vant'

const app = createApp(App)
const pinia = createPinia()

// 注册 Vant 组件
app.use(pinia)
app.use(router)
app.use(Button)
app.use(Tabbar)
app.use(TabbarItem)
app.use(Sticky)
app.use(Field)
app.use(Icon)
app.use(Rate)
app.use(Empty)
app.use(Loading)
app.use(Divider)
app.use(VanImage)
app.use(ImagePreview)
app.use(Uploader)
app.use(Cell)
app.use(CellGroup)
app.use(Dialog)
app.use(Toast)

app.mount('#app')