<template>
  <div class="ring-gauge" :title="tooltip || undefined">
    <div class="ring-gauge__title">{{ title }}</div>
    <div class="ring-gauge__ring">
      <svg viewBox="0 0 120 120" class="ring-gauge__svg" aria-hidden="true">
        <circle class="ring-gauge__track" cx="60" cy="60" :r="radius" />
        <circle
          class="ring-gauge__value"
          cx="60"
          cy="60"
          :r="radius"
          :stroke="`rgb(var(--v-theme-${tone}))`"
          :stroke-dasharray="circumference"
          :stroke-dashoffset="offset"
        />
      </svg>
      <div class="ring-gauge__center">
        <span class="ring-gauge__percent">{{ percentText }}</span>
      </div>
    </div>
    <div class="ring-gauge__caption" :class="captionTone ? `text-${captionTone}` : ''">{{ caption }}</div>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'

const props = defineProps<{
  title: string
  percent: number | null
  caption?: string
  tooltip?: string
  // Colors the caption, e.g. BaoTa's load state.
  captionTone?: string
}>()

const radius = 50
const circumference = 2 * Math.PI * radius

const clamped = computed(() => {
  const value = Number(props.percent)
  return Number.isFinite(value) ? Math.min(100, Math.max(0, value)) : 0
})
const offset = computed(() => circumference * (1 - clamped.value / 100))
const percentText = computed(() => props.percent == null ? '-' : `${Math.round(clamped.value)}%`)
// Normal usage takes the theme color; high usage turns orange, then red.
const tone = computed(() => {
  if (clamped.value >= 90) return 'error'
  if (clamped.value >= 75) return 'warning'
  return 'primary'
})
</script>

<style scoped>
.ring-gauge {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 4px 8px;
  text-align: center;
}

.ring-gauge__title {
  color: rgba(var(--v-theme-on-surface), 0.72);
  font-size: 13px;
  font-weight: 500;
}

.ring-gauge__ring {
  position: relative;
  width: 112px;
  height: 112px;
}

.ring-gauge__svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.ring-gauge__track {
  fill: none;
  stroke: rgba(var(--v-theme-on-surface), 0.08);
  stroke-width: 9;
}

.ring-gauge__value {
  fill: none;
  stroke-width: 9;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.6s ease, stroke 0.3s ease;
}

.ring-gauge__center {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ring-gauge__percent {
  font-size: 22px;
  font-weight: 600;
  color: rgb(var(--v-theme-on-surface));
  font-variant-numeric: tabular-nums;
}

.ring-gauge__caption {
  min-height: 18px;
  max-width: 100%;
  overflow: hidden;
  color: rgba(var(--v-theme-on-surface), 0.6);
  font-size: 12.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 600px) {
  .ring-gauge__ring {
    width: 92px;
    height: 92px;
  }

  .ring-gauge__percent {
    font-size: 18px;
  }
}
</style>
