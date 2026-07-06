/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_OPENWRT_LITE?: string
}

declare module 'moment/locale/ru'
declare module 'moment/locale/vi'
declare module 'moment/locale/zh-cn'
declare module 'moment/locale/zh-tw'

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
