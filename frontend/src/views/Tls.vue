<template>
    <TlsVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    @close="closeModal"
    @save="saveModal"
  />
  <DeleteConfirm v-model="deleteDialog.visible" :target="deleteDialog.name" :loading="deleteDialog.loading" @confirm="confirmDelete" />
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </div>
    <v-text-field v-model="query" class="list-toolbar__search" :placeholder="$t('list.search')" prepend-inner-icon="mdi-magnify" clearable density="compact" hide-details />
  </div>
  <v-data-table
    class="list-table"
    :headers="headers"
    :items="filteredConfigs"
    item-value="id"
    v-model:items-per-page="itemsPerPage"
    :items-per-page-options="pageSizeItems"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hover
  >
    <template #item.name="{ item }">
      <span class="list-main">{{ item.name }}</span>
    </template>
    <template #item.sni="{ item }">
      <span class="list-mono">{{ item.server?.server_name?.length > 0 ? item.server.server_name : '-' }}</span>
    </template>
    <template #item.inbounds="{ item }">
      <span :title="tlsInbounds(item.id).join('\n') || undefined">{{ tlsInbounds(item.id).length || '-' }}</span>
    </template>
    <template #item.features="{ item }">
      <div class="d-flex flex-wrap ga-1">
        <v-chip v-if="item.server?.reality?.enabled" size="small" label variant="tonal" color="primary">REALITY</v-chip>
        <v-chip v-if="!isOpenWrtLite && item.server?.acme != undefined" size="small" label variant="tonal" color="success">ACME</v-chip>
        <v-chip v-if="item.server?.ech != undefined" size="small" label variant="tonal" color="info">ECH</v-chip>
        <span v-if="!item.server?.reality?.enabled && (isOpenWrtLite || item.server?.acme == undefined) && item.server?.ech == undefined">-</span>
      </div>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showModal(item.id)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="primary" @click="clone(item)">{{ $t('actions.clone') }}</v-btn>
        <v-btn v-if="tlsInbounds(item.id).length == 0" size="small" variant="text" color="error" @click="requestDelete(item)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import TlsVue from '@/layouts/modals/Tls.vue'
import Data from '@/store/modules/data'
import { computed, ref } from 'vue'
import { Inbound } from '@/types/inbounds'
import { tls } from '@/types/tls'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import { i18n } from '@/locales'
import { matchesQuery, useListTable } from '@/utils/listTable'

const isOpenWrtLite = import.meta.env.VITE_OPENWRT_LITE === 'true'

const tlsConfigs = computed((): any[] => {
  return Data().tlsConfigs
})

const inbounds = computed((): Inbound[] => {
  return Data().inbounds
})

const tlsInbounds = (id: number): string[] => {
  return inbounds.value.filter(i => i.tls_id == id).map(i => i.tag)  
}

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

const t = i18n.global.t
const query = ref('')
const { smAndDown, itemsPerPage, pageSizeItems } = useListTable()
const headers = computed(() => [
  { title: t('list.name'), key: 'name' },
  { title: 'SNI', key: 'sni', sortable: false },
  { title: t('pages.inbounds'), key: 'inbounds', sortable: false },
  { title: t('list.features'), key: 'features', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])
const filteredConfigs = computed(() => tlsConfigs.value.filter((item: any) =>
  matchesQuery(query.value, item.name, item.server?.server_name)))

const deleteDialog = ref({ visible: false, loading: false, id: 0, name: '' })
const requestDelete = (item: any) => {
  deleteDialog.value = { visible: true, loading: false, id: item.id, name: item.name }
}
const confirmDelete = async () => {
  deleteDialog.value.loading = true
  try {
    if (await Data().save('tls', 'del', deleteDialog.value.id)) deleteDialog.value.visible = false
  } finally {
    deleteDialog.value.loading = false
  }
}

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '{}' : JSON.stringify(tlsConfigs.value.findLast(t => t.id == id))
  modal.value.visible = true
}
const clone = (obj: any) => {
  let data = JSON.parse(JSON.stringify(obj))
  data.id = 0
  while (tlsConfigs.value.findIndex(t => t.name == data.name) != -1){
    data.name += "-copy"
  }
  saveModal(data)
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = async (data:tls) => {
  const success = await Data().save("tls", data.id > 0 ? "edit" : "new", data)
  if (success) modal.value.visible = false
}

</script>
