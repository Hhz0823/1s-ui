<template>
  <v-dialog :model-value="visible" transition="dialog-bottom-transition" width="min(980px, calc(100vw - 20px))" scrollable @update:model-value="close">
    <v-card class="relay-dialog">
      <v-card-title class="d-flex align-center">
        <span>{{ $t('pages.relay') }}</span>
        <v-spacer />
        <v-btn icon="mdi-close" variant="text" size="small" @click="close" />
      </v-card-title>
      <v-divider />
      <v-card-text class="relay-dialog-body">
        <v-tabs v-model="tab" align-tabs="center" grow class="mb-3">
          <v-tab value="ipv6">{{ $t('relay.ipv6Mode') }}</v-tab>
          <v-tab value="upstream">{{ $t('relay.upstreamMode') }}</v-tab>
          <v-tab value="pools">{{ $t('relay.pools') }}</v-tab>
        </v-tabs>

        <v-window v-model="tab">
          <v-window-item value="ipv6">
            <section class="relay-quick-section">
              <div class="relay-quick-heading">
                <div>
                  <div class="text-subtitle-1 font-weight-medium">{{ $t('relay.autoAddTitle') }}</div>
                  <a class="relay-source-link" href="https://github.com/help660vip/auto-add-ipv6" target="_blank" rel="noopener noreferrer">help660vip/auto-add-ipv6</a>
                </div>
              </div>
              <v-row class="relay-quick-fields mt-2">
                <v-col cols="12" sm="6" md="2">
                  <v-select v-model="form.interface" :items="interfaceItems" item-title="title" item-value="value" :label="$t('relay.interface')" clearable hide-details />
                </v-col>
                <v-col cols="6" sm="3" md="1">
                  <v-text-field v-model.number="form.count" type="number" min="1" max="100" :label="$t('relay.count')" :error-messages="relayCountValid ? [] : [$t('relay.countRange')]" hide-details="auto" />
                </v-col>
                <v-col cols="6" sm="3" md="2">
                  <v-text-field v-model.number="form.port_start" type="number" min="1" max="65535" :label="$t('relay.portStart')" hide-details />
                </v-col>
                <v-col cols="12" sm="6" md="2">
                  <v-text-field v-model="form.username_prefix" :label="$t('relay.usernamePrefix')" dir="ltr" hide-details />
                </v-col>
                <v-col cols="12" sm="6" md="3">
                  <v-select v-model="form.domain_strategy" :items="domainStrategyItems" item-title="title" item-value="value" :label="$t('relay.addressMode')" hide-details />
                </v-col>
                <v-col cols="12" sm="6" md="2" class="relay-quick-button-col">
                  <v-btn color="primary" block prepend-icon="mdi-access-point-plus" :loading="loading" :disabled="!canQuickCreateIPv6" @click="create('ipv6', true)">{{ $t('relay.autoAddCreate') }}</v-btn>
                </v-col>
              </v-row>
              <v-alert v-if="capabilityMessage" type="warning" variant="tonal" density="compact" class="mt-3">
                {{ capabilityMessage }}
              </v-alert>
            </section>

            <v-expansion-panels v-model="advancedPanel" variant="accordion" class="relay-advanced mt-3">
              <v-expansion-panel value="advanced">
                <v-expansion-panel-title>{{ $t('relay.advancedOptions') }}</v-expansion-panel-title>
                <v-expansion-panel-text>
                  <v-row density="compact">
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="form.name" :label="$t('relay.name')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="form.public_host" :label="$t('relay.publicHost')" dir="ltr" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-select v-model="form.protocol" :items="protocolItems" item-title="title" item-value="value" :label="$t('relay.protocol')" hide-details />
              </v-col>
              <v-col v-if="transportItems.length > 0" cols="12" sm="6" md="4">
                <v-select v-model="form.transport" :items="transportItems" item-title="title" item-value="value" :label="$t('relay.transport')" hide-details />
              </v-col>
              <v-col v-if="requiresTls" cols="12" sm="6" md="4">
                <v-select v-model.number="form.tls_id" :items="tlsItems" item-title="title" item-value="value" :label="$t('relay.tls')" hide-details />
              </v-col>
              <v-col v-else-if="supportsOptionalTls" cols="12" sm="6" md="4">
                <v-select v-model.number="form.tls_id" :items="optionalTlsItems" item-title="title" item-value="value" :label="$t('relay.tls')" hide-details />
              </v-col>
              <v-col v-if="form.protocol === 'shadowsocks'" cols="12" sm="6" md="4">
                <v-select v-model="form.shadowsocks_method" :items="shadowsocksMethods" :label="$t('relay.encryption')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model.number="form.password_length" type="number" min="8" max="64" :label="$t('relay.passwordLength')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="form.base_ipv6" :label="$t('relay.baseIPv6')" placeholder="2001:db8::1/64" dir="ltr" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model.number="form.prefix" type="number" min="1" max="128" :label="$t('relay.prefix')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-select v-model="form.domain_strategy" :items="domainStrategyItems" item-title="title" item-value="value" :label="$t('relay.addressMode')" hide-details />
              </v-col>
              <v-col cols="12">
                <v-textarea v-model="form.ipv6_text" :label="$t('relay.ipv6List')" :hint="$t('relay.ipv6ListHint')" persistent-hint rows="3" dir="ltr" hide-details="auto" />
              </v-col>
              <v-col cols="12">
                <v-switch v-model="form.add_system_addresses" color="primary" :label="$t('relay.addSystemAddresses')" hide-details />
              </v-col>
                  </v-row>
                </v-expansion-panel-text>
              </v-expansion-panel>
            </v-expansion-panels>
            <v-alert v-if="ipv6.length === 0" type="warning" variant="tonal" class="mt-3">
              {{ $t('relay.noIPv6') }}
            </v-alert>
            <v-alert type="info" variant="tonal" density="compact" class="mt-3">
              {{ $t('relay.generatedAddresses') }}
            </v-alert>
            <v-list v-if="ipv6.length > 0" density="compact" class="relay-list mt-3">
              <v-list-subheader>{{ $t('relay.detectedIPv6') }}</v-list-subheader>
              <v-list-item v-for="item in ipv6" :key="item.interface + item.address">
                <v-list-item-title dir="ltr">{{ item.address }}/{{ item.prefix }}</v-list-item-title>
                <v-list-item-subtitle>{{ item.interface }}</v-list-item-subtitle>
              </v-list-item>
            </v-list>
            <div class="relay-actions">
              <v-btn color="primary" prepend-icon="mdi-playlist-plus" :loading="loading" :disabled="!canCreateIPv6" @click="create('ipv6')">{{ $t('relay.createAdvanced') }}</v-btn>
              <v-btn variant="text" prepend-icon="mdi-refresh" :loading="refreshing" @click="loadData">{{ $t('actions.update') }}</v-btn>
            </div>
          </v-window-item>

          <v-window-item value="upstream">
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="form.name" :label="$t('relay.name')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="form.public_host" :label="$t('relay.publicHost')" dir="ltr" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model.number="form.port_start" type="number" min="1" max="65535" :label="$t('relay.portStart')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-select v-model="form.protocol" :items="protocolItems" item-title="title" item-value="value" :label="$t('relay.protocol')" hide-details />
              </v-col>
              <v-col v-if="transportItems.length > 0" cols="12" sm="6" md="4">
                <v-select v-model="form.transport" :items="transportItems" item-title="title" item-value="value" :label="$t('relay.transport')" hide-details />
              </v-col>
              <v-col v-if="requiresTls" cols="12" sm="6" md="4">
                <v-select v-model.number="form.tls_id" :items="tlsItems" item-title="title" item-value="value" :label="$t('relay.tls')" hide-details />
              </v-col>
              <v-col v-else-if="supportsOptionalTls" cols="12" sm="6" md="4">
                <v-select v-model.number="form.tls_id" :items="optionalTlsItems" item-title="title" item-value="value" :label="$t('relay.tls')" hide-details />
              </v-col>
              <v-col v-if="form.protocol === 'shadowsocks'" cols="12" sm="6" md="4">
                <v-select v-model="form.shadowsocks_method" :items="shadowsocksMethods" :label="$t('relay.encryption')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="form.username_prefix" :label="$t('relay.usernamePrefix')" dir="ltr" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model.number="form.password_length" type="number" min="8" max="64" :label="$t('relay.passwordLength')" hide-details />
              </v-col>
              <v-col cols="12">
                <v-textarea v-model="form.upstream_text" :label="$t('relay.upstreamList')" :hint="$t('relay.upstreamListHint')" persistent-hint rows="9" dir="ltr" hide-details="auto" />
              </v-col>
            </v-row>
            <div class="relay-actions">
              <v-btn color="primary" prepend-icon="mdi-playlist-plus" :loading="loading" @click="create('upstream')">{{ $t('relay.create') }}</v-btn>
            </div>
          </v-window-item>

          <v-window-item value="pools">
            <v-alert v-if="pools.length === 0" type="info" variant="tonal">{{ $t('relay.noPools') }}</v-alert>
            <v-row v-else>
              <v-col v-for="pool in pools" :key="pool.id" cols="12" md="6">
                <v-card variant="outlined" class="relay-pool-card">
                  <v-card-title class="d-flex align-center">
                    <span class="text-truncate">{{ pool.name }}</span>
                    <v-spacer />
                    <v-chip v-if="pool.source" size="small" color="primary" variant="tonal">auto-add-ipv6</v-chip>
                    <v-chip size="small" :color="pool.mode === 'ipv6' ? 'info' : 'secondary'" variant="tonal">{{ pool.mode }}</v-chip>
                  </v-card-title>
                  <v-card-text>
                    <div class="relay-pool-meta" dir="ltr">{{ poolAddressSummary(pool) }} / {{ pool.count }} {{ $t('relay.items') }}</div>
                    <v-textarea :model-value="exportText(pool)" rows="4" readonly dir="ltr" hide-details />
                  </v-card-text>
                  <v-card-actions class="relay-pool-actions">
                    <v-btn variant="tonal" prepend-icon="mdi-content-copy" @click="copy(exportText(pool))">{{ $t('relay.copy') }}</v-btn>
                    <v-btn
                      v-if="supportsBitBrowser(pool)"
                      variant="tonal"
                      color="primary"
                      prepend-icon="mdi-microsoft-excel"
                      :loading="downloading === pool.id"
                      @click="downloadBitBrowser(pool)"
                    >{{ $t('relay.exportBitBrowser') }}</v-btn>
                    <v-btn
                      class="relay-delete-button"
                      color="error"
                      variant="text"
                      icon="mdi-delete-outline"
                      :aria-label="$t('actions.del')"
                      :title="$t('actions.del')"
                      :loading="deleting === pool.id"
                      @click="remove(pool.id)"
                    />
                  </v-card-actions>
                </v-card>
              </v-col>
            </v-row>
            <div class="relay-actions">
              <v-btn variant="text" prepend-icon="mdi-refresh" :loading="refreshing" @click="loadData">{{ $t('actions.update') }}</v-btn>
            </div>
          </v-window-item>
        </v-window>
      </v-card-text>
      <v-divider />
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="close">{{ $t('actions.close') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, reactive, ref, watch } from 'vue'
import { push } from 'notivue'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'
import { i18n } from '@/locales'
import { copyText } from '@/utils/clipboard'
import { backendBaseUrl, backendFetch, downloadBackendFile } from '@/utils/backend'

interface IPv6Item { interface: string; address: string; prefix: number }
interface RelayItem { listen_port: number; username: string; password: string; ipv6?: string; protocol?: string; export?: string }
interface RelayPool { id: number; name: string; source?: string; mode: string; protocol?: string; domain_strategy?: string; listen_host: string; port_start: number; count: number; items: RelayItem[] }
interface RelayCapabilities { os: string; can_add_system_ipv6: boolean; unavailable_reason?: string }

const props = defineProps<{
  visible: boolean
  agentId?: number
  connectionHost?: string
  tlsConfigs?: any[]
}>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'changed'): void
}>()
const tab = ref('ipv6')
const advancedPanel = ref<string>()
const loading = ref(false)
const refreshing = ref(false)
const deleting = ref(0)
const downloading = ref(0)
const ipv6 = ref<IPv6Item[]>([])
const pools = ref<RelayPool[]>([])
const capabilities = ref<RelayCapabilities | null>(null)
const isRemote = computed(() => Number.isInteger(props.agentId) && Number(props.agentId) > 0)
const form = reactive({
  name: '', public_host: new URL(backendBaseUrl()).hostname, port_start: 30000, count: 10,
  username_prefix: 'relay', password_length: 12, interface: '', base_ipv6: '', prefix: 64,
  ipv6_text: '', upstream_text: '', add_system_addresses: true, protocol: 'socks',
  transport: 'http', tls_id: 0, domain_strategy: 'ipv6_only', shadowsocks_method: '2022-blake3-aes-256-gcm',
})

