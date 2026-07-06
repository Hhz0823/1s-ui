<template>
  <v-main class="app-main">
    <div class="app-page">
      <router-view />
    </div>
  </v-main>
</template>

<script lang="ts" setup>
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
