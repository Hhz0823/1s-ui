<template>
  <v-dialog v-model="xrayCheck.visible" width="min(720px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('xray.selfCheck') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-alert
          v-if="xrayCheck.result"
          :type="xrayCheck.result.healthy ? 'success' : 'error'"
          variant="tonal"
          class="mb-4"
        >{{ xrayCheck.result.healthy ? $t('xray.checkPassed') : (xrayCheck.result.error || $t('xray.checkFailed')) }}</v-alert>
        <v-list v-if="xrayCheck.result" density="compact" bg-color="transparent">
          <v-list-item :title="$t('xray.binary')" :subtitle="xrayCheck.result.version || xrayCheck.result.path || '-'">
            <template #append><v-icon :color="xrayCheck.result.binary_available ? 'success' : 'error'" :icon="xrayCheck.result.binary_available ? 'mdi-check-circle' : 'mdi-close-circle'" /></template>
          </v-list-item>
          <v-list-item :title="$t('xray.configValidation')">
            <template #append><v-icon :color="xrayCheck.result.config_valid ? 'success' : 'error'" :icon="xrayCheck.result.config_valid ? 'mdi-check-circle' : 'mdi-close-circle'" /></template>
          </v-list-item>
          <v-list-item :title="$t('xray.runtime')">
            <template #append><v-chip size="small" :color="xrayCheck.result.running ? 'success' : 'default'">{{ xrayCheck.result.running ? $t('online') : $t('disable') }}</v-chip></template>
          </v-list-item>
        </v-list>
        <div v-if="xrayCheck.result" class="xray-capability-groups">
          <div v-if="xrayCheck.result.checks?.length" class="mb-3">
            <div class="xray-capability-title">{{ $t('xray.checks') }}</div>
            <v-list density="compact" bg-color="transparent">
              <v-list-item v-for="(line, idx) in xrayCheck.result.checks" :key="idx" :title="line" />
            </v-list>
          </div>
          <div>
            <div class="xray-capability-title">{{ $t('xray.protocols') }}</div>
            <div class="xray-capabilities">
              <v-chip
                v-for="item in xrayCheck.result.protocols.filter((item: any) => item.supported)"
                :key="item.id"
                size="small"
                variant="tonal"
                color="primary"
                :title="item.reason || ''"
              >{{ item.name }}</v-chip>
            </div>
            <div class="xray-capability-title mt-2">{{ $t('xray.unsupportedProtocols') }}</div>
            <div class="xray-capabilities">
              <v-chip
                v-for="item in xrayCheck.result.protocols.filter((item: any) => !item.supported)"
                :key="'u-'+item.id"
                size="small"
                variant="outlined"
                :title="item.reason || ''"
              >{{ item.name }}</v-chip>
            </div>
          </div>
          <div>
            <div class="xray-capability-title">{{ $t('xray.transports') }}</div>
            <div class="xray-capabilities">
              <v-chip v-for="item in xrayCheck.result.transports.filter((item: any) => item.supported)" :key="item.id" size="small" variant="tonal" color="secondary">{{ item.name }}</v-chip>
            </div>
          </div>
        </div>
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="xrayCheck.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-stethoscope" :loading="xrayCheck.loading" @click="runXrayCheck">{{ $t('actions.test') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-dialog v-model="quickAdd.visible" transition="dialog-bottom-transition" width="min(560px, calc(100vw - 24px))">
    <v-card class="rounded-lg">
      <v-card-title>{{ $t('pages.quickAddNode') }}</v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="12">
            <v-select
              v-model="quickAdd.core_type"
              label="Core"
              :items="coreOptions"
              item-title="title"
              item-value="value"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12">
            <v-select
              v-model="quickAdd.protocol"
              :label="$t('pages.selectProtocol')"
              :items="protocolOptions"
              item-title="title"
              item-value="value"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12">
            <v-text-field
              v-model="quickAdd.tag"
              :label="$t('objects.tag')"
              hide-details
              @update:model-value="quickAdd.tagCustomized = true"
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="regenerateQuickAdd" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12">
            <v-text-field
              v-model.number="quickAdd.count"
              :label="$t('pages.quickAddCount')"
              :hint="$t('pages.quickAddCountHint')"
              type="number"
              min="1"
              :max="maxQuickAddCount"
              persistent-hint
              hide-details="auto"
            ></v-text-field>
          </v-col>
          <v-col cols="12">
            <v-text-field
              v-model.number="quickAdd.port"
              :label="$t('in.port')"
              type="number"
              hide-details
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="quickAdd.port = RandomUtil.randomIntRange(10000, 60000)" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasPassword">
            <v-text-field
              v-model="quickAdd.password"
              :label="$t('types.pw')"
              hide-details
              readonly
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="quickAdd.password = randomPasswordForMethod(quickAdd.method)" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasMethod">
            <v-select
              v-model="quickAdd.method"
              :label="$t('in.ssMethod')"
              :items="shadowsocksMethods"
              @update:model-value="quickAdd.password = randomPasswordForMethod($event)"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasObfs">
            <v-text-field
              v-model="quickAdd.obfsPassword"
              :label="$t('types.hy.obfs')"
              hide-details
              readonly
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="quickAdd.obfsPassword = RandomUtil.randomShadowsocksPassword(16)" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasHandshake">
            <v-text-field
              v-model="quickAdd.handshakeServer"
              :label="$t('types.shdwTls.hs')"
              hide-details
            ></v-text-field>
          </v-col>
        </v-row>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn color="secondary" variant="tonal" prepend-icon="mdi-ip-network" @click="quickAdd.visible = false; relayModal.visible = true">
          {{ $t('relay.batchCreate') }}
        </v-btn>
        <v-btn color="primary" variant="outlined" @click="quickAdd.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="quickAdd.loading" @click="createQuickNode">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <InboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :inTags="inTags"
    :tlsConfigs="tlsConfigs"
    @close="closeModal"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <RelayPool
    :visible="relayModal.visible"
    @close="relayModal.visible = false"
  />
  <v-dialog v-model="deleteDialog.visible" width="min(420px, calc(100vw - 24px))">
    <v-card :title="$t('actions.del')" rounded="lg">
      <v-divider />
      <v-card-text>
        {{ $t('confirm') }}
        <div class="delete-target-tag">{{ deleteDialog.tag }}</div>
      </v-card-text>
      <v-card-actions class="justify-end">
        <v-btn color="secondary" variant="outlined" :disabled="deleteDialog.loading" @click="closeDeleteDialog">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="deleteDialog.loading" @click="confirmDelete">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-row class="page-toolbar" align="center" justify="start">
    <v-col cols="auto" class="page-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
      <v-btn color="primary" variant="tonal" class="ml-2" @click="openQuickAdd">
        <v-icon start icon="mdi-lightning-bolt"></v-icon>
        {{ $t('pages.quickAddNode') }}
      </v-btn>
      <v-btn color="secondary" variant="tonal" prepend-icon="mdi-shuffle-variant" @click="relayModal.visible = true">
        {{ $t('pages.relay') }}
      </v-btn>
      <v-btn v-if="!isOpenWrtLite" variant="tonal" prepend-icon="mdi-stethoscope" @click="openXrayCheck">
        {{ $t('xray.selfCheck') }}
      </v-btn>
    </v-col>
  </v-row>
  <v-row v-if="showListControls" class="inbound-list-controls" align="center">
    <v-col cols="12" md="7">
      <v-text-field
        v-model="inboundQuery"
        :label="$t('inboundList.search')"
        prepend-inner-icon="mdi-magnify"
        clearable
        density="compact"
        hide-details
      />
    </v-col>
    <v-col cols="12" md="5" class="inbound-list-controls__meta">
      <span class="inbound-list-summary">
        {{ $t('inboundList.summary', { start: visibleStart, end: visibleEnd, total: filteredInbounds.length }) }}
      </span>
      <v-select
        v-model="itemsPerPage"
        :items="pageSizeOptions"
        :label="$t('inboundList.pageSize')"
        density="compact"
        hide-details
        class="inbound-page-size"
      />
    </v-col>
  </v-row>
  <v-alert v-if="filteredInbounds.length === 0" type="info" variant="tonal" class="inbound-empty-state">
    {{ $t('inboundList.noMatch') }}
  </v-alert>
  <v-row class="resource-grid">
    <v-col cols="12" sm="6" md="4" lg="3" xl="2" v-for="item in visibleInbounds" :key="item.id || item.tag" class="resource-col inbound-card-col">
      <v-card rounded="lg" elevation="1" :title="item.tag" class="resource-card inbound-resource-card">
        <v-card-subtitle>{{ item.core_type || 'sing-box' }} / {{ item.type }}</v-card-subtitle>
        <v-card-text class="resource-card__body">
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('in.addr') }}</v-col>
            <v-col cols="7" class="resource-value">
              {{ item.listen }}
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('in.port') }}</v-col>
            <v-col cols="7" class="resource-value">
              {{ item.listen_port }}
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('objects.tls') }}</v-col>
            <v-col cols="7" class="resource-value">
              {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('pages.clients') }}</v-col>
            <v-col cols="7" class="resource-value" :title="item.users?.length ? item.users.join('\n') : undefined">
              <template v-if="item.users">
                {{ item.users.length }}
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('online') }}</v-col>
            <v-col cols="7" class="resource-value">
              <template v-if="onlineTags.has(item.tag)">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions class="resource-actions">
          <v-btn icon="mdi-file-edit" size="small" variant="text" :title="$t('actions.edit')" :aria-label="$t('actions.edit')" @click="showModal(item.id)" />
          <v-btn icon="mdi-file-remove" size="small" variant="text" color="warning" :title="$t('actions.del')" :aria-label="$t('actions.del')" @click="requestDelete(item)" />
          <v-btn icon="mdi-content-duplicate" size="small" variant="text" :title="$t('actions.clone')" :aria-label="$t('actions.clone')" :loading="cloneLoadingId === item.id" @click="clone(item.id)" />
          <v-btn v-if="trafficEnabled" icon="mdi-chart-line" size="small" variant="text" :title="$t('stats.graphTitle')" :aria-label="$t('stats.graphTitle')" @click="showStats(item.tag)" />
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
  <div v-if="pageCount > 1" class="inbound-pagination">
    <v-pagination v-model="currentPage" :length="pageCount" :total-visible="7" density="comfortable" />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config } from '@/types/config'
