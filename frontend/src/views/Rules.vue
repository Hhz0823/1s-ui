<template>
  <RuleVue
    v-model="ruleModal.visible"
    :visible="ruleModal.visible"
    :index="ruleModal.index"
    :data="ruleModal.data"
    :clients="clients"
    :inTags="inboundTags"
    :outTags="outboundTags"
    :rsTags="rulesetTags"
    @close="closeRuleModal"
    @save="saveRuleModal"
  />
  <RulesetVue
    v-model="rulesetModal.visible"
    :visible="rulesetModal.visible"
    :index="rulesetModal.index"
    :data="rulesetModal.data"
    :outTags="outboundTags"
    @close="closeRulesetModal"
    @save="saveRulesetModal"
  />
  <RuleImport
    v-model="importRulesModal.visible"
    :visible="importRulesModal.visible"
    :existingRulesCount="rules.length"
    :existingRulesetsCount="rulesets.length"
    :existingRulesetTags="rulesetTags"
    @save="saveImportRule"
    @close="closeImportRule"
  />
  <RulesetImport
    v-model="importRulesetsModal.visible"
    :visible="importRulesetsModal.visible"
    :outTags="outboundTags"
    :rsTags="rulesetTags"
    @save="saveImportRulesets"
    @close="closeImportRulesets"
  />
  <DeleteConfirm v-model="deleteDialog.visible" :target="deleteDialog.target" @confirm="confirmDelete" />
  <div class="list-toolbar">
    <div class="list-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showRuleModal(-1)">{{ $t('rule.add') }}</v-btn>
      <v-btn color="primary" variant="outlined" prepend-icon="mdi-plus" @click="showRulesetModal(-1)">{{ $t('ruleset.add') }}</v-btn>
      <v-menu location="bottom start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" variant="outlined" prepend-icon="mdi-import" append-icon="mdi-menu-down">{{ $t('list.import') }}</v-btn>
        </template>
        <v-list density="compact" nav>
          <v-list-item prepend-icon="mdi-routes" :title="$t('rule.import.rulesTitle')" @click="showImportRule" />
          <v-list-item prepend-icon="mdi-download-multiple" :title="$t('rule.import.title')" @click="showImportRulesets" />
        </v-list>
      </v-menu>
      <v-btn color="warning" :variant="unchanged ? 'outlined' : 'flat'" prepend-icon="mdi-content-save-outline" :loading="loading" :disabled="unchanged" @click="saveConfig">
        {{ $t('actions.save') }}
      </v-btn>
      <span v-if="!unchanged" class="list-unsaved">{{ $t('list.unsaved') }}</span>
    </div>
    <v-text-field v-model="query" class="list-toolbar__search" :placeholder="$t('list.search')" prepend-inner-icon="mdi-magnify" clearable density="compact" hide-details />
  </div>
  <section class="list-panel">
    <header class="list-panel__head">{{ $t('basic.routing.title') }}</header>
    <div class="list-panel__body">
      <v-row dense>
        <v-col cols="12" sm="6" lg="3">
          <v-select hide-details :label="$t('basic.routing.defaultOut')" clearable
            @click:clear="delete route.final" :items="outboundTags" v-model="route.final"></v-select>
        </v-col>
        <v-col cols="12" sm="6" lg="3">
          <v-text-field v-model="route.default_interface" hide-details clearable
            @click:clear="delete route.default_interface" :label="$t('basic.routing.defaultIf')"></v-text-field>
        </v-col>
        <v-col cols="12" sm="6" lg="3">
          <v-text-field v-model.number="routeMark" hide-details type="number" min="0" :label="$t('basic.routing.defaultRm')"></v-text-field>
        </v-col>
        <v-col cols="12" sm="6" lg="3">
          <v-switch v-model="route.auto_detect_interface" color="primary" :label="$t('basic.routing.autoBind')" hide-details></v-switch>
        </v-col>
      </v-row>
    </div>
  </section>
  <div class="list-section-title">
    {{ $t('rule.ruleset') }}
    <span class="list-count">{{ rulesets.length }}</span>
  </div>
  <v-data-table
    class="list-table"
    :headers="rulesetHeaders"
    :items="rulesetRows"
    item-value="index"
    v-model:items-per-page="itemsPerPage"
    :items-per-page-options="pageSizeItems"
    :mobile="smAndDown"
    :hide-default-header="smAndDown"
    :no-data-text="$t('noData')"
    hover
  >
    <template #item.tag="{ item }">
      <span class="list-main">{{ item.ruleset.tag }}</span>
    </template>
    <template #item.type="{ item }">
      <v-chip size="small" label variant="tonal" :color="item.ruleset.type == 'remote' ? 'primary' : undefined">{{ rulesetType(item.ruleset.type) }}</v-chip>
    </template>
    <template #item.format="{ item }">
      <span class="list-mono">{{ item.ruleset.format ?? '-' }}</span>
    </template>
    <template #item.source="{ item }">
      <span class="list-mono list-clip" :title="item.ruleset.url ?? item.ruleset.path ?? undefined">{{ item.ruleset.url ?? item.ruleset.path ?? '-' }}</span>
    </template>
    <template #item.download_detour="{ item }">
      {{ item.ruleset.download_detour ?? '-' }}
    </template>
    <template #item.update_interval="{ item }">
      {{ item.ruleset.update_interval ?? '-' }}
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showRulesetModal(item.index)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete('ruleset', item.index)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
  <div class="list-section-title">
    {{ $t('pages.rules') }}
    <span class="list-count">{{ rules.length }}</span>
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
        <v-btn icon="mdi-arrow-down" size="x-small" variant="text" :disabled="item.index == rules.length - 1" :title="$t('list.moveDown')" @click="moveRule(item.index, item.index + 1)" />
      </div>
    </template>
    <template #item.type="{ item }">
      {{ item.rule.type != undefined ? $t('rule.logical') + ' (' + item.rule.mode + ')' : $t('rule.simple') }}
    </template>
    <template #item.conditions="{ item }">
      <span class="list-mono list-clip" :title="summary(item.rule) || undefined">{{ conditions(item.rule).join(', ') || '-' }}</span>
    </template>
    <template #item.action="{ item }">
      <v-chip size="small" label variant="tonal" :color="actionColor(item.rule.action)">{{ item.rule.action ?? '-' }}</v-chip>
    </template>
    <template #item.outbound="{ item }">
      <span class="list-main">{{ item.rule.outbound ?? '-' }}</span>
    </template>
    <template #item.invert="{ item }">
      <v-chip v-if="item.rule.invert" size="small" label variant="tonal" color="warning">{{ $t('yes') }}</v-chip>
      <span v-else>-</span>
    </template>
    <template #item.actions="{ item }">
      <div class="list-ops">
        <v-btn size="small" variant="text" color="primary" @click="showRuleModal(item.index)">{{ $t('actions.edit') }}</v-btn>
        <v-btn size="small" variant="text" color="error" @click="requestDelete('rule', item.index)">{{ $t('actions.del') }}</v-btn>
      </div>
    </template>
  </v-data-table>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, onBeforeMount } from 'vue'
