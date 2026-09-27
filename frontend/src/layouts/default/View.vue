<template>
  <v-main class="app-main">
    <div class="app-page" :class="{ 'app-page--route-enter': routeAnimating }">
      <v-alert
        v-if="showHostWarning"
        type="error"
        variant="tonal"
        density="comfortable"
        class="mb-4 host-req-alert"
        border="start"
        prominent
        icon="mdi-server-off"
      >
        <div class="text-subtitle-1 font-weight-bold">{{ $t('hostReq.title') }}</div>
        <div class="text-body-2 mt-1">{{ hostWarningText }}</div>
        <div class="text-caption mt-2 opacity-90">{{ $t('hostReq.hint') }}</div>
        <div class="text-caption mt-1">
          {{ $t('hostReq.minLabel') }}:
          {{ hostReq?.min_cpu_cores ?? 2 }} {{ $t('main.info.core') }} /
          {{ hostReq?.min_mem_gb ?? 2 }} GB
          ·
          {{ $t('hostReq.current') }}:
          {{ hostReq?.cpu_cores ?? '?' }} {{ $t('main.info.core') }} /
          {{ currentMemGb }} GB
        </div>
      </v-alert>
      <v-alert
        v-if="uiOutdated"
        type="warning"
        variant="tonal"
        density="comfortable"
        class="mb-4 host-req-alert"
        border="start"
        icon="mdi-update"
      >
        <div class="text-subtitle-2 font-weight-bold">{{ $t('uiOutdated.title') }}</div>
        <div class="text-body-2 mt-1">{{ $t('uiOutdated.text', { ui: uiOutdated.ui, panel: uiOutdated.panel }) }}</div>
      </v-alert>
      <router-view />
    </div>
  </v-main>
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Data from '@/store/modules/data'
import { i18n } from '@/locales'

const hostReq = computed(() => Data().hostRequirements)
const uiOutdated = computed(() => Data().uiOutdated)
const route = useRoute()
const routeAnimating = ref(false)
let routeAnimationTimer = 0

watch(() => route.path, async () => {
  routeAnimating.value = false
  await nextTick()
  routeAnimating.value = true
  window.clearTimeout(routeAnimationTimer)
  routeAnimationTimer = window.setTimeout(() => { routeAnimating.value = false }, 360)
})

onBeforeUnmount(() => window.clearTimeout(routeAnimationTimer))
// Only warn when used as cluster control plane (has agents) and under 2c/2G.
const showHostWarning = computed(() => {
  const r = hostReq.value
  return !!(r && r.applies === true && r.ok === false)
})
const currentMemGb = computed(() => {
  const bytes = hostReq.value?.mem_total_bytes
  if (!bytes) return '?'
  return (bytes / (1024 ** 3)).toFixed(2)
})
const hostWarningText = computed(() => {
  const r = hostReq.value
  if (!r) return ''
  const parts: string[] = []
  if (r.ok_cpu === false) {
    parts.push(i18n.global.t('hostReq.cpuFail', { current: r.cpu_cores, min: r.min_cpu_cores }))
  }
  if (r.ok_mem === false) {
    parts.push(i18n.global.t('hostReq.memFail', { current: currentMemGb.value, min: r.min_mem_gb }))
  }
  return parts.join(' · ') || i18n.global.t('hostReq.genericFail')
})
</script>

<style>
.app-main {
  min-height: 100vh;
  overflow-y: auto;
}

.app-page {
  --app-page-max: 1280px;
  position: relative;
  z-index: 1;
  width: min(100%, var(--app-page-max));
  margin-inline: auto;
  padding: 20px clamp(14px, 2.4vw, 32px) 40px;
  box-sizing: border-box;
}

.app-page:has(.home-dashboard) {
  width: 100%;
  max-width: none;
  padding: 0;
}

/* Keep host-requirement banner readable on full-bleed home page */
.app-page:has(.home-dashboard) > .host-req-alert {
  margin: 16px clamp(14px, 2.4vw, 32px) 0;
}

.host-req-alert {
  border-radius: 12px;
}

.app-root--drawer-expanded .app-page {
  --app-page-max: 1220px;
}

.app-page > .v-row {
  margin-inline: 0;
}

.app-page > .v-row + .v-row {
  margin-top: 14px;
}

.app-page > .page-toolbar,
.app-page > .v-row:first-of-type:not(.resource-grid) {
  align-items: center !important;
  justify-content: center !important;
  row-gap: 10px;
  margin-bottom: 14px;
}

