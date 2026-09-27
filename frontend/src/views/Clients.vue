
<template>
  <ClientModal 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :groups="groups"
    :inboundTags="inboundTags"
    @close="closeModal"
  />
  <ClientAddBulk 
    v-model="addBulkModal"
    :visible="addBulkModal"
    :groups="groups"
    :inboundTags="inboundTags"
    @close="closeAddBulk"
  />
  <ClientEditBulk 
    v-model="editBulkModal"
    :visible="editBulkModal"
    :inboundTags="inboundTags"
    :clients="clients"
    @close="closeEditBulk"
  />
  <QrCode
    v-model="qrcode.visible"
    :visible="qrcode.visible"
    :id="qrcode.id"
    @close="closeQrCode"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <v-dialog v-model="resetTrafficModal" width="min(420px, calc(100vw - 24px))">
    <v-card rounded="lg" :title="$t('actions.resetTraffic')">
      <v-divider></v-divider>
      <v-card-text>{{ $t('confirm') }}</v-card-text>
      <v-card-actions class="justify-end">
        <v-btn color="secondary" variant="outlined" :disabled="resetTrafficLoading" @click="resetTrafficModal = false">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="resetTrafficLoading" @click="resetTraffic">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <DeleteConfirm v-model="deleteDialog.visible" :target="deleteDialog.name" :loading="deleteDialog.loading" @confirm="confirmDelete" />
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
      <v-btn variant="outlined" prepend-icon="mdi-account-multiple-plus" @click="addBulk">{{ $t('actions.addbulk') }}</v-btn>
      <v-btn variant="outlined" prepend-icon="mdi-account-multiple-check" @click="editBulk">{{ $t('actions.editbulk') }}</v-btn>
      <v-btn variant="outlined" color="warning" prepend-icon="mdi-restore" @click="confirmResetTraffic">{{ $t('actions.resetTraffic') }}</v-btn>
    </div>
    <div class="list-toolbar__filters">
      <v-select
        v-model="stateFilter"
        class="list-toolbar__select"
        :items="filterItems"
        prepend-inner-icon="mdi-list-status"
        density="compact"
        hide-details
      />
      <v-select
        v-if="groups.length > 1"
        v-model="groupFilter"
        class="list-toolbar__select"
        :items="groupItems"
        prepend-inner-icon="mdi-folder-account-outline"
        density="compact"
        hide-details
      />
      <v-text-field v-model="query" class="list-toolbar__search" :placeholder="$t('list.search')" prepend-inner-icon="mdi-magnify" clearable density="compact" hide-details />
    </div>
  </div>
  <v-data-table
    class="list-table"
    :headers="headers"
    :items="filteredClients"
    item-value="id"
    v-model:items-per-page="itemsPerPage"
    :items-per-page-options="pageSizeItems"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hover
  >
    <template v-slot:item.name="{ item }">
      <div class="list-main" :class="{ 'text-disabled': !item.enable }">{{ item.name }}</div>
      <div v-if="item.desc" class="list-sub">{{ item.desc }}</div>
    </template>
    <template v-slot:item.group="{ item }">
      {{ item.group || '-' }}
    </template>
    <template v-slot:item.inbounds="{ item }">
      <span :title="inboundNames(item.inbounds) || undefined">{{ item.inbounds?.length ?? 0 }}</span>
    </template>
    <template v-slot:item.volume="{ item }">
      <div class="list-usage" :title="'↓' + HumanReadable.sizeFormat(item.down) + ' - ' + HumanReadable.sizeFormat(item.up) + '↑'">
        <span :class="item.volume > 0 && item.volume <= (item.up + item.down) ? 'text-error' : ''">
          {{ HumanReadable.sizeFormat(item.up + item.down) + ' / ' + (item.volume == 0 ? $t('unlimited') : HumanReadable.sizeFormat(item.volume)) }}
        </span>
        <v-progress-linear
          v-if="item.volume > 0"
          :model-value="percent(item)"
          :color="percentColor(item)"
          height="4"
          rounded
        />
      </div>
    </template>
    <template v-slot:item.expiry="{ item }">
      <span :class="item.expiry > 0 && item.expiry <= Date.now() / 1000 ? 'text-error' : ''"
        :title="item.expiry > 0 ? new Date(item.expiry * 1000).toLocaleString(locale) : undefined">{{ HumanReadable.remainedDays(item.expiry) }}</span>
    </template>
    <template v-slot:item.online="{ item }">
      <span class="list-status" :class="{ 'list-status--on': isOnline(item.name), 'list-status--off': !item.enable }">
        {{ !item.enable ? $t('disable') : isOnline(item.name) ? $t('online') : $t('list.idle') }}
      </span>
    </template>
    <template v-slot:item.createdAt="{ item }">
      <span v-if="item.createdAt > 0" :title="new Date(item.createdAt * 1000).toLocaleString(locale)">{{ new Date(item.createdAt * 1000).toLocaleDateString(locale) }}</span>
      <span v-else>-</span>
    </template>
    <template v-slot:item.onlineAt="{ item }">
      <span v-if="item.onlineAt > 0">{{ new Date(item.onlineAt * 1000).toLocaleString(locale) }}</span>
      <span v-else>-</span>
    </template>
    <template v-slot:item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showModal(item.id)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="primary" @click="showQrCode(item.id)">{{ $t('list.qrcode') }}</v-btn>
        <v-btn v-if="Data().enableTraffic" size="small" variant="text" color="primary" @click="showStats(item.name)">{{ $t('list.traffic') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete(item)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>
<style>
.v-data-table__tr--mobile td {
  height: fit-content;
  min-height: 36px !important;
}
.v-data-table__tr--mobile td div {
  width: auto;
  min-width: 0;
  max-width: 100%;
  white-space: normal;
  overflow-wrap: anywhere;
}
</style>
<script lang="ts" setup>
import Data from '@/store/modules/data'
import ClientModal from '@/layouts/modals/Client.vue'
import ClientAddBulk from '@/layouts/modals/ClientAddBulk.vue'
import ClientEditBulk from '@/layouts/modals/ClientEditBulk.vue'
import QrCode from '@/layouts/modals/QrCode.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Client } from '@/types/clients'
import { computed, ref } from 'vue'
import { HumanReadable } from '@/plugins/utils'
import { i18n, locale } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import { matchesQuery, useListTable } from '@/utils/listTable'

const { smAndDown, itemsPerPage, pageSizeItems } = useListTable('items-per-page')

const clients = computed((): any[] => {
  return Data().clients
})

const isOnline = (cname: string): boolean => {
  return Data().onlines?.user ? Data().onlines.user.includes(cname) : false
}

const inbounds = computed((): any[] => {
  return Data().inbounds?? []
})

const inboundTags = computed((): any[] => {
  if (!inbounds.value) return []
  return inbounds.value?.filter(i => i.tag != "" && i.users).map(i => { return { title: i.tag, value: i.id } })
})

const inboundNames = (ids: number[] | undefined) => {
  return (ids ?? []).map(id => inbounds.value.find(inb => inb.id == id)?.tag).filter(Boolean).join('\n')
}

const groups = computed((): string[] => {
  if (!clients.value) return []
  return Array.from(new Set(clients.value?.map(c => c.group)))
})

const query = ref('')
const stateFilter = ref('')
const groupFilter = ref('-')
const groupItems = computed(() => [
  { title: i18n.global.t('all'), value: '-' },
  ...groups.value.map(g => ({ title: g?.length > 0 ? g : i18n.global.t('none'), value: g })),
])

const filteredClients = computed(() => clients.value.filter(c => {
  if (groupFilter.value != '-' && c.group != groupFilter.value) return false
  if (!matchesQuery(query.value, c.name, c.desc, c.group)) return false
  switch (stateFilter.value) {
    case 'disable': return c.enable == false
    case 'expired': return c.expiry > 0 && c.expiry < Date.now() / 1000
    case 'online': return isOnline(c.name)
  }
  return true
}))

const filterItems = [
  { title: i18n.global.t('all'), value: '' },
  { title: i18n.global.t('disable'), value: 'disable' },
  { title: i18n.global.t('date.expired'), value: 'expired' },
  { title: i18n.global.t('online'), value: 'online' },
]

const headers = [
  { title: i18n.global.t('client.name'), key: 'name' },
  { title: i18n.global.t('client.group'), key: 'group' },
  { title: i18n.global.t('pages.inbounds'), key: 'inbounds', value: (item: any) => item.inbounds?.length ?? 0 },
  { title: i18n.global.t('stats.volume'), key: 'volume', value: (item: any) => item.up + item.down },
  { title: i18n.global.t('date.expiry'), key: 'expiry' },
  { title: i18n.global.t('list.status'), key: 'online', value: (item: any) => item.enable ? (isOnline(item.name) ? 2 : 1) : 0 },
  { title: i18n.global.t('date.created'), key: 'createdAt' },
  { title: i18n.global.t('date.lastOnline'), key: 'onlineAt' },
  { title: i18n.global.t('list.actions'), key: 'actions', sortable: false },
]

const modal = ref({
  visible: false,
  id: 0,
})


const showModal = async (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}

const deleteDialog = ref({ visible: false, loading: false, id: 0, name: '' })
const requestDelete = (client: any) => {
  deleteDialog.value = { visible: true, loading: false, id: client.id, name: client.name }
}
const confirmDelete = async () => {
  deleteDialog.value.loading = true
  try {
    if (await Data().save("clients", "del", deleteDialog.value.id)) deleteDialog.value.visible = false
  } finally {
    deleteDialog.value.loading = false
  }
}

const qrcode = ref({
  visible: false,
  id: 0,
})

const showQrCode = (id: number) => {
  qrcode.value.id = id
  qrcode.value.visible = true
}
const closeQrCode = () => {
  qrcode.value.visible = false
}

const stats = ref({
  visible: false,
  resource: "user",
  tag: "",
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}

const addBulkModal = ref(false)

const addBulk = () => {
  addBulkModal.value = true
}

const closeAddBulk = () => {
  addBulkModal.value = false
}

const editBulkModal = ref(false)

const editBulk = () => {
  editBulkModal.value = true
}

const closeEditBulk = () => {
  editBulkModal.value = false
}

const resetTrafficModal = ref(false)
const resetTrafficLoading = ref(false)

const confirmResetTraffic = () => {
  resetTrafficModal.value = true
}

const resetTraffic = async () => {
  resetTrafficLoading.value = true
  const msg = await HttpUtils.post('api/resetTraffic', {})
  resetTrafficLoading.value = false
  if (msg.success) {
    resetTrafficModal.value = false
    await Data().loadData()
  }
}

const percent = (c: Client) => { return c.volume>0 ? Math.round((c.up+c.down) *100 / c.volume) : 0 }
const percentColor = (c: Client) => { return (c.up+c.down) >= c.volume ? 'error' : percent(c)>90 ? 'warning' : 'success' }

</script>
