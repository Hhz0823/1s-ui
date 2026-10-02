<template>
  <v-dialog v-model="term.visible" width="min(960px, calc(100vw - 16px))" persistent scrim="true">
    <v-card class="term-card">
      <v-card-title class="term-title">
        <span>{{ $t('agent.terminal') }} · {{ node?.name }}</span>
        <div class="d-flex align-center ga-2">
          <v-chip size="small" :color="term.connected ? 'success' : 'error'" variant="tonal">{{ term.connected ? $t('online') : $t('agent.offline') }}</v-chip>
          <v-btn icon="mdi-close" size="small" variant="text" @click="closeTerminal" />
        </div>
      </v-card-title>
      <v-divider />
      <div ref="termEl" class="term-screen" tabindex="0" @keydown="onTermKey" @paste="onTermPaste" @click="focusTerm">{{ term.buffer }}</div>
      <v-card-text class="py-2"><div class="detail-muted">{{ $t('agent.terminalHint') }}</div></v-card-text>
    </v-card>
  </v-dialog>

  <div class="detail-layout">
    <aside class="server-sidebar">
      <div class="server-sidebar__title">{{ $t('monitor.serverList') }}</div>
      <div class="server-sidebar__list">
        <template v-for="group in sidebarGroups" :key="group.name">
          <div v-if="group.name" class="server-sidebar__group">{{ group.name }}</div>
          <div
            v-for="item in group.nodes"
            :key="item.id"
            role="link"
            tabindex="0"
            class="server-sidebar__item"
            :class="{ 'server-sidebar__item--active': item.id === nodeId }"
            @click="router.push(`/agents/${item.id}`)"
            @keydown.enter="router.push(`/agents/${item.id}`)"
          >
            <i class="status-dot" :class="item.online ? 'dot-online' : 'dot-offline'" />
            <span v-if="flagEmoji(item.region)">{{ flagEmoji(item.region) }}</span>
            <span class="server-sidebar__name">{{ item.name }}</span>
          </div>
        </template>
      </div>
    </aside>

    <section class="agent-detail-page">
      <header class="detail-header">
        <v-btn icon="mdi-arrow-left" variant="text" :title="$t('agent.backServers')" @click="router.push('/agents')" />
        <div class="detail-identity">
          <div class="identity-title">
            <span v-if="flagEmoji(node?.region)" class="identity-flag">{{ flagEmoji(node?.region) }}</span>
            <h1 class="adaptive-ink">{{ node?.name || ('#' + nodeId) }}</h1>
            <span v-if="node" class="status-badge" :class="node.online ? 'status-badge--on' : 'status-badge--off'">{{ node.online ? $t('online') : $t('agent.offline') }}</span>
          </div>
          <div class="identity-sub">
            <span class="adaptive-ink" dir="ltr">{{ node?.report.hostname || node?.remote_ip || '-' }}</span>
            <PriceTags v-if="node" :node="node" />
          </div>
        </div>
        <div class="detail-actions">
          <v-btn
            v-if="canControl"
            variant="tonal"
            prepend-icon="mdi-open-in-new"
            :loading="openingPanel"
            :disabled="!canOpenPanel"
            :title="canOpenPanel ? $t('agent.openPanel') : $t('agent.openPanelUnavailable')"
            @click="openManagedPanel"
          >{{ $t('agent.openPanel') }}</v-btn>
          <v-btn v-if="canControl" variant="tonal" prepend-icon="mdi-console" :disabled="!node?.controllable" @click="openTerminal">{{ $t('agent.terminal') }}</v-btn>
          <v-btn v-if="canControl" color="primary" prepend-icon="mdi-tune-vertical" :disabled="!node?.managed" @click="manageInbounds">{{ $t('agent.manageInbounds') }}</v-btn>
          <v-btn variant="tonal" prepend-icon="mdi-speedometer" :disabled="!node?.online" @click="speedtestOpen = true">{{ $t('relaySpeedtest.button') }}</v-btn>
          <v-btn icon="mdi-refresh" variant="tonal" :loading="loading" :title="$t('actions.update')" @click="refreshAll" />
        </div>
      </header>
      <RelaySpeedtest v-model="speedtestOpen" :server-id="nodeId" :server-name="node?.name || ('#' + nodeId)" />

      <v-progress-linear v-if="loading && !node" indeterminate />
      <v-alert v-else-if="errorMessage" type="error" variant="tonal">{{ errorMessage }}</v-alert>

      <template v-if="node">
        <section class="details-card">
          <div v-for="item in details" :key="item.label" class="details-item" :class="{ 'details-item--wide': item.wide }">
            <span>{{ item.label }}</span>
            <strong dir="auto">{{ item.value }}</strong>
            <small v-if="item.sub">{{ item.sub }}</small>
          </div>
          <div v-if="node.traffic_limit" class="details-item details-item--wide">
            <UsageBar :label="`${$t('monitor.periodTraffic')} · ${limitTypeLabel(node.traffic_limit_type)}`" :value="trafficPercent(node)" :text="`${bytes(trafficUsed(node))} / ${bytes(node.traffic_limit)}`" />
          </div>
        </section>

        <div class="live-strip">
          <UsageBar label="CPU" :value="node.online ? node.report.cpu_percent : undefined" />
          <UsageBar :label="$t('monitor.ram')" :value="node.online ? usagePercent(node.report.memory) : undefined" :detail="usageText(node.report.memory)" />
          <UsageBar :label="$t('agent.disk')" :value="node.online ? usagePercent(node.report.disk) : undefined" :detail="usageText(node.report.disk)" />
          <div class="live-fact">
            <span>{{ $t('monitor.pingProxy') }}</span>
            <strong dir="ltr" :class="latencyClass">{{ latencyValue }}</strong>
            <small class="core-line">
              <span :class="{ on: node.online && node.report.cores?.singbox_running }">sing-box</span>
              <span :class="{ on: node.online && node.report.cores?.xray_running }">Xray</span>
            </small>
          </div>
        </div>

        <v-tabs v-model="tab" color="primary" class="detail-tabs" density="comfortable">
          <v-tab value="load">{{ $t('monitor.loadTab') }}</v-tab>
          <v-tab value="ping">{{ $t('monitor.pingTab') }}</v-tab>
          <v-tab value="traffic">{{ $t('pages.portTraffic') }}</v-tab>
          <v-tab v-if="canControl" value="control">{{ $t('agent.control') }}</v-tab>
        </v-tabs>

        <div v-if="tab === 'load' || tab === 'ping'" class="range-bar">
          <v-btn-toggle v-model="range" mandatory density="compact" variant="outlined" divided color="primary">
            <v-btn v-for="item in ranges" :key="item.key" :value="item.key" size="small">{{ $t(`monitor.range.${item.key}`) }}</v-btn>
          </v-btn-toggle>
          <v-switch v-model="smooth" :label="$t('monitor.smooth')" color="primary" density="compact" hide-details inset class="smooth-switch" @update:model-value="saveSmooth" />
          <v-progress-circular v-if="metricsLoading" indeterminate size="18" width="2" />
        </div>

        <v-window v-model="tab">
          <v-window-item value="load">
            <div v-if="!rows.length" class="empty-state">{{ range === 'live' ? $t('noData') : $t('monitor.noHistory') }}</div>
            <div v-else class="chart-grid">
              <section class="chart-panel">
                <header><span>CPU · {{ $t('monitor.load') }}</span><strong>{{ percent(node.report.cpu_percent) }}</strong></header>
                <div class="chart-body"><Line :data="cpuChart" :options="cpuOptions as any" /></div>
              </section>
              <section class="chart-panel">
                <header><span>{{ $t('monitor.ram') }} · {{ $t('monitor.swap') }}</span><strong>{{ percent(usagePercent(node.report.memory)) }}</strong></header>
                <div class="chart-body"><Line :data="memoryChart" :options="percentOptions as any" /></div>
              </section>
              <section class="chart-panel">
                <header><span>{{ $t('agent.disk') }}</span><strong>{{ percent(usagePercent(node.report.disk)) }}</strong></header>
                <div class="chart-body"><Line :data="diskChart" :options="percentOptions as any" /></div>
              </section>
              <section class="chart-panel chart-panel--large">
                <header><span>{{ $t('monitor.network') }}</span><strong dir="ltr">↑ {{ rate(node.report.net_rate?.sent) }} · ↓ {{ rate(node.report.net_rate?.recv) }}</strong></header>
                <div class="chart-body"><Line :data="networkChart" :options="rateOptions as any" /></div>
              </section>
              <section class="chart-panel">
                <header><span>{{ $t('monitor.connections') }}</span><strong dir="ltr">TCP {{ node.report.tcp_conns ?? '-' }} · UDP {{ node.report.udp_conns ?? '-' }}</strong></header>
                <div class="chart-body"><Line :data="connChart" :options="countOptions as any" /></div>
              </section>
              <section class="chart-panel">
                <header><span>{{ $t('monitor.processes') }}</span><strong>{{ node.report.process_count ?? '-' }}</strong></header>
                <div class="chart-body"><Line :data="processChart" :options="countOptions as any" /></div>
              </section>
              <section v-if="range !== 'live'" class="chart-panel">
                <header><span>{{ $t('monitor.trafficPerPoint') }}</span><strong dir="ltr">↑ {{ bytes(rangeTraffic.sent) }} · ↓ {{ bytes(rangeTraffic.recv) }}</strong></header>
                <div class="chart-body"><Line :data="trafficChart" :options="bytesOptions as any" /></div>
              </section>
            </div>
          </v-window-item>

          <v-window-item value="ping">
            <div class="ping-stats">
              <div><span>{{ $t('agent.latency') }}</span><strong dir="ltr" :class="latencyClass">{{ latencyValue }}</strong></div>
              <div><span>{{ $t('agent.average') }}</span><strong dir="ltr">{{ pingStats.avg }}</strong></div>
              <div><span>P95</span><strong dir="ltr">{{ pingStats.p95 }}</strong></div>
              <div><span>{{ $t('monitor.loss') }}</span><strong dir="ltr">{{ pingStats.loss }}</strong></div>
            </div>
            <div v-if="!pingRows.length" class="empty-state">{{ range === 'live' ? $t('noData') : $t('monitor.noHistory') }}</div>
            <section v-else class="chart-panel chart-panel--large mt-3">
              <header><span>{{ $t('monitor.latency') }} · {{ $t('monitor.loss') }}</span><small class="detail-muted">{{ $t('monitor.pingHint') }}</small></header>
              <div class="chart-body chart-body--tall"><Line :data="pingChart" :options="pingOptions as any" /></div>
            </section>
          </v-window-item>

          <v-window-item value="traffic">
            <PortTraffic v-if="tab === 'traffic'" :key="nodeId" :agent-id="nodeId" embedded class="mt-4" />
          </v-window-item>

          <v-window-item v-if="canControl" value="control">
            <section class="control-panel">
              <v-alert v-if="!node.controllable" type="warning" variant="tonal" density="compact" class="mb-3">{{ $t('agent.controlNeedWs') }}</v-alert>
              <div class="control-actions">
                <v-btn variant="tonal" :disabled="!node.controllable || control.loading" @click="sendCmd('report_now')">{{ $t('agent.cmdReportNow') }}</v-btn>
                <v-btn variant="tonal" :disabled="!node.controllable || control.loading" @click="sendCmd('ping')">{{ $t('agent.cmdPing') }}</v-btn>
                <v-btn color="warning" variant="tonal" :disabled="!node.controllable || control.loading" @click="sendCmd('restart_singbox')">{{ $t('agent.cmdRestartSingBox') }}</v-btn>
                <v-btn color="warning" variant="tonal" :disabled="!node.controllable || control.loading" @click="sendCmd('restart_xray')">{{ $t('agent.cmdRestartXray') }}</v-btn>
                <v-btn color="error" variant="tonal" :disabled="!node.controllable || control.loading" @click="sendCmd('restart_agent')">{{ $t('agent.cmdRestartAgent') }}</v-btn>
              </div>
              <div class="control-form">
                <v-text-field v-model.number="control.interval" type="number" min="5" max="300" :label="$t('agent.intervalSeconds')" density="compact" hide-details />
                <v-btn variant="tonal" :disabled="!node.controllable || control.loading" @click="sendCmd('set_interval', { seconds: control.interval })">{{ $t('agent.cmdSetInterval') }}</v-btn>
              </div>
              <div class="control-form control-shell">
                <v-text-field v-model="control.shell" :label="$t('agent.execCommand')" :placeholder="$t('agent.execPlaceholder')" density="compact" hide-details dir="ltr" @keyup.enter="runShell" />
                <v-btn color="primary" variant="tonal" :loading="control.loading" :disabled="!node.controllable || !control.shell.trim()" @click="runShell">{{ $t('agent.sendCommand') }}</v-btn>
              </div>
              <v-textarea v-if="control.lastOutput" class="mt-3" :model-value="control.lastOutput" :label="$t('agent.output')" readonly auto-grow rows="4" dir="ltr" hide-details />
              <div class="command-title">{{ $t('agent.commandLog') }}</div>
              <div v-if="!node.commands?.length" class="empty-state">{{ $t('noData') }}</div>
              <v-list v-else density="compact" bg-color="transparent" class="command-log">
                <v-list-item v-for="item in node.commands.slice().reverse().slice(0, 12)" :key="item.id">
                  <v-list-item-title>
                    <v-chip size="x-small" :color="item.ok ? 'success' : 'error'" variant="tonal" class="me-2">{{ item.ok ? 'OK' : 'ERR' }}</v-chip>
                    {{ item.type }} <span class="detail-muted ms-2">{{ item.elapsed_ms }}ms</span>
                  </v-list-item-title>
                  <v-list-item-subtitle class="text-truncate" dir="ltr">{{ item.error || item.output || '-' }}</v-list-item-subtitle>
                </v-list-item>
              </v-list>
            </section>
          </v-window-item>
        </v-window>
      </template>
    </section>
  </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { push } from 'notivue'
