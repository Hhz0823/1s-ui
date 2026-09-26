<template>
  <v-dialog v-model="quickAdd.visible" width="min(560px, calc(100vw - 24px))" scrollable>
    <v-card class="remote-quick-dialog">
      <v-card-title class="justify-center text-center">{{ $t('pages.quickAddNode') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-row>
          <v-col cols="12" sm="6">
            <v-select v-model="quickAdd.core_type" label="Core" :items="coreOptions" item-title="title" item-value="value" hide-details />
          </v-col>
          <v-col cols="12" sm="6">
            <v-select v-model="quickAdd.protocol" :label="$t('pages.selectProtocol')" :items="protocolOptions" item-title="title" item-value="value" hide-details />
          </v-col>
          <v-col cols="12">
            <v-text-field v-model="quickAdd.tag" :label="$t('objects.tag')" hide-details>
              <template #append-inner>
                <v-btn icon="mdi-refresh" size="x-small" variant="text" :title="$t('actions.update')" @click="regenerateQuickAdd" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" sm="6">
            <v-text-field v-model.number="quickAdd.count" :label="$t('pages.quickAddCount')" :hint="$t('pages.quickAddCountHint')" type="number" min="1" max="100" persistent-hint hide-details="auto" />
          </v-col>
          <v-col cols="12" sm="6">
            <v-text-field v-model.number="quickAdd.port" :label="$t('in.port')" type="number" min="1" max="65535" hide-details>
              <template #append-inner>
                <v-btn icon="mdi-refresh" size="x-small" variant="text" :title="$t('actions.update')" @click="quickAdd.port = RandomUtil.randomIntRange(10000, 60000)" />
              </template>
            </v-text-field>
          </v-col>
          <v-col v-if="quickAdd.protocol === 'shadowsocks'" cols="12">
            <v-select v-model="quickAdd.method" :label="$t('in.ssMethod')" :items="shadowsocksMethods" hide-details />
          </v-col>
          <v-col v-if="quickAdd.protocol === 'hysteria2' && quickAdd.core_type === CoreTypes.SingBox" cols="12">
            <v-text-field v-model="quickAdd.obfs_password" :label="$t('types.hy.obfs')" hide-details>
              <template #append-inner>
                <v-btn icon="mdi-refresh" size="x-small" variant="text" :title="$t('actions.update')" @click="quickAdd.obfs_password = RandomUtil.randomShadowsocksPassword(16)" />
              </template>
            </v-text-field>
          </v-col>
          <NaiveQuickAdd
            v-if="quickAdd.protocol === 'naive'"
            :data="quickAdd.naive"
            :tls-configs="tlsConfigs"
          />
        </v-row>
      </v-card-text>
      <v-divider />
      <v-card-actions class="quick-add-dialog-actions justify-center">
        <v-btn class="quick-add-relay-action" color="secondary" variant="tonal" prepend-icon="mdi-ip-network" @click="quickAdd.visible = false; relayModal.visible = true">{{ $t('relay.batchCreate') }}</v-btn>
        <v-btn variant="outlined" @click="quickAdd.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="quickAdd.loading" @click="createQuickNodes">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <RelayPool
    :visible="relayModal.visible"
    :agent-id="nodeId"
    :connection-host="connectionHost"
    :tls-configs="tlsConfigs"
    @close="relayModal.visible = false"
    @changed="loadAll"
  />

  <InboundVue
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :inTags="inTags"
    :tlsConfigs="tlsConfigs"
    :dataSource="dataSource"
    :connectionHost="connectionHost"
    @close="closeModal"
  />

  <v-dialog v-model="remove.visible" width="min(420px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('actions.del') }}</v-card-title>
      <v-card-text class="text-center"><strong dir="ltr">{{ remove.item?.tag }}</strong></v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="remove.visible = false">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="remove.loading" @click="deleteInbound">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <header class="remote-header">
    <v-btn icon="mdi-arrow-left" variant="text" :title="$t('agent.backDetail')" @click="router.push(`/agents/${nodeId}`)" />
    <div class="remote-title">
      <h1 class="adaptive-ink">{{ $t('agent.remoteInbounds') }}</h1>
      <div class="adaptive-ink">{{ node?.name || ('#' + nodeId) }} · <span dir="ltr">{{ connectionHost || '-' }}</span></div>
    </div>
    <div class="remote-actions">
      <v-btn variant="tonal" prepend-icon="mdi-refresh" :loading="loading" @click="loadAll">{{ $t('actions.update') }}</v-btn>
      <v-btn color="primary" variant="tonal" prepend-icon="mdi-flash" :disabled="!supportsQuickAdd || !connectionHost" @click="openQuickAdd">{{ $t('pages.quickAddNode') }}</v-btn>
      <v-btn color="secondary" variant="tonal" prepend-icon="mdi-shuffle-variant" :disabled="!supportsRelay || !connectionHost" @click="relayModal.visible = true">{{ $t('pages.relay') }}</v-btn>
      <v-btn color="primary" prepend-icon="mdi-plus" :disabled="!node?.managed || !connectionHost" @click="openModal(0)">{{ $t('actions.add') }}</v-btn>
    </div>
  </header>

  <v-alert v-if="node && !node.managed" type="warning" variant="tonal" class="mb-4">{{ $t('agent.notManaged') }}</v-alert>
  <v-alert v-else-if="node && !connectionHost" type="warning" variant="tonal" class="mb-4">{{ $t('agent.publicHostRequired') }}</v-alert>
  <v-alert v-else-if="node && (!supportsQuickAdd || !supportsRelay)" type="warning" variant="tonal" class="mb-4">{{ $t('agent.remoteUpgradeRequired') }}</v-alert>
  <v-alert v-else type="info" variant="tonal" density="compact" class="mb-4">{{ $t('agent.remoteInboundHint') }}</v-alert>

  <div class="list-controls">
    <v-text-field v-model="query" prepend-inner-icon="mdi-magnify" :label="$t('inboundList.search')" density="compact" hide-details clearable />
    <span>{{ filtered.length }} {{ $t('objects.inbound') }}</span>
  </div>

  <v-progress-linear v-if="loading" indeterminate class="mb-3" />
  <v-alert v-if="!loading && filtered.length === 0" type="info" variant="tonal">{{ $t('inboundList.noMatch') }}</v-alert>
  <div v-else class="remote-grid">
    <article v-for="item in visible" :key="item.id || item.tag" class="remote-item">
      <header>
        <div>
          <strong>{{ item.tag }}</strong>
          <small>{{ item.core_type || 'sing-box' }} / {{ item.type }}</small>
        </div>
        <v-chip size="x-small" variant="tonal">{{ item.listen_port }}</v-chip>
      </header>
      <dl>
        <div><dt>{{ $t('in.addr') }}</dt><dd dir="ltr">{{ item.listen || '::' }}</dd></div>
        <div><dt>TLS</dt><dd>{{ item.tls_id ? $t('enable') : $t('disable') }}</dd></div>
      </dl>
      <footer>
        <v-btn icon="mdi-pencil-outline" size="small" variant="text" :title="$t('actions.edit')" @click="openModal(item.id)" />
        <v-btn icon="mdi-delete-outline" size="small" variant="text" color="error" :title="$t('actions.del')" @click="askDelete(item)" />
      </footer>
    </article>
  </div>

  <v-pagination v-if="pages > 1" v-model="page" :length="pages" :total-visible="7" class="mt-4" />
</template>

<script lang="ts" setup>
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { push } from 'notivue'
import { i18n } from '@/locales'
import InboundVue from '@/layouts/modals/Inbound.vue'
import RelayPool from '@/layouts/modals/RelayPool.vue'
import RandomUtil from '@/plugins/randomUtil'
import { CoreTypes } from '@/types/inbounds'
import { fetchBackendObject as api } from '@/utils/backend'
import NaiveQuickAdd from '@/components/NaiveQuickAdd.vue'
import { createNaiveQuickAddOptions, normalizeNaiveServer, parseNaiveExtraHeaders } from '@/types/naive'

const route = useRoute()
const router = useRouter()
const nodeId = Number(route.params.id)
const loading = ref(false)
const node = ref<any>(null)
const inbounds = ref<any[]>([])
const tlsConfigs = ref<any[]>([])
const revision = ref(0)
const editorInbound = ref<any>(null)
const query = ref('')
const page = ref(1)
const pageSize = 20
const modal = reactive({ visible: false, id: 0 })
const remove = reactive<{ visible: boolean, loading: boolean, item?: any }>({ visible: false, loading: false })
const relayModal = reactive({ visible: false })
const quickAdd = reactive({
  visible: false,
  loading: false,
  core_type: CoreTypes.SingBox,
  protocol: 'mixed',
  tag: '',
  count: 1,
  port: RandomUtil.randomIntRange(10000, 60000),
  method: '2022-blake3-aes-256-gcm',
  obfs_password: '',
  naive: createNaiveQuickAddOptions(''),
})

const connectionHost = computed(() => {
  if (node.value?.public_host) return node.value.public_host
  const remote = String(node.value?.remote_ip || '').replace(/^\[|\]$/g, '')
  if (remote) return remote
  return node.value?.report?.ipv4?.[0] || node.value?.report?.ipv6?.[0] || ''
})
const capabilities = computed(() => new Set<string>(node.value?.report?.panel?.capabilities || []))
const supportsQuickAdd = computed(() => Boolean(node.value?.managed) && capabilities.value.has('inbounds.quick_add.v1'))
const supportsNaiveQuickAdd = computed(() => capabilities.value.has('inbounds.quick_add.naive.v1'))
const supportsRelay = computed(() => Boolean(node.value?.managed) && capabilities.value.has('relay.v1'))
const xrayAvailable = computed(() => Boolean(node.value?.report?.cores?.xray_version))
const coreOptions = computed(() => {
  const values = [{ title: 'sing-box', value: CoreTypes.SingBox }]
  if (xrayAvailable.value) values.push({ title: 'Xray-core', value: CoreTypes.Xray })
  return values
})
const singBoxProtocolItems = [
  { title: 'Mixed', value: 'mixed' }, { title: 'SOCKS', value: 'socks' }, { title: 'HTTP', value: 'http' },
  { title: 'Shadowsocks', value: 'shadowsocks' }, { title: 'VMess', value: 'vmess' }, { title: 'Trojan', value: 'trojan' },
  { title: 'VLESS', value: 'vless' }, { title: 'Hysteria2', value: 'hysteria2' },
  { title: 'TUIC', value: 'tuic' }, { title: 'Naive', value: 'naive' }, { title: 'AnyTLS', value: 'anytls' },
  { title: 'Direct', value: 'direct' },
]
const xrayProtocolOptions = [
  { title: 'VLESS', value: 'vless' }, { title: 'VMess', value: 'vmess' }, { title: 'Trojan', value: 'trojan' },
  { title: 'Shadowsocks', value: 'shadowsocks' }, { title: 'SOCKS', value: 'socks' }, { title: 'HTTP', value: 'http' },
  { title: 'Mixed', value: 'mixed' }, { title: 'Hysteria2', value: 'hysteria2' }, { title: 'Dokodemo-door', value: 'dokodemo-door' },
]
const protocolOptions = computed(() => quickAdd.core_type === CoreTypes.Xray
  ? xrayProtocolOptions
  : singBoxProtocolItems.filter(item => item.value !== 'naive' || supportsNaiveQuickAdd.value))
const shadowsocksMethods = [
  'aes-128-gcm', 'aes-192-gcm', 'aes-256-gcm', 'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305',
]
const inTags = computed(() => inbounds.value.map(item => item.tag))
const filtered = computed(() => {
  const value = query.value.trim().toLocaleLowerCase()
  if (!value) return inbounds.value
  return inbounds.value.filter(item => [item.tag, item.type, item.core_type, item.listen, item.listen_port]
    .some(field => String(field ?? '').toLocaleLowerCase().includes(value)))
})
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const visible = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch(query, () => { page.value = 1 })
watch(pages, value => { if (page.value > value) page.value = value })
watch(() => quickAdd.core_type, (core) => {
  if (core === CoreTypes.Xray && !xrayProtocolOptions.some(item => item.value === quickAdd.protocol)) quickAdd.protocol = 'vless'
  if (core === CoreTypes.Xray && quickAdd.protocol === 'shadowsocks') quickAdd.method = '2022-blake3-aes-256-gcm'
})
watch(() => quickAdd.protocol, (protocol) => {
  if (!protocolOptions.value.some(item => item.value === protocol)) quickAdd.protocol = protocolOptions.value[0].value
})

const loadAll = async () => {
  if (!Number.isInteger(nodeId) || nodeId <= 0 || loading.value) return
  loading.value = true
  try {
    node.value = await api(`api/agents/${nodeId}`)
    if (node.value?.managed) {
      const result = await api(`api/agents/${nodeId}/inbounds`)
      inbounds.value = result?.inbounds || []
      tlsConfigs.value = result?.tls || []
      revision.value = Number(result?.revision || 0)
    } else {
      inbounds.value = []
    }
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.loadFailed') })
  } finally { loading.value = false }
}

const loadEditor = async (id: number) => {
  const result = await api(`api/agents/${nodeId}/inbounds/editor?id=${id || ''}`)
  revision.value = Number(result?.revision || 0)
  editorInbound.value = result?.inbound || null
  tlsConfigs.value = result?.tls || []
  dataSource.clients = result?.clients || []
  if (result?.inbounds) inbounds.value = result.inbounds
}

const dataSource = reactive<any>({
  clients: [],
  loadInbounds: async (ids: number[]) => {
    const id = Number(ids?.[0] || 0)
    if (!editorInbound.value || Number(editorInbound.value.id) !== id) await loadEditor(id)
    return editorInbound.value ? [JSON.parse(JSON.stringify(editorInbound.value))] : []
  },
  checkTag: (id: number, tag: string) => {
    const duplicate = inbounds.value.some(item => Number(item.id) !== Number(id) && item.tag === tag)
    if (duplicate) push.error({ message: `${i18n.global.t('error.dplData')}: ${i18n.global.t('objects.tag')}` })
    return duplicate
  },
  save: async (action: string, data: any, initUsers: number[]) => {
    try {
      const result = await api(`api/agents/${nodeId}/inbounds/save`, {
        method: 'POST',
        body: JSON.stringify({ action, data, init_users: initUsers, expected_revision: revision.value }),
      })
      revision.value = Number(result?.revision || revision.value)
      await loadAll()
      return true
    } catch (error: any) {
      const message = error?.message || i18n.global.t('failed')
      push.error({ message })
      if (message.includes('reload before saving')) await loadAll()
      return false
    }
  },
})

const openModal = async (id: number) => {
  try {
    await loadEditor(id)
    modal.id = id
    modal.visible = true
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.loadFailed') })
  }
}
const regenerateQuickAdd = () => {
  quickAdd.port = RandomUtil.randomIntRange(10000, 60000)
  quickAdd.tag = `${quickAdd.protocol}-${quickAdd.port}`
  quickAdd.obfs_password = RandomUtil.randomShadowsocksPassword(16)
  if (quickAdd.protocol === 'naive') {
    quickAdd.naive.username = 'naive-' + RandomUtil.randomSeq(6)
    quickAdd.naive.password = RandomUtil.randomShadowsocksPassword(32)
    quickAdd.naive.server = connectionHost.value
  }
}
const openQuickAdd = () => {
  if (!supportsQuickAdd.value) return
  quickAdd.count = 1
  regenerateQuickAdd()
  quickAdd.visible = true
}
const createQuickNodes = async () => {
  if (quickAdd.loading) return
  quickAdd.count = Math.min(100, Math.max(1, Math.floor(Number(quickAdd.count) || 1)))
  const port = Math.floor(Number(quickAdd.port))
  if (port < 1 || port > 65535) {
    push.error({ message: `${i18n.global.t('in.port')}: 1-65535` })
    return
  }
  let naiveExtraHeaders: Record<string, string> = {}
  if (quickAdd.protocol === 'naive') {
    try {
      quickAdd.naive.server = normalizeNaiveServer(quickAdd.naive.server || connectionHost.value)
      naiveExtraHeaders = parseNaiveExtraHeaders(quickAdd.naive.extra_headers_text)
    } catch {
      push.error({ message: i18n.global.t('types.naive.invalidOptions') })
      return
    }
    if (!quickAdd.naive.username.trim() || !quickAdd.naive.password) {
      push.error({ message: i18n.global.t('types.naive.identityRequired') })
      return
    }
  }
  quickAdd.loading = true
  try {
    const result = await api(`api/agents/${nodeId}/inbounds/quick-add`, {
      method: 'POST',
      body: JSON.stringify({
        core_type: quickAdd.core_type,
        protocol: quickAdd.protocol,
        tag: quickAdd.tag,
        count: quickAdd.count,
        port,
        method: quickAdd.method,
        obfs_password: quickAdd.obfs_password,
        password: quickAdd.protocol === 'naive' ? quickAdd.naive.password : undefined,
        naive_username: quickAdd.naive.username,
        naive_server: quickAdd.naive.server,
        naive_mode: quickAdd.naive.mode,
        naive_tls_id: quickAdd.naive.tls_id,
        naive_extra_headers: naiveExtraHeaders,
        naive_udp_over_tcp: quickAdd.naive.udp_over_tcp,
        naive_insecure_concurrency: quickAdd.naive.insecure_concurrency,
        naive_quic_congestion_control: quickAdd.naive.quic_congestion_control,
        expected_revision: revision.value,
      }),
    })
    revision.value = Number(result?.revision || revision.value)
    quickAdd.visible = false
    push.success({ message: i18n.global.t('agent.remoteQuickAddCreated', { count: result?.created?.length || quickAdd.count }) })
    await loadAll()
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('failed') })
    await loadAll()
  } finally {
    quickAdd.loading = false
  }
}
const closeModal = () => { modal.visible = false; editorInbound.value = null }
const askDelete = (item: any) => Object.assign(remove, { visible: true, loading: false, item })
const deleteInbound = async () => {
  if (!remove.item) return
  remove.loading = true
  const ok = await dataSource.save('del', remove.item.tag, [])
  if (ok) remove.visible = false
  remove.loading = false
}

