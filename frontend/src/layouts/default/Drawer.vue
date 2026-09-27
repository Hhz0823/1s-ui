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
            exact
            :to="item.path"
            :active="activePath === item.path"
            :aria-label="$t(item.title)"
            class="menu-item"
            :class="{ 'menu-item--active': activePath === item.path }"
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
              :open-delay="140"
              :close-delay="60"
              content-class="drawer-menu-tooltip"
            />
          </v-list-item>
        </v-list>
      </div>
    </div>

    <template v-slot:append>
      <v-divider class="drawer-divider" />
      <div class="drawer-footer">
        <v-list-item
          :title="$t('menu.logout')"
          :aria-label="$t('menu.logout')"
          @click="Logout"
          class="menu-item menu-item--logout"
        >
          <template v-slot:prepend>
            <div class="menu-icon-wrap">
              <v-icon icon="mdi-logout" size="20" />
            </div>
          </template>
          <v-tooltip
            v-if="!isMobile && !expanded"
            activator="parent"
            location="end"
            :text="$t('menu.logout')"
            :open-delay="140"
            :close-delay="60"
            content-class="drawer-menu-tooltip"
          />
        </v-list-item>
      </div>
    </template>
  </v-navigation-drawer>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { logout } from '@/plugins/httputil'
import { menuTrail, useMenuGroups } from './menu'

const props = defineProps(['isMobile', 'displayDrawer', 'expanded'])
const emit = defineEmits(['toggleDrawer', 'closeDrawer'])
const route = useRoute()

const showDrawer = computed({
  get: (): boolean => props.displayDrawer,
  set: (value: boolean) => {
    if (!value) emit('closeDrawer')
  },
})

const menuGroups = useMenuGroups()
const activePath = computed(() => menuTrail(route.path).item?.path)

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
  box-sizing: border-box;
  min-height: 44px;
  height: 44px;
  margin: 2px 4px;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}

.menu-item :deep(.v-list-item__overlay),
.menu-item :deep(.v-list-item__underlay) {
  opacity: 0 !important;
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
  background: transparent !important;
  transition: all 0.2s ease;
}

.menu-item--active .menu-icon-wrap {
  background: transparent !important;
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
  grid-template-columns: minmax(0, 1fr) 0 0 !important;
  width: 44px;
  min-width: 44px;
  max-width: 44px;
  margin: 2px auto;
  padding: 0 !important;
}

.v-navigation-drawer--rail .menu-title,
.v-navigation-drawer--rail :deep(.v-list-item__content) {
  display: none;
}

.v-navigation-drawer--rail :deep(.v-list-item__prepend) {
  display: flex;
  justify-content: center;
  width: 100%;
  margin-inline-end: 0;
}

.v-navigation-drawer--rail :deep(.v-list-item__prepend > .v-list-item__spacer) {
  display: none;
}

.v-navigation-drawer--rail .drawer-menu,
.v-navigation-drawer--rail .drawer-footer {
  padding-inline: 0;
}