import { Line } from 'vue-chartjs'
import { useTheme } from 'vuetify'
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
import type { AgentMetricPoint, AgentNode } from '@/types/agents'
import { fetchBackendObject as api, resolveBackendWebSocketUrl } from '@/utils/backend'
import Data from '@/store/modules/data'
import PortTraffic from '@/components/PortTraffic.vue'
import RelaySpeedtest from '@/components/RelaySpeedtest.vue'
import UsageBar from '@/components/monitor/UsageBar.vue'
import PriceTags from '@/components/monitor/PriceTags.vue'
import {
  bytes, ewma, flagEmoji, osName, percent, rate, trafficPercent, trafficUsed, uptimeText, usagePercent, usageText,
} from '@/utils/monitor'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const theme = useTheme()
const nodeId = computed(() => Number(route.params.id))
const node = ref<AgentNode | null>(null)
const allNodes = ref<AgentNode[]>([])
const loading = ref(false)
const openingPanel = ref(false)
const errorMessage = ref('')
const tab = ref('load')
const speedtestOpen = ref(false)
const control = reactive({ loading: false, shell: '', interval: 15, lastOutput: '' })
const term = reactive({ visible: false, connected: false, buffer: '' })
const termEl = ref<HTMLElement | null>(null)
const canControl = computed(() => Data().controllerMode.can_control !== false)
const canOpenPanel = computed(() => Boolean(
  canControl.value && node.value?.managed && node.value?.controllable && node.value?.report.panel?.public_url,
))
let termWs: WebSocket | null = null
let refreshTimer: number | undefined
let listTimer: number | undefined

