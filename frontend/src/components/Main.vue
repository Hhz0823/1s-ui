<template>
  <LogVue v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
  <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
  <UsageStats v-model:visible="usageStatsModal.visible" />
  <div class="dash">
    <section class="dash-card">
      <header class="dash-card__head">
        <span class="dash-card__title">{{ $t('dash.overview') }}</span>
        <div class="dash-card__tools">
          <v-btn size="small" variant="text" prepend-icon="mdi-backup-restore" @click="backupModal.visible = true">{{ $t('main.backup.title') }}</v-btn>
          <v-btn size="small" variant="text" prepend-icon="mdi-text-box-outline" @click="logModal.visible = true">{{ $t('basic.log.title') }}</v-btn>
          <v-btn size="small" variant="text" prepend-icon="mdi-chart-box-outline" @click="usageStatsModal.visible = true">{{ $t('main.stats.title') }}</v-btn>
        </div>
      </header>
      <div class="dash-overview">
        <router-link v-for="item in overview" :key="item.key" :to="item.to" class="dash-stat">
          <span class="dash-stat__label"><v-icon :icon="item.icon" size="16" />{{ item.label }}</span>
          <span class="dash-stat__value">{{ item.value }}</span>
        </router-link>
      </div>
    </section>

    <section class="dash-card">
      <header class="dash-card__head">
        <span class="dash-card__title">{{ $t('dash.status') }}</span>
      </header>
      <div class="dash-gauges">
        <RingGauge
          v-for="gauge in gauges"
          :key="gauge.key"
          :title="gauge.title"
          :percent="gauge.percent"
          :caption="gauge.caption"
          :caption-tone="gauge.captionTone"
          :tooltip="gauge.tooltip"
        />
      </div>
    </section>

    <div class="dash-grid">
      <section class="dash-card dash-grid__main">
        <header class="dash-card__head">
          <span class="dash-card__title">{{ $t('dash.monitor') }}</span>
          <div class="dash-tabs" role="tablist">
            <button
              v-for="tab in monitorTabs"
              :key="tab.value"
              type="button"
              role="tab"
              class="dash-tab"
              :class="{ 'dash-tab--active': monitorTab === tab.value }"
              :aria-selected="monitorTab === tab.value"
              @click="monitorTab = tab.value"
            >{{ tab.title }}</button>
          </div>
        </header>
        <div class="dash-card__body">
          <div class="dash-legend">
            <span v-for="entry in monitor.legend" :key="entry.label" class="dash-legend__item">
              <i v-if="entry.color" class="dash-legend__dot" :style="{ background: entry.color }"></i>
              <span class="dash-legend__label">{{ entry.label }}</span>
              <span class="dash-legend__value">{{ entry.value }}</span>
            </span>
          </div>
          <MonitorChart
            :labels="history.labels"
            :series="monitor.series"
            :format="monitor.format"
            :max="monitor.max"
            :suggested-max="monitor.max ? undefined : 1024"
          />
        </div>
      </section>

      <div class="dash-grid__side">
        <section class="dash-card">
          <header class="dash-card__head">
            <span class="dash-card__title">{{ $t('dash.system') }}</span>
            <v-btn icon="mdi-refresh" size="small" variant="text" :title="$t('actions.update')" @click="loadSystem" />
          </header>
          <dl class="dash-info">
            <template v-for="row in systemRows" :key="row.label">
              <dt>{{ row.label }}</dt>
              <dd :title="row.title || row.value">
                <v-chip v-if="row.chip" size="small" variant="tonal" :color="row.chip">{{ row.value }}</v-chip>
                <template v-else>{{ row.value }}</template>
              </dd>
            </template>
          </dl>
        </section>

        <section class="dash-card">
          <header class="dash-card__head">
            <span class="dash-card__title">{{ $t('dash.cores') }}</span>
          </header>
          <div class="dash-core">
            <div class="dash-core__head">
              <span class="dash-core__name">sing-box</span>
              <v-chip size="small" variant="flat" :color="status.sbd?.running ? 'success' : 'error'">
                {{ status.sbd?.running ? $t('main.info.runningYes') : $t('main.info.runningNo') }}
              </v-chip>
              <v-spacer />
              <v-btn size="small" variant="text" color="warning" prepend-icon="mdi-restart" :loading="loading" @click="restartSingbox">
                {{ $t('actions.restartSb') }}
              </v-btn>
            </div>
            <div class="dash-core__stats">
              <span>{{ $t('main.info.memory') }} <b>{{ HumanReadable.sizeFormat(status.sbd?.stats?.Alloc) }}</b></span>
              <span>{{ $t('main.info.threads') }} <b>{{ status.sbd?.stats?.NumGoroutine ?? '-' }}</b></span>
              <span>{{ $t('main.info.uptime') }} <b>{{ HumanReadable.formatSecond(status.sbd?.stats?.Uptime) }}</b></span>
              <span>{{ $t('online') }} <b>{{ onlineSummary }}</b></span>
            </div>
          </div>
          <div v-if="!isOpenWrtLite" class="dash-core">
            <div class="dash-core__head">
              <span class="dash-core__name">Xray-core</span>
              <v-chip size="small" variant="flat" :color="xrayState.color">{{ xrayState.text }}</v-chip>
              <v-spacer />
              <v-btn v-if="status.xry?.disabled" size="small" variant="text" color="primary" prepend-icon="mdi-cog-outline" @click="goXraySettings">
                {{ $t('setting.xrayEnable') }}
              </v-btn>
              <v-btn v-else-if="status.xry?.has_inbounds === false" size="small" variant="text" color="primary" prepend-icon="mdi-plus-circle-outline" @click="goXrayInbound">
                {{ $t('setting.xrayAddInbound') }}
              </v-btn>
              <template v-else>
                <v-btn size="small" variant="text" :color="status.xry?.running ? 'warning' : 'success'" :prepend-icon="status.xry?.running ? 'mdi-restart' : 'mdi-play-circle-outline'" :loading="loading" @click="restartXray">
                  {{ status.xry?.running ? $t('actions.restartXray') : $t('setting.xrayStart') }}
                </v-btn>
                <v-btn v-if="status.xry?.running" size="small" variant="text" color="error" prepend-icon="mdi-stop-circle-outline" :loading="loading" @click="stopXray">
                  {{ $t('setting.xrayStop') }}
                </v-btn>
              </template>
            </div>
            <div class="dash-core__stats">
              <span>{{ $t('main.info.uptime') }} <b>{{ HumanReadable.formatSecond(status.xry?.stats?.Uptime) }}</b></span>
              <span v-if="status.xry?.path" :title="status.xry?.path">Bin <b>{{ shortPath(status.xry?.path) }}</b></span>
              <span v-if="status.xry?.last_error" class="text-error" :title="status.xry?.last_error">{{ shortText(status.xry?.last_error) }}</span>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import RingGauge from '@/components/dashboard/RingGauge.vue'
