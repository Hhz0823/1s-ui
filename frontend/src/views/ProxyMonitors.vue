<template>
  <section class="proxy-monitors">
    <header class="pm-header">
      <div>
        <h1>{{ $t('proxyMonitor.title') }}</h1>
        <p>{{ $t('proxyMonitor.hint') }}</p>
      </div>
      <div class="pm-actions">
        <v-btn icon="mdi-refresh" variant="tonal" :loading="refreshing" :title="$t('actions.update')" @click="load" />
        <v-btn color="primary" prepend-icon="mdi-plus" @click="openEditor()">{{ $t('proxyMonitor.add') }}</v-btn>
      </div>
    </header>

    <div class="pm-summary">
      <div><span>{{ $t('proxyMonitor.status.up') }}</span><strong class="text-success">{{ count('up') }}</strong></div>
      <div><span>{{ $t('proxyMonitor.status.down') }}</span><strong class="text-error">{{ count('down') }}</strong></div>
      <div><span>{{ $t('proxyMonitor.status.unknown') }}</span><strong class="text-warning">{{ count('unknown') }}</strong></div>
      <div><span>{{ $t('proxyMonitor.total') }}</span><strong>{{ items.length }}</strong></div>
    </div>

    <v-alert v-if="errorMessage" type="error" variant="tonal" density="compact">{{ errorMessage }}</v-alert>
    <v-progress-linear v-if="loading" indeterminate />
    <v-alert v-else-if="!items.length && !errorMessage" type="info" variant="tonal">{{ $t('proxyMonitor.empty') }}</v-alert>

    <div class="pm-grid">
      <article v-for="item in items" :key="item.id" class="pm-card" @click="openDetail(item)">
        <header>
          <span class="pm-dot" :class="'pm-dot--' + item.status" />
          <strong class="pm-name">{{ item.name }}</strong>
          <v-chip size="x-small" variant="tonal" label>{{ protocol(item.type) }}</v-chip>
          <span class="pm-latency" :class="'text-' + statusColor(item.status)">
            {{ item.last?.ok ? item.last.latency_ms + ' ms' : $t('proxyMonitor.status.' + item.status) }}
          </span>
        </header>
        <div class="pm-meta" dir="auto">{{ item.host }}:{{ item.port }} · {{ checkedBy(item) }} · {{ every(item.interval) }}</div>
        <div class="pm-strip">
          <span v-for="(point, index) in strip(item)" :key="index" :class="point ? (point.ok ? 'ok' : 'fail') : ''"
            :style="point && point.ok ? { height: barHeight(item, point.latency_ms) } : undefined" :title="pointTitle(point)" />
        </div>
        <footer>
          <span v-if="item.checks">{{ $t('proxyMonitor.uptime24h') }} {{ item.uptime.toFixed(1) }}%</span>
          <span v-if="item.avg_latency > 0">{{ $t('proxyMonitor.avg') }} {{ Math.round(item.avg_latency) }} ms</span>
          <span v-if="item.last?.exit_ip" dir="ltr">{{ flag(item.last.country) }} {{ item.last.exit_ip }}</span>
        </footer>
        <div v-if="item.last && !item.last.ok && item.status !== 'paused'" class="pm-reason" :class="'text-' + statusColor(item.status)">
          {{ reason(item.last) }}
        </div>
        <div class="pm-card-actions" @click.stop>
          <v-btn size="small" variant="text" icon="mdi-play-circle-outline" :loading="checking === item.id" :title="$t('proxyMonitor.checkNow')" @click="checkNow(item)" />
          <v-btn size="small" variant="text" icon="mdi-pencil-outline" :title="$t('actions.edit')" @click="openEditor(item)" />
          <v-btn size="small" variant="text" icon="mdi-delete-outline" color="error" :title="$t('actions.del')" @click="remove(item)" />
        </div>
      </article>
    </div>

    <v-dialog v-model="editor.visible" max-width="600" scrollable>
      <v-card>
        <v-card-title>{{ editor.form.id ? $t('proxyMonitor.edit') : $t('proxyMonitor.add') }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="editor.form.link" :label="$t('proxyMonitor.link')" :hint="$t('proxyMonitor.linkHint')" persistent-hint
            placeholder="socks5://user:pass@203.0.113.5:1080" class="mb-3" />
          <v-text-field v-model="editor.form.name" :label="$t('proxyMonitor.name')" />
          <template v-if="!editor.form.link.trim()">
            <v-btn-toggle v-model="editor.form.type" mandatory density="compact" color="primary" variant="outlined" class="mb-4">
              <v-btn value="socks5">SOCKS5</v-btn>
              <v-btn value="http">HTTP</v-btn>
            </v-btn-toggle>
            <v-row dense>
              <v-col cols="8"><v-text-field v-model="editor.form.host" :label="$t('proxyMonitor.host')" dir="ltr" /></v-col>
              <v-col cols="4"><v-text-field v-model.number="editor.form.port" type="number" :label="$t('proxyMonitor.port')" dir="ltr" /></v-col>
            </v-row>
            <v-row dense>
              <v-col cols="6"><v-text-field v-model="editor.form.username" :label="$t('proxyMonitor.username')" autocomplete="off" /></v-col>
              <v-col cols="6">
                <v-text-field v-model="editor.password" type="password" :label="$t('proxyMonitor.password')" autocomplete="new-password"
                  :placeholder="editor.hadPassword ? $t('proxyMonitor.keepPassword') : ''" persistent-placeholder />
              </v-col>
            </v-row>
          </template>
          <v-select v-model="editor.form.server_id" :items="serverOptions" item-title="title" item-value="value"
            :label="$t('proxyMonitor.checkedBy')" :hint="$t('proxyMonitor.checkedByHint')" persistent-hint class="mb-3" />
          <v-select v-model="editor.form.interval" :items="intervalOptions" item-title="title" item-value="value" :label="$t('proxyMonitor.interval')" />
          <v-text-field v-model="editor.form.target" :label="$t('proxyMonitor.target')" :placeholder="$t('proxyMonitor.targetHint')" persistent-placeholder dir="ltr" />
          <v-switch v-model="editor.form.enabled" :label="$t('proxyMonitor.enabled')" color="primary" hide-details />
          <v-alert v-if="editor.error" type="error" variant="tonal" density="compact" class="mt-3">{{ editor.error }}</v-alert>
          <v-alert v-if="editor.result" :type="editor.result.ok ? 'success' : 'error'" variant="tonal" density="compact" class="mt-3">
            <template v-if="editor.result.ok">{{ resultLine(editor.result) }}</template>
            <template v-else>{{ reason(editor.result) }}</template>
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-btn variant="tonal" :loading="editor.testing" @click="test">{{ $t('proxyMonitor.test') }}</v-btn>
          <v-spacer />
          <v-btn variant="text" @click="editor.visible = false">{{ $t('actions.close') }}</v-btn>
          <v-btn color="primary" variant="tonal" :loading="editor.saving" @click="save">{{ $t('actions.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="detail.visible" max-width="760" scrollable>
      <v-card v-if="detail.item">
        <v-card-title class="d-flex align-center ga-2">
          <span class="pm-dot" :class="'pm-dot--' + detail.item.status" />
          <span class="text-truncate">{{ detail.item.name }}</span>
        </v-card-title>
        <v-card-subtitle dir="auto">{{ detail.item.host }}:{{ detail.item.port }} · {{ checkedBy(detail.item) }}</v-card-subtitle>
        <v-card-text>
          <v-chip-group v-model="detail.range" mandatory selected-class="text-primary" @update:model-value="loadDetail">
            <v-chip v-for="range in ranges" :key="range.value" :value="range.value" size="small" variant="outlined">{{ range.title }}</v-chip>
          </v-chip-group>
          <v-progress-linear v-if="detail.loading" indeterminate class="my-2" />
          <template v-if="detail.data">
            <div class="pm-summary pm-summary--detail">
              <div><span>{{ $t('proxyMonitor.availability') }}</span><strong>{{ availability }}</strong></div>
              <div><span>{{ $t('proxyMonitor.checks') }}</span><strong>{{ totalChecks }}</strong></div>
              <div><span>{{ $t('proxyMonitor.failures') }}</span><strong>{{ totalFailures }}</strong></div>
              <div v-if="detail.item.last?.ok"><span>{{ $t('proxyMonitor.latency') }}</span><strong>{{ detail.item.last.latency_ms }} ms</strong></div>
            </div>
            <div v-if="detail.item.last?.ok" class="text-caption text-medium-emphasis mb-2">{{ resultLine(detail.item.last) }}</div>
            <svg v-if="chart" class="pm-chart" viewBox="0 0 600 120" preserveAspectRatio="none">
              <path :d="chart.area" class="pm-chart__area" />
              <path :d="chart.line" class="pm-chart__line" />
              <rect v-for="(fail, index) in chart.failures" :key="index" :x="fail.x" y="0" :width="fail.width" height="120" class="pm-chart__fail" />
            </svg>
            <div v-if="chart" class="pm-chart__axis"><span>{{ chart.start }}</span><span>{{ $t('proxyMonitor.peak') }} {{ chart.peak }} ms</span><span>{{ chart.end }}</span></div>
            <h3 v-if="detail.data.exit_ips.length">{{ $t('proxyMonitor.exitIps') }}</h3>
            <div v-for="exit in detail.data.exit_ips" :key="exit.ip" class="pm-row">
              <span dir="ltr">{{ flag(exit.country) }} {{ exit.ip }}</span>
              <small>{{ exit.checks }} · {{ dateTime(exit.last_seen) }}</small>
            </div>
            <h3 v-if="detail.data.failures.length">{{ $t('proxyMonitor.recentFailures') }}</h3>
            <div v-for="(failure, index) in detail.data.failures" :key="'f' + index" class="pm-row">
              <span>{{ reason(failure) }}</span>
              <small>{{ dateTime(failure.time) }}</small>
            </div>
          </template>
        </v-card-text>
        <v-card-actions>
          <v-btn variant="tonal" :loading="checking === detail.item.id" @click="checkNow(detail.item)">{{ $t('proxyMonitor.checkNow') }}</v-btn>
          <v-spacer />
          <v-btn variant="text" @click="detail.visible = false">{{ $t('actions.close') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </section>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { i18n } from '@/locales'
import { fetchBackendObject } from '@/utils/backend'

type Probe = {
  ok: boolean
  time: number
  connect_ms: number
  handshake_ms: number
  tls_ms: number
  ttfb_ms: number
  latency_ms: number
  status: number
  exit_ip?: string
  country?: string
  stage?: string
  error?: string
}

type Monitor = {
  id: number
  name: string
  type: string
  host: string
  port: number
  username: string
  has_password: boolean
  target: string
  server_id: number
  server_name: string
  interval: number
  enabled: boolean
  status: string
  last?: Probe
  uptime: number
  avg_latency: number
  checks: number
  recent: { time: number, ok: boolean, latency_ms: number }[]
}

type Detail = Monitor & {
  points: { time: number, checks: number, failures: number, latency_ms: number, max_ms: number }[]
  failures: Probe[]
  exit_ips: { ip: string, country: string, first_seen: number, last_seen: number, checks: number }[]
}

const t = (key: string, values?: Record<string, unknown>) => i18n.global.t(key, values ?? {})

const items = ref<Monitor[]>([])
const servers = ref<{ id: number, name: string, online: boolean }[]>([])
const loading = ref(true)
const refreshing = ref(false)
const errorMessage = ref('')
const checking = ref(0)
let timer: number | undefined

const emptyForm = () => ({
  id: 0, name: '', link: '', type: 'socks5', host: '', port: 1080, username: '',
  target: '', server_id: 0, interval: 60, enabled: true,
})
const editor = reactive({
  visible: false, form: emptyForm(), password: '', hadPassword: false,
  testing: false, saving: false, error: '', result: null as Probe | null,
})
const detail = reactive({ visible: false, item: null as Monitor | null, data: null as Detail | null, range: 86400, loading: false })

const ranges = computed(() => [
  { value: 3600, title: t('proxyMonitor.range.hour') },
  { value: 21600, title: t('proxyMonitor.range.sixHours') },
  { value: 86400, title: t('proxyMonitor.range.day') },
  { value: 259200, title: t('proxyMonitor.range.threeDays') },
])
const intervalOptions = computed(() => [30, 60, 300, 600, 1800, 3600].map(value => ({ value, title: every(value) })))
const serverOptions = computed(() => [
  { value: 0, title: t('proxyMonitor.panelHost') },
  ...servers.value.map(server => ({ value: server.id, title: server.name + (server.online ? '' : ` (${t('agent.offline')})`) })),
])

const load = async () => {
  if (refreshing.value) return
  refreshing.value = true
  try {
    items.value = await fetchBackendObject<Monitor[]>('api/proxy-monitors') || []
    errorMessage.value = ''
  } catch (error: any) {
    errorMessage.value = error?.message || t('failed')
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

const loadServers = async () => {
  try {
    const nodes = await fetchBackendObject<any[]>('api/agents') || []
    servers.value = nodes.map(node => ({ id: node.id, name: node.name, online: !!node.online }))
  } catch {
    servers.value = []
  }
}

const count = (status: string) => items.value.filter(item => item.status === status).length
const statusColor = (status: string) => ({ up: 'success', down: 'error', unknown: 'warning' } as Record<string, string>)[status] || 'medium-emphasis'
const protocol = (type: string) => type === 'socks5' ? 'SOCKS5' : type.toUpperCase()
const every = (seconds: number) => seconds >= 3600 && seconds % 3600 === 0
  ? t('proxyMonitor.everyHours', { n: seconds / 3600 })
  : seconds >= 60 && seconds % 60 === 0 ? t('proxyMonitor.everyMinutes', { n: seconds / 60 }) : t('proxyMonitor.everySeconds', { n: seconds })
const checkedBy = (item: Monitor) => item.server_id
  ? t('proxyMonitor.viaServer', { name: item.server_name || '#' + item.server_id })
  : t('proxyMonitor.viaPanel')
const flag = (country?: string) => country && /^[A-Z]{2}$/.test(country)
  ? String.fromCodePoint(...[...country].map(c => 0x1F1E6 + c.charCodeAt(0) - 65))
  : ''
const dateTime = (seconds: number) => seconds ? new Date(seconds * 1000).toLocaleString() : ''

const strip = (item: Monitor) => {
  const slots = Math.max(30, item.recent.length)
  return [...Array(slots - item.recent.length).fill(null), ...item.recent]
}
const barHeight = (item: Monitor, latency: number) => {
  const max = Math.max(1, ...item.recent.filter(point => point.ok).map(point => point.latency_ms))
  return `${25 + 75 * latency / max}%`
}
const pointTitle = (point: Monitor['recent'][number] | null) => point
  ? `${dateTime(point.time)} · ${point.ok ? point.latency_ms + ' ms' : t('proxyMonitor.status.down')}`
  : ''

// One line for a failed check: the step that failed, then the detail.
const reason = (probe: Probe) => {
  const stage = probe.stage || ''
  const known = ['server', 'config', 'connect', 'auth', 'tunnel', 'tls', 'http'].includes(stage)
  const title = known ? t('proxyMonitor.stage.' + stage) : t('failed')
  return probe.error ? `${title}: ${probe.error}` : title
}
const resultLine = (probe: Probe) => [
  `${t('proxyMonitor.latency')} ${probe.latency_ms} ms`,
  `${t('proxyMonitor.connect')} ${probe.connect_ms} ms`,
  probe.handshake_ms ? `${t('proxyMonitor.handshake')} ${probe.handshake_ms} ms` : '',
  probe.tls_ms ? `TLS ${probe.tls_ms} ms` : '',
  probe.exit_ip ? `${t('proxyMonitor.exit')} ${flag(probe.country)} ${probe.exit_ip}` : '',
].filter(Boolean).join(' · ')

const openEditor = (item?: Monitor) => {
  editor.form = item
    ? { id: item.id, name: item.name, link: '', type: item.type, host: item.host, port: item.port, username: item.username,
      target: item.target, server_id: item.server_id, interval: item.interval, enabled: item.enabled }
    : emptyForm()
  editor.password = ''
  editor.hadPassword = !!item?.has_password
  editor.error = ''
  editor.result = null
  editor.visible = true
  void loadServers()
}

// An empty password field keeps the stored password of an edited monitor.
const payload = () => ({
  ...editor.form,
  port: Number(editor.form.port) || 0,
  password: editor.password || (editor.hadPassword ? null : ''),
})

const test = async () => {
  editor.testing = true
  editor.error = ''
  editor.result = null
  try {
    editor.result = await fetchBackendObject<Probe>('api/proxy-monitors/test', { method: 'POST', body: JSON.stringify(payload()) })
  } catch (error: any) {
    editor.error = error?.message || t('failed')
  } finally {
    editor.testing = false
  }
}

const save = async () => {
  editor.saving = true
  editor.error = ''
  try {
    await fetchBackendObject('api/proxy-monitors', { method: 'POST', body: JSON.stringify(payload()) })
    editor.visible = false
    await load()
  } catch (error: any) {
    editor.error = error?.message || t('failed')
  } finally {
    editor.saving = false
  }
}

const checkNow = async (item: Monitor) => {
  checking.value = item.id
  try {
    await fetchBackendObject(`api/proxy-monitors/${item.id}/check`, { method: 'POST', body: '{}' })
    await load()
    if (detail.visible && detail.item?.id === item.id) await loadDetail()
  } catch (error: any) {
    errorMessage.value = error?.message || t('failed')
  } finally {
    checking.value = 0
  }
}

const remove = async (item: Monitor) => {
  if (!confirm(t('proxyMonitor.deleteConfirm', { name: item.name }))) return
  try {
    await fetchBackendObject(`api/proxy-monitors/${item.id}/delete`, { method: 'POST', body: '{}' })
    await load()
  } catch (error: any) {
    errorMessage.value = error?.message || t('failed')
  }
}

const openDetail = (item: Monitor) => {
  detail.item = item
  detail.data = null
  detail.visible = true
  void loadDetail()
}

const loadDetail = async () => {
  if (!detail.item) return
  detail.loading = true
  try {
    const data = await fetchBackendObject<Detail>(`api/proxy-monitors/${detail.item.id}?range=${detail.range}`)
    detail.data = data
    detail.item = data
  } catch (error: any) {
    errorMessage.value = error?.message || t('failed')
  } finally {
    detail.loading = false
  }
}

const totalChecks = computed(() => detail.data?.points.reduce((sum, point) => sum + point.checks, 0) || 0)
const totalFailures = computed(() => detail.data?.points.reduce((sum, point) => sum + point.failures, 0) || 0)
const availability = computed(() => totalChecks.value
  ? `${((totalChecks.value - totalFailures.value) * 100 / totalChecks.value).toFixed(2)}%`
  : '-')

// Latency line over the range; failed slices are shaded.
const chart = computed(() => {
  const points = detail.data?.points || []
  if (points.length < 2) return null
  const first = points[0].time
  const span = Math.max(1, points[points.length - 1].time - first)
  const good = points.filter(point => point.latency_ms > 0)
  const peak = Math.max(1, ...good.map(point => point.latency_ms))
  const x = (time: number) => (time - first) / span * 600
  const y = (value: number) => 120 - value / (peak * 1.15) * 120
  const line = good.map((point, index) => `${index ? 'L' : 'M'}${x(point.time).toFixed(1)},${y(point.latency_ms).toFixed(1)}`).join(' ')
  const area = good.length ? `${line} L${x(good[good.length - 1].time).toFixed(1)},120 L${x(good[0].time).toFixed(1)},120 Z` : ''
  const width = Math.max(2, 600 / points.length)
  const failures = points.filter(point => point.failures > 0).map(point => ({ x: Math.max(0, x(point.time) - width / 2), width }))
  const time = (seconds: number) => new Date(seconds * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return { line, area, failures, peak: Math.round(peak), start: time(first), end: time(points[points.length - 1].time) }
})

const handleVisibility = () => { if (!document.hidden) void load() }

onMounted(() => {
  void load()
  void loadServers()
  timer = window.setInterval(() => { if (!document.hidden) void load() }, 15000)
  document.addEventListener('visibilitychange', handleVisibility)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('visibilitychange', handleVisibility)
})
</script>

<style scoped>
.proxy-monitors { display: grid; gap: 16px; }
.pm-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.pm-header h1 { margin: 0; font-size: 1.35rem; line-height: 1.3; }
.pm-header p { margin: 4px 0 0; color: rgba(var(--v-theme-on-surface), .62); max-width: 760px; }
.pm-actions { display: flex; align-items: center; gap: 10px; }
.pm-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.pm-summary > div { padding: 12px 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.pm-summary span { display: block; font-size: .78rem; color: rgba(var(--v-theme-on-surface), .6); }
.pm-summary strong { font-size: 1.3rem; }
.pm-summary--detail { margin: 8px 0 12px; }
.pm-summary--detail > div { padding: 8px 10px; }
.pm-summary--detail strong { font-size: 1.05rem; }
.pm-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 12px; }
.pm-card { position: relative; padding: 14px 14px 10px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); cursor: pointer; }
.pm-card:hover { border-color: rgba(var(--v-theme-primary), .5); }
.pm-card > header { display: flex; align-items: center; gap: 8px; }
.pm-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pm-latency { font-weight: 600; font-size: .9rem; white-space: nowrap; }
.pm-dot { width: 10px; height: 10px; border-radius: 50%; flex: none; background: rgba(var(--v-theme-on-surface), .3); }
.pm-dot--up { background: rgb(var(--v-theme-success)); }
.pm-dot--down { background: rgb(var(--v-theme-error)); }
.pm-dot--unknown { background: rgb(var(--v-theme-warning)); }
.pm-meta { margin-top: 4px; font-size: .78rem; color: rgba(var(--v-theme-on-surface), .6); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pm-strip { display: flex; align-items: flex-end; gap: 2px; height: 28px; margin: 10px 0 8px; }
.pm-strip span { flex: 1; height: 100%; border-radius: 2px; background: rgba(var(--v-theme-on-surface), .08); }
.pm-strip span.ok { background: rgb(var(--v-theme-success)); }
.pm-strip span.fail { background: rgb(var(--v-theme-error)); }
.pm-card footer { display: flex; flex-wrap: wrap; gap: 4px 12px; font-size: .78rem; color: rgba(var(--v-theme-on-surface), .62); }
.pm-reason { margin-top: 4px; font-size: .78rem; overflow-wrap: anywhere; }
.pm-card-actions { display: flex; justify-content: flex-end; margin-top: 4px; }
.pm-chart { width: 100%; height: 120px; display: block; }
.pm-chart__line { fill: none; stroke: rgb(var(--v-theme-primary)); stroke-width: 2; vector-effect: non-scaling-stroke; }
.pm-chart__area { fill: rgba(var(--v-theme-primary), .14); stroke: none; }
.pm-chart__fail { fill: rgba(var(--v-theme-error), .22); }
.pm-chart__axis { display: flex; justify-content: space-between; font-size: .75rem; color: rgba(var(--v-theme-on-surface), .6); margin-bottom: 12px; }
.pm-row { display: flex; justify-content: space-between; gap: 12px; padding: 6px 0; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.pm-row small { color: rgba(var(--v-theme-on-surface), .6); white-space: nowrap; }
h3 { margin: 14px 0 4px; font-size: .95rem; }
@media (max-width: 600px) {
  .pm-header { flex-direction: column; align-items: stretch; }
  .pm-actions { justify-content: space-between; }
  .pm-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
