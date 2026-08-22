<template>
  <section class="user-traffic-page">
    <header class="page-header">
      <div>
        <h1 class="adaptive-ink">{{ $t('userTraffic.title') }}</h1>
        <p class="adaptive-ink">{{ $t('userTraffic.hint') }}</p>
      </div>
      <v-btn icon="mdi-refresh" variant="tonal" :loading="refreshing" :title="$t('actions.update')" @click="load" />
    </header>

    <div class="range-toolbar">
      <v-btn-toggle v-model="range" mandatory divided color="primary" density="comfortable" @update:model-value="rangeChanged">
        <v-btn v-for="option in rangeOptions" :key="option.value" :value="option.value">{{ option.title }}</v-btn>
      </v-btn-toggle>
      <span v-if="result.start && result.end" class="range-caption adaptive-ink" dir="ltr">{{ rangeCaption }}</span>
    </div>

    <div class="view-toolbar">
      <v-btn-toggle v-model="viewMode" mandatory divided color="primary" density="comfortable" @update:model-value="viewChanged">
        <v-btn value="combined" prepend-icon="mdi-view-dashboard-outline">{{ $t('userTraffic.combinedView') }}</v-btn>
        <v-btn value="trend" prepend-icon="mdi-chart-line">{{ $t('userTraffic.trendView') }}</v-btn>
        <v-btn value="ranking" prepend-icon="mdi-format-list-numbered">{{ $t('userTraffic.rankingView') }}</v-btn>
      </v-btn-toggle>
    </div>

    <div v-if="range === 'custom'" class="custom-range">
      <v-text-field v-model="customStart" type="datetime-local" :label="$t('stats.from')" density="compact" variant="outlined" hide-details />
      <v-text-field v-model="customEnd" type="datetime-local" :label="$t('stats.to')" density="compact" variant="outlined" hide-details />
      <v-btn color="primary" variant="tonal" prepend-icon="mdi-check" @click="applyCustomRange">{{ $t('userTraffic.applyRange') }}</v-btn>
    </div>

    <v-alert v-if="errorMessage" type="error" variant="tonal" density="compact">{{ errorMessage }}</v-alert>
    <v-alert v-else-if="!loading && !result.enabled" type="warning" variant="tonal" density="compact">
      {{ $t('userTraffic.historyDisabled') }}
    </v-alert>
    <v-progress-linear v-if="loading" indeterminate />

    <template v-if="!loading && result.enabled">
      <section class="summary-grid">
        <div>
          <span>{{ $t('userTraffic.totalTraffic') }}</span>
          <strong dir="ltr">{{ bytes(result.summary.total_bytes) }}</strong>
          <small dir="ltr">↑ {{ bytes(result.summary.upload_bytes) }} · ↓ {{ bytes(result.summary.download_bytes) }}</small>
        </div>
        <div>
          <span>{{ $t('userTraffic.averageBandwidth') }}</span>
          <strong dir="ltr">{{ rate(result.summary.average_upload_bytes_per_sec + result.summary.average_download_bytes_per_sec) }}</strong>
          <small dir="ltr">↑ {{ rate(result.summary.average_upload_bytes_per_sec) }} · ↓ {{ rate(result.summary.average_download_bytes_per_sec) }}</small>
        </div>
        <div>
          <span>{{ $t('userTraffic.peakBandwidth') }}</span>
          <strong dir="ltr">{{ rate(result.summary.peak_upload_bytes_per_sec + result.summary.peak_download_bytes_per_sec) }}</strong>
          <small dir="ltr">↑ {{ rate(result.summary.peak_upload_bytes_per_sec) }} · ↓ {{ rate(result.summary.peak_download_bytes_per_sec) }}</small>
        </div>
        <div>
          <span>{{ $t('userTraffic.activeUsers') }}</span>
          <strong>{{ result.summary.active_users }}</strong>
          <small>{{ $t('userTraffic.retention', { days: result.retention_days, seconds: result.bucket_seconds }) }}</small>
        </div>
      </section>

      <section v-if="viewMode !== 'ranking'" class="trend-panel">
        <header class="trend-header">
          <div>
            <h2>{{ $t('userTraffic.trendTitle') }}</h2>
            <p>{{ $t('userTraffic.trendHint') }}</p>
          </div>
          <div class="trend-controls">
            <v-select
              v-model="chartUser"
              :items="chartUserOptions"
              :label="$t('userTraffic.chartUser')"
              density="compact"
              variant="outlined"
              hide-details
              @update:model-value="loadTrend"
            />
            <v-btn-toggle v-model="chartMetric" mandatory divided color="primary" density="compact">
              <v-btn value="bandwidth">{{ $t('userTraffic.bandwidthView') }}</v-btn>
              <v-btn value="traffic">{{ $t('userTraffic.trafficView') }}</v-btn>
            </v-btn-toggle>
          </div>
        </header>
        <v-progress-linear v-if="chartLoading" indeterminate />
        <v-alert v-else-if="trendError" type="error" variant="tonal" density="compact">{{ trendError }}</v-alert>
        <v-alert v-else-if="!chartHasData" type="info" variant="tonal" density="compact">{{ $t('userTraffic.trendEmpty') }}</v-alert>
        <div v-else class="trend-chart"><Line :data="chartData" :options="chartOptions as any" /></div>
      </section>

      <section v-if="viewMode !== 'trend'" class="ranking-section">
        <div class="ranking-toolbar">
          <div>
            <h2 class="adaptive-ink">{{ $t('userTraffic.ranking') }}</h2>
            <p class="adaptive-ink">{{ $t('userTraffic.rankingHint') }}</p>
          </div>
          <v-text-field
            v-model="query"
            prepend-inner-icon="mdi-magnify"
            :label="$t('userTraffic.search')"
            density="compact"
            variant="outlined"
            clearable
            hide-details
          />
          <v-select v-model="sortKey" :items="sortOptions" density="compact" variant="outlined" hide-details />
        </div>

        <v-alert v-if="!sortedItems.length" type="info" variant="tonal">{{ $t('userTraffic.empty') }}</v-alert>

        <v-table v-else-if="!smAndDown" class="ranking-table" density="compact">
          <thead>
            <tr>
              <th>#</th>
              <th>{{ $t('client.name') }}</th>
              <th>{{ $t('agent.status') }}</th>
              <th>{{ $t('stats.upload') }} / {{ $t('stats.download') }}</th>
              <th>{{ $t('userTraffic.totalTraffic') }}</th>
              <th>{{ $t('userTraffic.averageBandwidth') }}</th>
              <th>{{ $t('userTraffic.peakBandwidth') }}</th>
              <th>{{ $t('userTraffic.lastActive') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(item, index) in pagedItems" :key="item.name">
              <td><v-chip size="small" :color="rankColor(displayRank(index))" variant="tonal">{{ displayRank(index) }}</v-chip></td>
              <td><strong>{{ item.name }}</strong><small>{{ [item.group, item.description].filter(Boolean).join(' · ') || '-' }}</small></td>
              <td><v-chip size="small" :color="statusColor(item)" variant="tonal">{{ statusLabel(item) }}</v-chip></td>
              <td dir="ltr">↑ {{ bytes(item.upload_bytes) }}<br>↓ {{ bytes(item.download_bytes) }}</td>
              <td>
                <strong dir="ltr">{{ bytes(item.total_bytes) }}</strong>
                <v-progress-linear :model-value="trafficPercent(item.total_bytes)" height="4" rounded color="primary" />
              </td>
              <td dir="ltr">↑ {{ rate(item.average_upload_bytes_per_sec) }}<br>↓ {{ rate(item.average_download_bytes_per_sec) }}</td>
              <td dir="ltr">↑ {{ rate(item.peak_upload_bytes_per_sec) }}<br>↓ {{ rate(item.peak_download_bytes_per_sec) }}</td>
              <td>{{ dateTime(item.last_active) }}</td>
            </tr>
          </tbody>
        </v-table>

        <div v-else-if="sortedItems.length" class="ranking-cards">
          <article v-for="(item, index) in pagedItems" :key="item.name">
            <header>
              <div class="rank-name"><v-chip size="small" :color="rankColor(displayRank(index))" variant="tonal">#{{ displayRank(index) }}</v-chip><strong>{{ item.name }}</strong></div>
              <v-chip size="small" :color="statusColor(item)" variant="tonal">{{ statusLabel(item) }}</v-chip>
            </header>
            <p>{{ [item.group, item.description].filter(Boolean).join(' · ') || '-' }}</p>
            <v-progress-linear :model-value="trafficPercent(item.total_bytes)" height="5" rounded color="primary" />
            <dl>
              <div><dt>{{ $t('userTraffic.totalTraffic') }}</dt><dd dir="ltr">{{ bytes(item.total_bytes) }}</dd></div>
              <div><dt>{{ $t('stats.upload') }} / {{ $t('stats.download') }}</dt><dd dir="ltr">↑ {{ bytes(item.upload_bytes) }} · ↓ {{ bytes(item.download_bytes) }}</dd></div>
              <div><dt>{{ $t('userTraffic.averageBandwidth') }}</dt><dd dir="ltr">↑ {{ rate(item.average_upload_bytes_per_sec) }} · ↓ {{ rate(item.average_download_bytes_per_sec) }}</dd></div>
              <div><dt>{{ $t('userTraffic.peakBandwidth') }}</dt><dd dir="ltr">↑ {{ rate(item.peak_upload_bytes_per_sec) }} · ↓ {{ rate(item.peak_download_bytes_per_sec) }}</dd></div>
              <div><dt>{{ $t('userTraffic.lastActive') }}</dt><dd>{{ dateTime(item.last_active) }}</dd></div>
            </dl>
          </article>
        </div>

        <v-pagination v-if="pageCount > 1" v-model="page" :length="pageCount" :total-visible="smAndDown ? 5 : 7" density="comfortable" />
      </section>
    </template>
  </section>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Line } from 'vue-chartjs'
import { useDisplay, useTheme } from 'vuetify'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { i18n } from '@/locales'
import { fetchBackendObject } from '@/utils/backend'

type RangeKey = '1h' | '6h' | '24h' | '7d' | '30d' | 'custom'
type ViewMode = 'combined' | 'trend' | 'ranking'
type ChartMetric = 'bandwidth' | 'traffic'
type TrafficItem = {
  rank: number
  name: string
  group: string
  description: string
  exists: boolean
  enabled: boolean
  online: boolean
  upload_bytes: number
  download_bytes: number
  total_bytes: number
  average_upload_bytes_per_sec: number
  average_download_bytes_per_sec: number
  peak_upload_bytes_per_sec: number
  peak_download_bytes_per_sec: number
  last_active: number
}
type TrafficSummary = {
  active_users: number
  upload_bytes: number
  download_bytes: number
  total_bytes: number
  average_upload_bytes_per_sec: number
  average_download_bytes_per_sec: number
  peak_upload_bytes_per_sec: number
  peak_download_bytes_per_sec: number
}
type TrafficResponse = {
  enabled: boolean
  start: number
  end: number
  bucket_seconds: number
  retention_days: number
  summary: TrafficSummary
  items: TrafficItem[]
}
type TrendResponse = {
  stats: Record<string, [number, number]>
  startTime: number
  bucketSpan: number
  numBuckets: number
}

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const emptySummary = (): TrafficSummary => ({
  active_users: 0, upload_bytes: 0, download_bytes: 0, total_bytes: 0,
  average_upload_bytes_per_sec: 0, average_download_bytes_per_sec: 0,
  peak_upload_bytes_per_sec: 0, peak_download_bytes_per_sec: 0,
})
const result = ref<TrafficResponse>({ enabled: true, start: 0, end: 0, bucket_seconds: 60, retention_days: 30, summary: emptySummary(), items: [] })
const { smAndDown } = useDisplay()
const theme = useTheme()
const range = ref<RangeKey>('24h')
const viewMode = ref<ViewMode>('combined')
const chartMetric = ref<ChartMetric>('bandwidth')
const chartUser = ref('__all__')
const trend = ref<TrendResponse>({ stats: {}, startTime: 0, bucketSpan: 60, numBuckets: 0 })
const customStart = ref('')
const customEnd = ref('')
const query = ref('')
const sortKey = ref('total')
const page = ref(1)
const pageSize = 20
const loading = ref(true)
const refreshing = ref(false)
const chartLoading = ref(false)
const errorMessage = ref('')
const trendError = ref('')
let timer: number | undefined
let trendRequest = 0

const ranges: Record<Exclude<RangeKey, 'custom'>, number> = { '1h': 3600, '6h': 21600, '24h': 86400, '7d': 604800, '30d': 2592000 }
const rangeOptions = computed(() => [
  { value: '1h', title: `1${i18n.global.t('date.h')}` },
  { value: '6h', title: `6${i18n.global.t('date.h')}` },
  { value: '24h', title: `24${i18n.global.t('date.h')}` },
  { value: '7d', title: `7${i18n.global.t('date.d')}` },
  { value: '30d', title: `30${i18n.global.t('date.d')}` },
  { value: 'custom', title: i18n.global.t('userTraffic.custom') },
])
const sortOptions = computed(() => [
  { value: 'total', title: i18n.global.t('userTraffic.sortTotal') },
  { value: 'upload', title: i18n.global.t('userTraffic.sortUpload') },
  { value: 'download', title: i18n.global.t('userTraffic.sortDownload') },
  { value: 'average', title: i18n.global.t('userTraffic.sortAverage') },
  { value: 'peak', title: i18n.global.t('userTraffic.sortPeak') },
  { value: 'recent', title: i18n.global.t('userTraffic.sortRecent') },
])
const chartUserOptions = computed(() => [
  { value: '__all__', title: i18n.global.t('userTraffic.allUsers') },
  ...result.value.items.map(item => ({ value: item.name, title: item.name })),
])
const trendValues = (direction: 0 | 1) => Array.from({ length: trend.value.numBuckets }, (_, index) => {
  const value = trend.value.stats[String(index)]?.[direction] || 0
  return chartMetric.value === 'bandwidth' ? value / Math.max(1, trend.value.bucketSpan) : value
})
const chartHasData = computed(() => Object.values(trend.value.stats).some(value => value[0] > 0 || value[1] > 0))
const chartLabels = computed(() => Array.from({ length: trend.value.numBuckets }, (_, index) => {
  const date = new Date((trend.value.startTime + index * trend.value.bucketSpan) * 1000)
  const longRange = result.value.end - result.value.start > 2 * 86400
  return date.toLocaleString([], {
    month: longRange ? '2-digit' : undefined,
    day: longRange ? '2-digit' : undefined,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}))
const chartData = computed(() => ({
  labels: chartLabels.value,
  datasets: [
    { label: i18n.global.t('stats.upload'), data: trendValues(0), borderColor: '#f59e0b', backgroundColor: '#f59e0b1f', borderWidth: 2, pointRadius: 0, pointHitRadius: 8, tension: 0.28, fill: true },
    { label: i18n.global.t('stats.download'), data: trendValues(1), borderColor: '#0891b2', backgroundColor: '#0891b21f', borderWidth: 2, pointRadius: 0, pointHitRadius: 8, tension: 0.28, fill: true },
  ],
}))
const chartOptions = computed(() => {
  const onSurface = theme.current.value.colors['on-surface']
  const format = (value: number) => chartMetric.value === 'bandwidth' ? rate(value) : bytes(value)
  return {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: { labels: { color: onSurface, boxWidth: 10, usePointStyle: true } },
      tooltip: { callbacks: { label: (context: any) => `${context.dataset.label}: ${format(context.parsed.y || 0)}` } },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: onSurface, maxTicksLimit: smAndDown.value ? 4 : 8 } },
      y: { beginAtZero: true, grid: { color: `${onSurface}1a` }, ticks: { color: onSurface, callback: (value: any) => format(Number(value)) } },
    },
  }
})