import { computed, ref, watch } from 'vue'
import { CoreTypes, createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import { i18n } from '@/locales'
import { push } from 'notivue'
import RelayPool from '@/layouts/modals/RelayPool.vue'

const isOpenWrtLite = import.meta.env.VITE_OPENWRT_LITE === 'true'

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> Data().inbounds
})

const tlsConfigs = computed((): any[] => {
  return <any[]> Data().tlsConfigs
})

const inTags = computed((): string[] => {
  return [...inbounds.value?.map(i => i.tag), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

const onlineTags = computed(() => new Set<string>(Data().onlines.inbound ?? []))
const trafficEnabled = computed(() => Data().enableTraffic)

const pageSizeOptions = [20, 40, 80]
const savedPageSize = Number(localStorage.getItem('inboundsPageSize'))
const itemsPerPage = ref(pageSizeOptions.includes(savedPageSize) ? savedPageSize : 20)
const currentPage = ref(1)
const inboundQuery = ref('')

const filteredInbounds = computed<any[]>(() => {
  const query = (inboundQuery.value || '').trim().toLocaleLowerCase()
  if (!query) return inbounds.value
  return inbounds.value.filter((item: any) => [
    item.tag,
    item.core_type || 'sing-box',
    item.type,
    item.listen,
    item.listen_port,
  ].some((value) => String(value ?? '').toLocaleLowerCase().includes(query)))
})

const pageCount = computed(() => Math.max(1, Math.ceil(filteredInbounds.value.length / itemsPerPage.value)))
const visibleInbounds = computed<any[]>(() => {
  const start = (currentPage.value - 1) * itemsPerPage.value
  return filteredInbounds.value.slice(start, start + itemsPerPage.value)
})
const visibleStart = computed(() => filteredInbounds.value.length === 0 ? 0 : (currentPage.value - 1) * itemsPerPage.value + 1)
const visibleEnd = computed(() => Math.min(currentPage.value * itemsPerPage.value, filteredInbounds.value.length))
const showListControls = computed(() => inbounds.value.length > pageSizeOptions[0] || Boolean(inboundQuery.value))

watch(inboundQuery, () => {
  currentPage.value = 1
})
watch(itemsPerPage, (value) => {
  localStorage.setItem('inboundsPageSize', String(value))
  currentPage.value = 1
})
watch(pageCount, (value) => {
  if (currentPage.value > value) currentPage.value = value
})

const modal = ref({
  visible: false,
  id: 0,
})

const relayModal = ref({ visible: false })

const deleteDialog = ref({
  visible: false,
  loading: false,
  id: 0,
  tag: '',
})

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const quickAdd = ref({
  visible: false,
  core_type: CoreTypes.SingBox,
  protocol: 'mixed',
  tag: '',
  tagCustomized: false,
  count: 1,
  port: RandomUtil.randomIntRange(10000, 60000),
  password: '',
  method: '2022-blake3-aes-256-gcm',
  obfsPassword: '',
  handshakeServer: 'www.microsoft.com',
  hasPassword: false,
  hasMethod: false,
  hasObfs: false,
  hasHandshake: false,
  loading: false,
})

const xrayCheck = ref<{ visible: boolean, loading: boolean, result: any }>({
  visible: false,
  loading: false,
  result: null,
})

const runXrayCheck = async () => {
  xrayCheck.value.loading = true
  try {
    const msg = await HttpUtils.get('api/checkXray')
    xrayCheck.value.result = msg.success ? msg.obj : { healthy: false, error: msg.msg }
  } catch (error: any) {
    xrayCheck.value.result = { healthy: false, error: error?.message || i18n.global.t('xray.checkFailed') }
  } finally {
    xrayCheck.value.loading = false
  }
}

const openXrayCheck = () => {
  xrayCheck.value.visible = true
  runXrayCheck()
}

const coreOptions = computed(() => {
  const items = [{ title: 'sing-box', value: CoreTypes.SingBox }]
  if (!isOpenWrtLite) items.push({ title: 'Xray-core', value: CoreTypes.Xray })
  return items
})

watch(() => quickAdd.value.protocol, (val) => {
  quickAdd.value.hasPassword = val === 'shadowsocks'
  quickAdd.value.hasMethod = val === 'shadowsocks' && quickAdd.value.core_type !== CoreTypes.Xray
  quickAdd.value.hasObfs = val === 'hysteria2' && quickAdd.value.core_type !== CoreTypes.Xray
  quickAdd.value.hasHandshake = val === 'shadowtls'
  if (quickAdd.value.core_type === CoreTypes.Xray && val === 'shadowsocks') {
    quickAdd.value.method = '2022-blake3-aes-256-gcm'
  }
  regenerateQuickAdd(false)
})

watch(() => quickAdd.value.core_type, (val) => {
  if (isOpenWrtLite && val !== CoreTypes.SingBox) {
    quickAdd.value.core_type = CoreTypes.SingBox
    return
  }
  if (val === CoreTypes.Xray) {
    const allowed = xrayProtocolOptions.some((item) => item.value === quickAdd.value.protocol)
    if (!allowed) quickAdd.value.protocol = 'vless'
    if (quickAdd.value.protocol === 'shadowsocks') {
      quickAdd.value.method = '2022-blake3-aes-256-gcm'
      quickAdd.value.hasMethod = false
    }
	quickAdd.value.hasObfs = false
  } else {
    quickAdd.value.hasMethod = quickAdd.value.protocol === 'shadowsocks'
	quickAdd.value.hasObfs = quickAdd.value.protocol === 'hysteria2'
  }
  regenerateQuickAdd(false)
})

const closeModal = () => {
  modal.value.visible = false
}

const requestDelete = (item: Inbound) => {
  deleteDialog.value.id = item.id
  deleteDialog.value.tag = item.tag
  deleteDialog.value.visible = true
}

const closeDeleteDialog = () => {
  if (deleteDialog.value.loading) return
  deleteDialog.value.visible = false
}

const confirmDelete = async () => {
  if (!deleteDialog.value.id || !deleteDialog.value.tag) return
  deleteDialog.value.loading = true
  try {
    const success = await Data().save('inbounds', 'del', deleteDialog.value.tag)
    if (success) deleteDialog.value.visible = false
  } finally {
    deleteDialog.value.loading = false
  }
}

const cloneLoadingId = ref(0)

const clone = async (id: number) => {
  cloneLoadingId.value = id
  try {
    const inboundArray = await Data().loadInbounds([id])
    const inbound = inboundArray[0]
    if (!inbound) return
    const newTag = inbound.type + "-" + RandomUtil.randomSeq(3)
    const newInbound = createInbound(inbound.type, { ...inbound,
      id: 0,
      tag: newTag,
      listen_port: RandomUtil.randomIntRange(10000, 60000),
    })
    await Data().save("inbounds", "new", newInbound)
  } finally {
    cloneLoadingId.value = 0
  }
}



const singBoxProtocolOptions = [
  { title: 'Mixed', value: 'mixed' },
  { title: 'SOCKS', value: 'socks' },
  { title: 'HTTP', value: 'http' },
  { title: 'Shadowsocks', value: 'shadowsocks' },
  { title: 'VMess', value: 'vmess' },
  { title: 'Trojan', value: 'trojan' },
  { title: 'VLESS', value: 'vless' },
  { title: 'Hysteria2', value: 'hysteria2' },
  { title: 'ShadowTLS', value: 'shadowtls' },
  { title: 'TUIC', value: 'tuic' },
  { title: 'Naive', value: 'naive' },
  { title: 'AnyTLS', value: 'anytls' },
  { title: 'Direct', value: 'direct' },
]

const xrayProtocolOptions = [
  { title: 'VLESS', value: 'vless' },
  { title: 'VMess', value: 'vmess' },
  { title: 'Trojan', value: 'trojan' },
  { title: 'Shadowsocks', value: 'shadowsocks' },
  { title: 'SOCKS', value: 'socks' },
  { title: 'HTTP', value: 'http' },
  { title: 'Mixed', value: 'mixed' },
  { title: 'Hysteria2', value: 'hysteria2' },
  { title: 'Dokodemo-door', value: 'dokodemo-door' },
]

const protocolOptions = computed(() => {
  if (quickAdd.value.core_type === CoreTypes.Xray) {
    return xrayProtocolOptions
  }
  return singBoxProtocolOptions
})

const shadowsocksMethods = [
  'aes-128-gcm',
  'aes-192-gcm',
  'aes-256-gcm',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm',
  '2022-blake3-aes-256-gcm',
  '2022-blake3-chacha20-poly1305',
]

const maxQuickAddCount = 100

const randomPasswordForMethod = (method: string): string => {
  if (method === '2022-blake3-aes-128-gcm') return RandomUtil.randomShadowsocksPassword(16)
  if (method.startsWith('2022')) return RandomUtil.randomShadowsocksPassword(32)
  return RandomUtil.randomSeq(16)
}

const regenerateQuickAdd = (resetTag = true) => {
  const port = RandomUtil.randomIntRange(10000, 60000)
  if (resetTag || !quickAdd.value.tagCustomized) {
    quickAdd.value.tag = quickAdd.value.protocol + '-' + port
    quickAdd.value.tagCustomized = false
  }
  quickAdd.value.port = port
  quickAdd.value.password = randomPasswordForMethod(quickAdd.value.method)
}

const openQuickAdd = () => {
  quickAdd.value.count = 1
  quickAdd.value.tagCustomized = false
  regenerateQuickAdd()
  quickAdd.value.visible = true
}

const needsTls = ['vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'naive', 'anytls']

const pinnedSha256FromCertificate = async (certificate: string[]): Promise<string[]> => {
  try {
    const resp = await fetch('api/pinnedSha256', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cert: certificate.join('\n') }),
      credentials: 'include',
    })
    const msg = await resp.json()
    if (msg.success && Array.isArray(msg.obj)) return msg.obj
  } catch (e) {
    console.error('pinnedSha256FromCertificate error:', e)
  }
  return []
}

const genSelfSignedTls = async (serverName: string): Promise<number> => {
  let tlsName = 'auto-' + quickAdd.value.tag
  while (Data().tlsConfigs.find((t: any) => t.name === tlsName)) {
    tlsName += '-copy'
  }
  const cleanServerName = (serverName || quickAdd.value.tag).replace(/[^a-zA-Z0-9.-]/g, '-')
  try {
    const keyMsg = await HttpUtils.get('api/keypairs', { k: 'tls', o: cleanServerName })
    if (!keyMsg.success || !keyMsg.obj || !keyMsg.obj.length) return 0
    const lines: string[] = keyMsg.obj.filter((l: string) => l && l.trim())
    if (lines.length < 4) return 0
    const privateKey: string[] = []
    const publicKey: string[] = []
    let inKey = false, inCert = false
    for (const line of lines) {
      const t = line.trim()
      if (!t) continue
      if (t === '-----BEGIN PRIVATE KEY-----') { inKey = true; inCert = false; privateKey.push(t) }
      else if (t === '-----END PRIVATE KEY-----') { inKey = false; privateKey.push(t) }
      else if (t === '-----BEGIN CERTIFICATE-----') { inCert = true; inKey = false; publicKey.push(t) }
      else if (t === '-----END CERTIFICATE-----') { inCert = false; publicKey.push(t) }
      else { if (inKey) privateKey.push(t); if (inCert) publicKey.push(t) }
    }
    if (!privateKey.length || !publicKey.length) return 0
    const pinnedSha256 = await pinnedSha256FromCertificate(publicKey)
    if (!pinnedSha256.length) return 0
    const tlsConfig = {
      id: 0,
      name: tlsName,
      server: {
        enabled: true,
        key: privateKey,
        certificate: publicKey,
      },
      client: {
        pinned_peer_certificate_sha256: pinnedSha256,
      }
    }
    const success = await Data().save('tls', 'new', tlsConfig)
    if (success) {
      const saved = Data().tlsConfigs.find((t: any) => t.name === tlsName)
      if (saved && saved.id) return saved.id
    }
  } catch (e) {
    console.error('genSelfSignedTls error:', e)
  }
  return 0
}

const normalizeQuickAddCount = (): number => {
  const value = Number(quickAdd.value.count)
  const count = Number.isFinite(value) ? Math.floor(value) : 1
  quickAdd.value.count = Math.min(maxQuickAddCount, Math.max(1, count))
  return quickAdd.value.count
}

const availableQuickPorts = (count: number): number[] => {
  const occupied = new Set<number>(
    inbounds.value
      .map((item: any) => Number(item.listen_port))
      .filter((port: number) => Number.isInteger(port) && port > 0)
  )
  const ports: number[] = []
  let candidate = Number(quickAdd.value.port)
  if (!Number.isInteger(candidate) || candidate < 1 || candidate > 65535) {
    candidate = RandomUtil.randomIntRange(10000, 60000)
  }
  for (let index = 0; index < count; index++) {
    let attempts = 0
    while (occupied.has(candidate) && attempts < 65535) {
      candidate = candidate >= 65535 ? 1 : candidate + 1
      attempts++
    }
    if (attempts >= 65535) return []
    ports.push(candidate)
    occupied.add(candidate)
    candidate = candidate >= 65535 ? 1 : candidate + 1
  }
  return ports
}

const uniqueQuickTags = (count: number, baseTag: string): string[] => {
  const used = new Set<string>(inbounds.value.map((item: any) => item.tag).filter(Boolean))
  const tags: string[] = []
  for (let index = 0; index < count; index++) {
    const suffix = count > 1 ? `-${index + 1}` : ''
    const initial = `${baseTag}${suffix}`
    let tag = initial
    let copy = 1
    while (used.has(tag)) {
      tag = `${initial}-copy${copy}`
      copy++
    }
    used.add(tag)
    tags.push(tag)
  }
  return tags
}

const quickAddListenAddress = (): string => {
  const host = location.hostname.replace(/^\[|\]$/g, '')
  return host.includes(':') ? '::' : '0.0.0.0'
}

const createQuickNode = async () => {
  quickAdd.value.loading = true
  const count = normalizeQuickAddCount()
  const proto = quickAdd.value.protocol
  const ports = availableQuickPorts(count)
  const baseTag = quickAdd.value.tag.trim() || `${proto}-${ports[0] || RandomUtil.randomIntRange(10000, 60000)}`
  const tags = uniqueQuickTags(count, baseTag)

  if (!ports.length) {
    quickAdd.value.loading = false
    push.error('No available ports for quick add.')
    return
  }

  let tlsId = 0
  if (needsTls.includes(proto)) {
    tlsId = await genSelfSignedTls(tags[0])
    if (tlsId === 0) {
      quickAdd.value.loading = false
      push.error('TLS generation failed. Please create TLS certificate in TLS Settings first.')
      return
    }
  }
  const needsClient = ['shadowsocks', 'vmess', 'vless', 'trojan', 'naive', 'hysteria2', 'tuic', 'anytls', 'shadowtls']
  let createdCount = 0
  const isXray = quickAdd.value.core_type === CoreTypes.Xray
  for (let index = 0; index < count; index++) {
    const clientName = 'user-' + RandomUtil.randomSeq(6)
    const nodePassword = index === 0 ? quickAdd.value.password : randomPasswordForMethod(quickAdd.value.method)
    const uuid = RandomUtil.randomUUID()
    const inbound = createInbound(proto, {
      id: 0,
      core_type: quickAdd.value.core_type,
      tag: tags[index],
      listen: quickAddListenAddress(),
      listen_port: ports[index],
    } as any)

    switch (proto) {
      case 'shadowsocks':
        ;(inbound as any).method = isXray ? '2022-blake3-aes-256-gcm' : quickAdd.value.method || '2022-blake3-aes-256-gcm'
        ;(inbound as any).password = nodePassword || randomPasswordForMethod((inbound as any).method)
        inbound.addrs = []
        inbound.out_json = {}
        break
      case 'vmess':
        ;(inbound as any).tls_id = tlsId
        ;(inbound as any).transport = isXray ? { type: 'ws', path: '/', host: location.hostname } : { type: 'http' }
        inbound.addrs = []
        inbound.out_json = {}
        break
      case 'vless':
        ;(inbound as any).tls_id = tlsId
        ;(inbound as any).transport = isXray ? { type: 'xhttp', path: '/xhttp', host: location.hostname, mode: 'auto' } : {}
        inbound.addrs = []
        inbound.out_json = {}
        break
      case 'trojan':
        ;(inbound as any).tls_id = tlsId
        ;(inbound as any).transport = isXray ? { type: 'ws', path: '/', host: location.hostname } : {}
        inbound.addrs = []
        inbound.out_json = {}
        break
      case 'shadowtls':
        ;(inbound as any).version = 3
        ;(inbound as any).password = nodePassword || RandomUtil.randomShadowsocksPassword(16)
        ;(inbound as any).handshake = { server: quickAdd.value.handshakeServer || 'www.microsoft.com', server_port: 443 }
        break
      case 'hysteria2':
        ;(inbound as any).tls_id = tlsId
        if (!isXray) {
          ;(inbound as any).obfs = { type: 'salamander', password: quickAdd.value.obfsPassword || RandomUtil.randomShadowsocksPassword(16) }
        }
        break
      case 'tuic':
        ;(inbound as any).tls_id = tlsId
        ;(inbound as any).congestion_control = 'cubic'
        break
      case 'naive':
        ;(inbound as any).tls_id = tlsId
        break
      case 'anytls':
        ;(inbound as any).tls_id = tlsId
        ;(inbound as any).padding_scheme = [
          'stop=8', '0=30-30', '1=100-400',
          '2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000',
          '3=9-9,500-1000', '4=500-1000', '5=500-1000', '6=500-1000', '7=500-1000'
        ]
        break
      case 'mixed':
      case 'socks':
      case 'http':
        inbound.addrs = []
        inbound.out_json = {}
        break
    }

    let initUsers: number[] | undefined
    if (needsClient.includes(proto)) {
      const protoConfig: any = {}
      switch (proto) {
        case 'shadowsocks': protoConfig.shadowsocks = { name: clientName, password: (inbound as any).password }; break
        case 'vmess': protoConfig.vmess = { name: clientName, uuid, alterId: 0 }; break
        case 'vless': protoConfig.vless = { name: clientName, uuid, flow: isXray ? '' : 'xtls-rprx-vision' }; break
        case 'trojan': protoConfig.trojan = { name: clientName, password: nodePassword }; break
        case 'naive': protoConfig.naive = { username: clientName, password: nodePassword }; break
        case 'hysteria2': protoConfig.hysteria2 = { name: clientName, password: nodePassword }; break
        case 'tuic': protoConfig.tuic = { name: clientName, uuid, password: nodePassword }; break
        case 'anytls': protoConfig.anytls = { name: clientName, password: nodePassword }; break
        case 'shadowtls': protoConfig.shadowtls = { name: clientName, password: (inbound as any).password }; break
      }
      const client = { enable: true, name: clientName, config: protoConfig, inbounds: [], links: [], volume: 0, expiry: 0, up: 0, down: 0, desc: '', group: '' }
      const clientBody = new URLSearchParams({ object: 'clients', action: 'new', data: JSON.stringify(client) })
      try {
        const clientResp = await fetch('api/save', { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: clientBody.toString(), credentials: 'include' })
        const clientMsg = await clientResp.json()
        const savedClient = clientMsg.success && clientMsg.obj?.clients?.find((c: any) => c.name === clientName)
        if (savedClient?.id) initUsers = [savedClient.id]
      } catch (e) {
        console.error('Quick add client creation error:', e)
      }
      if (!initUsers) break
    }

    if (await Data().save('inbounds', 'new', inbound, initUsers)) createdCount++
    else break
  }
  quickAdd.value.loading = false
  if (createdCount === count) {
    quickAdd.value.visible = false
  }
}

const stats = ref({
  visible: false,
  resource: "inbound",
  tag: "",
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>

<style scoped>
.xray-capability-groups {
  display: grid;
  gap: 12px;
}

.xray-capability-title {
  margin-bottom: 6px;
  color: rgba(var(--v-theme-on-surface), 0.65);
  font-size: 12px;
  font-weight: 600;
}

.xray-capabilities {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.inbound-list-controls {
  max-width: 980px;
  margin-inline: auto !important;
}

.inbound-list-controls__meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

.inbound-list-summary {
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 13px;
  white-space: nowrap;
}

.inbound-page-size {
  flex: 0 0 132px;
  max-width: 132px;
}

.inbound-empty-state {
  max-width: 720px;
  margin: 20px auto;
}

.inbound-card-col {
  content-visibility: auto;
  contain-intrinsic-size: 266px 264px;
}

.inbound-pagination {
  display: flex;
  justify-content: center;
  padding: 18px 0 2px;
}

.delete-target-tag {
  margin-top: 8px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

@media (max-width: 600px) {
  .inbound-list-controls__meta {
    justify-content: space-between;
  }
}
</style>