const interfaceItems = computed(() => [...new Set(ipv6.value.map((item) => item.interface))].map((value) => ({ title: value, value })))
const relayCountValid = computed(() => Number.isInteger(form.count) && form.count >= 1 && form.count <= 100)
const canCreateIPv6 = computed(() => relayCountValid.value && (ipv6.value.length > 0 || form.base_ipv6.trim().length > 0))
const canQuickCreateIPv6 = computed(() => canCreateIPv6.value && capabilities.value?.can_add_system_ipv6 === true)
const capabilityMessage = computed(() => {
  const capability = capabilities.value
  if (!capability || capability.can_add_system_ipv6) return ''
  const key = ['unsupported_os', 'root_required', 'iproute2_required'].includes(capability.unavailable_reason || '')
    ? capability.unavailable_reason
    : 'unavailable'
  return i18n.global.t(`relay.capability.${key}`, { os: capability.os || i18n.global.t('relay.capability.unknownOS') })
})
const protocolItems = [
  { title: 'SOCKS5', value: 'socks' }, { title: 'HTTP', value: 'http' }, { title: 'Mixed', value: 'mixed' },
  { title: 'Shadowsocks', value: 'shadowsocks' }, { title: 'VLESS', value: 'vless' }, { title: 'VMess', value: 'vmess' },
  { title: 'Trojan', value: 'trojan' }, { title: 'Hysteria2', value: 'hysteria2' }, { title: 'TUIC', value: 'tuic' },
  { title: 'Naive', value: 'naive' }, { title: 'AnyTLS', value: 'anytls' },
]
const transportItems = computed(() => ['vless', 'vmess', 'trojan'].includes(form.protocol)
  ? [{ title: 'HTTP', value: 'http' }, { title: 'WebSocket', value: 'ws' }, { title: 'gRPC', value: 'grpc' }, { title: 'HTTPUpgrade', value: 'httpupgrade' }]
  : [])
