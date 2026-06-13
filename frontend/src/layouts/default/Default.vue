<template>
  <v-app class="app-root">
    <drawer :isMobile="isMobile" :displayDrawer="displayDrawer" @toggleDrawer="toggleDrawer" />
    <default-bar :isMobile="isMobile" @toggleDrawer="toggleDrawer" />
    <default-view />
  </v-app>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'
import DefaultBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import DefaultView from './View.vue'
import { useDisplay } from 'vuetify'

const { smAndDown } = useDisplay()
const displayDrawer = ref(false)

const toggleDrawer = () => {
  displayDrawer.value = !displayDrawer.value
}

const isMobile = computed((): boolean => {
  displayDrawer.value = !smAndDown.value
  return smAndDown.value
})
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