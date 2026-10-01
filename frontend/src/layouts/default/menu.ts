import { computed } from 'vue'
import Data from '@/store/modules/data'

export interface MenuItem {
  title: string
  icon: string
  path: string
}

export interface MenuGroup {
  label: string
  items: MenuItem[]
}

// Navigation shared by the side menu, the top menu and the header breadcrumb.
export const allMenuGroups: MenuGroup[] = [
  {
    label: 'menu.group.overview',
    items: [
      { title: 'pages.home', icon: 'mdi-view-dashboard-outline', path: '/' },
      { title: 'pages.agents', icon: 'mdi-server-network', path: '/agents' },
      { title: 'pages.proxyMonitors', icon: 'mdi-shield-check-outline', path: '/proxy-monitors' },
      { title: 'pages.portTraffic', icon: 'mdi-chart-timeline-variant', path: '/port-traffic' },
      { title: 'pages.userTraffic', icon: 'mdi-trophy-outline', path: '/user-traffic' },
    ],
  },
  {
    label: 'menu.group.proxy',
    items: [
      { title: 'pages.inbounds', icon: 'mdi-arrow-down-bold-circle-outline', path: '/inbounds' },
      { title: 'pages.clients', icon: 'mdi-account-group-outline', path: '/clients' },
      { title: 'pages.outbounds', icon: 'mdi-arrow-up-bold-circle-outline', path: '/outbounds' },
      { title: 'pages.endpoints', icon: 'mdi-access-point-network', path: '/endpoints' },
    ],
  },
  {
    label: 'menu.group.system',
    items: [
      { title: 'pages.services', icon: 'mdi-cog-outline', path: '/services' },
      { title: 'pages.tls', icon: 'mdi-shield-lock-outline', path: '/tls' },
      { title: 'pages.basics', icon: 'mdi-tune-variant', path: '/basics' },
    ],
  },
  {
    label: 'menu.group.routing',
    items: [
      { title: 'pages.rules', icon: 'mdi-routes', path: '/rules' },
      { title: 'pages.dns', icon: 'mdi-dns-outline', path: '/dns' },
    ],
  },
  {
    label: 'menu.group.admin',
    items: [
      { title: 'pages.sdwan', icon: 'mdi-lan-connect', path: '/sdwan' },
      { title: 'pages.admins', icon: 'mdi-account-tie-outline', path: '/admins' },
      { title: 'pages.settings', icon: 'mdi-cog-outline', path: '/settings' },
    ],
  },
]

const monitorPaths = new Set(['/', '/agents', '/proxy-monitors', '/admins', '/settings'])

// Monitoring-only controllers show the pages they can use.
export const useMenuGroups = () => computed(() => {
  if (Data().controllerMode.profile !== 'monitor') return allMenuGroups
  return allMenuGroups
    .map(group => ({ ...group, items: group.items.filter(item => monitorPaths.has(item.path)) }))
    .filter(group => group.items.length)
})

export const useMenuItems = () => {
  const groups = useMenuGroups()
  return computed(() => groups.value.flatMap(group => group.items))
}

// menuTrail finds the group and item a route belongs to; nested pages such as
// /agents/3/inbounds belong to their top-level entry.
export const menuTrail = (path: string): { group?: MenuGroup, item?: MenuItem } => {
  for (const group of allMenuGroups) {
    const item = group.items.find(entry => entry.path === path)
      ?? group.items.find(entry => entry.path !== '/' && path.startsWith(entry.path + '/'))
    if (item) return { group, item }
  }
  return {}
}
