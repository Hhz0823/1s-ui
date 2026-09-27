<template>
  <DnsVue
    v-model="dnsModal.visible"
    :visible="dnsModal.visible"
    :index="dnsModal.index"
    :data="dnsModal.data"
    :tsTags="tsTags"
    :rslvdTags="rslvdTags"
    @close="closeDnsModal"
    @save="saveDnsModal"
  />
  <DnsRuleVue
    v-model="dnsRuleModal.visible"
    :visible="dnsRuleModal.visible"
    :index="dnsRuleModal.index"
    :data="dnsRuleModal.data"
    :clients="clients"
    :inTags="inboundTags"
    :serverTags="dnsServerTags"
    :ruleSets="ruleSets"
    @close="closeDnsRuleModal"
    @save="saveDnsRuleModal"
  />
  <DeleteConfirm v-model="deleteDialog.visible" :target="deleteDialog.target" @confirm="confirmDelete" />
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showDnsModal(-1)">{{ $t('dns.add') }}</v-btn>
      <v-btn color="primary" variant="outlined" prepend-icon="mdi-plus" @click="showDnsRuleModal(-1)">{{ $t('dns.rule.add') }}</v-btn>
      <v-btn color="warning" :variant="unchanged ? 'outlined' : 'flat'" prepend-icon="mdi-content-save-outline" :loading="loading" :disabled="unchanged" @click="saveConfig">
        {{ $t('actions.save') }}
      </v-btn>
      <span v-if="!unchanged" class="list-unsaved">{{ $t('list.unsaved') }}</span>
    </div>
    <v-text-field v-model="query" class="list-toolbar__search" :placeholder="$t('list.search')" prepend-inner-icon="mdi-magnify" clearable density="compact" hide-details />
  </div>
  <section class="list-panel">
    <header class="list-panel__head">{{ $t('pages.basics') }}</header>
    <div class="list-panel__body">
      <v-row dense>
        <v-col cols="12" sm="6" lg="3">
          <v-select
            hide-details
            :label="$t('dns.final')"
            :items="[ {title: $t('dns.firstServer'), value: ''}, ...dnsServerTags]"
            v-model="finalDns">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" lg="3">
          <v-select
            hide-details
            :label="$t('dns.domainStrategy')"
            clearable
            @click:clear="delete dns.strategy"
            :items="['prefer_ipv4','prefer_ipv6','ipv4_only','ipv6_only']"
            v-model="dns.strategy">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" lg="3">
          <v-text-field
            v-model="dns.client_subnet" hide-details
            clearable @click:clear="delete dns.client_subnet"
            :label="$t('dns.rule.action.clientSubnet')"></v-text-field>
        </v-col>
        <v-col cols="12" sm="6" lg="3">
          <v-text-field
            v-model.number="dns.cache_capacity"
            type="number" min="1024" hide-details
            clearable @click:clear="delete dns.cache_capacity"
            :label="$t('dns.cacheCapacity')"></v-text-field>
        </v-col>
      </v-row>
      <div class="list-panel__checks">
        <v-checkbox v-model="dns.disable_cache" hide-details :label="$t('dns.disableCache')" />
        <v-checkbox v-model="dns.disable_expire" hide-details :label="$t('dns.disableExpire')" />
        <v-checkbox v-model="dns.independent_cache" hide-details :label="$t('dns.independentCache')" />
        <v-checkbox v-model="dns.reverse_mapping" hide-details :label="$t('dns.reverseMapping')" />
      </div>
    </div>
  </section>
  <div class="list-section-title">
    {{ $t('dns.title') }}
    <span class="list-count">{{ dns.servers.length }}</span>
  </div>
  <v-data-table
    class="list-table"
    :headers="serverHeaders"
    :items="serverRows"
    item-value="index"
    v-model:items-per-page="itemsPerPage"
    :items-per-page-options="pageSizeItems"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hover
  >
    <template #item.tag="{ item }">
      <span class="list-main">{{ item.server.tag }}</span>
      <v-chip v-if="item.server.tag && item.server.tag == dns.final" size="x-small" label variant="tonal" color="primary" class="ms-2">{{ $t('dns.final') }}</v-chip>
    </template>
    <template #item.type="{ item }">
      <v-chip size="small" label variant="tonal" color="primary">{{ item.server.type }}</v-chip>
    </template>
    <template #item.address="{ item }">
      <span class="list-mono">{{ hostPort(item.server.server, item.server.server_port) }}</span>
    </template>
    <template #item.tls="{ item }">
      <v-chip v-if="Object.hasOwn(item.server, 'tls')" size="small" label variant="tonal" :color="item.server.tls?.enabled ? 'success' : undefined">{{ $t(item.server.tls?.enabled ? 'enable' : 'disable') }}</v-chip>
      <span v-else>-</span>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showDnsModal(item.index)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete('server', item.index)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
  <div class="list-section-title">
    {{ $t('dns.rule.title') }}
    <span class="list-count">{{ dnsRules.length }}</span>
    <span class="list-hint">{{ $t('list.orderHint') }}</span>
  </div>
  <v-data-table
    class="list-table list-table--ordered"
    :headers="ruleHeaders"
    :items="ruleRows"
    item-value="index"
    :items-per-page="-1"
    :row-props="ruleRowProps"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hide-default-footer
    hover
  >
    <template #item.order="{ item }">
      <div class="list-order">
        <v-icon v-if="!smAndDown" class="list-order__handle" icon="mdi-drag-vertical" size="18" />
        <span class="list-order__number">{{ item.index + 1 }}</span>
        <v-btn icon="mdi-arrow-up" size="x-small" variant="text" :disabled="item.index == 0" :title="$t('list.moveUp')" @click="moveRule(item.index, item.index - 1)" />
        <v-btn icon="mdi-arrow-down" size="x-small" variant="text" :disabled="item.index == dnsRules.length - 1" :title="$t('list.moveDown')" @click="moveRule(item.index, item.index + 1)" />
      </div>
    </template>
    <template #item.type="{ item }">
      {{ item.rule.type != undefined ? $t('rule.logical') + ' (' + item.rule.mode + ')' : $t('rule.simple') }}
    </template>
    <template #item.conditions="{ item }">
      <span class="list-mono list-clip" :title="summary(item.rule) || undefined">{{ conditions(item.rule).join(', ') || '-' }}</span>
    </template>
    <template #item.action="{ item }">
      <v-chip size="small" label variant="tonal" :color="item.rule.action == 'reject' ? 'error' : 'primary'">{{ item.rule.action ?? '-' }}</v-chip>
    </template>
    <template #item.server="{ item }">
      <span class="list-main">{{ item.rule.server ?? '-' }}</span>
    </template>
    <template #item.invert="{ item }">
      <v-chip v-if="item.rule.invert" size="small" label variant="tonal" color="warning">{{ $t('yes') }}</v-chip>
      <span v-else>-</span>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showDnsRuleModal(item.index)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete('rule', item.index)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, onBeforeMount } from 'vue'