const requiresTls = computed(() => ['trojan', 'hysteria2', 'tuic', 'naive', 'anytls'].includes(form.protocol))
const supportsOptionalTls = computed(() => ['vless', 'vmess'].includes(form.protocol))
const tlsItems = computed(() => ((isRemote.value ? props.tlsConfigs : Data().tlsConfigs) ?? []).map((tls: any) => ({ title: tls.name, value: tls.id })))
const optionalTlsItems = computed(() => [{ title: i18n.global.t('disable'), value: 0 }, ...tlsItems.value])
const domainStrategyItems = computed(() => [
  { title: i18n.global.t('relay.ipv6Only'), value: 'ipv6_only' },
  { title: i18n.global.t('relay.preferIPv6'), value: 'prefer_ipv6' },
])
const shadowsocksMethods = [
  'aes-128-gcm', 'aes-192-gcm', 'aes-256-gcm', 'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305',
]

watch(() => form.protocol, (protocol) => {
  if (!['vless', 'vmess', 'trojan'].includes(protocol)) form.transport = ''
  else if (!form.transport) form.transport = 'http'
  if (!requiresTls.value && !supportsOptionalTls.value) form.tls_id = 0
})

const loadData = async () => {
  refreshing.value = true
  try {
    let data: any
    if (isRemote.value) {
      const response = await backendFetch(`api/agents/${props.agentId}/relay`)
      const msg = await response.json()
      if (!response.ok || !msg.success) throw new Error(msg.msg || response.statusText)
      data = msg.obj
    } else {
      const msg = await HttpUtils.get('api/relay')
      if (!msg.success) throw new Error(msg.msg || i18n.global.t('failed'))
      data = msg.obj
    }
    ipv6.value = data?.ipv6 ?? []
    pools.value = (data?.pools ?? []).map((pool: any) => ({ ...pool, items: parseItems(pool.items) }))
    capabilities.value = data?.capabilities ?? null
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.loadFailed') })
  } finally {
    refreshing.value = false
  }
}

