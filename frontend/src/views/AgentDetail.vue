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

  <section class="agent-detail-page">
    <header class="detail-header">
      <v-btn icon="mdi-arrow-left" variant="text" :title="$t('agent.backServers')" @click="router.push('/agents')" />
      <div class="detail-identity">
        <div class="identity-title">
          <i class="status-dot" :class="node?.online ? 'dot-online' : 'dot-offline'" />
          <h1 class="adaptive-ink">{{ node?.name || ('#' + nodeId) }}</h1>
        </div>
        <p class="adaptive-ink" dir="ltr">{{ node?.report.hostname || node?.remote_ip || '-' }}</p>
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
        <v-btn icon="mdi-refresh" variant="tonal" :loading="loading" :title="$t('actions.update')" @click="loadNode(false)" />
      </div>
    </header>

    <v-progress-linear v-if="loading && !node" indeterminate />
    <v-alert v-else-if="errorMessage" type="error" variant="tonal">{{ errorMessage }}</v-alert>

    <template v-if="node">
      <section class="system-strip">
        <div><span>{{ $t('agent.status') }}</span><strong :class="node.online ? 'text-success' : 'text-error'">{{ node.online ? $t('online') : $t('agent.offline') }}</strong></div>
        <div><span>{{ $t('agent.uptime') }}</span><strong>{{ formatUptime(node.report.uptime) }}</strong></div>
        <div><span>{{ $t('agent.platform') }}</span><strong dir="ltr">{{ platform(node) }}</strong></div>
        <div><span>{{ $t('agent.memory') }}</span><strong>{{ bytes(node.report.memory?.total) }}</strong></div>
        <div><span>{{ $t('agent.disk') }}</span><strong>{{ bytes(node.report.disk?.total) }}</strong></div>
        <div><span>{{ $t('agent.connection') }}</span><strong>{{ connectionLabel(node) }}</strong></div>
      </section>

      <div class="live-metric-grid">
        <section class="live-metric">
          <header><span>CPU</span><strong>{{ percent(node.report.cpu_percent) }}</strong></header>
          <v-progress-linear :model-value="clampPercent(node.report.cpu_percent)" :color="metricColor(node.report.cpu_percent)" height="5" rounded />
          <small>{{ node.report.cpu_cores || '-' }} {{ $t('agent.cpuCores') }}</small>
        </section>
        <section class="live-metric">
          <header><span>MEM</span><strong>{{ percent(usagePercent(node.report.memory)) }}</strong></header>
          <v-progress-linear :model-value="usagePercent(node.report.memory)" :color="metricColor(usagePercent(node.report.memory))" height="5" rounded />
          <small>{{ usage(node.report.memory) }}</small>
        </section>
        <section class="live-metric">
          <header><span>STG</span><strong>{{ percent(usagePercent(node.report.disk)) }}</strong></header>
          <v-progress-linear :model-value="usagePercent(node.report.disk)" :color="metricColor(usagePercent(node.report.disk))" height="5" rounded />
          <small>{{ usage(node.report.disk) }}</small>
        </section>
        <section class="live-metric">
          <header><span>{{ $t('agent.load') }}</span><strong dir="ltr">{{ metricNumber(node.report.load?.load1) }}</strong></header>
          <v-progress-linear :model-value="loadPercent(node)" :color="metricColor(loadPercent(node))" height="5" rounded />
          <small dir="ltr">5m {{ metricNumber(node.report.load?.load5) }} · 15m {{ metricNumber(node.report.load?.load15) }}</small>
        </section>
        <section class="live-metric">
          <header><span>Ping</span><strong dir="ltr">{{ latencyValue(node) }}</strong></header>
          <v-progress-linear :model-value="latencyProgress(node)" :color="latencyColor(node)" height="5" rounded />
          <small dir="ltr">{{ latencyMeta(node) }}</small>
        </section>
        <section class="live-metric">
          <header><span>{{ $t('agent.processes') }}</span><strong>{{ node.report.process_count ?? '-' }}</strong></header>
          <div class="core-line">
            <span><i :class="node.report.cores?.singbox_running ? 'dot-online' : 'dot-offline'" /> sing-box</span>
            <span><i :class="node.report.cores?.xray_running ? 'dot-online' : 'dot-offline'" /> Xray</span>
          </div>
        </section>
      </div>

      <v-tabs v-model="tab" align-tabs="center" color="primary" class="detail-tabs">
        <v-tab value="detail" :aria-label="$t('agent.realtime')">{{ $t('agent.realtime') }}</v-tab>
        <v-tab value="network" :aria-label="$t('agent.network')">{{ $t('agent.network') }}</v-tab>
        <v-tab value="traffic" :aria-label="$t('pages.portTraffic')">{{ $t('pages.portTraffic') }}</v-tab>
        <v-tab v-if="canControl" value="control" :aria-label="$t('agent.control')">{{ $t('agent.control') }}</v-tab>
      </v-tabs>

      <v-window v-model="tab">
        <v-window-item value="detail">
          <div v-if="!history.length" class="empty-state">{{ $t('noData') }}</div>
          <div v-else class="chart-grid">
            <section class="chart-panel">
              <header><span>CPU</span><strong>{{ percent(node.report.cpu_percent) }}</strong></header>
              <div class="chart-body"><Line :data="cpuChart" :options="percentChartOptions as any" /></div>
            </section>
            <section class="chart-panel">
              <header><span>{{ $t('agent.memory') }}</span><strong>{{ percent(usagePercent(node.report.memory)) }}</strong></header>
              <div class="chart-body"><Line :data="memoryChart" :options="percentChartOptions as any" /></div>
            </section>
            <section class="chart-panel">
              <header><span>{{ $t('agent.disk') }}</span><strong>{{ percent(usagePercent(node.report.disk)) }}</strong></header>
              <div class="chart-body"><Line :data="diskChart" :options="percentChartOptions as any" /></div>
            </section>
            <section class="chart-panel">
              <header><span>{{ $t('agent.processes') }}</span><strong>{{ node.report.process_count ?? '-' }}</strong></header>
              <div class="chart-body"><Line :data="processChart" :options="countChartOptions as any" /></div>
            </section>
          </div>
        </v-window-item>

        <v-window-item value="network">
          <div class="network-summary">
            <div><span>Upload</span><strong dir="ltr">{{ rate(node.report.net_rate?.sent) }}</strong><small dir="ltr">{{ bytes(node.report.network?.sent) }}</small></div>
            <div><span>Download</span><strong dir="ltr">{{ rate(node.report.net_rate?.recv) }}</strong><small dir="ltr">{{ bytes(node.report.network?.recv) }}</small></div>
            <div><span>{{ $t('agent.addresses') }}</span><strong dir="ltr">{{ addressList(node) }}</strong></div>
          </div>
          <section v-if="history.length" class="chart-panel network-chart">
            <header><span>{{ $t('agent.netRate') }}</span><strong dir="ltr">↑ {{ rate(node.report.net_rate?.sent) }} · ↓ {{ rate(node.report.net_rate?.recv) }}</strong></header>
            <div class="chart-body chart-body-wide"><Line :data="networkChart" :options="networkChartOptions as any" /></div>
          </section>
        </v-window-item>

        <v-window-item value="traffic">
          <PortTraffic v-if="tab === 'traffic'" :agent-id="nodeId" embedded class="mt-4" />
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
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
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
import type { AgentNode, AgentUsage } from '@/types/agents'
import { fetchBackendObject as api, resolveBackendWebSocketUrl } from '@/utils/backend'
import Data from '@/store/modules/data'
import PortTraffic from '@/components/PortTraffic.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const route = useRoute()
const router = useRouter()
const theme = useTheme()
const nodeId = Number(route.params.id)
const node = ref<AgentNode | null>(null)
const loading = ref(false)
const openingPanel = ref(false)
const errorMessage = ref('')
const tab = ref('detail')
const control = reactive({ loading: false, shell: '', interval: 15, lastOutput: '' })
const term = reactive({ visible: false, connected: false, buffer: '' })
const termEl = ref<HTMLElement | null>(null)
const canControl = computed(() => Data().controllerMode.can_control !== false)
const canOpenPanel = computed(() => Boolean(
  canControl.value && node.value?.managed && node.value?.controllable && node.value?.report.panel?.public_url,
))
let termWs: WebSocket | null = null
let refreshTimer: number | undefined