import MonitorChart from '@/components/dashboard/MonitorChart.vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useTheme } from 'vuetify'
import { i18n, locale } from '@/locales'
import LogVue from '@/layouts/modals/Logs.vue'
import Backup from '@/layouts/modals/Backup.vue'
import UsageStats from '@/layouts/modals/UsageStats.vue'
import router from '@/router'

const t = i18n.global.t
const isOpenWrtLite = import.meta.env.VITE_OPENWRT_LITE === 'true'
const theme = useTheme()
const loading = ref(false)
const status = ref<any>({})
const sys = ref<any>(null)

// ---- Live status (BaoTa refreshes every 3 seconds) ----
const pollInterval = 3000
const historyPoints = 40
const statusKeys = ['cpu', 'lod', 'mem', 'dsk', 'swp', 'net', 'dio', 'sbd', ...(isOpenWrtLite ? [] : ['xry'])]
const history = reactive({
  labels: [] as string[],
  up: [] as number[],
  down: [] as number[],
  read: [] as number[],
  write: [] as number[],
  cpu: [] as number[],
  mem: [] as number[],
})
const rates = reactive({ up: 0, down: 0, read: 0, write: 0 })
let lastSample: { time: number, net: any, dio: any } | null = null

const usagePercent = (value: any) => value?.total > 0 ? (value.current * 100) / value.total : null