const parseItems = (items: any): RelayItem[] => {
  if (Array.isArray(items)) return items
  try { return JSON.parse(items || '[]') } catch { return [] }
}

const poolAddressSummary = (pool: RelayPool) => {
  if (pool.mode !== 'ipv6') return pool.listen_host
  const addresses = [...new Set(pool.items.map((item) => item.ipv6).filter(Boolean))]
  const mode = pool.domain_strategy === 'prefer_ipv6'
    ? i18n.global.t('relay.preferIPv6')
    : i18n.global.t('relay.ipv6Only')
  if (addresses.length === 0) return `${pool.listen_host} / ${mode}`
  if (addresses.length <= 2) return `${pool.listen_host} -> ${addresses.join(', ')} / ${mode}`
  return `${pool.listen_host} -> ${addresses[0]}, ${addresses[1]} ... (+${addresses.length - 2} IPv6) / ${mode}`
}

const create = async (mode: 'ipv6' | 'upstream', quick = false) => {
  if (mode === 'ipv6' && !relayCountValid.value) {
    push.error({ message: i18n.global.t('relay.countRange') })
    return
  }
  loading.value = true
  try {
    const payload: any = { ...form, mode }
    payload.source = quick ? 'help660vip/auto-add-ipv6' : ''
    if (quick) {
      payload.protocol = 'socks'
      payload.add_system_addresses = true
      payload.tls_id = 0
      payload.transport = ''
    }
    payload.ipv6_addresses = form.ipv6_text.split(/\r?\n/).map((v) => v.trim()).filter(Boolean)
    // Keep the raw text as the source of truth so bracketed IPv6 SOCKS5 entries
    // are parsed by the backend without losing their colon-separated address.
    payload.upstreams = []
    payload.upstream_text = form.upstream_text
    delete payload.ipv6_text
    const endpoint = isRemote.value ? `api/agents/${props.agentId}/relay/create` : 'api/relay/create'
    const response = await backendFetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    let msg: any
    try { msg = await response.json() } catch { msg = { success: false, msg: i18n.global.t('relay.invalidResponse') } }
    if (msg.success) {
      const allocatedStart = Number(msg.obj?.port_start)
      const allocatedCount = Number(msg.obj?.count)
      if (Number.isInteger(allocatedStart) && Number.isInteger(allocatedCount) && allocatedStart > 0 && allocatedCount > 0) {
        const allocatedEnd = allocatedStart + allocatedCount - 1
        if (allocatedEnd < 65535) form.port_start = allocatedEnd + 1
        push.success({ message: i18n.global.t('relay.createdRange', { start: allocatedStart, end: allocatedEnd }) })
      } else {
        push.success({ message: i18n.global.t('relay.created') })
      }
      tab.value = 'pools'
      form.name = ''
      form.ipv6_text = ''
      form.upstream_text = ''
      await loadData()
      if (isRemote.value) emit('changed')
      else await Data().loadData()
    } else {
      const egressMatch = String(msg.msg || '').match(/relay_ipv6_egress_unreachable\|([^|]+)\|/)
      push.error({
        message: egressMatch
          ? i18n.global.t('relay.ipv6EgressUnavailable', { address: egressMatch[1] })
          : msg.msg || i18n.global.t('relay.createFailed'),
      })
    }
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('relay.createFailed') })
  } finally {
    loading.value = false
  }
}

