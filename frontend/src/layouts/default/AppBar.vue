<template>
  <v-app-bar :elevation="0" class="app-bar" :height="56">
    <template v-slot:prepend>
      <v-btn
        v-if="isMobile || menuPosition === 'top'"
        icon
        variant="text"
        size="small"
        @click="$emit('toggleDrawer')"
        class="menu-toggle"
      >
        <v-icon icon="mdi-menu" size="22" />
      </v-btn>
    </template>

    <!-- Top Menu Items (when menuPosition is 'top') -->
    <div v-if="menuPosition === 'top' && !isMobile" class="top-menu-bar">
      <router-link
        v-for="item in menuItems"
        :key="item.path"
        :to="item.path"
        custom
        v-slot="{ navigate, isActive }"
      >
        <v-btn
          variant="text"
          size="small"
          class="top-menu-item"
          :class="{ 'top-menu-item--active': isActive }"
          @click="navigate"
        >
          <v-icon :icon="item.icon" size="16" class="me-1" />
          {{ $t(item.title) }}
        </v-btn>
      </router-link>
    </div>

    <v-app-bar-title v-if="menuPosition !== 'top' || isMobile" class="app-bar-title">
      <span class="page-title">{{ $t(String(route.name)) }}</span>
    </v-app-bar-title>

    <template v-slot:append>
      <div class="app-bar-actions">
        <v-menu :close-on-content-click="false" location="bottom end">
          <template v-slot:activator="{ props }">
            <v-btn icon variant="text" size="small" v-bind="props" class="action-btn">
              <v-icon icon="mdi-translate" size="20" />
            </v-btn>
          </template>
          <v-card class="menu-card" elevation="8">
            <v-list density="compact" class="menu-dropdown">
              <v-list-item
                v-for="lang in languages"
                :key="lang.value"
                @click="changeLocale(lang.value)"
                :active="isActiveLocale(lang.value)"
                class="dropdown-item"
              >
                <v-list-item-title>{{ lang.title }}</v-list-item-title>
              </v-list-item>
            </v-list>
          </v-card>
        </v-menu>

        <v-menu :close-on-content-click="false" location="bottom end">
          <template v-slot:activator="{ props }">
            <v-btn icon variant="text" size="small" v-bind="props" class="action-btn">
              <v-icon icon="mdi-palette-outline" size="20" />
            </v-btn>
          </template>
          <v-card class="menu-card theme-card" elevation="8">
            <div class="theme-grid">
              <button
                v-for="th in themes"
                :key="th.value"
                @click="changeTheme(th.value)"
                class="theme-chip"
                :class="{ 'theme-chip--active': isActiveTheme(th.value) }"
              >
                <v-icon :icon="th.icon" size="18" />
                <span>{{ $t('theme.' + th.value) }}</span>
              </button>
            </div>
          </v-card>
        </v-menu>
      </div>
    </template>
  </v-app-bar>
</template>

<script lang="ts" setup>
import { useLocale, useTheme } from 'vuetify'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { languages } from '@/locales'

defineProps(['isMobile', 'menuPosition', 'menuItems'])

const route = useRoute()
const { locale: i18nLocale } = useI18n()
const vuetifyLocale = useLocale()
const theme = useTheme()

const changeLocale = (l: string) => {
  i18nLocale.value = l
  vuetifyLocale.current.value = l
  localStorage.setItem('locale', l)
  window.location.reload()
}
const isActiveLocale = (l: string) => i18nLocale.value === l

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'midnight', icon: 'mdi-weather-night' },
  { value: 'ocean', icon: 'mdi-waves' },
  { value: 'sunset', icon: 'mdi-weather-sunset' },
  { value: 'forest', icon: 'mdi-pine-tree' },
  { value: 'sakura', icon: 'mdi-flower' },
  { value: 'cyberpunk', icon: 'mdi-robot' },
  { value: 'nord', icon: 'mdi-snowflake' },
  { value: 'dracula', icon: 'mdi-bat' },
  { value: 'system', icon: 'mdi-laptop' },
]

const changeTheme = (th: string) => {
  theme.change(th)
  localStorage.setItem('theme', th)
}
const isActiveTheme = (th: string) => {
  const current = localStorage.getItem('theme') ?? 'system'
  return current == th
}
</script>

<style scoped>
.app-bar {
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  background: rgba(var(--v-theme-surface), 0.72) !important;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.06);
}

.app-bar-title {
  display: flex;
  align-items: center;
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.app-bar-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-right: 8px;
}

.action-btn {
  opacity: 0.7;
  transition: all 0.2s ease;
}

.action-btn:hover {
  opacity: 1;
  background: rgba(var(--v-theme-primary), 0.08);
}

.menu-toggle {
  margin-left: 4px;
}

.menu-card {
  border-radius: 14px !important;
  overflow: hidden;
  min-width: 140px;
}

.menu-dropdown {
  padding: 4px !important;
  background: transparent !important;
}

.dropdown-item {
  border-radius: 8px !important;
  min-height: 36px;
}

.theme-card {
  min-width: 240px;
}

.theme-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  padding: 8px;
}

.theme-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  background: transparent;
  cursor: pointer;
  font-size: 12.5px;
  font-weight: 500;
  color: rgb(var(--v-theme-on-surface));
  transition: all 0.2s ease;
  font-family: inherit;
}

.theme-chip:hover {
  background: rgba(var(--v-theme-primary), 0.08);
}

.theme-chip--active {
  background: rgba(var(--v-theme-primary), 0.12);
  border-color: rgba(var(--v-theme-primary), 0.3);
  color: rgb(var(--v-theme-primary));
}
</style>