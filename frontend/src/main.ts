/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Composables
import { createApp, ref } from 'vue'
import { reloadForNewChunks } from '@/utils/chunkReload'

// Components
import App from './App.vue'

// Use router
import router from './router'

// Store
import store from './store'

// Plugins
import { registerPlugins } from '@/plugins'

// Locale
import { i18n } from '@/locales'
import Vue3PersianDatetimePicker from 'vue3-persian-datetime-picker'

// Notivue
import { createNotivue } from 'notivue'
import 'notivue/notification.css'
import 'notivue/animations.css'
const notivue = createNotivue({
  position: 'bottom-center',
  limit: 4,
  enqueue: false,
  avoidDuplicates: true,
  notifications: {
    global: {
      duration: 3000
    }
  },
})

// Page chunks missing after a UI update: the router reloads into the page the
// user was opening; this catches any other lazily loaded chunk.
window.addEventListener('vite:preloadError', () => {
  window.setTimeout(() => reloadForNewChunks(), 200)
})

const loading = ref(false)

const app = createApp(App)
app.provide('loading', loading)

registerPlugins(app)

app
  .use(store)
  .use(router)
  .use(i18n)
  .use(notivue)
  .component('DatePicker', Vue3PersianDatetimePicker)
  .mount('#app')