// Rates use the real time between samples; counters that went backwards
// (reboot, interface reset) count as zero.
const recordSample = (sample: any) => {
  const now = Date.now()
  if (lastSample && sample.net && sample.dio && lastSample.net && lastSample.dio) {
    const seconds = Math.max(0.5, (now - lastSample.time) / 1000)
    const delta = (current: number, previous: number) => Math.max(0, (Number(current) - Number(previous)) / seconds)
    rates.up = delta(sample.net.sent, lastSample.net.sent)
    rates.down = delta(sample.net.recv, lastSample.net.recv)
    rates.read = delta(sample.dio.read, lastSample.dio.read)
    rates.write = delta(sample.dio.write, lastSample.dio.write)
    const push = (list: number[], value: number) => {
      list.push(value)
      if (list.length > historyPoints) list.shift()
    }
    push(history.up, rates.up)
    push(history.down, rates.down)
    push(history.read, rates.read)
    push(history.write, rates.write)
    push(history.cpu, Number(sample.cpu) || 0)
    push(history.mem, usagePercent(sample.mem) ?? 0)
    history.labels.push(new Date(now).toLocaleTimeString(locale, { hour12: false }))
    if (history.labels.length > historyPoints) history.labels.shift()
  }
  lastSample = { time: now, net: sample.net, dio: sample.dio }
}

let statusPending = false
const loadStatus = async () => {
  if (statusPending) return
  statusPending = true
  try {
    const msg = await HttpUtils.get('api/status', { r: statusKeys.join(',') })
    if (msg.success) {
      status.value = msg.obj
      recordSample(msg.obj)
    }
  } finally {
    statusPending = false
  }
}

const loadSystem = async () => {
  const msg = await HttpUtils.get('api/status', { r: 'sys' })
  if (msg.success) sys.value = msg.obj.sys
}

let timer: ReturnType<typeof setInterval> | null = null
const handleVisibility = () => {
  if (!document.hidden) void loadStatus()
}

onMounted(() => {
  void loadSystem()
  void loadStatus()
  timer = setInterval(() => { if (!document.hidden) void loadStatus() }, pollInterval)
  document.addEventListener('visibilitychange', handleVisibility)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', handleVisibility)
})

// ---- Overview ----
const overview = computed(() => {
  const data = Data()
  const clients: any[] = data.clients || []
  const traffic = clients.reduce((sum, client) => sum + Number(client.up || 0) + Number(client.down || 0), 0)
  const servers = Number(data.controllerMode?.agent_count || 0)
  const items = [
    { key: 'inbounds', icon: 'mdi-arrow-down-bold-circle-outline', label: t('pages.inbounds'), value: (data.inbounds || []).length, to: '/inbounds' },
    { key: 'clients', icon: 'mdi-account-group-outline', label: t('pages.clients'), value: clients.length, to: '/clients' },
    { key: 'online', icon: 'mdi-account-check-outline', label: t('dash.onlineUsers'), value: data.onlines?.user?.length || 0, to: '/clients' },
    { key: 'outbounds', icon: 'mdi-arrow-up-bold-circle-outline', label: t('pages.outbounds'), value: (data.outbounds || []).length, to: '/outbounds' },
    { key: 'traffic', icon: 'mdi-swap-vertical-bold', label: t('dash.userTraffic'), value: traffic > 0 ? HumanReadable.sizeFormat(traffic) : '0', to: '/user-traffic' },
  ]
  if (data.controllerMode?.profile !== 'client' || servers > 0) {
    items.push({ key: 'servers', icon: 'mdi-server-network', label: t('pages.agents'), value: servers, to: '/agents' })
  }
  if (data.controllerMode?.profile === 'monitor') return items.filter(item => item.key === 'servers')
  return items
})

// ---- Status gauges ----
const sizePair = (value: any) => {
  if (!value?.total) return '-'
  return `${HumanReadable.sizeFormat(value.current, 1)} / ${HumanReadable.sizeFormat(value.total, 1)}`
}

