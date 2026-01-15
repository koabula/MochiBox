import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import './style.css'
import App from './App.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      welcome: 'Welcome to MochiBox'
    },
    zh: {
      welcome: '欢迎使用 MochiBox'
    }
  }
})

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(i18n)
app.mount('#app')
