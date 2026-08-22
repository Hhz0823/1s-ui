export type SuiRuntimeConfig = {
  backendUrl?: string
  basePath?: string
}

const runtime = window.__SUI_CONFIG__ ?? {}

const normalizeBasePath = (value: string | undefined) => {
  const pathname = new URL(value?.trim() || '/', window.location.origin).pathname
  return pathname === '/' ? '/' : `/${pathname.replace(/^\/+|\/+$/g, '')}/`
}

const normalizeBackendBase = (value: string | undefined) => {
  if (!value?.trim()) return new URL('/', window.location.origin)
  const base = new URL(value.trim(), window.location.origin)
  base.search = ''
  base.hash = ''
  if (!base.pathname.endsWith('/')) base.pathname += '/'
  return base
}

export const runtimeConfig = Object.freeze({
  backendUrl: runtime.backendUrl ?? import.meta.env.VITE_BACKEND_URL ?? '',
  basePath: normalizeBasePath(runtime.basePath ?? import.meta.env.VITE_BASE_PATH),
})

const backendBase = normalizeBackendBase(runtimeConfig.backendUrl)

export const backendBaseUrl = () => backendBase.toString()

export const resolveBackendUrl = (path: string) =>
  new URL(path.replace(/^\/+/, ''), backendBase).toString()

export const resolveBackendWebSocketUrl = (path: string) => {
  const url = new URL(resolveBackendUrl(path))
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

export const resolveFrontendUrl = (path = '') =>
  new URL(path.replace(/^\/+/, ''), new URL(runtimeConfig.basePath, window.location.origin)).toString()

export const backendFetch = (path: string, init: RequestInit = {}) => {
  const headers = new Headers(init.headers)
  if (!headers.has('X-Requested-With')) headers.set('X-Requested-With', 'XMLHttpRequest')
  return fetch(resolveBackendUrl(path), { ...init, credentials: 'include', headers })
}

type ApiEnvelope<T> = {
  success: boolean
  msg?: string
  obj?: T
}

export const fetchBackendObject = async <T = any>(path: string, init: RequestInit = {}): Promise<T> => {
  const headers = new Headers(init.headers)
  if (init.body != null && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await backendFetch(path, { ...init, headers })
  const result = await response.json().catch(() => null) as ApiEnvelope<T> | null
  if (!response.ok || !result?.success) throw new Error(result?.msg || response.statusText || 'Invalid response')
  return result.obj as T
}

const responseFilename = (header: string | null) => {
  const encoded = header?.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  if (encoded) {
    try { return decodeURIComponent(encoded) } catch { /* use the regular filename fallback */ }
  }
  return header?.match(/filename="?([^";]+)"?/i)?.[1]
}

export const downloadBackendFile = async (path: string, fallbackName: string) => {
  const response = await backendFetch(path)
  if (!response.ok) throw new Error((await response.text()).trim() || response.statusText)

  const url = URL.createObjectURL(await response.blob())
  const link = document.createElement('a')
  link.href = url
  link.download = responseFilename(response.headers.get('Content-Disposition')) || fallbackName
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}