// Chart ranges; live uses the samples kept in memory, the rest stored history.
const ranges = [
  { key: 'live', seconds: 0 },
  { key: '1h', seconds: 3600 },
  { key: '6h', seconds: 6 * 3600 },
  { key: '1d', seconds: 86400 },
  { key: '7d', seconds: 7 * 86400 },
  { key: '30d', seconds: 30 * 86400 },
]
const range = ref('live')
const metrics = ref<AgentMetricPoint[]>([])
const metricsLoading = ref(false)
const readSmooth = () => { try { return localStorage.getItem('monitorSmooth') === '1' } catch { return false } }
const smooth = ref(readSmooth())
const saveSmooth = () => { try { localStorage.setItem('monitorSmooth', smooth.value ? '1' : '0') } catch { /* private mode */ } }

const loadNode = async (silent = true) => {
  if (loading.value) return
  loading.value = true
  const id = nodeId.value
  try {
    const result = await api(`api/agents/${id}`)
    if (id === nodeId.value) node.value = result
    errorMessage.value = ''
  } catch (error: any) {
    errorMessage.value = error?.message || i18n.global.t('agent.loadFailed')
    if (!silent) push.error({ message: errorMessage.value })
  } finally { loading.value = false }
}
const loadList = async () => {
  try { allNodes.value = await api('api/agents') || [] } catch { /* the sidebar is optional */ }
}
const loadMetrics = async () => {
  const selected = ranges.find(item => item.key === range.value)
  if (!selected?.seconds) { metrics.value = []; return }
  metricsLoading.value = true
  const id = nodeId.value
  try {
    const result = await api(`api/agents/${id}/metrics?range=${selected.seconds}`) || []
    if (id === nodeId.value) metrics.value = result
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.loadFailed') })
  } finally { metricsLoading.value = false }
}
const refreshAll = () => { void loadNode(false); void loadMetrics() }

