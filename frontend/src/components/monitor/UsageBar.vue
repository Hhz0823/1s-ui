<template>
  <div class="usage-bar" :class="{ 'usage-bar--compact': compact }">
    <div v-if="!compact" class="usage-bar__head">
      <span>{{ label }}</span>
      <strong>{{ valueText }}</strong>
    </div>
    <div class="usage-bar__track">
      <i class="usage-bar__fill" :class="`usage-bar__fill--${tone}`" :style="{ transform: `scaleX(${fill / 100})` }" />
    </div>
    <small v-if="compact" class="usage-bar__value">{{ valueText }}</small>
    <small v-if="!compact && detail" class="usage-bar__detail" dir="ltr">{{ detail }}</small>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { clampPercent, percent, usageTone } from '@/utils/monitor'

const props = defineProps<{ label?: string, value?: number, detail?: string, compact?: boolean, text?: string }>()
const fill = computed(() => clampPercent(props.value))
const tone = computed(() => props.value == null ? 'idle' : usageTone(props.value))
const valueText = computed(() => props.text ?? percent(props.value))
</script>

<style scoped>
.usage-bar { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.usage-bar__head { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; font-size: 0.8rem; }
.usage-bar__head span { color: rgba(var(--v-theme-on-surface), 0.6); }
.usage-bar__head strong { font-weight: 500; white-space: nowrap; }
.usage-bar__track { position: relative; height: 8px; border-radius: 4px; overflow: hidden; background: rgba(var(--v-theme-on-surface), 0.1); }
.usage-bar--compact .usage-bar__track { height: 6px; border-radius: 3px; }
.usage-bar__fill { position: absolute; inset: 0; transform-origin: left center; border-radius: inherit; transition: transform 0.5s ease-out; }
.usage-bar__fill--success { background: #30a46c; }
.usage-bar__fill--warning { background: #f76b15; }
.usage-bar__fill--error { background: #e5484d; }
.usage-bar__fill--idle { background: transparent; }
.usage-bar__detail, .usage-bar__value { color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.7rem; line-height: 1.2; }
.usage-bar__value { text-align: center; }
</style>
