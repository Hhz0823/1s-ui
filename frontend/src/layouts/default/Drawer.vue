<template>
  <v-navigation-drawer
    v-model="showDrawer"
    :temporary="isMobile"
    :expand-on-hover="!isMobile"
    :rail="!isMobile"
    :permanent="!isMobile"
    :width="isMobile ? 300 : 260"
    @click="isMobile ? $emit('toggleDrawer') : null"
    class="app-drawer"
  >
    <div class="drawer-header">
      <div class="drawer-logo">
        <v-img src="@/assets/logo.svg" :width="36" :height="36" />
        <span class="drawer-brand">1S-UI</span>
      </div>
      <v-btn v-if="isMobile" icon variant="text" size="small" @click.stop="$emit('toggleDrawer')">
        <v-icon icon="mdi-close" size="20" />
      </v-btn>
    </div>

    <v-divider class="drawer-divider" />

    <div class="drawer-menu">
      <div v-for="group in menuGroups" :key="group.label" class="menu-group">
        <div class="group-label">{{ $t(group.label) }}</div>
        <v-list density="compact" nav class="menu-list">
          <v-list-item
            v-for="item in group.items"
            :key="item.title"
            link
            :to="item.path"
            :active="router.currentRoute.value.path === item.path"
            class="menu-item"
            :class="{ 'menu-item--active': router.currentRoute.value.path === item.path }"
          >
            <template v-slot:prepend>
              <div class="menu-icon-wrap">
                <v-icon :icon="item.icon" size="20" />
              </div>
            </template>
            <v-list-item-title class="menu-title">{{ $t(item.title) }}</v-list-item-title>
          </v-list-item>
        </v-list>
      </div>
    </div>

    <template v-slot:append>
      <v-divider class="drawer-divider" />
      <div class="drawer-footer">
        <v-list-item
          prepend-icon="mdi-logout"
          :title="$t('menu.logout')"
          @click="Logout"
          class="menu-item menu-item--logout"
        />
      </div>
    </template>
  </v-navigation-drawer>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import router from '@/router'
import { logout } from '@/plugins/httputil'

const props = defineProps(['isMobile', 'displayDrawer'])

const showDrawer = computed((): boolean => {
  return props.displayDrawer
})

const menuGroups = [
  {
    label: 'menu.group.overview',
    items: [
      { title: 'pages.home', icon: 'mdi-view-dashboard-outline', path: '/' },
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
      { title: 'pages.admins', icon: 'mdi-account-tie-outline', path: '/admins' },
      { title: 'pages.settings', icon: 'mdi-cog-outline', path: '/settings' },
    ],
  },
]

const Logout = async () => {
  logout()
}
</script>

<style scoped>
.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 12px;
  min-height: 64px;
}

.drawer-logo {
  display: flex;
  align-items: center;
  gap: 12px;
}

.drawer-brand {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.5px;
  background: linear-gradient(135deg, rgb(var(--v-theme-primary)), rgb(var(--v-theme-secondary)));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.drawer-divider {
  margin: 0 16px;
  opacity: 0.15;
}

.drawer-menu {
  padding: 8px 8px;
  overflow-y: auto;
  flex: 1;
}

.menu-group {
  margin-bottom: 4px;
}

.group-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  color: rgba(var(--v-theme-on-surface), 0.45);
  padding: 12px 16px 4px;
}

.menu-list {
  padding: 0 !important;
  background: transparent !important;
}

.menu-item {
  border-radius: 10px !important;
  margin: 1px 4px;
  min-height: 40px;
  transition: all 0.2s ease;
}

.menu-item:hover {
  background: rgba(var(--v-theme-primary), 0.08) !important;
}

.menu-item--active {
  background: rgba(var(--v-theme-primary), 0.12) !important;
}

.menu-item--active .menu-title {
  font-weight: 600;
  color: rgb(var(--v-theme-primary));
}

.menu-item--active .menu-icon-wrap {
  color: rgb(var(--v-theme-primary));
}

.menu-icon-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.menu-item--active .menu-icon-wrap {
  background: rgba(var(--v-theme-primary), 0.15);
}

.menu-title {
  font-size: 13.5px;
  font-weight: 500;
  letter-spacing: 0.01em;
}

.menu-item--logout {
  color: rgb(var(--v-theme-error));
}

.menu-item--logout:hover {
  background: rgba(var(--v-theme-error), 0.08) !important;
}

.drawer-footer {
  padding: 8px;
}

.v-navigation-drawer--rail .group-label {
  display: none;
}

.v-navigation-drawer--rail .menu-item {
  margin: 1px 2px;
}

.v-navigation-drawer--rail .drawer-header {
  padding: 16px 12px 12px;
  justify-content: center;
}

.v-navigation-drawer--rail .drawer-brand {
  display: none;
}
</style>