import RuleVue from '@/layouts/modals/Rule.vue'
import RulesetVue from '@/layouts/modals/Ruleset.vue'
import RulesetImport from '@/layouts/modals/RulesetImport.vue'
import RuleImport from '@/layouts/modals/RuleImport.vue'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import { Config } from '@/types/config'
import { actionKeys, ruleset } from '@/types/rules'
import { FindDiff } from '@/plugins/utils'
import { i18n } from '@/locales'
import { matchesQuery, useListTable, useRowReorder } from '@/utils/listTable'
import { conditionKeys, ruleSummary } from '@/utils/ruleSummary'

const oldConfig = ref({})
const loading = ref(false)
const appConfig = computed((): Config => {
  return <Config> Data().config
})

onBeforeMount(async () => {
  loading.value = true
  while (Data().lastLoad == 0) {
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
})

const routeMark = computed({
  get() { return route.value.default_mark ?? 0 },
  set(v:number) { v>0 ? route.value.default_mark = v : delete appConfig.value.route.default_mark }
})

const unchanged = computed(() => FindDiff.deepCompare(appConfig.value, oldConfig.value))

const saveConfig = async () => {
  loading.value = true
  const success = await Data().save("config", "set", appConfig.value)
  if (success) {
    oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  }
  loading.value = false
}

const clients = computed((): string[] => Data().clients.map((c:any) => c.name))
const route = computed((): any => appConfig.value.route ?? {})

const rules = computed((): any[] => {
  const data = route.value
  if (!data) return []
  if (!('rules' in data) || !Array.isArray(data.rules)) data.rules = []
  return data.rules
})

const rulesets = computed((): any[] => {
  const data = route.value
  if (!data) return []
  if (!('rule_set' in data) || !Array.isArray(data.rule_set)) data.rule_set = []
  return data.rule_set
})

const rulesetTags = computed((): string[] => rulesets.value.map((rs:any) => rs.tag))

const outboundTags = computed((): string[] => [
  ...Data().outbounds?.map((o:any) => o.tag),
  ...Data().endpoints?.map((e:any) => e.tag)
])

const inboundTags = computed((): string[] => [
  ...Data().inbounds?.map((o:any) => o.tag),
  ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)
])

const t = i18n.global.t
const query = ref('')
const { smAndDown, itemsPerPage, pageSizeItems } = useListTable()