const exportText = (pool: RelayPool) => pool.items.map((item) => {
  if (item.protocol && !['socks', 'mixed'].includes(item.protocol) && item.export) return item.export
  const host = pool.listen_host.replace(/^\[|\]$/g, '')
  return `${host}:${item.listen_port}:${item.username}:${item.password}`
}).join('\n')

const supportsBitBrowser = (pool: RelayPool) => {
  const protocol = pool.protocol || pool.items.find((item) => item.protocol)?.protocol || 'socks'
  return ['socks', 'mixed'].includes(protocol)
}

const downloadBitBrowser = async (pool: RelayPool) => {
  downloading.value = pool.id
  try {
    const endpoint = isRemote.value
      ? `api/agents/${props.agentId}/relay/${pool.id}/bitbrowser.xlsx`
      : `api/relay/${pool.id}/bitbrowser.xlsx`
    const safeName = (pool.name || `relay-${pool.id}`).replace(/[^a-zA-Z0-9._-]+/g, '-').replace(/^-+|-+$/g, '') || `relay-${pool.id}`
    await downloadBackendFile(endpoint, `1s-ui-bitbrowser-${safeName}.xlsx`)
    push.success({ message: i18n.global.t('relay.exportBitBrowserReady') })
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('relay.exportBitBrowserFailed') })
  } finally {
    downloading.value = 0
  }
}

const copy = async (value: string) => {
  try {
    await copyText(value)
    push.success({ message: i18n.global.t('relay.copied') })
  } catch {
    push.error({ message: i18n.global.t('relay.copyFailed') })
  }
}