watch(range, () => void loadMetrics())
watch(nodeId, () => {
  closeTerminal()
  node.value = null
  metrics.value = []
  control.lastOutput = ''
  void loadNode(false)
  void loadMetrics()
})

const sidebarGroups = computed(() => {
  const groups = new Map<string, AgentNode[]>()
  const sorted = allNodes.value.slice().sort((a, b) => Number(a.sort_weight || 0) - Number(b.sort_weight || 0) || a.name.localeCompare(b.name))
  for (const item of sorted) {
    const key = item.group || ''
    groups.set(key, [...(groups.get(key) || []), item])
  }
  // Ungrouped servers come last.
  return [...groups.entries()]
    .sort(([a], [b]) => (a ? 0 : 1) - (b ? 0 : 1) || a.localeCompare(b))
    .map(([name, nodes]) => ({ name, nodes }))
})

const details = computed(() => {
  const value = node.value
  if (!value) return []
  const report = value.report
  const traffic = value.traffic
  return [
    { label: t('monitor.cpuModel'), value: report.cpu_model ? `${report.cpu_model} (x${report.cpu_cores || '?'})` : `${report.cpu_cores || '-'} ${t('agent.cpuCores')}` },
    { label: t('monitor.arch'), value: report.arch || '-' },
    { label: t('monitor.virtualization'), value: report.virtualization || '-' },
    { label: t('monitor.os'), value: osName(value) || '-', sub: report.kernel ? `${t('monitor.kernel')}: ${report.kernel}` : '' },
    { label: t('monitor.networkSpeed'), value: `↑ ${rate(report.net_rate?.sent)} / ↓ ${rate(report.net_rate?.recv)}` },
    { label: t('monitor.totalTraffic'), value: `↑ ${bytes(report.network?.sent)} / ↓ ${bytes(report.network?.recv)}` },
    {
      label: t('monitor.periodTraffic'),
      value: `↑ ${bytes(traffic?.sent)} / ↓ ${bytes(traffic?.recv)}`,
      sub: traffic?.next_reset ? t('monitor.nextReset', { date: new Date(traffic.next_reset * 1000).toLocaleDateString() }) : '',
    },
    { label: t('monitor.ram'), value: bytes(report.memory?.total) },
    { label: t('monitor.swap'), value: bytes(report.swap?.total) },
    { label: t('agent.disk'), value: bytes(report.disk?.total) },
    { label: t('agent.uptime'), value: value.online ? uptimeText(report.uptime, t) : '-' },
    { label: t('monitor.lastUpdated'), value: value.last_seen ? new Date(value.last_seen * 1000).toLocaleString() : '-' },
    { label: t('agent.addresses'), value: [...(report.ipv4 || []), ...(report.ipv6 || [])].join(', ') || value.remote_ip || '-', wide: true },
    { label: t('agent.connection'), value: connectionLabel(value), sub: `${t('monitor.agentVersion')}: ${value.version || '-'}` },
    ...(value.remark ? [{ label: t('monitor.remark'), value: value.remark, wide: true }] : []),
  ]
})