const loadNode = async (silent = true) => {
  if (loading.value) return
  loading.value = true
  try {
    node.value = await api(`api/agents/${nodeId}`)
    errorMessage.value = ''
  } catch (error: any) {
    errorMessage.value = error?.message || i18n.global.t('agent.loadFailed')
    if (!silent) push.error({ message: errorMessage.value })
  } finally { loading.value = false }
}

const history = computed(() => node.value?.history || [])
const labels = computed(() => history.value.map(sample => new Date(sample.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })))
const dataset = (label: string, data: number[], color: string, fill = true) => ({
  label, data, borderColor: color, backgroundColor: `${color}1f`, borderWidth: 2, pointRadius: 0, pointHitRadius: 8, tension: 0.28, fill,
})
const cpuChart = computed(() => ({ labels: labels.value, datasets: [dataset('CPU', history.value.map(sample => sample.cpu_percent || 0), '#2563eb')] }))
const memoryChart = computed(() => ({ labels: labels.value, datasets: [
  dataset('Memory', history.value.map(sample => sample.mem_percent || 0), '#16a34a'),
  dataset('Swap', history.value.map(sample => sample.swap_percent || 0), '#f59e0b', false),
] }))
const diskChart = computed(() => ({ labels: labels.value, datasets: [dataset('Disk', history.value.map(sample => sample.disk_percent || 0), '#d946ef')] }))
const processChart = computed(() => ({ labels: labels.value, datasets: [dataset('Process', history.value.map(sample => sample.process_count || 0), '#e11d48')] }))
const networkChart = computed(() => ({ labels: labels.value, datasets: [
  dataset('Upload', history.value.map(sample => sample.net_sent_rate || 0), '#7c3aed'),
  dataset('Download', history.value.map(sample => sample.net_recv_rate || 0), '#0891b2'),
] }))
const baseChartOptions = computed(() => {
  const onSurface = theme.current.value.colors['on-surface']
  return {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: { display: true, labels: { color: onSurface, boxWidth: 10, usePointStyle: true } },
      tooltip: { enabled: true },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: onSurface, maxTicksLimit: 6 } },
      y: { beginAtZero: true, grid: { color: `${onSurface}1a` }, ticks: { color: onSurface } },
    },
  }
})
const percentChartOptions = computed(() => ({ ...baseChartOptions.value, scales: { ...baseChartOptions.value.scales, y: { ...baseChartOptions.value.scales.y, min: 0, max: 100, ticks: { ...baseChartOptions.value.scales.y.ticks, callback: (value: any) => `${value}%` } } } }))
const countChartOptions = computed(() => baseChartOptions.value)
const networkChartOptions = computed(() => ({ ...baseChartOptions.value, scales: { ...baseChartOptions.value.scales, y: { ...baseChartOptions.value.scales.y, ticks: { ...baseChartOptions.value.scales.y.ticks, callback: (value: any) => rate(Number(value)) } } } }))