const rulesetHeaders = computed(() => [
  { title: t('objects.tag'), key: 'tag', sortable: false },
  { title: t('list.type'), key: 'type', sortable: false },
  { title: t('ruleset.format'), key: 'format', sortable: false },
  { title: t('list.source'), key: 'source', sortable: false },
  { title: t('objects.outbound'), key: 'download_detour', sortable: false },
  { title: t('ruleset.interval'), key: 'update_interval', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])
// Rules match top-down, so the table keeps their order instead of sorting.
const ruleHeaders = computed(() => [
  { title: '#', key: 'order', sortable: false, width: 132 },
  { title: t('list.type'), key: 'type', sortable: false },
  { title: t('list.conditions'), key: 'conditions', sortable: false },
  { title: t('list.ruleAction'), key: 'action', sortable: false },
  { title: t('objects.outbound'), key: 'outbound', sortable: false },
  { title: t('rule.invert'), key: 'invert', sortable: false },
  { title: t('list.actions'), key: 'actions', sortable: false },
])

const rulesetType = (type: string) => {
  const key = 'ruleset.' + type
  return i18n.global.te(key) ? t(key) : type
}

const rulesetRows = computed(() => rulesets.value
  .map((ruleset: any, index: number) => ({ ruleset, index }))
  .filter(row => matchesQuery(query.value, row.ruleset.tag, row.ruleset.type, row.ruleset.format,
    row.ruleset.url, row.ruleset.path, row.ruleset.download_detour)))

const conditions = (rule: any) => conditionKeys(rule, actionKeys)
const summary = (rule: any) => ruleSummary(rule, actionKeys)

const actionColor = (action: string) => {
  switch (action) {
    case 'route': return 'primary'
    case 'reject': return 'error'
    case 'hijack-dns': return 'info'
    default: return undefined
  }
}

const ruleRows = computed(() => rules.value
  .map((rule: any, index: number) => ({ rule, index }))
  .filter(row => matchesQuery(query.value, row.rule.action, row.rule.outbound, row.rule.mode, summary(row.rule))))

const deleteDialog = ref({ visible: false, kind: 'rule', index: -1, target: '' })
const requestDelete = (kind: 'rule' | 'ruleset', index: number) => {
  const target = kind == 'ruleset'
    ? rulesets.value[index]?.tag ?? ''
    : `#${index + 1} ${rules.value[index]?.outbound ?? rules.value[index]?.action ?? ''}`.trim()
  deleteDialog.value = { visible: true, kind, index, target }
}
const confirmDelete = () => {
  const { kind, index } = deleteDialog.value
  if (kind == 'ruleset') rulesets.value.splice(index, 1)
  else rules.value.splice(index, 1)
  deleteDialog.value.visible = false
}

const ruleModal = ref({ visible: false, index: -1, data: "" })
const showRuleModal = (index: number) => {
  ruleModal.value.index = index
  ruleModal.value.data = index == -1 ? '' : JSON.stringify(rules.value[index])
  ruleModal.value.visible = true
}
const closeRuleModal = () => { ruleModal.value.visible = false }
const saveRuleModal = (data:any) => {
  if (ruleModal.value.index == -1) rules.value.push(data)
  else rules.value[ruleModal.value.index] = data
  ruleModal.value.visible = false
}

const rulesetModal = ref({ visible: false, index: -1, data: "" })
const showRulesetModal = (index: number) => {
  rulesetModal.value.index = index
  rulesetModal.value.data = index == -1 ? '' : JSON.stringify(rulesets.value[index])
  rulesetModal.value.visible = true
}
const closeRulesetModal = () => { rulesetModal.value.visible = false }
const saveRulesetModal = (data:ruleset) => {
  if (rulesetModal.value.index == -1) rulesets.value.push(data)
  else rulesets.value[rulesetModal.value.index] = data
  rulesetModal.value.visible = false
}

const { move: moveRule, rowProps: ruleRowProps } = useRowReorder(() => rules.value, smAndDown)

const importRulesModal = ref({ visible: false })

function showImportRule() {
  importRulesModal.value.visible = true
}

function closeImportRule() {
  importRulesModal.value.visible = false
}

function saveImportRule(block: any, mode: 'merge' | 'replace', applyFinal: boolean) {
  if (mode === 'replace') {
    route.value.rules = block.rules ?? []
    route.value.rule_set = block.rule_set ?? []
  } else {
    const existingTags = new Set(rulesetTags.value)
    if (block.rules) rules.value.push(...block.rules)
    if (block.rule_set) {
      for (const rs of block.rule_set) {
        if (!existingTags.has(rs.tag)) rulesets.value.push(rs)
      }
    }
  }
  if (applyFinal && block.final) route.value.final = block.final
  importRulesModal.value.visible = false
}

const importRulesetsModal = ref({ visible: false })

function showImportRulesets() {
  importRulesetsModal.value.visible = true
}

function closeImportRulesets() {
  importRulesetsModal.value.visible = false
}

function saveImportRulesets(items: any[]) {
  rulesets.value.push(...items)
  importRulesetsModal.value.visible = false
}
</script>
