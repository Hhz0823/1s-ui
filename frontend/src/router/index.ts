// Composables
import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Setup from '@/views/Setup.vue'
import Data from '@/store/modules/data'
import { fetchBackendObject, runtimeConfig } from '@/utils/backend'
import { isChunkLoadError, reloadForNewChunks } from '@/utils/chunkReload'

const routes = [
  {
    path: '/login',
    name: 'pages.login',
    component: Login,
  },
  {
    path: '/setup',
    name: 'pages.setup',
    component: Setup,
  },
  {
    path: '/',
    component: () => import('@/layouts/default/Default.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '/',
        name: 'pages.home',
        component: () => import('@/views/Home.vue'),
      },
      {
        path: '/inbounds',
        name: 'pages.inbounds',
        component: () => import('@/views/Inbounds.vue'),
      },
      {
        path: '/port-traffic',
        name: 'pages.portTraffic',
        component: () => import('@/components/PortTraffic.vue'),
      },
      {
        path: '/user-traffic',
        name: 'pages.userTraffic',
        component: () => import('@/views/UserTraffic.vue'),
      },
      {
        path: '/clients',
        name: 'pages.clients',
        component: () => import('@/views/Clients.vue'),
      },  
      {
        path: '/outbounds',
        name: 'pages.outbounds',
        component: () => import('@/views/Outbounds.vue'),
      },
      {
        path: '/services',
        name: 'pages.services',
        component: () => import('@/views/Services.vue'),
      },
      {
        path: '/endpoints',
        name: 'pages.endpoints',
        component: () => import('@/views/Endpoints.vue'),
      },
      {
        path: '/rules',
        name: 'pages.rules',
        component: () => import('@/views/Rules.vue'),
      },
      {
        path: '/tls',
        name: 'pages.tls',
        component: () => import('@/views/Tls.vue'),
      },
      {
        path: '/basics',
        name: 'pages.basics',
        component: () => import('@/views/Basics.vue'),
      },
      {
        path: '/dns',
        name: 'pages.dns',
        component: () => import('@/views/Dns.vue'),
      },
      {
        path: '/admins',
        name: 'pages.admins',
        component: () => import('@/views/Admins.vue'),
      },
      {
        path: '/settings',
        name: 'pages.settings',
        component: () => import('@/views/Settings.vue'),
      },
      {
        path: '/agents',
        name: 'pages.agents',
        component: () => import('@/views/Agents.vue'),
      },
      {
        path: '/proxy-monitors',
        name: 'pages.proxyMonitors',
        component: () => import('@/views/ProxyMonitors.vue'),
      },
      {
        path: '/proxy-client',
        name: 'pages.proxyClient',
        component: () => import('@/views/ProxyClient.vue'),
      },
      {
        path: '/agents/:id',
        name: 'agent.detail',
        component: () => import('@/views/AgentDetail.vue'),
      },
      {
        path: '/agents/:id/inbounds',
        name: 'agent.remoteInbounds',
        component: () => import('@/views/AgentInbounds.vue'),
      },
      {
        path: '/sdwan',
        name: 'pages.sdwan',
        component: () => import('@/views/Sdwan.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(runtimeConfig.basePath),
  routes,
  scrollBehavior: (_to, _from, savedPosition) => savedPosition ?? { left: 0, top: 0 },
})

const DATA_REFRESH_MS = 15000
let intervalId: ReturnType<typeof setInterval> | undefined

const refreshData = () => {
  if (!document.hidden) void Data().loadData()
}

const monitorRouteAllowed = (path: string) =>
  path === '/' || path === '/agents' || /^\/agents\/[^/]+$/.test(path) || path === '/proxy-monitors' || path === '/settings' || path === '/admins'

const stopDataInterval = () => {
  if (!intervalId) return
  clearInterval(intervalId)
  intervalId = undefined
}

router.beforeEach(async (to) => {
  let access: { required: boolean; authenticated: boolean }
  try {
    access = await fetchBackendObject('api/setup-status')
  } catch {
    stopDataInterval()
    if (to.path !== '/login') return '/login'
    return
  }

  if (access.required) {
    stopDataInterval()
    if (to.path !== '/setup') return '/setup'
    return
  }
  if (!access.authenticated) {
    stopDataInterval()
    if (to.path !== '/login') return '/login'
    return
  }
  if (to.path === '/login' || to.path === '/setup') return '/'

  const data = Data()
  await data.loadControllerMode()
  loadDataInterval()
  if (data.controllerMode.profile === 'monitor' && !monitorRouteAllowed(to.path)) return '/agents'
})

const loadDataInterval = () => {
  if (intervalId) return
  refreshData()
  intervalId = setInterval(refreshData, DATA_REFRESH_MS)
}

router.onError((error, to) => {
  if (isChunkLoadError(error)) reloadForNewChunks(router.resolve(to).href)
})

document.addEventListener('visibilitychange', () => {
  if (intervalId && !document.hidden) refreshData()
})

export default router