.app-page > .page-toolbar > .v-col,
.app-page > .v-row:first-of-type:not(.resource-grid) > .v-col {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.app-page .page-toolbar__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.app-page .page-toolbar .ml-2,
.app-page > .v-row:first-of-type .ml-2 {
  margin-inline-start: 0 !important;
}

.app-page > .v-card {
  width: 100%;
  border-radius: 16px !important;
}

.resource-grid,
.app-page > .v-row:has(> .v-col > .v-card) {
  justify-content: center;
  align-items: stretch;
  row-gap: 14px;
}

.resource-col,
.app-page > .v-row:has(> .v-col > .v-card) > .v-col:has(> .v-card) {
  display: flex;
  flex: 0 1 266px !important;
  max-width: 288px !important;
  padding: 6px !important;
}

.resource-card,
.app-page > .v-row > .v-col > .v-card {
  width: 100%;
  min-width: 0 !important;
  min-height: 100%;
  display: flex !important;
  flex-direction: column;
  border-radius: 10px !important;
}

.resource-card > .v-card-title,
.app-page > .v-row > .v-col > .v-card > .v-card-title {
  justify-content: center;
  text-align: center;
  min-height: 44px;
  padding: 12px 14px 4px;
  font-size: 14px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.resource-card > .v-card-subtitle,
.app-page > .v-row > .v-col > .v-card > .v-card-subtitle {
  display: block;
  text-align: center;
  padding: 7px 12px;
  font-size: 12px;
  white-space: normal;
  overflow-wrap: anywhere;
}

.resource-card__body,
.app-page > .v-row > .v-col > .v-card > .v-card-text {
  flex: 1 1 auto;
  padding: 8px 14px !important;
  font-size: 13px;
}

.resource-row,
.app-page > .v-row > .v-col > .v-card > .v-card-text > .v-row {
  min-height: 28px;
  align-items: center;
  margin: 0;
  padding: 3px 0;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.06);
}

.resource-row:last-child,
.app-page > .v-row > .v-col > .v-card > .v-card-text > .v-row:last-child {
  border-bottom: 0;
}

.resource-row > .v-col,
.app-page > .v-row > .v-col > .v-card > .v-card-text > .v-row > .v-col {
  min-width: 0;
  padding: 2px 0;
  overflow-wrap: anywhere;
}

.resource-label,
.app-page > .v-row > .v-col > .v-card > .v-card-text > .v-row > .v-col:first-child {
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-weight: 500;
}

.resource-value,
.app-page > .v-row > .v-col > .v-card > .v-card-text > .v-row > .v-col:last-child {
  text-align: end;
  font-weight: 600;
}

.resource-actions,
.app-page > .v-row > .v-col > .v-card > .v-card-actions {
  justify-content: center;
  gap: 6px;
  min-height: 44px;
  padding: 6px 10px !important;
}

.resource-actions .v-btn,
.app-page > .v-row > .v-col > .v-card > .v-card-actions .v-btn {
  width: 32px;
  height: 32px;
}

.resource-actions .v-icon,
.app-page > .v-row > .v-col > .v-card > .v-card-actions .v-icon {
  font-size: 19px;
}

.app-page > .v-row > .v-col.v-card-subtitle {
  flex: 0 0 100% !important;
  max-width: 100% !important;
  margin-top: 8px;
  border-radius: 12px;
  background: rgba(var(--v-theme-surface), 0.45);
  backdrop-filter: blur(14px) saturate(160%);
  -webkit-backdrop-filter: blur(14px) saturate(160%);
}

.app-data-table {
  overflow: hidden;
  border-radius: 14px !important;
  background: rgba(var(--v-theme-surface), 0.72) !important;
  backdrop-filter: blur(16px) saturate(180%);
  -webkit-backdrop-filter: blur(16px) saturate(180%);
}

@media (max-width: 760px) {
  .app-page {
    padding: 16px 10px 32px;
  }

  .resource-col,
  .app-page > .v-row:has(> .v-col > .v-card) > .v-col:has(> .v-card) {
    flex-basis: min(100%, 340px) !important;
    max-width: min(100%, 340px) !important;
  }

  .resource-card__body,
  .app-page > .v-row > .v-col > .v-card > .v-card-text {
    padding: 10px 14px !important;
  }
}
</style>