const gauges = computed(() => {
  const s = status.value
  const cpus = Number(s.lod?.cpus || sys.value?.cpuCount || 1)
  // BaoTa: load1 against twice the CPU count.
  const loadPercent = typeof s.lod?.load1 === 'number' ? Math.min(100, (s.lod.load1 / (cpus * 2)) * 100) : null
  let loadCaption = ''
  let loadTone = ''
  if (loadPercent != null) {
    if (loadPercent <= 30) [loadCaption, loadTone] = [t('dash.loadSmooth'), 'success']
    else if (loadPercent <= 70) [loadCaption, loadTone] = [t('dash.loadNormal'), 'success']
    else if (loadPercent <= 90) [loadCaption, loadTone] = [t('dash.loadSlow'), 'warning']
    else [loadCaption, loadTone] = [t('dash.loadBlocked'), 'error']
  }
  const loads = ['load1', 'load5', 'load15'].map(key => typeof s.lod?.[key] === 'number' ? s.lod[key].toFixed(2) : '-').join(' / ')
  const items: any[] = [
    { key: 'load', title: t('dash.load'), percent: loadPercent, caption: loadCaption, captionTone: loadTone, tooltip: t('dash.loadAverages', { values: loads }) },
    { key: 'cpu', title: t('dash.cpu'), percent: typeof s.cpu === 'number' ? s.cpu : null, caption: `${cpus} ${t('main.info.core')}`, tooltip: sys.value?.cpuType || '' },
    { key: 'mem', title: t('dash.memory'), percent: usagePercent(s.mem), caption: sizePair(s.mem) },
    { key: 'disk', title: t('dash.disk'), percent: usagePercent(s.dsk), caption: sizePair(s.dsk) },
  ]
  if (s.swp?.total > 0) items.push({ key: 'swap', title: t('dash.swap'), percent: usagePercent(s.swp), caption: sizePair(s.swp) })
  return items
})

// ---- Monitor chart ----
const monitorTab = ref<'net' | 'dio' | 'cpu' | 'mem'>('net')
const monitorTabs = computed(() => [
  { value: 'net' as const, title: t('dash.tabTraffic') },
  { value: 'dio' as const, title: t('dash.tabDisk') },
  { value: 'cpu' as const, title: 'CPU' },
  { value: 'mem' as const, title: t('dash.tabMemory') },
])
const rate = (value: number) => value > 0 ? `${HumanReadable.sizeFormat(value, 1)}/s` : `0 ${t('stats.B')}/s`
const percent = (value: number) => `${Math.round(value)}%`

const monitor = computed(() => {
  const colors = theme.current.value.colors
  const first = String(colors.primary)
  const second = String(colors.warning)
  switch (monitorTab.value) {
    case 'dio':
      return {
        series: [
          { label: t('dash.read'), data: history.read, color: first },
          { label: t('dash.write'), data: history.write, color: second },
        ],
        format: rate,
        max: undefined,
        legend: [
          { label: t('dash.read'), value: rate(rates.read), color: first },
          { label: t('dash.write'), value: rate(rates.write), color: second },
        ],
      }
    case 'cpu':
      return {
        series: [{ label: 'CPU', data: history.cpu, color: first }],
        format: percent,
        max: 100,
        legend: [{ label: t('dash.usage'), value: typeof status.value.cpu === 'number' ? percent(status.value.cpu) : '-', color: first }],
      }
    case 'mem':
      return {
        series: [{ label: t('dash.tabMemory'), data: history.mem, color: first }],
        format: percent,
        max: 100,
        legend: [{ label: t('dash.usage'), value: sizePair(status.value.mem), color: first }],
      }
    default:
      return {
        series: [
          { label: t('dash.upload'), data: history.up, color: first },
          { label: t('dash.download'), data: history.down, color: second },
        ],
        format: rate,
        max: undefined,
        legend: [
          { label: t('dash.upload'), value: rate(rates.up), color: first },
          { label: t('dash.download'), value: rate(rates.down), color: second },
          { label: t('dash.totalSent'), value: HumanReadable.sizeFormat(status.value.net?.sent), color: '' },
          { label: t('dash.totalRecv'), value: HumanReadable.sizeFormat(status.value.net?.recv), color: '' },
        ],
      }
  }
})

// ---- System info ----
const hostReqChipColor = computed(() => {
  const req = sys.value?.requirements ?? Data().hostRequirements
  if (!req) return 'primary'
  if (req.applies === true && req.ok === false) return 'error'
  if (req.meets_cluster_rec === false) return 'warning'
  return 'success'
})

