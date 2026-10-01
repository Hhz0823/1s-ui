<template>
  <div v-if="items.length" class="price-tags">
    <span v-for="item in items" :key="item.key" class="price-tag" :style="{ '--tag': item.color }">
      <i v-if="item.dot" class="price-tag__dot" />{{ item.label }}
    </span>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AgentNode } from '@/types/agents'
import { cycleKey, daysLeft, parseTags, tagPalette } from '@/utils/monitor'

const props = withDefaults(defineProps<{ node: AgentNode, showIp?: boolean }>(), { showIp: true })
const { t } = useI18n()

// Tags follow the monitor card: IP families, price, expiry, then custom tags.
const items = computed(() => {
  const node = props.node
  const result: { key: string, label: string, color: string, dot?: boolean }[] = []
  if (props.showIp) {
    if (node.report.ipv4?.length) result.push({ key: 'v4', label: 'V4', color: tagPalette.green, dot: true })
    if (node.report.ipv6?.length) result.push({ key: 'v6', label: 'V6', color: tagPalette.green, dot: true })
  }
  const price = Number(node.price || 0)
  if (price === -1) result.push({ key: 'price', label: t('monitor.free'), color: tagPalette.iris })
  else if (price > 0) {
    const key = cycleKey(node.billing_cycle)
    const cycle = key ? t(`monitor.cycle.${key}`) : t('monitor.cycleDays', { n: node.billing_cycle })
    result.push({ key: 'price', label: `${node.currency || '￥'}${price}/${cycle}`, color: tagPalette.iris })
  }
  const days = daysLeft(node.expire_at)
  if (days != null) {
    const label = days <= 0 ? t('monitor.expired') : days > 36500 ? t('monitor.longTerm') : t('monitor.expiresIn', { n: days })
    const color = days <= 7 ? tagPalette.red : days <= 15 ? tagPalette.orange : tagPalette.green
    result.push({ key: 'expire', label, color })
  }
  parseTags(node.tags).forEach((tag, index) => result.push({ key: `tag-${index}`, label: tag.label, color: tag.color }))
  return result
})
</script>

<style scoped>
.price-tags { display: flex; flex-wrap: wrap; gap: 4px; min-width: 0; }
.price-tag { display: inline-flex; align-items: center; gap: 4px; padding: 1px 6px; border-radius: 4px; font-size: 0.7rem; line-height: 1.5; white-space: nowrap; color: var(--tag); background: color-mix(in srgb, var(--tag) 14%, transparent); }
.price-tag__dot { width: 6px; height: 6px; border-radius: 50%; background: var(--tag); box-shadow: 0 0 0 2px color-mix(in srgb, var(--tag) 30%, transparent); }
</style>
