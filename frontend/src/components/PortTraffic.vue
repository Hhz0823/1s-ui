<template>
  <section class="port-traffic" :class="{ 'port-traffic--embedded': embedded }">
    <header class="traffic-header">
      <div>
        <h1 v-if="!embedded">{{ $t('portTraffic.title') }}</h1>
        <h2 v-else>{{ $t('portTraffic.title') }}</h2>
        <p>{{ $t(agentId ? 'portTraffic.remoteHint' : 'portTraffic.hint') }}</p>
      </div>
      <div class="traffic-actions">
        <span v-if="sampledAt" class="text-caption text-medium-emphasis">{{ $t('portTraffic.sampledAt') }} {{ sampledAt }}</span>
        <v-btn icon="mdi-refresh" variant="tonal" :loading="refreshing" :title="$t('actions.update')" @click="load" />
      </div>
    </header>

    <v-alert v-if="errorMessage" type="error" variant="tonal" density="compact">{{ errorMessage }}</v-alert>
    <v-progress-linear v-if="loading" indeterminate class="mb-3" />
    <v-alert v-else-if="!items.length && !errorMessage" type="info" variant="tonal">{{ $t('portTraffic.empty') }}</v-alert>

    <v-table v-if="!loading && items.length && !xs" class="traffic-table" density="compact">
      <thead>
        <tr>
          <th>{{ $t('objects.inbound') }}</th>
          <th>{{ $t('portTraffic.port') }}</th>
          <th>{{ $t('portTraffic.protocol') }}</th>
          <th>{{ $t('agent.status') }}</th>
          <th>{{ $t('portTraffic.realtime') }}</th>
          <th>{{ $t('portTraffic.total') }}</th>
          <th>{{ $t('portTraffic.monthly') }}</th>
          <th>{{ $t('portTraffic.limit') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="item.id">
          <td><strong>{{ item.tag }}</strong><small>#{{ item.id }}</small></td>
          <td dir="ltr">{{ address(item) }}</td>
          <td><strong>{{ item.type }}</strong><small>{{ item.core_type || 'sing-box' }}</small></td>
          <td><v-chip size="small" :color="!item.supported ? 'warning' : item.online ? 'success' : 'default'" variant="tonal">{{ !item.supported ? $t('portTraffic.unsupported') : item.online ? $t('online') : $t('agent.offline') }}</v-chip></td>
          <td dir="ltr">↑ {{ rate(item.upload_bps) }}<br>↓ {{ rate(item.download_bps) }}</td>
          <td dir="ltr">↑ {{ bytes(item.upload_bytes) }}<br>↓ {{ bytes(item.download_bytes) }}</td>
          <td class="traffic-quota">
            <div class="traffic-quota__line">
              <span dir="ltr">{{ quota(item) }}</span>
              <v-chip v-if="item.exhausted" size="x-small" color="error" variant="tonal">{{ $t('portTraffic.exhausted') }}</v-chip>
              <v-btn v-if="canReset(item)" icon="mdi-restore" size="x-small" variant="text" :loading="resetting == item.id" :title="$t('portTraffic.resetUsage')" @click="resetUsage(item)" />
            </div>
            <v-progress-linear v-if="item.traffic_limit > 0" :model-value="quotaPercent(item)" :color="quotaColor(item)" height="4" rounded />
            <small v-if="item.traffic_limit > 0">{{ $t('portTraffic.nextReset') }} {{ resetDate(item) }}</small>
          </td>
          <td>{{ limits(item) }}<small v-if="item.supported && item.ip_limit > 0">{{ ipText(item) }}</small></td>
        </tr>
      </tbody>
    </v-table>

    <div v-if="!loading && items.length && xs" class="traffic-cards">
      <article v-for="item in items" :key="item.id" class="traffic-card">
        <header>
          <div><strong>{{ item.tag }}</strong><small dir="ltr">{{ address(item) }}</small></div>
          <v-chip size="small" :color="!item.supported ? 'warning' : item.online ? 'success' : 'default'" variant="tonal">{{ !item.supported ? $t('portTraffic.unsupported') : item.online ? $t('online') : $t('agent.offline') }}</v-chip>
        </header>
        <div class="traffic-card__meta"><span>{{ item.type }}</span><span>{{ item.core_type || 'sing-box' }}</span></div>
        <dl>
          <div><dt>{{ $t('portTraffic.realtime') }}</dt><dd dir="ltr">↑ {{ rate(item.upload_bps) }} · ↓ {{ rate(item.download_bps) }}</dd></div>
          <div><dt>{{ $t('portTraffic.total') }}</dt><dd dir="ltr">↑ {{ bytes(item.upload_bytes) }} · ↓ {{ bytes(item.download_bytes) }}</dd></div>
          <div>
            <dt>{{ $t('portTraffic.monthly') }}</dt>
            <dd>
              <span dir="ltr">{{ quota(item) }}</span>
              <v-chip v-if="item.exhausted" size="x-small" color="error" variant="tonal" class="ms-1">{{ $t('portTraffic.exhausted') }}</v-chip>
              <v-btn v-if="canReset(item)" icon="mdi-restore" size="x-small" variant="text" :loading="resetting == item.id" :title="$t('portTraffic.resetUsage')" @click="resetUsage(item)" />
              <v-progress-linear v-if="item.traffic_limit > 0" :model-value="quotaPercent(item)" :color="quotaColor(item)" height="4" rounded class="mt-1" />
              <small v-if="item.traffic_limit > 0" class="d-block">{{ $t('portTraffic.nextReset') }} {{ resetDate(item) }}</small>
            </dd>
          </div>
          <div><dt>{{ $t('portTraffic.limit') }}</dt><dd>{{ limits(item) }}<small v-if="item.supported && item.ip_limit > 0" class="d-block">{{ ipText(item) }}</small></dd></div>
        </dl>
      </article>
    </div>
  </section>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useDisplay } from 'vuetify'
import { i18n } from '@/locales'
import { fetchBackendObject } from '@/utils/backend'
import HttpUtils from '@/plugins/httputil'

type TrafficItem = {
  id: number
  tag: string
  type: string
  core_type: string
  listen: string
  port: number
  online: boolean
  supported: boolean
  upload_bps: number
  download_bps: number
  upload_bytes: number
  download_bytes: number
  upload_limit: number
  download_limit: number
  traffic_limit: number
  traffic_used: number
  traffic_reset_day: number
  next_reset: number
  exhausted: boolean
  ip_limit: number
  active_ips: number
}

const props = defineProps<{ agentId?: number; embedded?: boolean }>()
const { xs } = useDisplay()
const items = ref<TrafficItem[]>([])
const sampled = ref(0)
const loading = ref(true)
const refreshing = ref(false)
const errorMessage = ref('')
const resetting = ref(0)
let timer: number | undefined

const endpoint = computed(() => props.agentId ? `api/agents/${props.agentId}/port-traffic` : 'api/port-traffic')
const sampledAt = computed(() => {
  if (!sampled.value) return ''
  const milliseconds = sampled.value > 1e12 ? sampled.value : sampled.value * 1000
  return new Date(milliseconds).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
})

const load = async () => {
  if (refreshing.value) return
  refreshing.value = true
  try {
    const result = await fetchBackendObject<{ sampled_at: number; items: TrafficItem[] }>(endpoint.value)
    sampled.value = Number(result?.sampled_at || 0)
    items.value = result?.items || []
    errorMessage.value = ''
  } catch (error: any) {
    errorMessage.value = error?.message || i18n.global.t('portTraffic.loadFailed')
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

const address = (item: TrafficItem) => {
  const host = item.listen?.includes(':') && !item.listen.startsWith('[') ? `[${item.listen}]` : item.listen || '*'
  return `${host}:${item.port}`
}
const rate = (value = 0) => value < 1024
  ? `${Math.round(value)} B/s`
  : value < 1024 ** 2 ? `${(value / 1024).toFixed(1)} KiB/s` : `${(value / 1024 ** 2).toFixed(2)} MiB/s`
const bytes = (value = 0) => value < 1024
  ? `${Math.round(value)} B`
  : value < 1024 ** 2 ? `${(value / 1024).toFixed(1)} KiB`
    : value < 1024 ** 3 ? `${(value / 1024 ** 2).toFixed(1)} MiB` : `${(value / 1024 ** 3).toFixed(2)} GiB`
const limit = (value = 0) => value > 0 ? rate(value) : i18n.global.t('portTraffic.unlimited')
const limits = (item: TrafficItem) => item.supported
  ? `↑ ${limit(item.upload_limit)} · ↓ ${limit(item.download_limit)}`
  : i18n.global.t('portTraffic.unsupported')

const quota = (item: TrafficItem) => item.traffic_limit > 0
  ? `${bytes(item.traffic_used)} / ${bytes(item.traffic_limit)}`
  : bytes(item.traffic_used)
const quotaPercent = (item: TrafficItem) => item.traffic_limit > 0
  ? Math.min(100, (item.traffic_used / item.traffic_limit) * 100)
  : 0
const quotaColor = (item: TrafficItem) => {
  const percent = quotaPercent(item)
  return percent >= 100 ? 'error' : percent >= 80 ? 'warning' : 'primary'
}
const resetDate = (item: TrafficItem) => item.next_reset
  ? new Date(item.next_reset * 1000).toLocaleDateString()
  : ''
const ipText = (item: TrafficItem) => `${i18n.global.t('portTraffic.activeIps')} ${item.active_ips || 0} / ${item.ip_limit}`
// Resetting goes to this panel's own inbounds only.
const canReset = (item: TrafficItem) => !props.agentId && item.supported && item.traffic_used > 0
const resetUsage = async (item: TrafficItem) => {
  if (resetting.value) return
  resetting.value = item.id
  try {
    const msg = await HttpUtils.post(`api/port-traffic/${item.id}/reset`, {})
    if (msg.success) await load()
  } finally {
    resetting.value = 0
  }
}

const handleVisibility = () => { if (!document.hidden) void load() }

onMounted(() => {
  void load()
  timer = window.setInterval(() => { if (!document.hidden) void load() }, 3000)
  document.addEventListener('visibilitychange', handleVisibility)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('visibilitychange', handleVisibility)
})
</script>

<style scoped>
.port-traffic { display: grid; gap: 16px; }
.traffic-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.traffic-header h1, .traffic-header h2 { margin: 0; line-height: 1.3; }
.traffic-header h1 { font-size: 1.35rem; }
.traffic-header h2 { font-size: 1.05rem; }
.traffic-header p { margin: 4px 0 0; color: rgba(var(--v-theme-on-surface), .62); }
.traffic-actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; }
.traffic-table { border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.traffic-table th { white-space: nowrap; }
.traffic-table td { padding-block: 10px !important; }
.traffic-table td strong, .traffic-table td small { display: block; }
.traffic-table td small { margin-top: 2px; color: rgba(var(--v-theme-on-surface), .58); }
.traffic-quota { min-width: 150px; }
.traffic-quota__line { display: flex; align-items: center; gap: 4px; }
.traffic-cards { display: grid; gap: 10px; }
.traffic-card { padding: 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.traffic-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.traffic-card header strong, .traffic-card header small { display: block; }
.traffic-card header small { margin-top: 3px; color: rgba(var(--v-theme-on-surface), .62); }
.traffic-card__meta { display: flex; gap: 8px; margin: 10px 0; color: rgba(var(--v-theme-on-surface), .62); font-size: .78rem; }
.traffic-card dl { display: grid; gap: 8px; margin: 0; }
.traffic-card dl > div { display: grid; grid-template-columns: 78px minmax(0, 1fr); gap: 8px; }
.traffic-card dt { color: rgba(var(--v-theme-on-surface), .58); }
.traffic-card dd { min-width: 0; margin: 0; text-align: end; overflow-wrap: anywhere; }
@media (max-width: 600px) {
  .traffic-header { align-items: stretch; flex-direction: column; }
  .traffic-actions { justify-content: space-between; }
}
</style>
