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
  <v-dialog v-model="quickAdd.visible" transition="dialog-bottom-transition" width="min(560px, calc(100vw - 24px))" scrollable>
    <v-card class="rounded-lg">
      <v-card-title class="justify-center text-center">{{ $t('pages.quickAddNode') }}</v-card-title>
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
          <NaiveQuickAdd
            v-if="quickAdd.protocol === 'naive'"
            :data="quickAdd.naive"
            :tls-configs="tlsConfigs"
          />
          <VlessQuickAdd
            v-if="quickAdd.protocol === 'vless'"
            v-model:core-type="quickAdd.core_type"
            :data="quickAdd.vless"
            :xray-available="!isOpenWrtLite"
            :port="Number(quickAdd.port)"
          />
        </v-row>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions class="quick-add-dialog-actions">
        <v-btn class="quick-add-relay-action" color="secondary" variant="tonal" prepend-icon="mdi-ip-network" @click="quickAdd.visible = false; relayModal.visible = true">
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
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
      <v-btn color="primary" variant="outlined" prepend-icon="mdi-lightning-bolt" @click="openQuickAdd">{{ $t('pages.quickAddNode') }}</v-btn>
      <v-btn variant="outlined" prepend-icon="mdi-shuffle-variant" @click="relayModal.visible = true">{{ $t('pages.relay') }}</v-btn>
      <v-btn v-if="!isOpenWrtLite" variant="outlined" prepend-icon="mdi-stethoscope" @click="openXrayCheck">{{ $t('xray.selfCheck') }}</v-btn>
    </div>
    <v-text-field
      v-model="inboundQuery"
      class="list-toolbar__search"
      :placeholder="$t('inboundList.search')"
      prepend-inner-icon="mdi-magnify"
      clearable
      density="compact"
      hide-details
    />
  </div>
  <v-data-table
    class="list-table"
    :headers="inboundHeaders"
    :items="filteredInbounds"
    item-value="tag"
    v-model:items-per-page="itemsPerPage"
    v-model:page="currentPage"
    :items-per-page-options="pageSizeItems"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="inboundQuery ? $t('inboundList.noMatch') : $t('noData')"
    hover
  >
    <template #item.tag="{ item }">
      <span class="list-main">{{ item.tag }}</span>
    </template>
    <template #item.type="{ item }">
      <v-chip size="small" label variant="tonal" :color="item.core_type === 'xray' ? 'info' : 'primary'" class="me-2">{{ item.core_type || 'sing-box' }}</v-chip>
      <span>{{ item.type }}</span>
      <v-chip v-if="item.cdn?.domain" size="small" label variant="tonal" color="warning" class="ms-2" prepend-icon="mdi-cloud-download-outline" :title="$t('quickAdd.cdnDownlink')">{{ item.cdn.domain }}:{{ item.cdn.port }}</v-chip>
    </template>
    <template #item.listen="{ item }">
      <span class="list-mono">{{ listenAddress(item) }}</span>
    </template>
    <template #item.tls_id="{ item }">
      <v-chip size="small" label variant="tonal" :color="item.tls_id > 0 ? 'success' : undefined">{{ item.tls_id > 0 ? $t('enable') : $t('disable') }}</v-chip>
    </template>
    <template #item.users="{ item }">
      <span :title="item.users?.length ? item.users.join('\n') : undefined">{{ item.users?.length ?? '-' }}</span>
    </template>
    <template #item.online="{ item }">
      <span class="list-status" :class="{ 'list-status--on': onlineTags.has(item.tag) }">{{ onlineTags.has(item.tag) ? $t('online') : $t('list.idle') }}</span>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showModal(item.id)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="primary" :loading="cloneLoadingId === item.id" @click="clone(item.id)">{{ $t('actions.clone') }}</v-btn>
        <v-btn v-if="trafficEnabled" size="small" variant="text" color="primary" @click="showStats(item.tag)">{{ $t('list.traffic') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete(item)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config } from '@/types/config'
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { CoreTypes, createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import { i18n } from '@/locales'
import { push } from 'notivue'
import RelayPool from '@/layouts/modals/RelayPool.vue'
import { fetchBackendObject } from '@/utils/backend'
import NaiveQuickAdd from '@/components/NaiveQuickAdd.vue'
import VlessQuickAdd from '@/components/VlessQuickAdd.vue'
import { createNaiveQuickAddOptions, normalizeNaiveServer, parseNaiveExtraHeaders } from '@/types/naive'
import { createVlessQuickAddOptions, normalizeCdnDomain, normalizeRealityServer, vlessCdnVariants } from '@/types/vless'

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
const itemsPerPage = ref([...pageSizeOptions, -1].includes(savedPageSize) ? savedPageSize : 20)
const currentPage = ref(1)
const inboundQuery = ref('')
const { smAndDown } = useDisplay()
const pageSizeItems = computed(() => [
  ...pageSizeOptions.map(value => ({ value, title: String(value) })),
  { value: -1, title: i18n.global.t('list.all') },
])
const inboundHeaders = computed(() => [
  { title: i18n.global.t('objects.tag'), key: 'tag' },
  { title: i18n.global.t('list.protocol'), key: 'type' },
  { title: i18n.global.t('list.listen'), key: 'listen', sortable: false },
  { title: 'TLS', key: 'tls_id' },
  { title: i18n.global.t('pages.clients'), key: 'users', sortable: false },
  { title: i18n.global.t('list.status'), key: 'online', sortable: false },
  { title: i18n.global.t('list.actions'), key: 'actions', sortable: false },
])
const listenAddress = (item: any) => {
  const host = String(item.listen || '')
  const port = item.listen_port ?? ''
  return host.includes(':') ? `[${host}]:${port}` : `${host}:${port}`
}

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

watch(inboundQuery, () => {
  currentPage.value = 1
})
watch(itemsPerPage, (value) => {
  localStorage.setItem('inboundsPageSize', String(value))
  currentPage.value = 1
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
  hasPassword: false,
  hasMethod: false,
  hasObfs: false,
  naive: createNaiveQuickAddOptions(location.hostname),
  vless: createVlessQuickAddOptions(),
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
  if (quickAdd.value.protocol === 'naive') {
    quickAdd.value.naive.username = 'naive-' + RandomUtil.randomSeq(6)
    quickAdd.value.naive.password = RandomUtil.randomShadowsocksPassword(32)
    if (!quickAdd.value.naive.server) quickAdd.value.naive.server = location.hostname
  }
}

const openQuickAdd = () => {
  quickAdd.value.count = 1
  quickAdd.value.tagCustomized = false
  regenerateQuickAdd()
  quickAdd.value.visible = true
}

const normalizeQuickAddCount = (): number => {
  const value = Number(quickAdd.value.count)
  const count = Number.isFinite(value) ? Math.floor(value) : 1
  quickAdd.value.count = Math.min(maxQuickAddCount, Math.max(1, count))
  return quickAdd.value.count
}

// The panel builds the nodes (certificates, REALITY keys, users and share
// links) exactly as it does for managed servers.
const createQuickNode = async () => {
  if (quickAdd.value.loading) return
  const count = normalizeQuickAddCount()
  const proto = quickAdd.value.protocol
  const port = Math.floor(Number(quickAdd.value.port))
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    push.error({ message: `${i18n.global.t('in.port')}: 1-65535` })
    return
  }

  let naiveServer = ''
  let naiveExtraHeaders: Record<string, string> = {}
  if (proto === 'naive') {
    try {
      naiveServer = normalizeNaiveServer(quickAdd.value.naive.server || location.hostname)
      naiveExtraHeaders = parseNaiveExtraHeaders(quickAdd.value.naive.extra_headers_text)
    } catch {
      push.error({ message: i18n.global.t('types.naive.invalidOptions') })
      return
    }
    if (!quickAdd.value.naive.username.trim() || !quickAdd.value.naive.password) {
      push.error({ message: i18n.global.t('types.naive.identityRequired') })
      return
    }
  }
  let realityServer = ''
  let cdnDomain = ''
  if (proto === 'vless') {
    try {
      realityServer = normalizeRealityServer(quickAdd.value.vless.reality_server)
    } catch {
      push.error({ message: i18n.global.t('quickAdd.invalidRealityServer') })
      return
    }
    if (quickAdd.value.vless.cdn_enabled && vlessCdnVariants.includes(quickAdd.value.vless.variant)) {
      try {
        cdnDomain = normalizeCdnDomain(quickAdd.value.vless.cdn_domain)
      } catch {
        push.error({ message: i18n.global.t('quickAdd.invalidCdnDomain') })
        return
      }
    }
  }

  let password: string | undefined
  if (proto === 'naive') password = quickAdd.value.naive.password
  else if (proto === 'shadowsocks') password = quickAdd.value.password

  quickAdd.value.loading = true
  try {
    const result = await fetchBackendObject<any>('api/inbounds/quick-add', {
      method: 'POST',
      body: JSON.stringify({
        core_type: quickAdd.value.core_type,
        protocol: proto,
        tag: quickAdd.value.tag.trim(),
        count,
        port,
        password,
        method: quickAdd.value.method,
        obfs_password: quickAdd.value.obfsPassword,
        naive_username: quickAdd.value.naive.username.trim(),
        naive_server: naiveServer,
        naive_mode: quickAdd.value.naive.mode,
        naive_tls_id: Number(quickAdd.value.naive.tls_id) || 0,
        naive_extra_headers: naiveExtraHeaders,
        naive_udp_over_tcp: quickAdd.value.naive.udp_over_tcp,
        naive_insecure_concurrency: quickAdd.value.naive.mode === 'https'
          ? Math.min(4, Math.max(0, Number(quickAdd.value.naive.insecure_concurrency) || 0))
          : 0,
        naive_quic_congestion_control: quickAdd.value.naive.quic_congestion_control,
        vless_variant: proto === 'vless' ? quickAdd.value.vless.variant : undefined,
        reality_server: proto === 'vless' ? realityServer : undefined,
        cdn_domain: cdnDomain || undefined,
        cdn_port: cdnDomain ? Number(quickAdd.value.vless.cdn_port) || 0 : undefined,
        public_host: location.hostname.replace(/^\[|\]$/g, ''),
      }),
    })
    quickAdd.value.visible = false
    push.success({ message: i18n.global.t('quickAdd.created', { count: result?.created?.length || count }) })
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('failed') })
  } finally {
    await Data().loadData()
    quickAdd.value.loading = false
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

.delete-target-tag {
  margin-top: 8px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

</style>