const sendCmd = async (type: string, args?: Record<string, any>) => {
  if (!canControl.value || !node.value) return
  control.loading = true
  try {
    const result = await api(`api/agents/${nodeId}/command`, { method: 'POST', body: JSON.stringify({ type, args: args || {} }) })
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
const manageInbounds = () => { if (canControl.value && node.value?.managed) void router.push(`/agents/${nodeId}/inbounds`) }

const openManagedPanel = async () => {
  if (!canOpenPanel.value || openingPanel.value) return
  const targetName = `sui-managed-${Date.now()}`
  const popup = window.open('about:blank', targetName)
  if (!popup) return push.error({ message: i18n.global.t('agent.popupBlocked') })
  popup.opener = null
  openingPanel.value = true
  try {
    const grant = await api(`api/agents/${nodeId}/panel-access`, { method: 'POST', body: '{}' })
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
  termWs = new WebSocket(resolveBackendWebSocketUrl(`api/agents/${nodeId}/terminal?cols=100&rows=30`))
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

const clampPercent = (value?: number) => Math.max(0, Math.min(100, Number(value) || 0))
const usagePercent = (value?: AgentUsage) => value?.total ? Number(value.used || 0) * 100 / value.total : undefined
const percent = (value?: number) => {
  if (value == null || Number.isNaN(value)) return '-'
  const digits = value > 0 && value < 1 ? 2 : 1
  return `${value.toFixed(digits)}%`
}
const usage = (value?: AgentUsage) => value?.total ? `${bytes(value.used)} / ${bytes(value.total)}` : '-'
const bytes = (value?: number) => {
  if (!value) return '0 B'
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KiB`
  if (value < 1024 ** 3) return `${(value / (1024 ** 2)).toFixed(1)} MiB`
  if (value < 1024 ** 4) return `${(value / (1024 ** 3)).toFixed(1)} GiB`
  return `${(value / (1024 ** 4)).toFixed(2)} TiB`
}
const rate = (value?: number) => {
  if (value == null) return '-'
  if (value < 1024) return `${Math.round(value)} B/s`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KiB/s`
  return `${(value / (1024 ** 2)).toFixed(2)} MiB/s`
}
const metricNumber = (value?: number) => value == null || Number.isNaN(value) ? '-' : value.toFixed(2)
const metricColor = (value?: number) => Number(value || 0) >= 90 ? 'error' : Number(value || 0) >= 70 ? 'warning' : 'success'
const loadPercent = (value: AgentNode) => clampPercent((Number(value.report.load?.load1) || 0) * 100 / Math.max(1, Number(value.report.cpu_cores || 1)))
const platform = (value: AgentNode) => [value.report.os, value.report.arch].filter(Boolean).join('/') || '-'
const connectionLabel = (value: AgentNode) => value.ws_connected || value.conn_mode === 'ws' || value.report.conn_mode === 'ws' ? i18n.global.t('agent.connWs') : value.online ? i18n.global.t('agent.connHttp') : '-'
const formatUptime = (seconds?: number) => {
  if (!seconds) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return days ? `${days}d ${hours}h` : hours ? `${hours}h ${minutes}m` : `${minutes}m`
}
const latencyValue = (value: AgentNode) => value.online && value.latency?.last_ms != null ? `${value.latency.last_ms} ms` : '-'
const latencyProgress = (value: AgentNode) => value.online && value.latency?.last_ms != null ? clampPercent(value.latency.last_ms / 5) : 0
const latencyColor = (value: AgentNode) => !value.online || value.latency?.last_ms == null ? 'default' : (value.latency.loss_pct || 0) >= 20 || value.latency.last_ms >= 250 ? 'error' : (value.latency.loss_pct || 0) > 0 || value.latency.last_ms >= 100 ? 'warning' : 'success'
const latencyMeta = (value: AgentNode) => value.latency?.samples ? `${i18n.global.t('agent.average')} ${metricNumber(value.latency.average_ms)} ms · P95 ${value.latency.p95_ms ?? '-'} ms · ${i18n.global.t('agent.loss')} ${Number(value.latency.loss_pct || 0).toFixed(0)}%` : i18n.global.t('noData')
const addressList = (value: AgentNode) => [...(value.report.ipv4 || []), ...(value.report.ipv6 || [])].join(', ') || '-'

onMounted(() => {
  void loadNode(false)
  refreshTimer = window.setInterval(() => { if (!document.hidden) void loadNode(true) }, 5000)
})
onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  closeTerminal()
})
</script>

<style scoped>
.agent-detail-page { display: grid; gap: 16px; padding-bottom: 8px; }
:global(.app-main:has(.agent-detail-page)) { height: 100dvh; min-height: 0; overflow-y: auto !important; overscroll-behavior-y: contain; scrollbar-gutter: stable; touch-action: pan-y; -webkit-overflow-scrolling: touch; }
.detail-header { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 10px; }
.detail-identity { min-width: 0; }
.identity-title { display: flex; align-items: center; gap: 10px; }
.identity-title h1 { margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 1.32rem; letter-spacing: 0; }
.detail-identity p { margin: 3px 0 0 18px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: rgba(var(--v-theme-on-surface), 0.62); font-size: 0.82rem; }
.detail-actions, .control-actions, .control-form { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.status-dot, .core-line i { display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.dot-online { background: rgb(var(--v-theme-success)); box-shadow: 0 0 0 3px rgba(var(--v-theme-success), 0.12); }
.dot-offline { background: rgb(var(--v-theme-error)); box-shadow: 0 0 0 3px rgba(var(--v-theme-error), 0.12); }
.system-strip { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); overflow: hidden; }
.system-strip > div { min-width: 0; padding: 12px 14px; border-right: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.system-strip > div:last-child { border-right: 0; }
.system-strip span, .system-strip strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.system-strip span { color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.72rem; }
.system-strip strong { margin-top: 4px; font-size: 0.82rem; }
.live-metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 9px; }
.live-metric { min-width: 0; min-height: 92px; padding: 12px 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); display: flex; flex-direction: column; gap: 10px; }
.live-metric header { display: flex; justify-content: space-between; gap: 12px; }
.live-metric header span { color: rgba(var(--v-theme-on-surface), 0.64); font-size: 0.78rem; }
.live-metric header strong { font-size: 1rem; white-space: nowrap; }
.live-metric small { margin-top: auto; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: rgba(var(--v-theme-on-surface), 0.58); }
.core-line { display: flex; flex-wrap: wrap; gap: 12px; margin-top: auto; font-size: 0.76rem; }
.core-line span { display: inline-flex; align-items: center; gap: 6px; }
.detail-tabs { border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.chart-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; padding-top: 14px; }
.chart-panel, .control-panel { min-width: 0; padding: 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.chart-panel header { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 8px; }
.chart-panel header span { color: rgba(var(--v-theme-on-surface), 0.65); font-size: 0.82rem; }
.chart-body { height: 190px; }
.chart-body-wide { height: 260px; }
.network-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; padding-top: 14px; }
.network-summary > div { min-width: 0; padding: 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.network-summary span, .network-summary strong, .network-summary small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.network-summary span { color: rgba(var(--v-theme-on-surface), 0.62); font-size: 0.78rem; }
.network-summary strong { margin-top: 5px; }
.network-summary small { margin-top: 3px; color: rgba(var(--v-theme-on-surface), 0.58); }
.network-chart { margin-top: 10px; }
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
@media (max-width: 900px) {
  .detail-header { grid-template-columns: 40px minmax(0, 1fr); }
  .detail-actions { grid-column: 1 / -1; justify-content: center; }
  .system-strip { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .system-strip > div:nth-child(3) { border-right: 0; }
  .system-strip > div:nth-child(-n+3) { border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
  .live-metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 600px) {
  .system-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .system-strip > div { border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
  .system-strip > div:nth-child(even) { border-right: 0; }
  .system-strip > div:nth-last-child(-n+2) { border-bottom: 0; }
  .live-metric-grid, .chart-grid, .network-summary { grid-template-columns: minmax(0, 1fr); }
  .detail-actions .v-btn:not(:last-child) { flex: 1 1 135px; }
  .detail-tabs :deep(.v-btn) { min-width: 0; padding-inline: 10px; }
}
</style>