const capitalize = (value: string) => value ? value.charAt(0).toUpperCase() + value.slice(1) : value

const systemRows = computed(() => {
  const info = sys.value
  if (!info) return []
  const rows: { label: string, value: string, title?: string, chip?: string }[] = [
    { label: t('main.info.host'), value: info.hostName || '-' },
  ]
  if (info.os) rows.push({ label: t('dash.os'), value: capitalize(info.os) + (info.arch ? ` (${info.arch})` : '') })
  if (info.kernel) rows.push({ label: t('dash.kernel'), value: info.kernel })
  rows.push({
    label: t('main.info.cpu'),
    value: `${info.cpuCount} ${t('main.info.core')}` + (info.memTotal ? ` · ${(info.memTotal / 1024 ** 3).toFixed(1)} GB` : ''),
    title: info.cpuType,
    chip: hostReqChipColor.value,
  })
  if (info.ipv4?.length) rows.push({ label: 'IPv4', value: info.ipv4[0] + (info.ipv4.length > 1 ? ` +${info.ipv4.length - 1}` : ''), title: info.ipv4.join('\n') })
  if (info.ipv6?.length) rows.push({ label: 'IPv6', value: info.ipv6[0] + (info.ipv6.length > 1 ? ` +${info.ipv6.length - 1}` : ''), title: info.ipv6.join('\n') })
  rows.push({ label: t('dash.panelVersion'), value: `v${info.appVersion}` })
  rows.push({
    label: t('main.info.uptime'),
    value: HumanReadable.formatSecond(Date.now() / 1000 - Number(info.bootTime || 0)),
    title: t('main.info.startupTime') + ': ' + new Date(Number(info.bootTime || 0) * 1000).toLocaleString(locale),
  })
  return rows
})

// ---- Cores ----
const onlineSummary = computed(() => {
  const onlines = Data().onlines
  if (!status.value.sbd?.running) return '-'
  return `${onlines?.user?.length || 0} ${t('pages.clients')} · ${onlines?.inbound?.length || 0} ${t('pages.inbounds')}`
})

const xrayState = computed(() => {
  const xry = status.value.xry
  if (xry?.disabled) return { color: 'warning', text: t('setting.xrayDisabled') }
  if (xry?.running) return { color: 'success', text: t('main.info.runningYes') }
  if (xry?.has_inbounds === false) return { color: 'warning', text: t('setting.xrayNotConfigured') }
  return { color: 'error', text: t('main.info.runningNo') }
})

const logModal = ref({ visible: false })
const backupModal = ref({ visible: false })
const usageStatsModal = ref({ visible: false })

const restartSingbox = async () => {
  loading.value = true
  try {
    await HttpUtils.post('api/restartSb', {})
    await loadStatus()
  } finally {
    loading.value = false
  }
}

const restartXray = async () => {
  if (status.value.xry?.disabled) return goXraySettings()
  if (status.value.xry?.has_inbounds === false) return goXrayInbound()
  if (!status.value.xry?.running && status.value.xry?.exclusive_run) {
    if (!window.confirm(t('setting.xrayStartExclusiveConfirm'))) return
  }
  loading.value = true
  try {
    await HttpUtils.post('api/restartXray', {})
    await loadStatus()
  } finally {
    loading.value = false
  }
}

const stopXray = async () => {
  loading.value = true
  try {
    await HttpUtils.post('api/stopXray', {})
    await loadStatus()
  } finally {
    loading.value = false
  }
}

const goXrayInbound = () => router.push('/inbounds')
const goXraySettings = () => router.push('/settings')

const shortPath = (path?: string) => path ? path.split(/[\\/]/).pop() || path : '-'
const shortText = (text?: string) => {
  if (!text) return '-'
  return text.length > 40 ? text.substring(0, 40) + '…' : text
}
</script>

<style scoped>
.dash {
  display: grid;
  gap: 16px;
}

.dash-card {
  min-width: 0;
  background: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-theme-on-surface), 0.09);
  border-radius: var(--app-surface-radius, 8px);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