// rows normalizes live samples and stored points into one shape.
const rows = computed(() => {
  if (range.value !== 'live') {
    return metrics.value.map(point => ({
      time: point.time, cpu: point.cpu, mem: point.mem, swap: point.swap, disk: point.disk, load1: point.load1,
      processes: point.processes, tcp: point.tcp, udp: point.udp, up: point.net_sent_rate, down: point.net_recv_rate,
      sent: point.net_sent, recv: point.net_recv,
    }))
  }
  return (node.value?.history || []).map(sample => ({
    time: sample.time, cpu: sample.cpu_percent, mem: sample.mem_percent, swap: sample.swap_percent ?? null, disk: sample.disk_percent ?? null,
    load1: sample.load1 ?? null, processes: sample.process_count ?? null, tcp: sample.tcp_conns ?? null, udp: sample.udp_conns ?? null,
    up: sample.net_sent_rate ?? null, down: sample.net_recv_rate ?? null, sent: null, recv: null,
  }))
})
type Row = typeof rows.value[number]
const longRange = computed(() => ['7d', '30d'].includes(range.value))
const timeLabel = (time: number) => {
  const date = new Date(time * 1000)
  if (longRange.value) return date.toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  if (range.value === 'live') return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
const labels = computed(() => rows.value.map(row => timeLabel(row.time)))
const series = (pick: (row: Row) => number | null | undefined, list: Row[] = rows.value) => {
  const values = list.map(row => { const value = pick(row); return value == null ? null : Number(value) })
  return smooth.value ? ewma(values) : values
}
const palette = { blue: '#0090ff', green: '#30a46c', amber: '#f76b15', violet: '#8e4ec6', cyan: '#00a2c7', ruby: '#e54666', gray: '#8d8d8d' }
const line = (label: string, data: (number | null)[], color: string, extra: Record<string, unknown> = {}) => ({
  label, data, borderColor: color, backgroundColor: color, borderWidth: 2, pointRadius: data.length <= 30 ? 2 : 0, pointHitRadius: 8,
  tension: 0.3, fill: false, spanGaps: false, ...extra,
})
const cpuChart = computed(() => ({ labels: labels.value, datasets: [
  line('CPU', series(row => row.cpu), palette.blue),
  line(t('monitor.load'), series(row => row.load1), palette.amber, { yAxisID: 'y1', borderDash: [4, 3] }),
] }))
const memoryChart = computed(() => ({ labels: labels.value, datasets: [
  line(t('monitor.ram'), series(row => row.mem), palette.green),
  line(t('monitor.swap'), series(row => row.swap), palette.amber),
] }))
const diskChart = computed(() => ({ labels: labels.value, datasets: [line(t('agent.disk'), series(row => row.disk), palette.violet)] }))
const networkChart = computed(() => ({ labels: labels.value, datasets: [
  line(t('monitor.upSpeed'), series(row => row.up), palette.violet),
  line(t('monitor.downSpeed'), series(row => row.down), palette.cyan),
] }))
const connChart = computed(() => ({ labels: labels.value, datasets: [
  line('TCP', series(row => row.tcp), palette.blue),
  line('UDP', series(row => row.udp), palette.ruby),
] }))
const processChart = computed(() => ({ labels: labels.value, datasets: [line(t('monitor.processes'), series(row => row.processes), palette.ruby)] }))
const trafficChart = computed(() => ({ labels: labels.value, datasets: [
  line('↑', rows.value.map(row => row.sent), palette.violet),
  line('↓', rows.value.map(row => row.recv), palette.cyan),
] }))
const rangeTraffic = computed(() => rows.value.reduce((total, row) => ({ sent: total.sent + Number(row.sent || 0), recv: total.recv + Number(row.recv || 0) }), { sent: 0, recv: 0 }))

// Ping comes from the controller's probes over the agent connection.
const pingRows = computed(() => {
  if (range.value !== 'live') return metrics.value.map(point => ({ time: point.time, ms: point.ping_ms > 0 ? point.ping_ms : null, loss: point.ping_loss }))
  return (node.value?.latency_history || []).map(point => ({ time: point.time, ms: point.ok ? point.ms : null, loss: point.ok ? 0 : 100 }))
})
const pingChart = computed(() => {
  const ms = pingRows.value.map(row => row.ms)
  return {
    labels: pingRows.value.map(row => timeLabel(row.time)),
    datasets: [
      line(t('monitor.latency'), smooth.value ? ewma(ms) : ms, palette.blue),
      line(t('monitor.loss'), pingRows.value.map(row => row.loss), palette.ruby, { yAxisID: 'y1', borderWidth: 1, borderDash: [4, 3] }),
    ],
  }
})
const pingStats = computed(() => {
  const values = pingRows.value.map(row => row.ms).filter((value): value is number => value != null).sort((a, b) => a - b)
  const losses = pingRows.value.map(row => Number(row.loss || 0))
  const avg = values.length ? values.reduce((a, b) => a + b, 0) / values.length : undefined
  const p95 = values.length ? values[Math.min(values.length - 1, Math.floor(values.length * 0.95))] : undefined
  const loss = losses.length ? losses.reduce((a, b) => a + b, 0) / losses.length : undefined
  return {
    avg: avg == null ? '-' : `${avg.toFixed(1)} ms`,
    p95: p95 == null ? '-' : `${Math.round(p95)} ms`,
    loss: loss == null ? '-' : `${loss.toFixed(1)}%`,
  }
})

const baseOptions = computed(() => {
  const onSurface = theme.current.value.colors['on-surface']
  return {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: { display: true, labels: { color: onSurface, boxWidth: 8, boxHeight: 8, usePointStyle: true, pointStyle: 'rectRounded' } },
      tooltip: { enabled: true },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: onSurface, maxTicksLimit: 6, maxRotation: 0 } },
      y: { beginAtZero: true, grid: { color: `${onSurface}1a` }, ticks: { color: onSurface, maxTicksLimit: 5 } },
    },
  }
})
const withY = (y: Record<string, unknown>, y1?: Record<string, unknown>, tooltip?: (label: string, value: number) => string) => {
  const base = baseOptions.value
  const scales: Record<string, unknown> = { ...base.scales, y: { ...base.scales.y, ...y, ticks: { ...base.scales.y.ticks, ...(y.ticks as object || {}) } } }
  if (y1) scales.y1 = { position: 'right', beginAtZero: true, grid: { display: false }, ...y1, ticks: { color: base.scales.y.ticks.color, maxTicksLimit: 5, ...(y1.ticks as object || {}) } }
  const plugins = tooltip
    ? { ...base.plugins, tooltip: { enabled: true, callbacks: { label: (ctx: any) => tooltip(ctx.dataset.label, Number(ctx.parsed.y)) } } }
    : base.plugins
  return { ...base, plugins, scales }
}
const percentTick = (value: any) => `${value}%`
const percentOptions = computed(() => withY({ min: 0, max: 100, ticks: { callback: percentTick } }, undefined, (label, value) => `${label}: ${percent(value)}`))
const cpuOptions = computed(() => withY({ min: 0, max: 100, ticks: { callback: percentTick } }, {}, (label, value) => label === 'CPU' ? `CPU: ${percent(value)}` : `${label}: ${value.toFixed(2)}`))
const rateOptions = computed(() => withY({ ticks: { callback: (value: any) => rate(Number(value)) } }, undefined, (label, value) => `${label}: ${rate(value)}`))
const bytesOptions = computed(() => withY({ ticks: { callback: (value: any) => bytes(Number(value)) } }, undefined, (label, value) => `${label} ${bytes(value)}`))
const countOptions = computed(() => withY({}, undefined, (label, value) => `${label}: ${Math.round(value)}`))
const pingOptions = computed(() => withY(
  { ticks: { callback: (value: any) => `${value} ms` } },
  { min: 0, max: 100, ticks: { callback: percentTick } },
  (label, value) => label === t('monitor.loss') ? `${label}: ${percent(value)}` : `${label}: ${value.toFixed(1)} ms`,
))