void loadAll()
</script>

<style scoped>
.remote-quick-dialog > :deep(.v-card-title) { justify-content: center; text-align: center; }
.remote-header { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto; gap: 12px; align-items: center; margin-bottom: 18px; }
.remote-title { text-align: center; min-width: 0; }
.remote-title h1 { font-size: 1.25rem; margin: 0; }
.remote-title div { opacity: .7; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.remote-actions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.list-controls { display: flex; gap: 16px; align-items: center; margin-bottom: 14px; }
.list-controls .v-input { max-width: 420px; }
.remote-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 12px; }
.remote-item { border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; overflow: hidden; background: rgb(var(--v-theme-surface)); }
.remote-item > header { display: flex; justify-content: space-between; gap: 10px; padding: 13px 14px 10px; }
.remote-item header div { min-width: 0; }
.remote-item strong, .remote-item small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.remote-item small { opacity: .65; margin-top: 3px; }
.remote-item dl { margin: 0; padding: 0 14px 10px; }
.remote-item dl div { display: flex; justify-content: space-between; gap: 12px; padding: 5px 0; border-top: 1px solid rgba(var(--v-border-color), .08); }
.remote-item dt { opacity: .65; }
.remote-item dd { margin: 0; overflow: hidden; text-overflow: ellipsis; }
.remote-item footer { display: flex; justify-content: center; border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); min-height: 42px; }
@media (max-width: 700px) {
  .remote-header { grid-template-columns: 40px 1fr; }
  .remote-actions { grid-column: 1 / -1; justify-content: center; }
  .list-controls { align-items: stretch; flex-direction: column; gap: 8px; }
  .list-controls .v-input { max-width: none; }
}
</style>
