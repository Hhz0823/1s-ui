<template>
  <v-app
    class="app-root"
    :class="{
      'app-root--side-desktop': menuPosition !== 'top' && !isMobile,
      'app-root--drawer-expanded': drawerExpanded && menuPosition !== 'top' && !isMobile,
      'app-root--drawer-collapsed': !drawerExpanded && menuPosition !== 'top' && !isMobile,
    }"
  >
    <div v-if="bgImage" class="app-bg-image" :style="{ backgroundImage: `url(${bgImage})`, filter: `blur(${bgBlur}px) saturate(1.3)`, opacity: Number(bgOpacity) / 100 }"></div>
    <drawer
      v-if="menuPosition !== 'top' || isMobile"
      :isMobile="isMobile"
      :displayDrawer="drawerOpen"
      :expanded="drawerExpanded"
      @toggleDrawer="toggleDrawer"
      @closeDrawer="closeDrawer"
    />
    <default-bar
      :isMobile="isMobile"
      :menuPosition="menuPosition"
      :menuItems="menuItems"
      :drawerExpanded="drawerExpanded"
      @toggleDrawer="toggleDrawer"
    />
    <default-view />
  </v-app>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import DefaultBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import DefaultView from './View.vue'
import { useDisplay } from 'vuetify'

const { smAndDown } = useDisplay()
const drawerOpen = ref(false)
const drawerExpanded = ref(localStorage.getItem('drawerExpanded') === 'true')

const toggleDrawer = () => {
  if (isMobile.value) {
    drawerOpen.value = !drawerOpen.value
    return
  }

  if (menuPosition.value !== 'top') {
    drawerExpanded.value = !drawerExpanded.value
    localStorage.setItem('drawerExpanded', drawerExpanded.value ? 'true' : 'false')
  }
}

const closeDrawer = () => {
  if (isMobile.value) drawerOpen.value = false
}

const isMobile = computed((): boolean => {
  return smAndDown.value
})

import bgAsset from '@/assets/bg.jpg'
const bgImage = computed(() => localStorage.getItem('bgImage') || bgAsset)
const bgBlur = computed(() => localStorage.getItem('bgBlur') || '6')
const bgOpacity = computed(() => localStorage.getItem('bgOpacity') || '40')
const menuPosition = computed(() => localStorage.getItem('menuPosition') || 'side')

watch([smAndDown, menuPosition], ([mobile, position]) => {
  drawerOpen.value = !mobile && position !== 'top'
}, { immediate: true })

const menuItems = [
  { title: 'pages.home', icon: 'mdi-view-dashboard-outline', path: '/' },
  { title: 'pages.inbounds', icon: 'mdi-arrow-down-bold-circle-outline', path: '/inbounds' },
  { title: 'pages.clients', icon: 'mdi-account-group-outline', path: '/clients' },
  { title: 'pages.outbounds', icon: 'mdi-arrow-up-bold-circle-outline', path: '/outbounds' },
  { title: 'pages.endpoints', icon: 'mdi-access-point-network', path: '/endpoints' },
  { title: 'pages.services', icon: 'mdi-cog-outline', path: '/services' },
  { title: 'pages.tls', icon: 'mdi-shield-lock-outline', path: '/tls' },
  { title: 'pages.basics', icon: 'mdi-tune-variant', path: '/basics' },
  { title: 'pages.rules', icon: 'mdi-routes', path: '/rules' },
  { title: 'pages.dns', icon: 'mdi-dns-outline', path: '/dns' },
  { title: 'pages.admins', icon: 'mdi-account-tie-outline', path: '/admins' },
  { title: 'pages.settings', icon: 'mdi-cog-outline', path: '/settings' },
]
</script>

<style>
.app-root {
  background: rgb(var(--v-theme-background));
}

.v-card-subtitle {
  text-align: center;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  min-height: 20px;
}

.v-switch.v-input {
  padding-inline-start: 0.6rem;
}
</style>