const sendCmd = async (type: string, args?: Record<string, any>) => {
  if (!canControl.value || !node.value) return
  control.loading = true
  try {
    const result = await api(`api/agents/${nodeId.value}/command`, { method: 'POST', body: JSON.stringify({ type, args: args || {} }) })
    control.lastOutput = [result?.output, result?.error].filter(Boolean).join('\n') || JSON.stringify(result)
    if (result?.ok) push.success({ message: i18n.global.t('agent.controlSuccess') })
    else push.error({ message: result?.error || i18n.global.t('agent.controlFailed') })
    await loadNode(false)
  } catch (error: any) {
    control.lastOutput = error?.message || i18n.global.t('agent.controlFailed')
    push.error({ message: control.lastOutput })
  } finally { control.loading = false }
}
const runShell = () => { if (control.shell.trim()) void sendCmd('exec', { command: control.shell.trim() }) }
const manageInbounds = () => { if (canControl.value && node.value?.managed) void router.push(`/agents/${nodeId.value}/inbounds`) }

const openManagedPanel = async () => {
  if (!canOpenPanel.value || openingPanel.value) return
  const targetName = `sui-managed-${Date.now()}`
  const popup = window.open('about:blank', targetName)
  if (!popup) return push.error({ message: i18n.global.t('agent.popupBlocked') })
  popup.opener = null
  openingPanel.value = true
  try {
    const grant = await api(`api/agents/${nodeId.value}/panel-access`, { method: 'POST', body: '{}' })
    const form = document.createElement('form')
    const token = document.createElement('input')
    form.method = 'POST'
    form.action = new URL('api/managed-login', grant.panel_url).toString()
    form.target = targetName
    form.hidden = true
    token.type = 'hidden'
    token.name = 'token'
    token.value = grant.token
    form.appendChild(token)
    document.body.appendChild(form)
    form.submit()
    form.remove()
  } catch (error: any) {
    popup.close()
    push.error({ message: error?.message || i18n.global.t('agent.openPanelFailed') })
  } finally {
    openingPanel.value = false
  }
}