:global(.ui-style--glass) .dash-card,
:global(.ui-style--clear) .dash-card {
  background: rgba(var(--v-theme-surface), 0.62);
  backdrop-filter: blur(18px) saturate(170%);
  -webkit-backdrop-filter: blur(18px) saturate(170%);
}

.dash-card__head {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 50px;
  padding: 6px 12px 6px 20px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.07);
}

.dash-card__title {
  flex: 1 1 auto;
  min-width: 0;
  font-size: 15px;
  font-weight: 600;
  color: rgb(var(--v-theme-on-surface));
}

.dash-card__tools {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 4px;
}

.dash-card__tools :deep(.v-btn),
.dash-core__head :deep(.v-btn) {
  min-width: 0 !important;
  min-height: 32px !important;
  height: 32px !important;
  padding-inline: 8px !important;
  border: 0 !important;
  background: transparent !important;
  box-shadow: none !important;
}

.dash-card__body {
  padding: 12px 20px 16px;
}

/* Overview counters */
.dash-overview {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
}

.dash-stat {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px 20px 18px;
  color: inherit;
  text-decoration: none;
  border-inline-end: 1px solid rgba(var(--v-theme-on-surface), 0.06);
  transition: background 0.15s ease;
}

.dash-stat:last-child {
  border-inline-end: 0;
}

.dash-stat:hover {
  background: rgba(var(--v-theme-primary), 0.04);
}

.dash-stat:hover .dash-stat__value {
  color: rgb(var(--v-theme-primary));
}

.dash-stat__label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: rgba(var(--v-theme-on-surface), 0.6);
  font-size: 13px;
}

.dash-stat__value {
  font-size: 26px;
  font-weight: 600;
  line-height: 1.1;
  color: rgb(var(--v-theme-on-surface));
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

/* Status gauges */
.dash-gauges {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 8px;
  padding: 16px 12px 18px;
}

/* Monitor + side cards */
.dash-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 380px);
  gap: 16px;
  align-items: start;
}

.dash-grid__side {
  display: grid;
  gap: 16px;
  min-width: 0;
}

.dash-tabs {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}

.dash-tab {
  padding: 4px 12px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: rgba(var(--v-theme-on-surface), 0.66);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.dash-tab:hover {
  color: rgb(var(--v-theme-primary));
}

.dash-tab--active {
  border-color: rgba(var(--v-theme-primary), 0.35);
  background: rgba(var(--v-theme-primary), 0.08);
  color: rgb(var(--v-theme-primary));
  font-weight: 600;
}

.dash-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  margin-bottom: 10px;
  font-size: 13px;
}

.dash-legend__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.dash-legend__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.dash-legend__label {
  color: rgba(var(--v-theme-on-surface), 0.6);
}

.dash-legend__value {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

/* System info */
.dash-info {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px 16px;
  margin: 0;
  padding: 14px 20px 16px;
  font-size: 13px;
}

.dash-info dt {
  color: rgba(var(--v-theme-on-surface), 0.58);
  white-space: nowrap;
}

.dash-info dd {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  text-align: end;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Cores */
.dash-core {
  padding: 12px 20px 14px;
}

.dash-core + .dash-core {
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.07);
}

.dash-core__head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
}

.dash-core__name {
  font-weight: 600;
}

.dash-core__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin-top: 8px;
  color: rgba(var(--v-theme-on-surface), 0.6);
  font-size: 12.5px;
}

.dash-core__stats b {
  margin-inline-start: 4px;
  color: rgb(var(--v-theme-on-surface));
  font-weight: 600;
}

@media (max-width: 1200px) {
  .dash-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .dash-grid__side {
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  }
}

@media (max-width: 600px) {
  .dash {
    gap: 12px;
  }

  .dash-card__head {
    flex-wrap: wrap;
    padding-inline: 14px 8px;
  }

  .dash-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dash-stat {
    padding: 12px 14px;
    border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.06);
  }

  .dash-stat__value {
    font-size: 22px;
  }

  .dash-gauges {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dash-grid__side {
    grid-template-columns: minmax(0, 1fr);
  }

  .dash-card__body,
  .dash-info,
  .dash-core {
    padding-inline: 14px;
  }
}
</style>
