<template>
  <v-navigation-drawer
    v-model="showDrawer"
    :temporary="isMobile"
    :expand-on-hover="false"
    :rail="!isMobile && !expanded"
    :permanent="!isMobile"
    :width="isMobile ? 280 : 264"
    :rail-width="72"
    class="app-drawer"
    :class="{ 'app-drawer--expanded': expanded || isMobile, 'app-drawer--rail': !expanded && !isMobile }"
  >
    <div class="drawer-header">
      <div class="drawer-logo">
        <v-img src="@/assets/logo.svg" :width="36" :height="36" />
        <span class="drawer-brand">1S-UI</span>
      </div>
      <v-btn
        v-if="!isMobile"
        icon
        variant="text"
        size="small"
        class="drawer-toggle"
        @click.stop="$emit('toggleDrawer')"
      >
        <v-icon :icon="expanded ? 'mdi-chevron-left' : 'mdi-chevron-right'" size="20" />
      </v-btn>
      <v-btn v-else icon variant="text" size="small" @click.stop="$emit('closeDrawer')">
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
            @click="closeMobileDrawer"
          >
            <template v-slot:prepend>
              <div class="menu-icon-wrap">
                <v-icon :icon="item.icon" size="20" />
              </div>
            </template>
            <v-list-item-title class="menu-title">{{ $t(item.title) }}</v-list-item-title>
            <v-tooltip
              v-if="!isMobile && !expanded"
              activator="parent"
              location="end"
              :text="$t(item.title)"
            />
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
        >
          <v-tooltip
            v-if="!isMobile && !expanded"
            activator="parent"
            location="end"
            :text="$t('menu.logout')"
          />
        </v-list-item>
      </div>
    </template>
  </v-navigation-drawer>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import router from '@/router'
import { logout } from '@/plugins/httputil'

const props = defineProps(['isMobile', 'displayDrawer', 'expanded'])
const emit = defineEmits(['toggleDrawer', 'closeDrawer'])

const showDrawer = computed({
  get: (): boolean => props.displayDrawer,
  set: (value: boolean) => {
    if (!value) emit('closeDrawer')
  },
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

const closeMobileDrawer = () => {
  if (props.isMobile) emit('closeDrawer')
}
</script>

<style scoped>

/* ===== Liquid Glass Drawer ===== */
.app-drawer {
  --drawer-bg-rail: 0.3;
  --drawer-bg-expanded: 0.45;
  --drawer-blur-rail: 20px;
  --drawer-blur-expanded: 24px;
  transition:
    background 0.4s cubic-bezier(0.4, 0, 0.2, 1),
    backdrop-filter 0.4s cubic-bezier(0.4, 0, 0.2, 1),
    -webkit-backdrop-filter 0.4s cubic-bezier(0.4, 0, 0.2, 1),
    box-shadow 0.4s cubic-bezier(0.4, 0, 0.2, 1),
    border-color 0.4s cubic-bezier(0.4, 0, 0.2, 1),
    width 0.28s cubic-bezier(0.2, 0, 0, 1) !important;
  backdrop-filter: blur(var(--drawer-blur-rail)) saturate(180%) !important;
  -webkit-backdrop-filter: blur(var(--drawer-blur-rail)) saturate(180%) !important;
  background: rgba(var(--v-theme-surface), var(--drawer-bg-rail)) !important;
  border-right: 1px solid rgba(255, 255, 255, 0.08) !important;
  box-shadow: 1px 0 0 rgba(255, 255, 255, 0.05), 4px 0 16px rgba(0, 0, 0, 0.06) !important;
}

/* Expanded (desktop non-rail) -> transparent glass */
.v-navigation-drawer:not(.v-navigation-drawer--rail):not(.v-navigation-drawer--is-floating) {
  background: rgba(var(--v-theme-surface), var(--drawer-bg-expanded)) !important;
  backdrop-filter: blur(var(--drawer-blur-expanded)) saturate(180%) !important;
  -webkit-backdrop-filter: blur(var(--drawer-blur-expanded)) saturate(180%) !important;
  border-right: 1px solid rgba(255, 255, 255, 0.1) !important;
  box-shadow: 1px 0 0 rgba(255, 255, 255, 0.06), 4px 0 20px rgba(0, 0, 0, 0.08) !important;
}

/* Mobile temporary -> opaque */
.v-navigation-drawer--temporary.v-navigation-drawer--active {
  background: rgba(var(--v-theme-surface), 0.92) !important;
  backdrop-filter: blur(30px) saturate(180%) !important;
  -webkit-backdrop-filter: blur(30px) saturate(180%) !important;
}

/* Rail mode (not hovering) -> transparent glass */
.v-navigation-drawer--rail:not(.v-navigation-drawer--is-hovering) {
  background: rgba(var(--v-theme-surface), var(--drawer-bg-rail)) !important;
  backdrop-filter: blur(var(--drawer-blur-rail)) saturate(180%) !important;
  -webkit-backdrop-filter: blur(var(--drawer-blur-rail)) saturate(180%) !important;
}

/* Scrim override */
.v-navigation-drawer .v-overlay__scrim {
  background: transparent !important;
  backdrop-filter: none !important;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px 12px;
  min-height: 72px;
  transition: padding 0.28s cubic-bezier(0.2, 0, 0, 1), gap 0.28s cubic-bezier(0.2, 0, 0, 1);
}

.drawer-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.drawer-brand {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.5px;
  background: linear-gradient(135deg, rgb(var(--v-theme-primary)), rgb(var(--v-theme-secondary)));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  overflow: hidden;
  white-space: nowrap;
  transition: opacity 0.2s ease, max-width 0.28s cubic-bezier(0.2, 0, 0, 1);
}

.drawer-toggle {
  flex: 0 0 auto;
  opacity: 0.75;
}

.drawer-toggle:hover {
  opacity: 1;
  background: rgba(var(--v-theme-primary), 0.1);
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

.v-navigation-drawer--rail .menu-title,
.v-navigation-drawer--rail :deep(.v-list-item__content) {
  display: none;
}

.v-navigation-drawer--rail :deep(.v-list-item__prepend) {
  margin-inline-end: 0;
}

.v-navigation-drawer--rail .drawer-header {
  flex-direction: column;
  justify-content: center;
  padding: 14px 8px 10px;
  gap: 8px;
}

.v-navigation-drawer--rail .drawer-logo {
  justify-content: center;
}

.v-navigation-drawer--rail .drawer-brand {
  max-width: 0;
  opacity: 0;
}
</style>