const openTerminal = async () => {
  if (!canControl.value || !node.value?.controllable) return push.error({ message: i18n.global.t('agent.controlNeedWs') })
  closeTerminal()
  term.visible = true
  term.buffer = ''
  await nextTick()
  focusTerm()
  termWs = new WebSocket(resolveBackendWebSocketUrl(`api/agents/${nodeId.value}/terminal?cols=100&rows=30`))
  termWs.onopen = () => { term.connected = true }
  termWs.onclose = () => { term.connected = false }
  termWs.onerror = () => { term.connected = false; term.buffer += '\r\n[connection error]\r\n' }
  termWs.onmessage = (event) => {
    try {
      const message = JSON.parse(event.data)
      if (message.type === 'terminal_output' && message.data) {
        term.buffer += atob(message.data)
        if (term.buffer.length > 200000) term.buffer = term.buffer.slice(-150000)
        void nextTick(() => { if (termEl.value) termEl.value.scrollTop = termEl.value.scrollHeight })
      } else if (message.type === 'terminal_closed') {
        term.connected = false
        term.buffer += `\r\n[${message.error || 'closed'}]\r\n`
      } else if (message.type === 'terminal_opened') term.connected = true
    } catch { /* ignore malformed terminal frames */ }
  }
}
const closeTerminal = () => {
  if (termWs) {
    try { termWs.send(JSON.stringify({ type: 'close' })) } catch { /* connection may already be closed */ }
    termWs.close()
    termWs = null
  }
  term.visible = false
  term.connected = false
}
const focusTerm = () => termEl.value?.focus()
const sendTermRaw = (value: string) => { if (termWs?.readyState === WebSocket.OPEN) termWs.send(JSON.stringify({ type: 'input', data: value })) }
const onTermKey = (event: KeyboardEvent) => {
  if (!term.connected) return
  event.preventDefault()
  const keys: Record<string, string> = { Enter: '\r', Backspace: '\x7f', Tab: '\t', Escape: '\x1b', ArrowUp: '\x1b[A', ArrowDown: '\x1b[B', ArrowRight: '\x1b[C', ArrowLeft: '\x1b[D', Home: '\x1b[H', End: '\x1b[F', Delete: '\x1b[3~' }
  if (keys[event.key]) return sendTermRaw(keys[event.key])
  if (event.ctrlKey && event.key.length === 1) {
    const code = event.key.toLowerCase().charCodeAt(0) - 96
    if (code >= 1 && code <= 26) return sendTermRaw(String.fromCharCode(code))
  }
  if (event.key.length === 1) sendTermRaw(event.key)
}
const onTermPaste = (event: ClipboardEvent) => { event.preventDefault(); sendTermRaw(event.clipboardData?.getData('text') || '') }

const limitTypeLabel = (value?: string) => t(`monitor.limit.${value || 'sum'}`)
const connectionLabel = (value: AgentNode) => value.ws_connected || value.conn_mode === 'ws' || value.report.conn_mode === 'ws' ? i18n.global.t('agent.connWs') : value.online ? i18n.global.t('agent.connHttp') : '-'
const latencyValue = computed(() => node.value?.online && node.value.latency?.last_ms != null ? `${node.value.latency.last_ms} ms` : '-')
const latencyClass = computed(() => {
  const value = node.value
  if (!value?.online || value.latency?.last_ms == null) return 'text-medium-emphasis'
  const loss = Number(value.latency.loss_pct || 0)
  return loss >= 20 || value.latency.last_ms >= 250 ? 'text-error' : loss > 0 || value.latency.last_ms >= 100 ? 'text-warning' : 'text-success'
})

onMounted(() => {
  void loadNode(false)
  void loadList()
  refreshTimer = window.setInterval(() => {
    if (document.hidden) return
    void loadNode(true)
    // Stored history gains one point a minute; refresh short ranges only.
    if (range.value === '1h' || range.value === '6h') void loadMetrics()
  }, 5000)
  listTimer = window.setInterval(() => { if (!document.hidden) void loadList() }, 15000)
})
onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  if (listTimer) window.clearInterval(listTimer)
  closeTerminal()
})
</script>