const remove = async (id: number) => {
  if (!window.confirm(i18n.global.t('relay.deleteConfirm'))) return
  deleting.value = id
  try {
    const endpoint = isRemote.value
      ? `api/agents/${props.agentId}/relay/${id}/delete`
      : `api/relay/${id}/delete`
    const response = await backendFetch(endpoint, { method: 'POST' })
    let msg: any
    try { msg = await response.json() } catch { msg = { success: false, msg: i18n.global.t('relay.invalidResponse') } }
    if (msg.success) {
      push.success({ message: i18n.global.t('relay.deleted') })
      await loadData()
      if (isRemote.value) emit('changed')
      else await Data().loadData()
    } else {
      push.error({ message: msg.msg || i18n.global.t('relay.deleteFailed') })
    }
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('relay.deleteFailed') })
  } finally {
    deleting.value = 0
  }
}

const close = () => emit('close')
watch(() => props.visible, (visible) => {
  if (!visible) return
  if (isRemote.value && props.connectionHost) form.public_host = props.connectionHost
  loadData()
})
watch(() => props.connectionHost, (host) => {
  if (isRemote.value && host) form.public_host = host
})
</script>

<style scoped>
.relay-dialog { max-height: calc(100vh - 20px); display: flex; flex-direction: column; overflow: hidden !important; background: rgba(var(--v-theme-surface), 0.985) !important; }
.relay-dialog > :deep(.v-card-title), .relay-dialog > :deep(.v-card-actions) { position: relative; z-index: 2; background: rgba(var(--v-theme-surface), 0.99) !important; }
.relay-dialog-body { min-height: 0; background: rgba(var(--v-theme-surface), 0.34); overflow-y: auto !important; }
.relay-list { border: 1px solid rgba(var(--v-theme-on-surface), 0.08); border-radius: 10px; }
.relay-quick-section { padding: 14px; border: 1px solid rgba(var(--v-theme-primary), 0.2); border-radius: 8px; background: rgba(var(--v-theme-primary), 0.045); }
.relay-pool-actions { display: grid !important; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 40px; gap: 8px; }
.relay-pool-actions :deep(.v-btn) { margin: 0 !important; }
.relay-pool-actions :deep(.v-btn:not(.relay-delete-button)) { min-width: 0; }
.relay-delete-button { width: 40px; height: 40px; align-self: center; }
@media (max-width: 520px) {
  .relay-pool-actions { grid-template-columns: minmax(0, 1fr) 40px; }
  .relay-pool-actions :deep(.v-btn:not(.relay-delete-button)) { grid-column: 1; }
  .relay-delete-button { grid-column: 2; grid-row: 1 / span 2; }
}
.relay-quick-heading { display: flex; justify-content: center; text-align: center; }
.relay-source-link { color: rgb(var(--v-theme-primary)); font-size: 12px; text-decoration: none; }
.relay-source-link:hover { text-decoration: underline; }
.relay-quick-fields { display: grid; grid-template-columns: minmax(110px, 1fr) minmax(74px, .55fr) minmax(100px, .8fr) minmax(110px, 1fr) minmax(180px, 1.45fr) minmax(120px, 1fr); gap: 12px; margin-inline: 0; }
.relay-quick-fields > :deep(.v-col) { width: auto; max-width: none; flex: none; padding: 0; }
.relay-quick-button-col { display: flex; align-items: center; }
.relay-advanced :deep(.v-expansion-panel) { border-radius: 8px !important; box-shadow: none !important; border: 1px solid rgba(var(--v-theme-on-surface), 0.1); }
.relay-pool-card :deep(.v-card-title) { gap: 8px; }
.relay-actions { display: flex; justify-content: center; align-items: center; gap: 10px; margin-top: 18px; flex-wrap: wrap; }
.relay-pool-meta { opacity: 0.7; margin-bottom: 10px; overflow-wrap: anywhere; }
.relay-pool-card { height: 100%; }

@media (max-width: 959px) {
  .relay-quick-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 600px) {
  .relay-quick-fields { grid-template-columns: minmax(0, 1fr); }
  .relay-dialog { max-height: calc(100vh - 20px); }
  .relay-dialog > :deep(.v-card-title) { justify-content: center; padding-inline: 48px !important; }
  .relay-dialog > :deep(.v-card-title .v-btn) { position: absolute; inset-inline-end: 8px; }
  .relay-dialog-body { padding: 12px !important; }
  .relay-dialog-body :deep(.v-tabs) { overflow-x: hidden; }
  .relay-dialog-body :deep(.v-tab) { min-width: 0; padding-inline: 6px; font-size: 13px; white-space: nowrap; }
  .relay-dialog :deep(.v-card-actions) { flex-wrap: wrap; justify-content: center !important; }
  .relay-dialog :deep(.v-card-actions .v-btn) { flex: 1 1 140px; }
}
</style>