const sortedItems = computed(() => {
  const search = query.value?.trim().toLocaleLowerCase() || ''
  const items = result.value.items.filter(item => !search || [item.name, item.group, item.description].some(value => value?.toLocaleLowerCase().includes(search)))
  const score = (item: TrafficItem) => {
    if (sortKey.value === 'upload') return item.upload_bytes
    if (sortKey.value === 'download') return item.download_bytes
    if (sortKey.value === 'average') return item.average_upload_bytes_per_sec + item.average_download_bytes_per_sec
    if (sortKey.value === 'peak') return item.peak_upload_bytes_per_sec + item.peak_download_bytes_per_sec
    if (sortKey.value === 'recent') return item.last_active
    return item.total_bytes
  }
  return items.slice().sort((a, b) => score(b) - score(a) || a.name.localeCompare(b.name))
})
const pageCount = computed(() => Math.max(1, Math.ceil(sortedItems.value.length / pageSize)))
const pagedItems = computed(() => sortedItems.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const maxTraffic = computed(() => Math.max(1, ...sortedItems.value.map(item => item.total_bytes)))
const rangeCaption = computed(() => `${new Date(result.value.start * 1000).toLocaleString()} — ${new Date(result.value.end * 1000).toLocaleString()}`)

const requestWindow = () => {
  if (range.value === 'custom') {
    return { start: Math.floor(new Date(customStart.value).getTime() / 1000), end: Math.floor(new Date(customEnd.value).getTime() / 1000) }
  }
  const end = Math.floor(Date.now() / 1000)
  return { start: end - ranges[range.value], end }
}
const loadTrend = async () => {
  if (!result.value.enabled || !result.value.start || !result.value.end) {
    trend.value = { stats: {}, startTime: 0, bucketSpan: 60, numBuckets: 0 }
    return
  }
  const request = ++trendRequest
  chartLoading.value = true
  try {
    const params = new URLSearchParams({
      resource: 'user',
      tag: chartUser.value === '__all__' ? '' : chartUser.value,
      start: String(result.value.start),
      end: String(result.value.end),
    })
    const data = await fetchBackendObject<TrendResponse>(`api/stats?${params}`)
    if (request !== trendRequest) return
    trend.value = data
    trendError.value = ''
  } catch (error: any) {
    if (request !== trendRequest) return
    trend.value = { stats: {}, startTime: result.value.start, bucketSpan: result.value.bucket_seconds, numBuckets: 0 }
    trendError.value = error?.message || i18n.global.t('userTraffic.trendLoadFailed')
  } finally {
    if (request === trendRequest) chartLoading.value = false
  }
}
const load = async () => {
  if (refreshing.value) return
  const window = requestWindow()
  if (!Number.isFinite(window.start) || !Number.isFinite(window.end) || window.start >= window.end) {
    errorMessage.value = i18n.global.t('userTraffic.invalidRange')
    return
  }
  refreshing.value = true
  try {
    const params = new URLSearchParams({ start: String(window.start), end: String(window.end), limit: '200' })
    result.value = await fetchBackendObject<TrafficResponse>(`api/user-traffic?${params}`)
    if (chartUser.value !== '__all__' && !result.value.items.some(item => item.name === chartUser.value)) chartUser.value = '__all__'
    errorMessage.value = ''
    if (viewMode.value !== 'ranking') await loadTrend()
  } catch (error: any) {
    errorMessage.value = error?.message || i18n.global.t('userTraffic.loadFailed')
  } finally {
    loading.value = false
    refreshing.value = false
  }
}
const rangeChanged = () => {
  if (range.value === 'custom') {
    const end = new Date()
    const start = new Date(end.getTime() - 86400000)
    customStart.value = localDateInput(start)
    customEnd.value = localDateInput(end)
    return
  }
  void load()
}
const viewChanged = () => { if (viewMode.value !== 'ranking') void loadTrend() }
const applyCustomRange = () => { page.value = 1; void load() }
const localDateInput = (value: Date) => {
  const offset = value.getTimezoneOffset() * 60000
  return new Date(value.getTime() - offset).toISOString().slice(0, 16)
}
const displayRank = (index: number) => (page.value - 1) * pageSize + index + 1
const rankColor = (rank: number) => rank === 1 ? 'warning' : rank === 2 ? 'secondary' : rank === 3 ? 'info' : 'default'
const statusColor = (item: TrafficItem) => !item.exists ? 'error' : item.online ? 'success' : item.enabled ? 'default' : 'warning'
const statusLabel = (item: TrafficItem) => !item.exists ? i18n.global.t('userTraffic.deleted') : item.online ? i18n.global.t('online') : item.enabled ? i18n.global.t('agent.offline') : i18n.global.t('disable')
const trafficPercent = (value: number) => Math.max(0, Math.min(100, value * 100 / maxTraffic.value))
const dateTime = (unix: number) => unix > 0 ? new Date(unix * 1000).toLocaleString() : '-'
const bytes = (value = 0) => {
  if (value < 1024) return `${Math.round(value)} B`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KiB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MiB`
  if (value < 1024 ** 4) return `${(value / 1024 ** 3).toFixed(2)} GiB`
  return `${(value / 1024 ** 4).toFixed(2)} TiB`
}
const rate = (value = 0) => value < 1024 ? `${Math.round(value)} B/s` : value < 1024 ** 2 ? `${(value / 1024).toFixed(1)} KiB/s` : value < 1024 ** 3 ? `${(value / 1024 ** 2).toFixed(2)} MiB/s` : `${(value / 1024 ** 3).toFixed(2)} GiB/s`
const autoRefreshEnabled = () => range.value === '1h' || range.value === '6h' || range.value === '24h'
const handleVisibility = () => { if (!document.hidden && autoRefreshEnabled()) void load() }

watch([query, sortKey], () => { page.value = 1 })
onMounted(() => {
  void load()
  timer = window.setInterval(() => { if (!document.hidden && autoRefreshEnabled()) void load() }, 30000)
  document.addEventListener('visibilitychange', handleVisibility)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('visibilitychange', handleVisibility)
})
</script>

<style scoped>
.user-traffic-page { display: grid; gap: 16px; padding-bottom: 8px; }
.page-header, .range-toolbar, .ranking-toolbar, .trend-header { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.page-header h1, .ranking-toolbar h2, .trend-header h2 { margin: 0; letter-spacing: 0; }
.page-header h1 { font-size: 1.35rem; }
.ranking-toolbar h2, .trend-header h2 { font-size: 1.05rem; }
.page-header p, .ranking-toolbar p, .trend-header p { margin: 4px 0 0; color: rgba(var(--v-theme-on-surface), .62); }
.range-toolbar { flex-wrap: wrap; }
.range-toolbar :deep(.v-btn-toggle) { height: auto; flex-wrap: wrap; }
.range-caption { color: rgba(var(--v-theme-on-surface), .58); font-size: .76rem; }
.view-toolbar { display: flex; justify-content: center; }
.view-toolbar :deep(.v-btn-toggle) { height: auto; }
.custom-range { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; align-items: center; gap: 10px; }
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; overflow: hidden; background: rgb(var(--v-theme-surface)); }
.summary-grid > div { min-width: 0; padding: 14px 16px; border-right: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.summary-grid > div:last-child { border-right: 0; }
.summary-grid span, .summary-grid strong, .summary-grid small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.summary-grid span { color: rgba(var(--v-theme-on-surface), .58); font-size: .74rem; }
.summary-grid strong { margin-top: 5px; font-size: 1.05rem; }
.summary-grid small { margin-top: 4px; color: rgba(var(--v-theme-on-surface), .58); font-size: .72rem; }
.trend-panel, .ranking-section { display: grid; gap: 14px; }
.trend-panel { min-width: 0; padding: 14px 16px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.trend-controls { display: flex; align-items: center; gap: 10px; }
.trend-controls > .v-select { width: 220px; }
.trend-chart { height: 300px; min-width: 0; }
.ranking-toolbar > :first-child { min-width: 220px; }
.ranking-toolbar > .v-input { max-width: 260px; }
.ranking-table { border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.ranking-table th { white-space: nowrap; }
.ranking-table td { padding-block: 10px !important; }
.ranking-table td strong, .ranking-table td small { display: block; }
.ranking-table td small { margin-top: 3px; color: rgba(var(--v-theme-on-surface), .58); max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ranking-table .v-progress-linear { min-width: 100px; margin-top: 6px; }
.ranking-cards { display: grid; gap: 10px; }
.ranking-cards article { padding: 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.ranking-cards header, .rank-name { display: flex; align-items: center; gap: 10px; }
.ranking-cards header { justify-content: space-between; }
.ranking-cards p { margin: 8px 0; color: rgba(var(--v-theme-on-surface), .58); font-size: .78rem; }
.ranking-cards dl { display: grid; gap: 8px; margin: 12px 0 0; }
.ranking-cards dl > div { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 8px; }
.ranking-cards dt { color: rgba(var(--v-theme-on-surface), .58); }
.ranking-cards dd { min-width: 0; margin: 0; text-align: end; overflow-wrap: anywhere; }
@media (max-width: 900px) {
  .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .summary-grid > div:nth-child(2) { border-right: 0; }
  .summary-grid > div:nth-child(-n+2) { border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
  .ranking-toolbar { align-items: stretch; flex-wrap: wrap; }
  .ranking-toolbar > :first-child { flex: 1 1 100%; }
  .ranking-toolbar > .v-input { flex: 1 1 220px; max-width: none; }
  .trend-header { align-items: stretch; flex-direction: column; }
  .trend-controls > .v-select { flex: 1 1 220px; width: auto; }
}
@media (max-width: 600px) {
  .page-header { align-items: flex-start; }
  .range-toolbar { align-items: stretch; }
  .range-toolbar :deep(.v-btn-toggle) { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); width: 100%; }
  .range-toolbar :deep(.v-btn) { min-width: 0; padding-inline: 8px; }
  .view-toolbar :deep(.v-btn-toggle) { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); width: 100%; }
  .view-toolbar :deep(.v-btn) { min-width: 0; padding-inline: 6px; }
  .custom-range { grid-template-columns: minmax(0, 1fr); }
  .summary-grid small { min-height: 2em; white-space: normal; line-height: 1.35; }
  .trend-panel { padding: 12px; }
  .trend-controls { align-items: stretch; flex-direction: column; }
  .trend-controls > .v-select { flex: 0 0 auto; width: 100%; }
  .trend-controls :deep(.v-btn-toggle) { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); width: 100%; }
  .trend-chart { height: 250px; }
}
@media (max-width: 360px) {
  .view-toolbar :deep(.v-btn__prepend) { display: none; }
}
</style>
