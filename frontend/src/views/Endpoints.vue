<template>
  <EndpointVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    :tags="endpointTags"
    @close="closeModal"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <QrCode
    v-model="qrcode.visible"
    :visible="qrcode.visible"
    :data="qrcode.data"
    @close="closeQrCode"
  />
  <DeleteConfirm v-model="deleteDialog.visible" :target="deleteDialog.tag" :loading="deleteDialog.loading" @confirm="confirmDelete" />
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </div>
    <v-text-field v-model="query" class="list-toolbar__search" :placeholder="$t('list.search')" prepend-inner-icon="mdi-magnify" clearable density="compact" hide-details />
  </div>
  <v-data-table
    class="list-table"
    :headers="headers"
    :items="filteredEndpoints"
    item-value="tag"
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
      <v-chip size="small" label variant="tonal" color="primary">{{ item.type }}</v-chip>
    </template>
    <template #item.address="{ item }">
      <span class="list-mono">{{ item.address?.length > 0 ? item.address.join(', ') : '-' }}</span>
    </template>
    <template #item.listen_port="{ item }">
      {{ item.listen_port > 0 ? item.listen_port : '-' }}
    </template>
    <template #item.peers="{ item }">
      {{ item.peers?.length ?? '-' }}
    </template>
    <template #item.online="{ item }">
      <span class="list-status" :class="{ 'list-status--on': onlines.includes(item.tag) }">{{ onlines.includes(item.tag) ? $t('online') : $t('list.idle') }}</span>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showModal(item.id)">{{ $t('actions.edit') }}</v-btn>
        <v-btn v-if="item.type == 'wireguard' && item.peers?.length > 0" size="small" variant="text" color="primary" @click="showQrCode(item.id)">{{ $t('list.qrcode') }}</v-btn>
        <v-btn v-if="Data().enableTraffic" size="small" variant="text" color="primary" @click="showStats(item.tag)">{{ $t('list.traffic') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete(item.tag)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import EndpointVue from '@/layouts/modals/Endpoint.vue'
import Stats from '@/layouts/modals/Stats.vue'
import QrCode from '@/layouts/modals/WgQrCode.vue'
import { Endpoint } from '@/types/endpoints'
import { computed, ref } from 'vue'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import { i18n } from '@/locales'
import { matchesQuery, useListTable } from '@/utils/listTable'

const endpoints = computed((): Endpoint[] => {
  return <Endpoint[]> Data().endpoints
})

const endpointTags = computed((): any[] => {
  return endpoints.value?.map((o:Endpoint) => o.tag)
})

const onlines = computed(() => {
  return [...Data().onlines.inbound?? [], ...Data().onlines.outbound??[] ]
})

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

const t = i18n.global.t
const query = ref('')
const { smAndDown, itemsPerPage, pageSizeItems } = useListTable()
const headers = computed(() => [
  { title: t('objects.tag'), key: 'tag' },
  { title: t('list.type'), key: 'type' },
  { title: t('in.addr'), key: 'address', sortable: false },
  { title: t('in.port'), key: 'listen_port' },
  { title: t('types.wg.peers'), key: 'peers', sortable: false },
  { title: t('list.status'), key: 'online', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])
const filteredEndpoints = computed(() => endpoints.value.filter((item: any) =>
  matchesQuery(query.value, item.tag, item.type, item.address?.join(' '), item.listen_port)))

const deleteDialog = ref({ visible: false, loading: false, tag: '' })
const requestDelete = (tag: string) => {
  deleteDialog.value = { visible: true, loading: false, tag }
}
const confirmDelete = async () => {
  deleteDialog.value.loading = true
  try {
    if (await Data().save('endpoints', 'del', deleteDialog.value.tag)) deleteDialog.value.visible = false
  } finally {
    deleteDialog.value.loading = false
  }
}

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(endpoints.value.findLast(o => o.id == id))
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}

const stats = ref({
  visible: false,
  resource: "endpoint",
  tag: "",
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}

const qrcode = ref({
  visible: false,
  data: <any>{},
})

const showQrCode = (id: number) => {
  qrcode.value.data = endpoints.value.findLast(o => o.id == id)
  qrcode.value.visible = true
}
const closeQrCode = () => {
  qrcode.value.visible = false
}
</script>