<template>
  <section class="proxy-client">
    <header class="pc-header">
      <div>
        <h1>{{ $t('proxyClient.title') }}</h1>
        <p>{{ $t('proxyClient.hint') }}</p>
      </div>
      <div class="pc-actions">
        <v-select v-model="server" :items="deviceOptions" item-title="title" item-value="value" item-props="props"
          :label="$t('proxyClient.device')" density="compact" hide-details class="pc-device" @update:model-value="switchDevice" />
        <v-btn icon="mdi-refresh" variant="tonal" :loading="refreshing" :title="$t('actions.update')" @click="load()" />
      </div>
    </header>

    <v-alert v-if="errorMessage" type="error" variant="tonal" density="compact" closable @click:close="errorMessage = ''">{{ errorMessage }}</v-alert>
    <v-progress-linear v-if="loading" indeterminate />

    <template v-if="state">
      <div class="pc-status">
        <div class="pc-status__switch">
          <v-switch :model-value="state.settings.enabled" color="primary" hide-details inset :loading="busy === 'mode'"
            :disabled="!!busy" @update:model-value="setEnabled(!!$event)" />
          <div>
            <strong>{{ state.settings.enabled ? $t('proxyClient.on') : $t('proxyClient.off') }}</strong>
            <div class="pc-sub">
              <span class="pc-dot" :class="statusClass" />
              {{ statusText }}
            </div>
          </div>
        </div>
        <div class="pc-status__mode">
          <div class="pc-label">{{ $t('proxyClient.mode') }}</div>
          <v-btn-toggle :model-value="state.settings.mode" mandatory density="compact" color="primary" variant="outlined" divided
            :disabled="!!busy" @update:model-value="setMode">
            <v-btn value="rule">{{ $t('proxyClient.modes.rule') }}</v-btn>
            <v-btn value="gfw">{{ $t('proxyClient.modes.gfw') }}</v-btn>
            <v-btn value="global">{{ $t('proxyClient.modes.global') }}</v-btn>
          </v-btn-toggle>
        </div>
        <div class="pc-status__exit">
          <div class="pc-label">{{ $t('proxyClient.exit') }}</div>
          <div class="pc-exit">
            <span v-if="exit" :class="exit.ok ? '' : 'text-error'" dir="auto">
              {{ exit.ok ? `${flag(exit.country)} ${exit.ip} · ${exit.delay_ms} ms` : exit.error }}
            </span>
            <span v-else class="text-medium-emphasis">—</span>
            <v-btn size="small" variant="tonal" :loading="busy === 'exit'" :disabled="!state.applied || !!busy" @click="checkExit">{{ $t('proxyClient.checkExit') }}</v-btn>
          </div>
        </div>
      </div>
      <v-alert v-if="state.settings.enabled && state.problem" type="warning" variant="tonal" density="compact">{{ problemText(state.problem) }}</v-alert>

      <v-tabs v-model="tab" color="primary" density="compact">
        <v-tab value="nodes">{{ $t('proxyClient.tabs.nodes') }} ({{ state.nodes.length }})</v-tab>
        <v-tab value="subscriptions">{{ $t('proxyClient.tabs.subscriptions') }} ({{ state.subscriptions.length }})</v-tab>
        <v-tab value="settings">{{ $t('proxyClient.tabs.settings') }}</v-tab>
      </v-tabs>

      <v-window v-model="tab">
        <v-window-item value="nodes">
          <div class="pc-toolbar">
            <v-select v-model="nodeFilter" :items="filterOptions" item-title="title" item-value="value" density="compact" hide-details class="pc-filter" />
            <v-text-field v-model="search" :placeholder="$t('proxyClient.search')" prepend-inner-icon="mdi-magnify" density="compact" hide-details clearable class="pc-search" />
            <v-spacer />
            <v-btn variant="tonal" prepend-icon="mdi-auto-fix" :color="state.settings.auto ? 'primary' : undefined" :disabled="!!busy" @click="select(0)">
              {{ $t('proxyClient.auto') }}
            </v-btn>
            <v-btn variant="tonal" prepend-icon="mdi-timer-outline" :loading="busy === 'test'" :disabled="!!busy || !visibleNodes.length" @click="testNodes(visibleNodes.map(node => node.id))">
              {{ $t('proxyClient.testAll') }}
            </v-btn>
            <v-btn color="primary" prepend-icon="mdi-plus" :disabled="!!busy" @click="nodeDialog.visible = true">{{ $t('proxyClient.addNodes') }}</v-btn>
          </div>
          <v-alert v-if="!state.nodes.length" type="info" variant="tonal" density="compact">{{ $t('proxyClient.noNodes') }}</v-alert>
          <div class="pc-nodes">
            <div v-for="node in visibleNodes" :key="node.id" class="pc-node" :class="{ 'pc-node--active': activeId === node.id }">
              <v-icon :icon="activeId === node.id ? 'mdi-radiobox-marked' : 'mdi-radiobox-blank'" :color="activeId === node.id ? 'primary' : undefined" size="small" />
              <div class="pc-node__main">
                <div class="pc-node__name" dir="auto">{{ node.name }}</div>
                <div class="pc-node__meta">
                  <v-chip size="x-small" variant="tonal" label>{{ protocolName(node.protocol) }}</v-chip>
                  <span dir="ltr">{{ node.host }}:{{ node.port }}</span>
                  <span v-if="node.subscription">· {{ node.subscription }}</span>
                </div>
              </div>
              <div class="pc-node__delay" :title="node.test_error || ''">
                <span v-if="node.tcp_ms" class="text-medium-emphasis">TCP {{ node.tcp_ms }} ms</span>
                <strong v-if="node.delay_ms" :class="delayColor(node.delay_ms)">{{ node.delay_ms }} ms</strong>
                <strong v-else-if="node.tested_at" class="text-error">{{ /timed out|timeout/i.test(node.test_error) ? $t('proxyClient.timeout') : $t('proxyClient.failed') }}</strong>
              </div>
              <div class="pc-node__actions">
                <v-btn size="small" variant="text" icon="mdi-timer-outline" :title="$t('proxyClient.test')" :disabled="!!busy" @click="testNodes([node.id])" />
                <v-btn size="small" :variant="activeId === node.id ? 'flat' : 'tonal'" color="primary" :disabled="!!busy || (!state.settings.auto && state.settings.node_id === node.id)" @click="select(node.id)">
                  {{ $t('proxyClient.use') }}
                </v-btn>
                <v-btn v-if="!node.subscription_id" size="small" variant="text" icon="mdi-delete-outline" color="error" :title="$t('actions.del')" :disabled="!!busy" @click="deleteNode(node)" />
              </div>
            </div>
          </div>
        </v-window-item>

        <v-window-item value="subscriptions">
          <div class="pc-toolbar">
            <v-spacer />
            <v-btn variant="tonal" prepend-icon="mdi-cloud-sync-outline" :loading="busy === 'update'" :disabled="!!busy || !state.subscriptions.length" @click="updateSubscription(0)">
              {{ $t('proxyClient.updateAll') }}
            </v-btn>
            <v-btn color="primary" prepend-icon="mdi-plus" :disabled="!!busy" @click="openSubscription()">{{ $t('proxyClient.addSubscription') }}</v-btn>
          </div>
          <v-alert v-if="!state.subscriptions.length" type="info" variant="tonal" density="compact">{{ $t('proxyClient.noSubscriptions') }}</v-alert>
          <div v-for="item in state.subscriptions" :key="item.id" class="pc-sub-card">
            <div class="pc-sub-card__head">
              <strong dir="auto">{{ item.name }}</strong>
              <v-chip v-if="!item.enabled" size="x-small" variant="tonal" label>{{ $t('proxyClient.disabled') }}</v-chip>
              <v-spacer />
              <v-btn size="small" variant="text" icon="mdi-cloud-sync-outline" :title="$t('proxyClient.update')" :disabled="!!busy" @click="updateSubscription(item.id)" />
              <v-btn size="small" variant="text" icon="mdi-pencil-outline" :title="$t('actions.edit')" :disabled="!!busy" @click="openSubscription(item)" />
              <v-btn size="small" variant="text" icon="mdi-delete-outline" color="error" :title="$t('actions.del')" :disabled="!!busy" @click="deleteSubscription(item)" />
            </div>
            <div class="pc-sub-card__meta" dir="ltr">{{ maskUrl(item.url) }}</div>
            <div class="pc-sub-card__meta">
              <span>{{ $t('proxyClient.nodeCount', { n: item.node_count }) }}</span>
              <span v-if="item.total">· {{ $t('proxyClient.traffic') }} {{ bytes(item.upload + item.download) }} / {{ bytes(item.total) }}</span>
              <span v-if="item.expire">· {{ $t('proxyClient.expire') }} {{ date(item.expire) }}</span>
              <span v-if="item.updated_at">· {{ $t('proxyClient.updatedAt') }} {{ dateTime(item.updated_at) }}</span>
              <span>· {{ item.auto_update ? $t('proxyClient.everyHours', { n: item.auto_update }) : $t('proxyClient.manualUpdate') }}</span>
            </div>
            <div v-if="item.total" class="pc-quota"><span :style="{ width: Math.min(100, (item.upload + item.download) * 100 / item.total) + '%' }" /></div>
            <div v-if="item.last_error" class="text-error pc-sub-card__meta">{{ item.last_error }}</div>
          </div>
        </v-window-item>

        <v-window-item value="settings">
          <div v-if="form" class="pc-settings">
            <section>
              <h3>{{ $t('proxyClient.access') }}</h3>
              <v-switch v-model="form.tun" color="primary" hide-details :label="$t('proxyClient.tun')" />
              <div class="pc-help">{{ $t('proxyClient.tunHint') }}</div>
              <v-alert v-if="form.tun && tunProblem" type="warning" variant="tonal" density="compact" class="mt-2">{{ tunProblem }}</v-alert>
              <template v-if="state.platform.openwrt">
                <v-switch v-model="form.dns_hijack" color="primary" hide-details :disabled="!form.tun" :label="$t('proxyClient.dnsHijack')" />
                <div class="pc-help">{{ $t('proxyClient.dnsHijackHint') }}</div>
              </template>
              <v-switch v-model="form.mixed" color="primary" hide-details :label="$t('proxyClient.mixed')" class="mt-2" />
              <v-row v-if="form.mixed" dense class="mt-1">
                <v-col cols="12" sm="5">
                  <v-select v-model="form.mixed_listen" :items="listenOptions" item-title="title" item-value="value" :label="$t('proxyClient.mixedListen')" density="compact" />
                </v-col>
                <v-col cols="12" sm="3">
                  <v-text-field v-model.number="form.mixed_port" type="number" :label="$t('proxyClient.port')" density="compact" dir="ltr" />
                </v-col>
                <v-col cols="6" sm="2">
                  <v-text-field v-model="form.mixed_username" :label="$t('proxyMonitor.username')" density="compact" autocomplete="off" />
                </v-col>
                <v-col cols="6" sm="2">
                  <v-text-field v-model="form.mixed_password" type="password" :label="$t('proxyMonitor.password')" density="compact" autocomplete="new-password" />
                </v-col>
              </v-row>
            </section>

            <section>
              <h3>{{ $t('proxyClient.autoSection') }}</h3>
              <v-row dense>
                <v-col cols="12" sm="6">
                  <v-select v-model="form.auto_subscription" :items="autoSubscriptionOptions" item-title="title" item-value="value" :label="$t('proxyClient.autoFrom')" density="compact" />
                </v-col>
                <v-col cols="12" sm="6">
                  <v-text-field v-model="form.auto_filter" :label="$t('proxyClient.autoFilter')" :placeholder="$t('proxyClient.autoFilterHint')" persistent-placeholder density="compact" />
                </v-col>
                <v-col cols="6">
                  <v-select v-model="form.auto_interval" :items="[60, 180, 300, 600, 1800]" :item-title="(value: number) => $t('proxyMonitor.everyMinutes', { n: value / 60 })" :label="$t('proxyClient.autoInterval')" density="compact" />
                </v-col>
                <v-col cols="6">
                  <v-text-field v-model.number="form.auto_tolerance" type="number" suffix="ms" :label="$t('proxyClient.autoTolerance')" density="compact" />
                </v-col>
                <v-col cols="12">
                  <v-text-field v-model="form.test_url" :label="$t('proxyClient.testUrl')" density="compact" dir="ltr" />
                </v-col>
              </v-row>
            </section>

            <section>
              <h3>DNS</h3>
              <v-row dense>
                <v-col cols="12" sm="6">
                  <v-text-field v-model="form.direct_dns" :label="$t('proxyClient.directDns')" :hint="$t('proxyClient.directDnsHint')" persistent-hint density="compact" dir="ltr" />
                </v-col>
                <v-col cols="12" sm="6">
                  <v-text-field v-model="form.remote_dns" :label="$t('proxyClient.remoteDns')" :hint="$t('proxyClient.remoteDnsHint')" persistent-hint density="compact" dir="ltr" />
                </v-col>
                <v-col cols="6" sm="3">
                  <v-text-field v-model.number="form.dns_port" type="number" :label="$t('proxyClient.dnsPort')" density="compact" dir="ltr" />
                </v-col>
                <v-col cols="6" sm="9">
                  <v-switch v-model="form.ipv6" color="primary" hide-details :label="$t('proxyClient.ipv6')" density="compact" />
                </v-col>
              </v-row>
            </section>

            <section>
              <h3>{{ $t('proxyClient.routing') }}</h3>
              <v-switch v-model="form.block_quic" color="primary" hide-details :label="$t('proxyClient.blockQuic')" />
              <v-row dense class="mt-1">
                <v-col cols="12" sm="6">
                  <v-textarea v-model="lists.bypass_ips" :label="$t('proxyClient.bypassIps')" :hint="$t('proxyClient.ipsHint')" persistent-hint rows="3" auto-grow density="compact" dir="ltr" />
                </v-col>
                <v-col cols="12" sm="6">
                  <v-textarea v-model="lists.proxy_ips" :label="$t('proxyClient.proxyIps')" :hint="$t('proxyClient.proxyIpsHint')" persistent-hint rows="3" auto-grow density="compact" dir="ltr" />
                </v-col>
                <v-col cols="12" sm="6">
                  <v-textarea v-model="lists.direct_domains" :label="$t('proxyClient.directDomains')" :hint="$t('proxyClient.domainsHint')" persistent-hint rows="3" auto-grow density="compact" dir="ltr" />
                </v-col>
                <v-col cols="12" sm="6">
                  <v-textarea v-model="lists.proxy_domains" :label="$t('proxyClient.proxyDomains')" :hint="$t('proxyClient.domainsHint')" persistent-hint rows="3" auto-grow density="compact" dir="ltr" />
                </v-col>
              </v-row>
            </section>

            <section>
              <h3>{{ $t('proxyClient.ruleFiles') }}</h3>
              <div v-for="rule in state.rule_sets" :key="rule.tag" class="pc-rule">
                <span>{{ $t('proxyClient.rules.' + rule.tag.replace('client-', '')) }}</span>
                <small v-if="rule.ready" class="text-medium-emphasis">{{ bytes(rule.size) }} · {{ dateTime(rule.updated_at) }}</small>
                <small v-else class="text-warning">{{ $t('proxyClient.notDownloaded') }}</small>
              </div>
              <div class="pc-help">{{ $t('proxyClient.ruleFilesHint') }}</div>
              <v-btn variant="tonal" size="small" prepend-icon="mdi-download-outline" class="mt-2" :loading="busy === 'rules'" :disabled="!!busy" @click="updateRules">{{ $t('proxyClient.updateRules') }}</v-btn>
            </section>

            <div class="pc-save">
              <v-btn variant="text" :disabled="!!busy" @click="resetForm">{{ $t('proxyClient.revert') }}</v-btn>
              <v-btn color="primary" variant="tonal" :loading="busy === 'settings'" :disabled="!!busy" @click="saveSettings">{{ $t('actions.save') }}</v-btn>
            </div>
          </div>
        </v-window-item>
      </v-window>
    </template>

    <v-dialog v-model="nodeDialog.visible" max-width="620">
      <v-card>
        <v-card-title>{{ $t('proxyClient.addNodes') }}</v-card-title>
        <v-card-text>
          <v-textarea v-model="nodeDialog.links" :label="$t('proxyClient.links')" :hint="$t('proxyClient.linksHint')" persistent-hint rows="6" dir="ltr" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="nodeDialog.visible = false">{{ $t('actions.close') }}</v-btn>
          <v-btn color="primary" variant="tonal" :loading="busy === 'nodes'" :disabled="!nodeDialog.links.trim()" @click="addNodes">{{ $t('actions.add') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="subDialog.visible" max-width="620">
      <v-card>
        <v-card-title>{{ subDialog.form.id ? $t('proxyClient.editSubscription') : $t('proxyClient.addSubscription') }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="subDialog.form.name" :label="$t('proxyMonitor.name')" />
          <v-text-field v-model="subDialog.form.url" :label="$t('proxyClient.subscriptionUrl')" :hint="$t('proxyClient.subscriptionUrlHint')" persistent-hint dir="ltr" class="mb-3" />
          <v-row dense>
            <v-col cols="6">
              <v-select v-model="subDialog.form.auto_update" :items="autoUpdateOptions" item-title="title" item-value="value" :label="$t('proxyClient.autoUpdate')" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="subDialog.form.user_agent" :label="$t('proxyClient.userAgent')" placeholder="v2rayN/7.13.8" persistent-placeholder dir="ltr" />
            </v-col>
          </v-row>
          <v-switch v-model="subDialog.form.proxy_update" color="primary" hide-details :label="$t('proxyClient.proxyUpdate')" />
          <v-switch v-model="subDialog.form.enabled" color="primary" hide-details :label="$t('proxyClient.enabled')" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="subDialog.visible = false">{{ $t('actions.close') }}</v-btn>
          <v-btn color="primary" variant="tonal" :loading="busy === 'subscription'" :disabled="!subDialog.form.url.trim()" @click="saveSubscription">{{ $t('actions.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </section>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { i18n } from '@/locales'
import { fetchBackendObject } from '@/utils/backend'

type Settings = {
  enabled: boolean, mode: string, node_id: number, auto: boolean, auto_subscription: number, auto_filter: string
  auto_interval: number, auto_tolerance: number, test_url: string, tun: boolean, mixed: boolean, mixed_listen: string
  mixed_port: number, mixed_username: string, mixed_password: string, direct_dns: string, remote_dns: string
  dns_port: number, dns_hijack: boolean, ipv6: boolean, block_quic: boolean
  bypass_ips: string[], proxy_ips: string[], direct_domains: string[], proxy_domains: string[]
}
type Node = {
  id: number, subscription_id: number, subscription?: string, name: string, protocol: string, host: string, port: number
  tcp_ms: number, delay_ms: number, tested_at: number, test_error: string
}
type Subscription = {
  id: number, name: string, url: string, user_agent: string, auto_update: number, enabled: boolean, updated_at: number
  last_error: string, node_count: number, upload: number, download: number, total: number, expire: number, proxy_update: boolean
}
type State = {
  settings: Settings
  platform: { os: string, openwrt: boolean, root: boolean, tun_device: boolean, nftables: boolean, dnsmasq: boolean }
  running: boolean, applied: boolean, problem?: string, current?: string, current_id?: number
  subscriptions: Subscription[], nodes: Node[], rule_sets: { tag: string, ready: boolean, size: number, updated_at: number }[]
  dns_hijacked: boolean, testing: boolean
}
type Exit = { ok: boolean, delay_ms: number, ip?: string, country?: string, error?: string }

const t = (key: string, values?: Record<string, unknown>) => i18n.global.t(key, values ?? {})
const route = useRoute()

const server = ref(Number(route.query.server) || 0)
const devices = ref<{ id: number, name: string, online: boolean, supported: boolean }[]>([])
const state = ref<State | null>(null)
const form = ref<Settings | null>(null)
const lists = reactive({ bypass_ips: '', proxy_ips: '', direct_domains: '', proxy_domains: '' })
const loading = ref(true)
const refreshing = ref(false)
const busy = ref('')
const errorMessage = ref('')
const exit = ref<Exit | null>(null)
const tab = ref('nodes')
const nodeFilter = ref(-1)
const search = ref('')
const nodeDialog = reactive({ visible: false, links: '' })
const emptySubscription = () => ({ id: 0, name: '', url: '', user_agent: '', auto_update: 24, enabled: true, proxy_update: false })
const subDialog = reactive({ visible: false, form: emptySubscription() })
let timer: number | undefined

const endpoint = computed(() => server.value ? `api/proxy-client?server=${server.value}` : 'api/proxy-client')

const deviceOptions = computed(() => [
  { value: 0, title: t('proxyClient.thisDevice'), props: {} },
  ...devices.value.filter(device => device.id).map(device => ({
    value: device.id,
    title: device.name + (!device.online ? ` (${t('agent.offline')})` : !device.supported ? ` (${t('relaySpeedtest.needsUpdate')})` : ''),
    props: { disabled: !device.online || !device.supported },
  })),
])
const listenOptions = computed(() => [
  { value: 'lan', title: t('proxyClient.listen.lan') },
  { value: 'local', title: t('proxyClient.listen.local') },
  { value: 'all', title: t('proxyClient.listen.all') },
])
const autoUpdateOptions = computed(() => [
  { value: 0, title: t('proxyClient.manualUpdate') },
  ...[6, 12, 24, 48, 168].map(value => ({ value, title: t('proxyClient.everyHours', { n: value }) })),
])
const filterOptions = computed(() => [
  { value: -1, title: t('proxyClient.allNodes') },
  { value: 0, title: t('proxyClient.manualNodes') },
  ...(state.value?.subscriptions || []).map(item => ({ value: item.id, title: item.name })),
])
const autoSubscriptionOptions = computed(() => [
  { value: 0, title: t('proxyClient.allNodes') },
  ...(state.value?.subscriptions || []).map(item => ({ value: item.id, title: item.name })),
])
const visibleNodes = computed(() => {
  const keyword = (search.value || '').trim().toLowerCase()
  return (state.value?.nodes || []).filter(node =>
    (nodeFilter.value < 0 || node.subscription_id === nodeFilter.value) &&
    (!keyword || node.name.toLowerCase().includes(keyword) || node.host.toLowerCase().includes(keyword)))
})
const activeId = computed(() => {
  if (!state.value) return 0
  if (state.value.current_id) return state.value.current_id
  return state.value.settings.auto ? 0 : state.value.settings.node_id
})
const statusClass = computed(() => {
  if (!state.value?.settings.enabled) return 'pc-dot--off'
  return state.value.applied ? 'pc-dot--on' : 'pc-dot--warn'
})
const statusText = computed(() => {
  const value = state.value
  if (!value) return ''
  if (!value.settings.enabled) return t('proxyClient.stateOff')
  if (!value.applied) return t('proxyClient.stateNotRunning')
  const node = value.nodes.find(item => item.id === value.current_id)
  const name = node?.name || value.current || ''
  return value.settings.auto ? t('proxyClient.stateAuto', { name }) : t('proxyClient.stateNode', { name })
})
const tunProblem = computed(() => {
  const platform = state.value?.platform
  if (!platform) return ''
  if (platform.os !== 'linux') return t('proxyClient.problems.needLinux')
  if (!platform.root) return t('proxyClient.problems.needRoot')
  if (!platform.tun_device) return t('proxyClient.problems.needTun')
  if (!platform.nftables) return t('proxyClient.problems.noNftables')
  return ''
})

const problemText = (problem: string) => {
  const known: Record<string, string> = {
    'no node is selected': 'noNode',
    'no node matches the automatic selection': 'noAutoNode',
    'the transparent proxy needs Linux': 'needLinux',
    'the transparent proxy needs the panel to run as root': 'needRoot',
    'sing-box is not running': 'notRunning',
  }
  const key = known[problem] || (problem.startsWith('/dev/net/tun') ? 'needTun' : '')
  return key ? t('proxyClient.problems.' + key) : problem
}

const protocolNames: Record<string, string> = {
  vless: 'VLESS', vmess: 'VMess', trojan: 'Trojan', shadowsocks: 'SS', hysteria: 'Hysteria', hysteria2: 'Hysteria2',
  tuic: 'TUIC', anytls: 'AnyTLS', naive: 'Naive', socks: 'SOCKS5', http: 'HTTP', shadowtls: 'ShadowTLS',
}
const protocolName = (type: string) => protocolNames[type] || type.toUpperCase()
const delayColor = (delay: number) => delay < 300 ? 'text-success' : delay < 800 ? 'text-warning' : 'text-error'
const flag = (country?: string) => country && /^[A-Z]{2}$/.test(country)
  ? String.fromCodePoint(...[...country].map(c => 0x1F1E6 + c.charCodeAt(0) - 65))
  : ''
const bytes = (value: number) => {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let index = 0
  while (value >= 1024 && index < units.length - 1) { value /= 1024; index++ }
  return `${value.toFixed(index ? 1 : 0)} ${units[index]}`
}
const date = (seconds: number) => new Date(seconds * 1000).toLocaleDateString()
const dateTime = (seconds: number) => seconds ? new Date(seconds * 1000).toLocaleString() : ''
// Subscription addresses usually carry an account token; show the host only.
const maskUrl = (value: string) => {
  try {
    const parsed = new URL(value)
    return `${parsed.protocol}//${parsed.host}/…`
  } catch {
    return value
  }
}

const resetForm = () => {
  if (!state.value) return
  form.value = JSON.parse(JSON.stringify(state.value.settings))
  lists.bypass_ips = state.value.settings.bypass_ips.join('\n')
  lists.proxy_ips = state.value.settings.proxy_ips.join('\n')
  lists.direct_domains = state.value.settings.direct_domains.join('\n')
  lists.proxy_domains = state.value.settings.proxy_domains.join('\n')
}

const applyState = (next: State | null, resetSettings = false) => {
  if (!next) return
  const first = !state.value
  state.value = next
  if (first || resetSettings || !form.value) resetForm()
}

const load = async (resetSettings = false) => {
  if (refreshing.value) return
  refreshing.value = true
  try {
    applyState(await fetchBackendObject<State>(endpoint.value), resetSettings)
    errorMessage.value = ''
  } catch (error: any) {
    errorMessage.value = error?.message || t('failed')
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

const loadDevices = async () => {
  try {
    devices.value = await fetchBackendObject<any[]>('api/proxy-client/devices') || []
  } catch {
    devices.value = []
  }
}

const switchDevice = () => {
  state.value = null
  form.value = null
  exit.value = null
  loading.value = true
  void load(true)
}

// call runs one client action; a state comes back for most of them.
const call = async (name: string, action: string, data?: unknown, resetSettings = false) => {
  busy.value = name
  try {
    const result = await fetchBackendObject<any>(endpoint.value, { method: 'POST', body: JSON.stringify({ action, data }) })
    if (result && result.settings) applyState(result as State, resetSettings)
    errorMessage.value = ''
    return result
  } catch (error: any) {
    errorMessage.value = error?.message || t('failed')
    return null
  } finally {
    busy.value = ''
  }
}

const setEnabled = (enabled: boolean) => call('mode', 'mode', { enabled })
const setMode = (mode: string) => call('mode', 'mode', { mode })
const select = (nodeId: number) => call('select', 'select', { node_id: nodeId })
const testNodes = (ids: number[]) => call('test', 'test', { ids })
const updateSubscription = (id: number) => call('update', 'subscription.update', { id })
const updateRules = () => call('rules', 'rules.update')
const checkExit = async () => {
  const result = await call('exit', 'exit')
  if (result) exit.value = result as Exit
}

const addNodes = async () => {
  const result = await call('nodes', 'nodes.add', { links: nodeDialog.links })
  if (result) {
    nodeDialog.visible = false
    nodeDialog.links = ''
  }
}

const deleteNode = async (node: Node) => {
  if (!confirm(t('proxyClient.deleteNodeConfirm', { name: node.name }))) return
  await call('nodes', 'nodes.delete', { ids: [node.id] })
}

const openSubscription = (item?: Subscription) => {
  subDialog.form = item
    ? { id: item.id, name: item.name, url: item.url, user_agent: item.user_agent, auto_update: item.auto_update, enabled: item.enabled, proxy_update: item.proxy_update }
    : emptySubscription()
  subDialog.visible = true
}

const saveSubscription = async () => {
  const result = await call('subscription', 'subscription', subDialog.form)
  if (result) subDialog.visible = false
}

const deleteSubscription = async (item: Subscription) => {
  if (!confirm(t('proxyClient.deleteSubscriptionConfirm', { name: item.name }))) return
  await call('update', 'subscription.delete', { id: item.id })
}

const splitLines = (value: string) => value.split(/[\n,]/).map(item => item.trim()).filter(Boolean)

const saveSettings = async () => {
  if (!form.value || !state.value) return
  const settings: Settings = {
    ...form.value,
    // The status bar owns these; keep what the device has now.
    enabled: state.value.settings.enabled, mode: state.value.settings.mode,
    node_id: state.value.settings.node_id, auto: state.value.settings.auto,
    mixed_port: Number(form.value.mixed_port) || 0, dns_port: Number(form.value.dns_port) || 0,
    auto_tolerance: Number(form.value.auto_tolerance) || 0,
    bypass_ips: splitLines(lists.bypass_ips), proxy_ips: splitLines(lists.proxy_ips),
    direct_domains: splitLines(lists.direct_domains), proxy_domains: splitLines(lists.proxy_domains),
  }
  await call('settings', 'settings', settings, true)
}

onMounted(() => {
  void loadDevices()
  void load(true)
  timer = window.setInterval(() => { if (!document.hidden && !busy.value) void load() }, 10000)
})
onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })
</script>

<style scoped>
.proxy-client { display: grid; gap: 14px; }
.pc-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.pc-header h1 { margin: 0; font-size: 1.35rem; line-height: 1.3; }
.pc-header p { margin: 4px 0 0; color: rgba(var(--v-theme-on-surface), .62); max-width: 780px; }
.pc-actions { display: flex; align-items: center; gap: 10px; }
.pc-device { min-width: 240px; }
.pc-status { display: grid; grid-template-columns: minmax(200px, 1fr) auto minmax(220px, 1fr); gap: 16px; align-items: center; padding: 14px 16px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.pc-status__switch { display: flex; align-items: center; gap: 12px; min-width: 0; }
.pc-status__switch :deep(.v-switch) { flex: none; }
.pc-sub { display: flex; align-items: center; gap: 6px; font-size: .82rem; color: rgba(var(--v-theme-on-surface), .66); overflow-wrap: anywhere; }
.pc-label { font-size: .75rem; color: rgba(var(--v-theme-on-surface), .6); margin-bottom: 4px; }
.pc-exit { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; font-size: .88rem; }
.pc-dot { width: 9px; height: 9px; border-radius: 50%; flex: none; background: rgba(var(--v-theme-on-surface), .3); }
.pc-dot--on { background: rgb(var(--v-theme-success)); }
.pc-dot--warn { background: rgb(var(--v-theme-warning)); }
.pc-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin: 12px 0; }
.pc-filter { max-width: 220px; }
.pc-search { max-width: 260px; }
.pc-nodes { display: grid; gap: 6px; }
.pc-node { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; gap: 10px; align-items: center; padding: 8px 12px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.pc-node--active { border-color: rgba(var(--v-theme-primary), .6); }
.pc-node__name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pc-node__meta { display: flex; align-items: center; gap: 6px; font-size: .78rem; color: rgba(var(--v-theme-on-surface), .62); overflow: hidden; white-space: nowrap; }
.pc-node__delay { display: flex; flex-direction: column; align-items: flex-end; font-size: .8rem; font-variant-numeric: tabular-nums; }
.pc-node__actions { display: flex; align-items: center; gap: 4px; }
.pc-sub-card { padding: 10px 14px; margin-bottom: 8px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.pc-sub-card__head { display: flex; align-items: center; gap: 8px; }
.pc-sub-card__meta { font-size: .8rem; color: rgba(var(--v-theme-on-surface), .64); overflow-wrap: anywhere; }
.pc-quota { height: 4px; margin: 6px 0 2px; border-radius: 2px; background: rgba(var(--v-theme-on-surface), .08); overflow: hidden; }
.pc-quota span { display: block; height: 100%; background: rgb(var(--v-theme-primary)); }
.pc-settings { display: grid; gap: 14px; margin-top: 12px; }
.pc-settings section { padding: 12px 16px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); }
.pc-settings h3 { margin: 0 0 6px; font-size: .98rem; }
.pc-help { font-size: .8rem; color: rgba(var(--v-theme-on-surface), .62); margin: 0 0 4px 2px; }
.pc-rule { display: flex; justify-content: space-between; gap: 12px; padding: 4px 0; }
.pc-save { display: flex; justify-content: flex-end; gap: 10px; }
@media (max-width: 860px) {
  .pc-status { grid-template-columns: 1fr; }
}
@media (max-width: 600px) {
  .pc-header { flex-direction: column; align-items: stretch; }
  .pc-filter, .pc-search { flex: 1 1 100%; max-width: none; }
  .pc-device { min-width: 0; flex: 1; }
  .pc-node { grid-template-columns: auto minmax(0, 1fr); }
  .pc-node__delay { grid-column: 2; flex-direction: row; gap: 8px; justify-content: flex-start; }
  .pc-node__actions { grid-column: 2; }
}
</style>