<style scoped>
.detail-layout { display: flex; align-items: flex-start; gap: 16px; }
.server-sidebar { position: sticky; top: 8px; flex: 0 0 240px; max-height: calc(100dvh - 32px); display: flex; flex-direction: column; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); overflow: hidden; }
.server-sidebar__title { padding: 12px 14px; font-weight: 600; font-size: 0.9rem; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.server-sidebar__list { overflow-y: auto; padding-bottom: 6px; }
.server-sidebar__group { position: sticky; top: 0; z-index: 1; padding: 6px 14px; font-size: 0.72rem; font-weight: 600; color: rgb(var(--v-theme-primary)); background: rgba(var(--v-theme-primary), 0.08); backdrop-filter: blur(6px); }
.server-sidebar__item { width: 100%; display: flex; align-items: center; gap: 8px; padding: 8px 14px; border-left: 4px solid transparent; font-size: 0.84rem; cursor: pointer; transition: background 0.15s; }
.server-sidebar__item:hover { background: rgba(var(--v-theme-primary), 0.05); }
.server-sidebar__item--active { border-left-color: rgb(var(--v-theme-primary)); background: rgba(var(--v-theme-primary), 0.12); color: rgb(var(--v-theme-primary)); font-weight: 600; }
.server-sidebar__name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.agent-detail-page { flex: 1 1 auto; min-width: 0; display: grid; gap: 14px; padding-bottom: 8px; }
:global(.app-main:has(.agent-detail-page)) { height: 100dvh; min-height: 0; overflow-y: auto !important; overscroll-behavior-y: contain; scrollbar-gutter: stable; touch-action: pan-y; -webkit-overflow-scrolling: touch; }
.detail-header { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 10px; }
.detail-identity { min-width: 0; display: grid; gap: 4px; }
.identity-title { display: flex; align-items: center; gap: 10px; min-width: 0; }
.identity-flag { font-size: 1.5rem; line-height: 1; }
.identity-title h1 { margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 1.35rem; font-weight: 700; letter-spacing: 0; }
.identity-sub { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; color: rgba(var(--v-theme-on-surface), 0.62); font-size: 0.8rem; }
.detail-actions, .control-actions, .control-form { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.status-badge { padding: 1px 8px; border-radius: 4px; font-size: 0.74rem; font-weight: 500; white-space: nowrap; }
.status-badge--on { color: #218358; background: rgba(48, 164, 108, 0.16); }
.status-badge--off { color: #ce2c31; background: rgba(229, 72, 77, 0.16); }
.status-dot { flex-shrink: 0; display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.dot-online { background: rgb(var(--v-theme-success)); }
.dot-offline { background: rgb(var(--v-theme-error)); }

.details-card { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 24px; padding: 16px 18px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); }
.details-item { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.details-item--wide { grid-column: 1 / -1; }
.details-item > span { font-size: 0.78rem; font-weight: 600; }
.details-item > strong { font-size: 0.86rem; font-weight: 400; color: rgba(var(--v-theme-on-surface), 0.78); word-break: break-word; }
.details-item > small { font-size: 0.72rem; color: rgba(var(--v-theme-on-surface), 0.55); }

.live-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; padding: 14px 18px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); }
.live-fact { min-width: 0; display: flex; flex-direction: column; gap: 2px; font-size: 0.8rem; }
.live-fact > span { color: rgba(var(--v-theme-on-surface), 0.6); }
.live-fact > strong { font-weight: 500; }
.core-line { display: flex; gap: 6px; }
.core-line span { padding: 0 5px; border-radius: 4px; font-size: 0.68rem; color: rgba(var(--v-theme-on-surface), 0.45); background: rgba(var(--v-theme-on-surface), 0.07); }
.core-line span.on { color: #218358; background: rgba(48, 164, 108, 0.16); }

.detail-tabs { border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.range-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; overflow-x: auto; }
.smooth-switch { flex: 0 0 auto; }
.chart-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.chart-panel, .control-panel { min-width: 0; padding: 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); }
.chart-panel--large { grid-column: 1 / -1; }
.chart-panel header { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 8px; }
.chart-panel header span { color: rgba(var(--v-theme-on-surface), 0.65); font-size: 0.82rem; }
.chart-panel header strong { font-size: 0.86rem; font-weight: 500; white-space: nowrap; }
.chart-body { height: 200px; }
.chart-body--tall { height: 300px; }
.ping-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.ping-stats > div { padding: 12px 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); display: flex; flex-direction: column; gap: 2px; }
.ping-stats span { font-size: 0.78rem; color: rgba(var(--v-theme-on-surface), 0.6); }
.ping-stats strong { font-weight: 500; }
.control-panel { margin-top: 14px; }
.control-form { margin-top: 12px; }
.control-form > :first-child { flex: 1 1 220px; }
.control-shell > :first-child { flex-basis: 480px; }
.command-title { margin: 18px 0 8px; color: rgba(var(--v-theme-on-surface), 0.65); font-size: 0.82rem; }
.command-log { max-height: 260px; overflow: auto; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; }
.empty-state { padding: 32px 16px; text-align: center; color: rgba(var(--v-theme-on-surface), 0.56); }
.detail-muted { color: rgba(var(--v-theme-on-surface), 0.62); font-size: 0.82rem; }
.term-card { background: #0b1020 !important; color: #d7e0ff; }
.term-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.term-screen { height: min(62vh, 520px); overflow: auto; padding: 12px 14px; background: #070b16; color: #c8facc; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 13px; line-height: 1.35; white-space: pre-wrap; word-break: break-word; outline: none; }
@media (max-width: 1280px) {
  .server-sidebar { display: none; }
}
@media (max-width: 1100px) {
  .chart-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 900px) {
  .detail-header { grid-template-columns: 40px minmax(0, 1fr); }
  .detail-actions { grid-column: 1 / -1; justify-content: center; }
  .live-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 600px) {
  .details-card { grid-template-columns: minmax(0, 1fr); }
  .chart-grid, .ping-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .chart-grid { grid-template-columns: minmax(0, 1fr); }
  .detail-actions .v-btn:not(:last-child) { flex: 1 1 135px; }
  .detail-tabs :deep(.v-btn) { min-width: 0; padding-inline: 10px; }
}
</style>
