<template>
  <AdminModal 
    v-model="editModal.visible"
    :visible="editModal.visible"
    :user="editModal.user"
    @close="closeEditModal"
    @save="saveEditModal"
  />
  <ChangeModal 
    v-model="changesModal.visible"
    :visible="changesModal.visible"
    :admins="users.map((u:any) => u.username)"
    :actor="changesModal.actor"
    @close="closeChangesModal"
  />
  <TokenModal 
    v-model="tokenModal.visible"
    :visible="tokenModal.visible"
    @close="closeTokenModal"
  />
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-history" @click="showChangesModal('')">{{ $t('admin.changes') }}</v-btn>
      <v-btn color="primary" variant="outlined" prepend-icon="mdi-key-variant" @click="showTokenModal()">{{ $t('admin.api.token') }}</v-btn>
    </div>
  </div>
  <v-data-table
    class="list-table"
    :headers="headers"
    :items="users"
    item-value="id"
    :items-per-page="-1"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hide-default-footer
    hover
  >
    <template #item.username="{ item }">
      <span class="list-main">{{ item.username }}</span>
    </template>
    <template #item.lastLogin="{ item }">
      {{ item.loginDate == '-' ? '-' : `${item.loginDate} ${item.loginTime}` }}
    </template>
    <template #item.ip="{ item }">
      <span class="list-mono">{{ item.ip }}</span>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showEditModal(item)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="primary" @click="showChangesModal(item.username)">{{ $t('admin.changes') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import AdminModal from '@/layouts/modals/Admin.vue'
import ChangeModal  from '@/layouts/modals/Changes.vue'
import TokenModal from '@/layouts/modals/Token.vue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { Ref, computed, ref, inject, onMounted } from 'vue'
import { useDisplay } from 'vuetify'

const loading:Ref = inject('loading')?? ref(false)

const users = ref(<any[]>[])
const { smAndDown } = useDisplay()
const headers = computed(() => [
  { title: i18n.global.t('login.username'), key: 'username' },
  { title: i18n.global.t('admin.lastLogin'), key: 'lastLogin', sortable: false },
  { title: 'IP', key: 'ip', sortable: false },
  { title: i18n.global.t('list.actions'), key: 'actions', sortable: false },
])

onMounted(async () => {
  loading.value = true
  await loadData()
  loading.value = false
})

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/users')
  loading.value = false
  if (msg.success) {
    msg.obj.forEach((u:any) => {
      const lastLogin = u.lastLogin.split(" ")
      const localLastLogin = lastLogin.length > 2 ? dateFormatted(Date.parse(lastLogin[0] + " " + lastLogin[1])) : "- -"
      const loginDateTime = localLastLogin.split(" ")
      users.value.push({
        id: u.id,
        username: u.username,
        loginDate: loginDateTime[0],
        loginTime: loginDateTime[1],
        ip: lastLogin[2]?? "-",
      })
    })
  }
}

const dateFormatted = (dt: number): string => {
  const locale = i18n.global.locale.value.replace('zh', 'zh-')
  const date = new Date(dt)
  return date.toLocaleString(locale)
}

const editModal = ref({
  visible: false,
  user: {},
})

const showEditModal = (user: any) => {
  editModal.value.user = user
  editModal.value.visible = true
}
const closeEditModal = () => {
  editModal.value.visible = false
  editModal.value.user = {}
}
const saveEditModal = async (data:any) => {
  loading.value=true
  const response = await HttpUtils.post('api/changePass',data)
  if(response.success){
    setTimeout(() => {
      loading.value=false
      editModal.value.visible = false
    }, 500)
  } else {
    loading.value=false
  }
}

const changesModal = ref({
  visible: false,
  actor: '',
})
const showChangesModal = (actor: string) => {
  changesModal.value.actor = actor
  changesModal.value.visible = true
}
const closeChangesModal = () => {
  changesModal.value.visible = false
  changesModal.value.actor = ''
}

const tokenModal = ref({
  visible: false,
})
const showTokenModal = () => {
  tokenModal.value.visible = true
}
const closeTokenModal = () => {
  tokenModal.value.visible = false
}
</script>