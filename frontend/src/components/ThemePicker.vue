<template>
  <div class="theme-picker" :class="{ 'theme-picker--compact': compact }">
    <template v-for="group in groups" :key="group.key">
      <div class="theme-picker__label">{{ $t(group.label) }}</div>
      <div class="theme-picker__grid">
        <button
          v-for="th in group.items"
          :key="th.value"
          type="button"
          class="theme-tile"
          :class="{ 'theme-tile--active': active === th.value }"
          :title="$t('theme.' + th.value)"
          @click="choose(th.value)"
        >
          <span class="theme-tile__preview" :style="previewStyle(th.value)">
            <span class="theme-tile__bar"></span>
            <span class="theme-tile__card">
              <span class="theme-tile__dot"></span>
              <span class="theme-tile__line"></span>
            </span>
          </span>
          <span class="theme-tile__name">
            <v-icon :icon="th.value === systemTheme ? 'mdi-laptop' : th.icon" size="14" />
            <span>{{ $t('theme.' + th.value) }}</span>
          </span>
          <v-icon v-if="active === th.value" class="theme-tile__check" icon="mdi-check-circle" size="16" />
        </button>
      </div>
    </template>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useTheme } from 'vuetify'
import { appearance, resolveTheme, setAppearance, systemTheme, themeChoices, themeSwatch } from '@/plugins/themes'

defineProps<{ compact?: boolean }>()

const theme = useTheme()
const active = computed(() => appearance.theme)

const groups = [
  { key: 'light', label: 'setting.themeLightGroup', items: themeChoices.filter(t => !t.dark) },
  { key: 'dark', label: 'setting.themeDarkGroup', items: themeChoices.filter(t => t.dark) },
  { key: 'auto', label: 'setting.themeAutoGroup', items: [{ value: systemTheme, icon: 'mdi-laptop', dark: false }] },
]

const previewStyle = (name: string) => {
  if (name !== systemTheme) {
    const c = themeSwatch(theme, name)
    return {
      '--tile-bg': c.background,
      '--tile-surface': c.surface,
      '--tile-primary': c.primary,
      '--tile-secondary': c.secondary,
      '--tile-ink': c.ink,
    }
  }
  // Half light, half dark: the two themes "system" switches between.
  const light = themeSwatch(theme, appearance.themeLight)
  const dark = themeSwatch(theme, appearance.themeDark)
  const now = themeSwatch(theme, resolveTheme(systemTheme))
  return {
    '--tile-bg': `linear-gradient(135deg, ${light.background} 50%, ${dark.background} 50%)`,
    '--tile-bar': 'transparent',
    '--tile-surface': now.surface,
    '--tile-primary': now.primary,
    '--tile-secondary': now.secondary,
    '--tile-ink': now.ink,
  }
}

const choose = (name: string) => setAppearance('theme', name)
</script>

<style scoped>
.theme-picker__label {
  margin: 4px 2px 6px;
  font-size: 12px;
  font-weight: 600;
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.theme-picker__label:not(:first-child) {
  margin-top: 12px;
}

.theme-picker__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
  gap: 8px;
}

.theme-tile {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.1);
  border-radius: var(--app-control-radius, 10px);
  background: rgb(var(--v-theme-surface));
  color: rgb(var(--v-theme-on-surface));
  font: inherit;
  text-align: start;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.theme-tile:hover {
  border-color: rgba(var(--v-theme-primary), 0.45);
}

.theme-tile:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 1px;
}

.theme-tile--active {
  border-color: rgb(var(--v-theme-primary));
  box-shadow: 0 0 0 1px rgb(var(--v-theme-primary));
}

.theme-tile__preview {
  position: relative;
  display: block;
  height: 46px;
  overflow: hidden;
  border-radius: calc(var(--app-control-radius, 10px) - 3px);
  background: var(--tile-bg);
  border: 1px solid rgba(128, 128, 128, 0.18);
}

.theme-tile__bar {
  position: absolute;
  inset: 0 auto 0 0;
  width: 18%;
  background: var(--tile-bar, var(--tile-surface));
  border-inline-end: 1px solid rgba(128, 128, 128, 0.18);
}

.theme-tile__card {
  position: absolute;
  inset: 8px 8px 8px 26%;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 6px;
  border-radius: 4px;
  background: var(--tile-surface);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.theme-tile__dot {
  flex: 0 0 auto;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--tile-primary);
}

.theme-tile__line {
  flex: 1 1 auto;
  height: 4px;
  border-radius: 2px;
  background: var(--tile-ink);
  opacity: 0.35;
  box-shadow: 0 -6px 0 -1px var(--tile-secondary);
}

.theme-tile__name {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  font-size: 12.5px;
  font-weight: 500;
}

.theme-tile__name span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.theme-tile--active .theme-tile__name {
  color: rgb(var(--v-theme-primary));
}

.theme-tile__check {
  position: absolute;
  top: 2px;
  right: 2px;
  color: rgb(var(--v-theme-primary));
  background: rgb(var(--v-theme-surface));
  border-radius: 50%;
}

.theme-picker--compact .theme-picker__grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 6px;
}

.theme-picker--compact .theme-tile__preview {
  height: 34px;
}

.theme-picker--compact .theme-tile__card {
  inset: 6px 6px 6px 26%;
}

.theme-picker--compact .theme-tile__name {
  font-size: 12px;
}
</style>
