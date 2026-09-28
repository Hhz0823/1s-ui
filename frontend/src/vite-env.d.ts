/// <reference types="vite/client" />

// The version in package.json, fixed at build time.
declare const __APP_VERSION__: string

interface ImportMetaEnv {
  readonly VITE_BACKEND_URL?: string
  readonly VITE_BASE_PATH?: string
  readonly VITE_DEV_PROXY_TARGET?: string
  readonly VITE_OPENWRT_LITE?: string
}

interface Window {
  __SUI_CONFIG__?: {
    backendUrl?: string
    basePath?: string
  }
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
