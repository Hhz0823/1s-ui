<template>
  <OutboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    :tags="outboundTags"
    @close="closeModal"
  />
  <OutboundBulk
    v-model="bulkModal.visible"
    :visible="bulkModal.visible"
    :outboundTags="outboundTags"
    @close="closeBulkModal"
  />
  <EndpointVue
    v-model="endpointModal.visible"
    :visible="endpointModal.visible"
    :id="endpointModal.id"
    :data="endpointModal.data"
    :tags="endpointTags"
    @close="closeEndpointModal"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <v-dialog v-model="deleteDialog.visible" width="min(420px, calc(100vw - 24px))">
    <v-card :title="$t('actions.del')" rounded="lg">
      <v-divider />
      <v-card-text>
        {{ $t('confirm') }}
        <div class="delete-target-tag">{{ deleteDialog.tag }}</div>
      </v-card-text>
      <v-card-actions class="justify-end">
        <v-btn color="secondary" variant="outlined" :disabled="deleteDialog.loading" @click="deleteDialog.visible = false">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="deleteDialog.loading" @click="confirmDelete">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
      <v-btn color="primary" variant="outlined" prepend-icon="mdi-playlist-plus" @click="showBulkModal">{{ $t('actions.addbulk') }}</v-btn>
      <v-btn variant="outlined" prepend-icon="mdi-cloud-sync" :loading="creatingWarp" :disabled="creatingWarp" @click="createWarpOutbound">
        {{ $t('actions.addWarp') }}
        <v-tooltip activator="parent" location="top" :text="$t('out.warpSafeTip')"></v-tooltip>
      </v-btn>
      <v-btn variant="outlined" prepend-icon="mdi-speedometer" :loading="testingAll" :disabled="testingAll || checkableTags.length === 0" @click="checkAllOutbounds">
        {{ $t('actions.testAll') }}
      </v-btn>
    </div>
    <v-text-field
      v-model="query"
      class="list-toolbar__search"
      :placeholder="$t('list.search')"
      prepend-inner-icon="mdi-magnify"
      clearable
      density="compact"
      hide-details
    />
  </div>
  <v-data-table
    class="list-table"
    :headers="headers"
    :items="filteredRows"
    item-value="key"
    v-model:items-per-page="itemsPerPage"
    :items-per-page-options="pageSizeItems"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hover
  >
    <template #item.tag="{ item }">
      <span class="list-main">{{ item.tag }}</span>
    </template>
    <template #item.type="{ item }">
      <v-chip size="small" label variant="tonal" :color="item.kind === 'warp' ? 'info' : 'primary'">{{ item.kind === 'warp' ? 'WARP' : item.type }}</v-chip>
    </template>
    <template #item.address="{ item }">
      <div class="list-mono">{{ item.address }}</div>
      <div v-if="item.kind === 'warp'" class="list-sub">{{ $t('types.wg.localIp') }}: {{ item.local }}</div>
      <div v-if="item.kind === 'warp' && formatWarpTrace(item.tag) !== '-'" class="list-sub">{{ $t('out.warpExit') }}: {{ formatWarpTrace(item.tag) }}</div>
    </template>
    <template #item.tls="{ item }">
      <v-chip v-if="item.tls" size="small" label variant="tonal" :color="item.tls === 'on' ? 'success' : undefined">{{ item.tls === 'on' ? $t('enable') : $t('disable') }}</v-chip>
      <span v-else>-</span>
    </template>
    <template #item.online="{ item }">
      <span class="list-status" :class="{ 'list-status--on': item.online }">{{ item.online ? $t('online') : $t('list.idle') }}</span>
    </template>
    <template #item.delay="{ item }">
      <div class="list-delay">
        <v-progress-circular v-if="checkResults[item.tag]?.loading" indeterminate size="16" width="2" />
        <template v-else-if="checkResults[item.tag]">
          <span v-if="checkResults[item.tag].success" class="text-success">{{ checkResults[item.tag].data?.Delay }} {{ $t('date.ms') }}</span>
          <span v-else class="text-error" :title="checkResults[item.tag].errorMessage || $t('failed')">{{ $t('failed') }}</span>
        </template>
        <v-btn
          size="small"
          variant="text"
          icon="mdi-speedometer"
          :title="item.kind === 'warp' ? $t('out.warpTrace') : $t('actions.test')"
          :disabled="checkResults[item.tag]?.loading"
          @click="item.kind === 'warp' ? checkWarpEndpoint(item.tag) : checkOutbound(item.tag)"
        />
      </div>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="item.kind === 'warp' ? showEndpointModal(item.id) : showModal(item.id)">{{ $t('actions.edit') }}</v-btn>
        <v-btn v-if="Data().enableTraffic" size="small" variant="text" color="primary" @click="showStats(item.tag, item.kind === 'warp' ? 'endpoint' : 'outbound')">{{ $t('list.traffic') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete(item)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import RandomUtil from '@/plugins/randomUtil'
import OutboundVue from '@/layouts/modals/Outbound.vue'
import OutboundBulk from '@/layouts/modals/OutboundBulk.vue'
import EndpointVue from '@/layouts/modals/Endpoint.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Outbound } from '@/types/outbounds'
import { Endpoint, EpTypes, createEndpoint } from '@/types/endpoints'
import { computed, ref } from 'vue'
import { useDisplay } from 'vuetify'
import { i18n } from '@/locales'

interface CheckResult {
  loading?: boolean
  success: boolean
  data?: { OK?: boolean; Delay?: number; Error?: string; IP?: string; Colo?: string; Loc?: string; Warp?: string } | null
  errorMessage?: string
}

const checkResults = ref<Record<string, CheckResult>>({})

const checkOutbound = async (tag: string) => {
  checkResults.value = { ...checkResults.value, [tag]: { loading: true, success: false } }
  const msg = await HttpUtils.get('api/checkOutbound', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  checkResults.value = {
    ...checkResults.value,
    [tag]: { loading: false, success, data: msg.obj ?? null, errorMessage }
  }
}

const checkWarpEndpoint = async (tag: string) => {
  checkResults.value = { ...checkResults.value, [tag]: { loading: true, success: false } }
  const msg = await HttpUtils.get('api/checkWarp', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  checkResults.value = {
    ...checkResults.value,
    [tag]: { loading: false, success, data: msg.obj ?? null, errorMessage }
  }
}

const testingAll = ref(false)

const checkableTags = computed(() => {
  return [
    ...outbounds.value.map((o) => o.tag),
    ...warpEndpoints.value.map((e: any) => e.tag),
  ].filter(Boolean)
})

const checkAllOutbounds = async () => {
  const tags = checkableTags.value
  if (tags.length === 0) return
  testingAll.value = true
  try {
    await Promise.all([
      ...outbounds.value.map((item) => checkOutbound(item.tag)),
      ...warpEndpoints.value.map((item: any) => checkWarpEndpoint(item.tag)),
    ])
  } finally {
    testingAll.value = false
  }
}

const outbounds = computed((): Outbound[] => {
  return <Outbound[]> Data().outbounds
})

const endpoints = computed((): Endpoint[] => {
  return <Endpoint[]> Data().endpoints
})

const warpEndpoints = computed((): Endpoint[] => {
  return endpoints.value.filter((e: any) => e.type === EpTypes.Warp)
})

const endpointTags = computed((): string[] => {
  return endpoints.value?.map((e: any) => e.tag) ?? []
})

const outboundTags = computed((): string[] => {
  return [...outbounds.value?.map((o: Outbound) => o.tag), ...endpointTags.value]
})

const onlines = computed(() => {
  return Data().onlines.outbound?? []
})

const endpointOnlines = computed(() => {
  return [...(Data().onlines.inbound ?? []), ...(Data().onlines.outbound ?? [])]
})

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

const t = i18n.global.t
const { smAndDown } = useDisplay()
const query = ref('')
const itemsPerPage = ref(20)
const pageSizeItems = computed(() => [
  ...[20, 40, 80].map(value => ({ value, title: String(value) })),
  { value: -1, title: t('list.all') },
])
const headers = computed(() => [
  { title: t('objects.tag'), key: 'tag' },
  { title: t('list.type'), key: 'type' },
  { title: t('list.server'), key: 'address', sortable: false },
  { title: 'TLS', key: 'tls', sortable: false },
  { title: t('list.status'), key: 'online', sortable: false },
  { title: t('out.delay'), key: 'delay', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])

const hostPort = (host: unknown, port: unknown) => {
  if (!host) return '-'
  const text = String(host)
  return (text.includes(':') ? `[${text}]` : text) + (port ? `:${port}` : '')
}

// Outbounds and WARP endpoints share one table.
const rows = computed(() => [
  ...outbounds.value.map((item: any) => ({
    key: 'out-' + item.tag,
    kind: 'outbound',
    id: item.id,
    tag: item.tag,
    type: item.type,
    address: hostPort(item.server, item.server_port),
    local: '',
    tls: Object.hasOwn(item, 'tls') ? (item.tls?.enabled ? 'on' : 'off') : '',
    online: onlines.value.includes(item.tag),
  })),
  ...warpEndpoints.value.map((item: any) => ({
    key: 'warp-' + item.tag,
    kind: 'warp',
    id: item.id,
    tag: item.tag,
    type: 'warp',
    address: formatEndpointPeer(item),
    local: formatEndpointAddress(item),
    tls: '',
    online: endpointOnlines.value.includes(item.tag),
  })),
])

const filteredRows = computed(() => {
  const value = (query.value || '').trim().toLocaleLowerCase()
  if (!value) return rows.value
  return rows.value.filter(row => [row.tag, row.type, row.address]
    .some(field => String(field ?? '').toLocaleLowerCase().includes(value)))
})

const deleteDialog = ref({ visible: false, loading: false, kind: 'outbound', tag: '' })
const requestDelete = (row: any) => {
  deleteDialog.value = { visible: true, loading: false, kind: row.kind, tag: row.tag }
}
const confirmDelete = async () => {
  deleteDialog.value.loading = true
  try {
    const object = deleteDialog.value.kind === 'warp' ? 'endpoints' : 'outbounds'
    if (await Data().save(object, 'del', deleteDialog.value.tag)) deleteDialog.value.visible = false
  } finally {
    deleteDialog.value.loading = false
  }
}

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(outbounds.value.findLast(o => o.id == id))
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}

const bulkModal = ref({ visible: false })

const showBulkModal = () => {
  bulkModal.value.visible = true
}

const closeBulkModal = () => {
  bulkModal.value.visible = false
}

const endpointModal = ref({
  visible: false,
  id: 0,
  data: "",
})

const showEndpointModal = (id: number) => {
  endpointModal.value.id = id
  endpointModal.value.data = id == 0 ? '' : JSON.stringify(warpEndpoints.value.findLast((e: any) => e.id == id))
  endpointModal.value.visible = true
}

const closeEndpointModal = () => {
  endpointModal.value.visible = false
}

const creatingWarp = ref(false)

const nextWarpTag = () => {
  let tag = `warp-${RandomUtil.randomSeq(4)}`
  let attempts = 0
  while (outboundTags.value.includes(tag) && attempts < 50) {
    tag = `warp-${RandomUtil.randomSeq(4)}`
    attempts++
  }
  return tag
}

const createWarpOutbound = async () => {
  creatingWarp.value = true
  try {
    const endpoint = createEndpoint(EpTypes.Warp, {
      tag: nextWarpTag(),
      listen_port: 0,
      system: false,
    })
    await Data().save("endpoints", "new", endpoint)
  } finally {
    creatingWarp.value = false
  }
}

const formatEndpointAddress = (endpoint: any) => {
  return endpoint.address?.length > 0 ? endpoint.address.join(', ') : '-'
}

const formatEndpointPeer = (endpoint: any) => {
  const peer = endpoint.peers?.[0]
  if (!peer) return '-'
  return `${peer.address ?? '-'}${peer.port ? ':' + peer.port : ''}`
}

const formatWarpTrace = (tag: string) => {
  const data = checkResults.value[tag]?.data
  if (!data?.IP) return '-'
  const region = [data.Loc, data.Colo].filter(Boolean).join(' / ')
  const warp = data.Warp ? ` · WARP ${data.Warp}` : ''
  return `${data.IP}${region ? ` · ${region}` : ''}${warp}`
}

const stats = ref({
  visible: false,
  resource: "outbound",
  tag: "",
})

const showStats = (tag: string, resource = "outbound") => {
  stats.value.resource = resource
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>