import DnsVue from '@/layouts/modals/Dns.vue'
import DnsRuleVue from '@/layouts/modals/DnsRule.vue'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import { Config } from '@/types/config'
import { actionDnsRuleKeys, dnsRule } from '@/types/dns'
import { FindDiff } from '@/plugins/utils'
import { i18n } from '@/locales'
import { hostPort, matchesQuery, useListTable, useRowReorder } from '@/utils/listTable'
import { conditionKeys, ruleSummary } from '@/utils/ruleSummary'

const oldConfig = ref(<any>{})
const loading = ref(false)

const appConfig = computed((): Config => {
  return <Config> Data().config
})

onBeforeMount( async () => {
  loading.value = true
  while (Data().lastLoad == 0) {
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
})

const tsTags = computed((): string[] => {
  return Data().endpoints?.filter((e:any) => e.type == "tailscale").map((e:any) => e.tag)
})

const rslvdTags = computed((): string[] => {
  return Data().services?.filter((e:any) => e.type == "resolved").map((e:any) => e.tag)
})

const clients = computed((): string[] => {
  return Data().clients.map((c:any) => c.name)
})

const unchanged = computed(() => {
  return FindDiff.deepCompare(appConfig.value.dns,oldConfig.value.dns)
})

const saveConfig = async () => {
  loading.value = true
  const success = await Data().save("config", "set", appConfig.value)
  if (success) {
    oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  }
  loading.value = false
}

const inboundTags = computed((): string[] => {
  return [...Data().inbounds?.map((o:any) => o.tag), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

// Old configs may lack the DNS block or its lists, also when the config
// arrives after the page is shown.
const dns = computed((): any => {
  const config = <any>appConfig.value
  if (!config.dns) config.dns = { servers: [], rules: [] }
  if (!Array.isArray(config.dns.servers)) config.dns.servers = []
  if (!Array.isArray(config.dns.rules)) config.dns.rules = []
  return config.dns
})

const dnsServerTags = computed((): string[] => {
  return dns.value?.servers?.filter((s:any) => s.tag && s.tag != "")?.map((s:any) => s.tag) ?? []
})

const finalDns = computed({
  get() { return dns.value?.final?? '' },
  set(v:string) { dns.value.final = v.length>0 ? v : undefined }
})


const dnsRules = computed((): dnsRule[] => {
  return <dnsRule[]>dns.value.rules
})

const ruleSets = computed((): string[] => {
  return appConfig.value?.route?.rule_set?.map((r:any) => r.tag) ?? []
})

const t = i18n.global.t
const query = ref('')
const { smAndDown, itemsPerPage, pageSizeItems } = useListTable()

const serverHeaders = computed(() => [
  { title: t('objects.tag'), key: 'tag', sortable: false },
  { title: t('list.type'), key: 'type', sortable: false },
  { title: t('dns.server'), key: 'address', sortable: false },
  { title: 'TLS', key: 'tls', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])
// DNS rules match top-down, so the table keeps their order instead of sorting.
const ruleHeaders = computed(() => [
  { title: '#', key: 'order', sortable: false, width: 132 },
  { title: t('list.type'), key: 'type', sortable: false },
  { title: t('list.conditions'), key: 'conditions', sortable: false },
  { title: t('list.ruleAction'), key: 'action', sortable: false },
  { title: t('dns.server'), key: 'server', sortable: false },
  { title: t('rule.invert'), key: 'invert', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])

const serverRows = computed((): { server: any, index: number }[] => dns.value.servers
  .map((server: any, index: number) => ({ server, index }))
  .filter((row: { server: any }) => matchesQuery(query.value, row.server.tag, row.server.type, row.server.server, row.server.server_port)))

const conditions = (rule: any) => conditionKeys(rule, actionDnsRuleKeys)
const summary = (rule: any) => ruleSummary(rule, actionDnsRuleKeys)

const ruleRows = computed(() => dnsRules.value
  .map((rule: any, index: number) => ({ rule, index }))
  .filter(row => matchesQuery(query.value, row.rule.action, row.rule.server, row.rule.mode, summary(row.rule))))

const deleteDialog = ref({ visible: false, kind: 'rule', index: -1, target: '' })
const requestDelete = (kind: 'server' | 'rule', index: number) => {
  const target = kind == 'server'
    ? dns.value.servers[index]?.tag ?? ''
    : `#${index + 1} ${dnsRules.value[index]?.server ?? dnsRules.value[index]?.action ?? ''}`.trim()
  deleteDialog.value = { visible: true, kind, index, target }
}
const confirmDelete = () => {
  const { kind, index } = deleteDialog.value
  if (kind == 'server') dns.value.servers.splice(index, 1)
  else dnsRules.value.splice(index, 1)
  deleteDialog.value.visible = false
}

const dnsModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showDnsModal = (index: number) => {
  dnsModal.value.index = index
  dnsModal.value.data = index == -1 ? '' : JSON.stringify(dns.value.servers[index])
  dnsModal.value.visible = true
}

const closeDnsModal = () => {
  dnsModal.value.visible = false
}

const saveDnsModal = (data:any) => {
  // New or Edit
  if (dnsModal.value.index == -1) {
    dns.value.servers.push(data)
  } else {
    dns.value.servers[dnsModal.value.index] = data
  }
  dnsModal.value.visible = false
}

const dnsRuleModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showDnsRuleModal = (index: number) => {
  dnsRuleModal.value.index = index
  dnsRuleModal.value.data = index == -1 ? '' : JSON.stringify(dnsRules.value[index])
  dnsRuleModal.value.visible = true
}

const closeDnsRuleModal = () => {
  dnsRuleModal.value.visible = false
}

const saveDnsRuleModal = (data:dnsRule) => {
  // New or Edit
  if (dnsRuleModal.value.index == -1) {
    dnsRules.value.push(data)
  } else {
    dnsRules.value[dnsRuleModal.value.index] = data
  }
  dnsRuleModal.value.visible = false
}

const { move: moveRule, rowProps: ruleRowProps } = useRowReorder(() => dnsRules.value, smAndDown)
</script>