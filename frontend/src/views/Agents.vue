<template>
  <v-dialog v-model="enroll.visible" width="min(680px, calc(100vw - 24px))" scrollable>
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.enroll') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">{{ $t('agent.note') }}</v-alert>
        <template v-if="!enroll.pairURL">
          <v-progress-linear v-if="enroll.apiLoading" indeterminate color="primary" class="mb-3" />
          <v-btn v-else color="primary" variant="tonal" block prepend-icon="mdi-refresh" @click="createEnrollmentAPI">
            {{ $t('agent.generateConnectionAPI') }}
          </v-btn>
        </template>
        <template v-else>
          <v-alert type="warning" variant="tonal" density="compact" class="mb-3">
            {{ $t('agent.pairSingleUse') }} {{ pairExpiryText }}
          </v-alert>
          <v-textarea :model-value="enroll.pairURL" :label="$t('agent.simpleAddress')" :hint="$t('agent.simpleAddressHint')" persistent-hint readonly dir="ltr" rows="2" auto-grow class="mb-3">
            <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enroll.pairURL)" /></template>
          </v-textarea>
          <v-expansion-panels v-if="enroll.managedCommand" variant="accordion" class="enroll-commands">
            <v-expansion-panel>
              <v-expansion-panel-title>{{ $t('agent.installCommands') }}</v-expansion-panel-title>
              <v-expansion-panel-text>
                <v-textarea v-if="enroll.managedCommand" :model-value="enroll.managedCommand" :label="$t('agent.managedCommand')" readonly dir="ltr" rows="3" auto-grow hide-details class="mb-3">
                  <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enroll.managedCommand)" /></template>
                </v-textarea>
              </v-expansion-panel-text>
            </v-expansion-panel>
          </v-expansion-panels>
        </template>
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="closeEnrollment">{{ $t('actions.close') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="connect.visible" width="min(580px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.connectController') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-alert v-if="localConnection.configured" type="success" variant="tonal" density="compact" class="mb-3">
          <strong>{{ $t('agent.localConnected') }}</strong>
          <div dir="ltr" class="mt-1">{{ localConnection.panel_url }}</div>
        </v-alert>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">{{ $t('agent.connectSimpleHint') }}</v-alert>
        <v-textarea v-model="connect.url" :label="$t('agent.connectInput')" :hint="$t('agent.connectInputHint')" persistent-hint rows="2" auto-grow dir="ltr" autofocus />
        <v-checkbox v-model="connect.insecure" :label="$t('agent.allowInsecure')" :hint="$t('agent.allowInsecureHint')" persistent-hint density="compact" hide-details="auto" />
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn v-if="localConnection.configured" color="warning" variant="tonal" :loading="connect.loading" @click="disconnectController">{{ $t('agent.disconnectController') }}</v-btn>
        <v-btn variant="outlined" @click="connect.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-link-variant-plus" :loading="connect.loading" :disabled="!connect.url.trim()" @click="connectToController">{{ $t('agent.connectNow') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="edit.visible" width="min(520px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.editNode') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-text-field v-model="edit.name" :label="$t('agent.name')" maxlength="80" hide-details class="mb-3" />
        <v-text-field v-model="edit.publicHost" :label="$t('agent.publicHost')" :hint="$t('agent.publicHostHint')" persistent-hint dir="ltr" />
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="edit.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="edit.loading" :disabled="!edit.name.trim()" @click="saveNode">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="batch.resultVisible" width="min(720px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.batchResult') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-list density="compact">
          <v-list-item v-for="item in batch.results" :key="item.node_id">
            <v-list-item-title>
              <v-chip size="x-small" :color="item.ok ? 'success' : 'error'" variant="tonal" class="me-2">{{ item.ok ? 'OK' : 'ERR' }}</v-chip>
              {{ item.name || ('#' + item.node_id) }}
            </v-list-item-title>
            <v-list-item-subtitle dir="ltr">{{ item.error || item.result?.output || item.result?.error || '-' }}</v-list-item-subtitle>
          </v-list-item>
        </v-list>
      </v-card-text>
      <v-card-actions class="justify-center"><v-btn variant="outlined" @click="batch.resultVisible = false">{{ $t('actions.close') }}</v-btn></v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="removeDialog.visible" width="min(420px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('actions.del') }}</v-card-title>
      <v-card-text class="text-center">{{ removeDialog.node?.name }}</v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="removeDialog.visible = false">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="removeDialog.loading" @click="deleteNode">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <section class="monitor-page">
    <header class="monitor-heading">
      <div>
        <h1>{{ $t('pages.agents') }}</h1>
        <p>{{ $t('agent.overviewHint') }}</p>
      </div>
      <div class="heading-actions">
        <v-chip v-if="localConnection.configured" color="success" prepend-icon="mdi-link-variant" size="small">
          {{ $t('agent.localConnected') }}
        </v-chip>
        <v-btn variant="tonal" prepend-icon="mdi-link-variant-plus" @click="openLocalConnect">{{ $t('agent.connectController') }}</v-btn>
        <v-btn
          v-if="controllerMode.profile === 'client'"
          color="primary"
          prepend-icon="mdi-server-network"
          :loading="controllerModeLoading"
          :disabled="!controllerMode.can_enable"
          :title="controllerMode.can_enable ? '' : serverRequirementText"
          @click="enableControllerMode"
        >{{ $t('setting.roleEnable') }}</v-btn>
        <v-btn v-else color="primary" prepend-icon="mdi-server-plus" :disabled="!serverMonitoringAvailable" :title="serverMonitoringAvailable ? '' : serverRequirementText" @click="openEnrollment">{{ $t('agent.enroll') }}</v-btn>
        <v-btn icon="mdi-refresh" variant="tonal" :loading="loading" :title="$t('actions.update')" @click="loadNodes" />
      </div>
    </header>

    <v-alert v-if="!controllerMode.enabled" type="info" variant="tonal" density="comfortable" class="mb-4" border="start" icon="mdi-server-outline">
      {{ $t('agent.controllerDisabledHint') }}
    </v-alert>
    <v-alert v-else-if="hostRequirements && !serverMonitoringAvailable" type="error" variant="tonal" density="comfortable" class="mb-4" border="start" icon="mdi-server-off">
      {{ serverRequirementText }}
    </v-alert>

    <div class="summary-grid">
      <section class="summary-item">
        <span>{{ $t('agent.totalServers') }}</span>
        <strong>{{ nodes.length }}</strong>
        <i class="summary-dot dot-primary" />
      </section>
      <section class="summary-item">
        <span>{{ $t('agent.onlineServers') }}</span>
        <strong>{{ onlineCount }}</strong>
        <i class="summary-dot dot-online" />
      </section>
      <section class="summary-item">
        <span>{{ $t('agent.offlineServers') }}</span>
        <strong>{{ offlineCount }}</strong>
        <i class="summary-dot dot-offline" />
      </section>
      <section class="summary-item summary-network">
        <span>{{ $t('agent.networkTotal') }}</span>
        <strong dir="ltr">↑ {{ rate(totalRate.sent) }} · ↓ {{ rate(totalRate.recv) }}</strong>
        <small dir="ltr">↑ {{ bytes(totalNetwork.sent) }} · ↓ {{ bytes(totalNetwork.recv) }}</small>
      </section>
    </div>

    <div class="monitor-controls">
      <v-text-field v-model="query" prepend-inner-icon="mdi-magnify" :placeholder="$t('agent.searchServers')" density="compact" hide-details clearable class="search-field" />
      <v-select v-model="sortKey" :items="sortOptions" item-title="title" item-value="value" density="compact" hide-details class="sort-field" />
      <v-btn :icon="sortDesc ? 'mdi-sort-descending' : 'mdi-sort-ascending'" variant="tonal" :title="$t('agent.sortDirection')" @click="sortDesc = !sortDesc" />
      <v-btn v-if="canControl" variant="tonal" prepend-icon="mdi-checkbox-multiple-marked" @click="selectAllControllable">{{ $t('agent.selectOnline') }}</v-btn>
      <v-btn v-if="canControl && selected.length" variant="text" @click="selected = []">{{ $t('agent.clearSelection') }}</v-btn>
    </div>

    <div v-if="canControl && selected.length" class="batch-bar">
      <strong>{{ $t('agent.batchSelected', { n: selected.length }) }}</strong>
      <v-btn size="small" variant="tonal" :loading="batch.loading" @click="batchCmd('report_now')">{{ $t('agent.cmdReportNow') }}</v-btn>
      <v-btn size="small" variant="tonal" :loading="batch.loading" @click="batchCmd('ping')">{{ $t('agent.cmdPing') }}</v-btn>
      <v-btn size="small" variant="tonal" color="warning" :loading="batch.loading" @click="batchCmd('restart_singbox')">{{ $t('agent.cmdRestartSingBox') }}</v-btn>
      <v-btn size="small" variant="tonal" color="warning" :loading="batch.loading" @click="batchCmd('restart_xray')">{{ $t('agent.cmdRestartXray') }}</v-btn>
      <v-text-field v-model="batch.shell" density="compact" hide-details :placeholder="$t('agent.execPlaceholder')" class="batch-shell" dir="ltr" />
      <v-btn size="small" color="primary" variant="tonal" :loading="batch.loading" :disabled="!batch.shell.trim()" @click="batchCmd('exec', { command: batch.shell.trim() })">{{ $t('agent.cmdExec') }}</v-btn>
    </div>

    <v-progress-linear v-if="loading && !nodes.length" indeterminate class="mb-3" />
    <v-alert v-if="!loading && filteredNodes.length === 0" type="info" variant="tonal">{{ nodes.length ? $t('agent.noMatch') : $t('agent.noNodes') }}</v-alert>
    <div v-else class="server-grid">
      <article v-for="node in filteredNodes" :key="node.id" class="server-row" :class="{ 'server-row--offline': !node.online }" @click="openDetail(node)">
        <div class="server-select" @click.stop>
          <v-checkbox-btn v-if="canControl" :model-value="selected.includes(node.id)" :disabled="!node.controllable" @update:model-value="toggleSelect(node.id, $event)" />
        </div>
        <div class="server-identity">
          <i class="status-dot" :class="node.online ? 'dot-online' : 'dot-offline'" />
          <div>
            <strong>{{ node.name }}</strong>
            <small dir="ltr">{{ node.report.hostname || node.remote_ip || '-' }}</small>
          </div>
        </div>
        <div class="server-metrics">
          <div class="mini-metric"><span>CPU</span><strong>{{ percent(node.report.cpu_percent) }}</strong><i :style="barStyle(node.report.cpu_percent)" /></div>
          <div class="mini-metric"><span>MEM</span><strong>{{ percent(usagePercent(node.report.memory)) }}</strong><i :style="barStyle(usagePercent(node.report.memory))" /></div>
          <div class="mini-metric"><span>STG</span><strong>{{ percent(usagePercent(node.report.disk)) }}</strong><i :style="barStyle(usagePercent(node.report.disk))" /></div>
          <div class="mini-metric"><span>Ping</span><strong dir="ltr">{{ latencyLabel(node) }}</strong></div>
          <div class="mini-metric"><span>Upload</span><strong dir="ltr">{{ rate(node.report.net_rate?.sent) }}</strong></div>
          <div class="mini-metric"><span>Download</span><strong dir="ltr">{{ rate(node.report.net_rate?.recv) }}</strong></div>
        </div>
        <div class="server-menu" @click.stop>
          <v-menu location="bottom end">
            <template #activator="{ props }"><v-btn v-bind="props" icon="mdi-dots-vertical" size="small" variant="text" :title="$t('actions.action')" /></template>
            <v-list density="compact">
              <v-list-item prepend-icon="mdi-information-outline" :title="$t('agent.detail')" @click="openDetail(node)" />
              <template v-if="canControl">
                <v-list-item prepend-icon="mdi-tune-vertical" :title="$t('agent.manageInbounds')" :disabled="!node.managed" @click="manageInbounds(node)" />
                <v-list-item prepend-icon="mdi-pencil-outline" :title="$t('agent.editNode')" @click="openEdit(node)" />
                <v-list-item prepend-icon="mdi-key-change" :title="$t('agent.rotate')" @click="rotateNode(node)" />
                <v-list-item prepend-icon="mdi-delete-outline" :title="$t('actions.del')" base-color="error" @click="askDelete(node)" />
              </template>
            </v-list>
          </v-menu>
        </div>
      </article>
    </div>
  </section>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { push } from 'notivue'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'
import type { AgentNode, AgentUsage } from '@/types/agents'
import { copyText } from '@/utils/clipboard'
import { fetchBackendObject as api, resolveFrontendUrl } from '@/utils/backend'

const router = useRouter()
const dataStore = Data()
const nodes = ref<AgentNode[]>([])
const loading = ref(false)
const query = ref('')
const sortKey = ref('default')
const sortDesc = ref(false)
const selected = ref<number[]>([])
const hostRequirements = computed(() => dataStore.hostRequirements)
const controllerModeLoading = ref(false)
const controllerMode = computed(() => dataStore.controllerMode)
const canControl = computed(() => controllerMode.value.can_control !== false)
const localConnection = reactive({ supported: false, installed: false, configured: false, running: false, panel_url: '', public_url: '', insecure: false })
const serverMonitoringAvailable = computed(() => controllerMode.value.enabled && hostRequirements.value?.can_enable_agents === true)
const currentHostMemoryGiB = computed(() => {
  const value = Number(hostRequirements.value?.mem_total_bytes || 0)
  return value > 0 ? (value / (1024 ** 3)).toFixed(2) : '?'
})
const serverRequirementText = computed(() => i18n.global.t('agent.hostRequirement', {
  cpu: hostRequirements.value?.cpu_cores ?? '?',
  memory: currentHostMemoryGiB.value,
}))

const enroll = reactive({ visible: false, apiLoading: false, pairURL: '', pairExpiresAt: 0, command: '', managedCommand: '' })
const connect = reactive({ visible: false, loading: false, url: '', insecure: false })
const edit = reactive({ visible: false, loading: false, id: 0, name: '', publicHost: '' })
const removeDialog = reactive<{ visible: boolean, loading: boolean, node?: AgentNode }>({ visible: false, loading: false })
const batch = reactive<{ loading: boolean, shell: string, results: any[], resultVisible: boolean }>({ loading: false, shell: '', results: [], resultVisible: false })
let refreshTimer: number | undefined

const pairExpiryText = computed(() => enroll.pairExpiresAt
  ? i18n.global.t('agent.pairExpires', { time: new Date(enroll.pairExpiresAt * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) })
  : '')

const sortOptions = computed(() => [
  { title: i18n.global.t('agent.sortDefault'), value: 'default' },
  { title: i18n.global.t('agent.name'), value: 'name' },
  { title: 'CPU', value: 'cpu' },
  { title: i18n.global.t('agent.memory'), value: 'memory' },
  { title: i18n.global.t('agent.disk'), value: 'disk' },
  { title: i18n.global.t('agent.latency'), value: 'latency' },
  { title: 'Upload', value: 'upload' },
  { title: 'Download', value: 'download' },
])
const onlineCount = computed(() => nodes.value.filter(node => node.online).length)
const offlineCount = computed(() => nodes.value.length - onlineCount.value)
const totalRate = computed(() => nodes.value.reduce((total, node) => ({
  sent: total.sent + Number(node.report.net_rate?.sent || 0),
  recv: total.recv + Number(node.report.net_rate?.recv || 0),
}), { sent: 0, recv: 0 }))
const totalNetwork = computed(() => nodes.value.reduce((total, node) => ({
  sent: total.sent + Number(node.report.network?.sent || 0),
  recv: total.recv + Number(node.report.network?.recv || 0),
}), { sent: 0, recv: 0 }))
const filteredNodes = computed(() => {
  const needle = query.value.trim().toLowerCase()
  const result = nodes.value.filter(node => !needle || [node.name, node.report.hostname, node.remote_ip, node.public_host, node.report.os, node.report.arch]
    .some(value => String(value || '').toLowerCase().includes(needle)))
  const value = (node: AgentNode): number | string => {
    switch (sortKey.value) {
      case 'name': return node.name.toLowerCase()
      case 'cpu': return Number(node.report.cpu_percent || 0)
      case 'memory': return Number(usagePercent(node.report.memory) || 0)
      case 'disk': return Number(usagePercent(node.report.disk) || 0)
      case 'latency': return Number(node.latency?.last_ms ?? Number.MAX_SAFE_INTEGER)
      case 'upload': return Number(node.report.net_rate?.sent || 0)
      case 'download': return Number(node.report.net_rate?.recv || 0)
      default: return node.id
    }
  }
  return result.slice().sort((a, b) => {
    const av = value(a)
    const bv = value(b)
    const order = typeof av === 'string' && typeof bv === 'string' ? av.localeCompare(bv) : Number(av) - Number(bv)
    return sortDesc.value ? -order : order
  })
})

const loadNodes = async () => {
  if (loading.value) return
  loading.value = true
  try {
    nodes.value = await api('api/agents') || []
    selected.value = canControl.value ? selected.value.filter(id => nodes.value.some(node => node.id === id && node.controllable)) : []
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.loadFailed') })
  } finally { loading.value = false }
}
const handleVisibilityChange = () => { if (!document.hidden) void loadNodes() }

const openEnrollment = () => {
  if (!controllerMode.value.enabled) return push.error({ message: i18n.global.t('agent.controllerDisabledHint') })
  if (!serverMonitoringAvailable.value) return push.error({ message: serverRequirementText.value })
  Object.assign(enroll, { visible: true, apiLoading: false, pairURL: '', pairExpiresAt: 0, command: '', managedCommand: '' })
  void createEnrollmentAPI()
}
const closeEnrollment = () => { enroll.visible = false; if (enroll.pairURL) void loadNodes() }
const createEnrollmentAPI = async () => {
  enroll.apiLoading = true
  try {
    const result = await api('api/agents/enrollment-link', { method: 'POST', body: '{}' })
    enroll.pairURL = result.simple_address || result.connect_url || ''
    enroll.pairExpiresAt = Number(result.pair_expires_at || 0)
    enroll.command = result.command || ''
    enroll.managedCommand = result.managed_command || ''
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.createFailed') }) }
  finally { enroll.apiLoading = false }
}
const rotateNode = async (node: AgentNode) => {
  try {
    const result = await api(`api/agents/${node.id}/rotate`, { method: 'POST', body: '{}' })
    Object.assign(enroll, {
      visible: true,
      apiLoading: false,
      pairURL: result.simple_address || result.connect_url || result.pair_url || '',
      pairExpiresAt: Number(result.pair_expires_at || 0),
      command: result.command,
      managedCommand: result.managed_command || '',
    })
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.rotateFailed') }) }
}
const openLocalConnect = () => Object.assign(connect, { visible: true, loading: false, url: '', insecure: false })
const loadControllerMode = async () => {
  controllerModeLoading.value = true
  await dataStore.loadControllerMode(true)
  controllerModeLoading.value = false
}
const loadLocalConnection = async () => {
  try {
    const result = await api('api/agents/local-connection')
    Object.assign(localConnection, { supported: false, installed: false, configured: false, running: false, panel_url: '', public_url: '', insecure: false }, result || {})
  } catch { /* unsupported platforms remain read-only */ }
}
const enableControllerMode = async () => {
  controllerModeLoading.value = true
  try {
    const result = await api('api/controller-mode', { method: 'POST', body: JSON.stringify({ profile: 'full' }) })
    dataStore.assignControllerMode(result)
    push.success({ message: i18n.global.t('setting.roleUpdated') })
  } catch (error: any) {
    push.error({ message: error?.message || serverRequirementText.value })
  } finally { controllerModeLoading.value = false }
}
const connectToController = async () => {
  if (connect.loading) return
  if (localConnection.configured && !window.confirm(i18n.global.t('agent.reconnectConfirm'))) return
  connect.loading = true
  try {
    const result = await api('api/agents/connect-local', {
      method: 'POST',
      body: JSON.stringify({ connect_url: connect.url.trim(), public_url: resolveFrontendUrl(), insecure: connect.insecure }),
    })
    connect.visible = false
    push.success({ message: i18n.global.t('agent.connectSuccess', { panel: result?.panel_url || '-' }) })
    await loadLocalConnection()
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.connectFailed') })
  } finally { connect.loading = false }
}
const disconnectController = async () => {
  if (!window.confirm(i18n.global.t('agent.disconnectConfirm'))) return
  connect.loading = true
  try {
    const result = await api('api/agents/disconnect-local', { method: 'POST', body: '{}' })
    Object.assign(localConnection, result || { configured: false, running: false, panel_url: '' })
    connect.visible = false
    push.success({ message: i18n.global.t('agent.disconnectSuccess') })
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.connectFailed') })
  } finally { connect.loading = false }
}
const openEdit = (node: AgentNode) => Object.assign(edit, { visible: true, loading: false, id: node.id, name: node.name, publicHost: node.public_host || '' })
const saveNode = async () => {
  edit.loading = true
  try {
    await api(`api/agents/${edit.id}`, { method: 'PATCH', body: JSON.stringify({ name: edit.name.trim(), public_host: edit.publicHost.trim() }) })
    edit.visible = false
    push.success({ message: i18n.global.t('agent.updateSuccess') })
    await loadNodes()
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.updateFailed') }) }
  finally { edit.loading = false }
}
const askDelete = (node: AgentNode) => Object.assign(removeDialog, { visible: true, loading: false, node })
const deleteNode = async () => {
  if (!removeDialog.node) return
  removeDialog.loading = true
  try {
    await api(`api/agents/${removeDialog.node.id}/delete`, { method: 'POST', body: '{}' })
    removeDialog.visible = false
    await loadNodes()
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.deleteFailed') }) }
  finally { removeDialog.loading = false }
}

const openDetail = (node: AgentNode) => void router.push(`/agents/${node.id}`)
const manageInbounds = (node: AgentNode) => { if (canControl.value && node.managed) void router.push(`/agents/${node.id}/inbounds`) }
const toggleSelect = (id: number, enabled: boolean | null) => {
  if (enabled && !selected.value.includes(id)) selected.value.push(id)
  if (!enabled) selected.value = selected.value.filter(value => value !== id)
}
const selectAllControllable = () => { selected.value = nodes.value.filter(node => node.controllable).map(node => node.id) }
const batchCmd = async (type: string, args?: Record<string, any>) => {
  if (!canControl.value || !selected.value.length) return
  batch.loading = true
  try {
    batch.results = await api('api/agents/batch-command', { method: 'POST', body: JSON.stringify({ ids: selected.value, type, args: args || {} }) }) || []
    batch.resultVisible = true
    push.success({ message: `${batch.results.filter((item: any) => item.ok).length}/${batch.results.length} OK` })
    await loadNodes()
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.controlFailed') }) }
  finally { batch.loading = false }
}

const clampPercent = (value?: number) => Math.max(0, Math.min(100, Number(value) || 0))
const usagePercent = (value?: AgentUsage) => value?.total ? Number(value.used || 0) * 100 / value.total : undefined
const percent = (value?: number) => {
  if (value == null || Number.isNaN(value)) return '-'
  const digits = value > 0 && value < 1 ? 2 : 1
  return `${value.toFixed(digits)}%`
}
const rate = (value?: number) => {
  if (value == null) return '-'
  if (value < 1024) return `${Math.round(value)} B/s`
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)}K/s`
  return `${(value / (1024 ** 2)).toFixed(2)}M/s`
}
const bytes = (value?: number) => {
  if (!value) return '0 B'
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KiB`
  if (value < 1024 ** 3) return `${(value / (1024 ** 2)).toFixed(1)} MiB`
  if (value < 1024 ** 4) return `${(value / (1024 ** 3)).toFixed(1)} GiB`
  return `${(value / (1024 ** 4)).toFixed(2)} TiB`
}
const latencyLabel = (node: AgentNode) => node.online && node.latency?.last_ms != null ? `${node.latency.last_ms} ms` : '-'
const barStyle = (value?: number) => {
  const current = clampPercent(value)
  const color = current >= 90 ? 'rgb(var(--v-theme-error))' : current >= 70 ? 'rgb(var(--v-theme-warning))' : 'rgb(var(--v-theme-success))'
  return { width: `${current}%`, backgroundColor: color }
}
const copy = async (value: string) => {
  try { await copyText(value); push.success({ message: i18n.global.t('success') }) }
  catch { push.error({ message: i18n.global.t('failed') }) }
}

onMounted(() => {
  void Promise.all([loadControllerMode(), loadLocalConnection(), loadNodes()])
  document.addEventListener('visibilitychange', handleVisibilityChange)
  refreshTimer = window.setInterval(() => { if (!document.hidden) void loadNodes() }, 10000)
})
onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<style scoped>
.monitor-page { display: grid; gap: 18px; }
.monitor-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.monitor-heading h1 { margin: 0; font-size: 1.35rem; line-height: 1.4; letter-spacing: 0; }
.monitor-heading p { margin: 4px 0 0; color: rgba(var(--v-theme-on-surface), 0.62); font-size: 0.9rem; }
.heading-actions, .monitor-controls, .batch-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.summary-item { position: relative; min-height: 92px; padding: 16px 18px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04); display: flex; flex-direction: column; justify-content: center; gap: 4px; }
.summary-item span { color: rgba(var(--v-theme-on-surface), 0.7); font-size: 0.86rem; }
.summary-item strong { font-size: 1.45rem; line-height: 1.2; }
.summary-item small { color: rgba(var(--v-theme-on-surface), 0.62); }
.summary-dot, .status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.summary-dot { position: absolute; right: 18px; bottom: 20px; }
.dot-primary { background: rgb(var(--v-theme-primary)); }
.dot-online { background: rgb(var(--v-theme-success)); box-shadow: 0 0 0 3px rgba(var(--v-theme-success), 0.12); }
.dot-offline { background: rgb(var(--v-theme-error)); box-shadow: 0 0 0 3px rgba(var(--v-theme-error), 0.12); }
.summary-network strong { font-size: 1rem; }
.monitor-controls { justify-content: flex-end; }
.search-field { flex: 1 1 240px; max-width: 420px; margin-right: auto; }
.sort-field { flex: 0 1 190px; }
.batch-bar { padding: 10px 12px; border: 1px solid rgba(var(--v-theme-primary), 0.25); border-radius: 8px; background: rgba(var(--v-theme-primary), 0.06); }
.batch-shell { flex: 1 1 190px; max-width: 260px; }
.server-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.server-row { min-width: 0; min-height: 72px; display: grid; grid-template-columns: 34px minmax(120px, 0.85fr) minmax(330px, 2.2fr) 34px; align-items: center; gap: 8px; padding: 10px 8px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); box-shadow: 0 2px 7px rgba(0, 0, 0, 0.035); cursor: pointer; transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease; }
.server-row:hover { border-color: rgba(var(--v-theme-primary), 0.42); box-shadow: 0 5px 14px rgba(0, 0, 0, 0.07); transform: translateY(-1px); }
.server-row--offline { opacity: 0.68; }
.server-select { display: flex; justify-content: center; }
.server-identity { min-width: 0; display: flex; align-items: center; gap: 10px; }
.server-identity > div { min-width: 0; }
.server-identity strong, .server-identity small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.server-identity strong { font-size: 0.88rem; }
.server-identity small { margin-top: 2px; color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.72rem; }
.server-metrics { min-width: 0; display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 9px; }
.mini-metric { min-width: 0; position: relative; padding-bottom: 4px; }
.mini-metric span, .mini-metric strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mini-metric span { color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.68rem; }
.mini-metric strong { margin-top: 2px; font-size: 0.76rem; font-weight: 600; }
.mini-metric i { position: absolute; left: 0; bottom: 0; display: block; max-width: 100%; height: 2px; border-radius: 2px; }
.server-menu { display: flex; justify-content: center; }
@media (max-width: 1180px) {
  .server-grid { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 800px) {
  .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .monitor-heading { align-items: center; }
  .monitor-heading > div:first-child { text-align: center; flex: 1; }
  .monitor-heading { flex-wrap: wrap; justify-content: center; }
  .heading-actions { justify-content: center; width: 100%; }
}
@media (max-width: 600px) {
  .monitor-page { gap: 14px; }
  .summary-grid { gap: 8px; }
  .summary-item { min-height: 82px; padding: 13px; }
  .summary-item strong { font-size: 1.2rem; }
  .summary-network strong { font-size: 0.78rem; }
  .summary-network small { font-size: 0.68rem; }
  .monitor-controls { justify-content: center; }
  .search-field { flex-basis: 100%; max-width: none; margin-right: 0; }
  .sort-field { flex: 1 1 150px; }
  .server-row { grid-template-columns: 30px minmax(0, 1fr) 32px; grid-template-areas: 'select identity menu' '. metrics metrics'; gap: 9px 6px; }
  .server-select { grid-area: select; }
  .server-identity { grid-area: identity; }
  .server-metrics { grid-area: metrics; grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .server-menu { grid-area: menu; }
}
</style>