.v-navigation-drawer--rail .menu-group {
  margin-bottom: 0;
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

<style>
/* Explicit mode selectors must stay unscoped so they can match body state. */
body.ui-style--solid .app-drawer.v-navigation-drawer.v-navigation-drawer--active {
  background: rgb(var(--v-theme-surface)) !important;
  border-right: 1px solid rgba(var(--v-theme-on-surface), 0.08) !important;
  box-shadow: 1px 0 0 rgba(var(--v-theme-on-surface), 0.03), 3px 0 12px rgba(15, 23, 42, 0.05) !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

body.ui-style--glass .app-drawer.v-navigation-drawer.v-navigation-drawer--active {
  background:
    radial-gradient(180px circle at var(--glass-pointer-x, 50%) var(--glass-pointer-y, 18%), rgba(255, 255, 255, 0.34), transparent 72%),
    linear-gradient(105deg, rgba(255, 255, 255, 0.16), transparent 46%, rgba(var(--v-theme-primary), 0.08)),
    rgba(var(--v-theme-surface), 0.28) !important;
  border-right: 1px solid rgba(255, 255, 255, 0.3) !important;
  box-shadow:
    inset -1px 0 0 rgba(var(--v-theme-on-surface), 0.05),
    inset 1px 0 0 rgba(255, 255, 255, 0.24),
    8px 0 30px rgba(15, 23, 42, 0.1) !important;
  backdrop-filter: blur(18px) saturate(190%) contrast(102%) !important;
  -webkit-backdrop-filter: blur(18px) saturate(190%) contrast(102%) !important;
}

body.ui-style--clear .app-drawer.v-navigation-drawer.v-navigation-drawer--active {
  background:
    linear-gradient(105deg, rgba(255, 255, 255, 0.12), transparent 56%),
    rgba(var(--v-theme-surface), 0.16) !important;
  border-right: 1px solid rgba(255, 255, 255, 0.22) !important;
  box-shadow: inset 1px 0 0 rgba(255, 255, 255, 0.18), 6px 0 24px rgba(15, 23, 42, 0.07) !important;
  backdrop-filter: blur(28px) saturate(185%) !important;
  -webkit-backdrop-filter: blur(28px) saturate(185%) !important;
}

body.ui-style--glass .app-drawer .menu-item {
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

body.ui-style--glass .app-drawer .menu-item.menu-item--active {
  border-color: rgba(var(--v-theme-primary), 0.3) !important;
  background:
    radial-gradient(100px circle at var(--glass-pointer-x, 50%) var(--glass-pointer-y, 50%), rgba(255, 255, 255, 0.3), transparent 72%),
    linear-gradient(145deg, rgba(255, 255, 255, 0.2), transparent 52%),
    rgba(var(--v-theme-primary), 0.11) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.34),
    0 7px 18px rgba(var(--v-theme-primary), 0.12) !important;
  backdrop-filter: blur(12px) saturate(175%) !important;
  -webkit-backdrop-filter: blur(12px) saturate(175%) !important;
}

@media (hover: hover) and (pointer: fine) {
  body.ui-style--glass .app-drawer .menu-item:not(.menu-item--active):hover {
    backdrop-filter: blur(10px) saturate(170%) !important;
    -webkit-backdrop-filter: blur(10px) saturate(170%) !important;
  }
}

body.ui-style--glass .app-drawer.v-navigation-drawer--rail .menu-item:hover {
  transform: translateY(-1px) !important;
}

body.ui-style--glass .app-drawer.v-navigation-drawer--rail .menu-item .menu-icon-wrap {
  transform: none !important;
  box-shadow: none !important;
}

@media (max-width: 960px) {
  body.ui-style--glass .app-drawer.v-navigation-drawer--temporary.v-navigation-drawer--active {
    background:
      linear-gradient(110deg, rgba(255, 255, 255, 0.18), transparent 52%),
      rgba(var(--v-theme-surface), 0.7) !important;
    backdrop-filter: blur(28px) saturate(190%) !important;
    -webkit-backdrop-filter: blur(28px) saturate(190%) !important;
  }
}

.drawer-menu-tooltip {
  padding: 6px 10px !important;
  border-radius: 8px !important;
  font-size: 12px !important;
  font-weight: 600 !important;
  letter-spacing: 0 !important;
  pointer-events: none !important;
}

body.ui-style--glass .drawer-menu-tooltip {
  color: rgb(var(--v-theme-on-surface)) !important;
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.2), transparent 55%),
    rgba(var(--v-theme-surface), 0.72) !important;
  border: 1px solid rgba(255, 255, 255, 0.28) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.3), 0 8px 20px rgba(15, 23, 42, 0.16) !important;
  backdrop-filter: blur(16px) saturate(180%) !important;
  -webkit-backdrop-filter: blur(16px) saturate(180%) !important;
}
</style>
