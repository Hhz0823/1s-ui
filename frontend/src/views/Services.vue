<template>
  <ServiceVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    :inTags="inTags"
    :tsTags="tsTags"
    :ssTags="ssTags"
    :tlsConfigs="tlsConfigs"
    @close="closeModal"
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
    :items="filteredServices"
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
    <template #item.listen="{ item }">
      <span class="list-mono">{{ hostPort(item.listen, item.listen_port) }}</span>
    </template>
    <template #item.tls_id="{ item }">
      <v-chip size="small" label variant="tonal" :color="item.tls_id > 0 ? 'success' : undefined">{{ item.tls_id > 0 ? $t('enable') : $t('disable') }}</v-chip>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showModal(item.id)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete(item.tag)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { Srv } from '@/types/services'
import { computed, ref } from 'vue'
import ServiceVue from '@/layouts/modals/Service.vue'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import { i18n } from '@/locales'
import { hostPort, matchesQuery, useListTable } from '@/utils/listTable'

const services = computed((): Srv[] => {
  return <Srv[]> Data().services
})

const srvTags = computed((): any[] => {
  return services.value?.map((o:Srv) => o.tag)
})

const tsTags = computed((): any[] => {
  return Data().endpoints?.filter((o:any) => o.type == "tailscale")?.map((o:any) => o.tag)
})

const ssTags = computed((): any[] => {
  return Data().inbounds?.filter((o:any) => o.type == "shadowsocks" && !o.users)?.map((o:any) => o.tag)
})

const inTags = computed((): any[] => {
  return [...Data().inbounds?.map((o:any) => o.tag).filter(t => t != null), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

const tlsConfigs = computed((): any[] => {
  return <any[]> Data().tlsConfigs
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
  { title: t('list.listen'), key: 'listen', sortable: false },
  { title: 'TLS', key: 'tls_id' },
  { title: t('list.actions'), key: 'actions', sortable: false },
])
const filteredServices = computed(() => services.value.filter((item: any) =>
  matchesQuery(query.value, item.tag, item.type, item.listen, item.listen_port)))

const deleteDialog = ref({ visible: false, loading: false, tag: '' })
const requestDelete = (tag: string) => {
  deleteDialog.value = { visible: true, loading: false, tag }
}
const confirmDelete = async () => {
  deleteDialog.value.loading = true
  try {
    if (await Data().save('services', 'del', deleteDialog.value.tag)) deleteDialog.value.visible = false
  } finally {
    deleteDialog.value.loading = false
  }
}

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(services.value.findLast(o => o.id == id))
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}

</script>