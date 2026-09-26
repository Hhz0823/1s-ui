<template>
  <header class="sdwan-header">
    <div class="sdwan-title">
      <h1 class="adaptive-ink">{{ $t('sdwan.title') }}</h1>
      <div class="adaptive-ink">{{ $t('sdwan.subtitle') }}</div>
    </div>
    <div class="sdwan-actions">
      <v-btn variant="tonal" prepend-icon="mdi-refresh" :loading="loading" @click="load">{{ $t('actions.update') }}</v-btn>
      <v-btn color="secondary" variant="tonal" prepend-icon="mdi-speedometer" :loading="busy === 'test'" :disabled="!state?.active || !!busy" @click="runTest">{{ $t('sdwan.testNow') }}</v-btn>
      <v-btn color="primary" variant="tonal" prepend-icon="mdi-sync" :loading="busy === 'resync'" :disabled="!state?.members?.length || !!busy" @click="resync">{{ $t('sdwan.resync') }}</v-btn>
    </div>
  </header>

  <v-alert type="info" variant="tonal" density="compact" class="mb-4">{{ $t('sdwan.howItWorks') }}</v-alert>
  <v-alert v-if="state && !state.can_control" type="warning" variant="tonal" class="mb-4">{{ $t('sdwan.controllerRequired') }}</v-alert>
  <v-alert v-for="(warning, index) in warnings" :key="index" type="warning" variant="tonal" density="compact" class="mb-2" closable>{{ warning }}</v-alert>

  <v-progress-linear v-if="loading && !state" indeterminate class="mb-3" />

  <template v-if="state">
    <section class="sdwan-overview">
      <div class="sdwan-stat">
        <span>{{ $t('sdwan.routing') }}</span>
        <v-chip :color="routingActive ? 'success' : undefined" size="small" variant="flat">
          {{ routingActive ? $t('sdwan.routingOn') : $t('sdwan.routingOff') }}
        </v-chip>
      </div>
      <div class="sdwan-stat">
        <span>{{ $t('sdwan.current') }}</span>
        <strong>{{ state.selected_name || '-' }}</strong>
        <small v-if="selectedMember?.delay != null">{{ selectedMember.delay }} ms</small>
      </div>
      <div class="sdwan-stat">
        <span>{{ $t('sdwan.members') }}</span>
        <strong>{{ onlineMembers }} / {{ state.members.length }}</strong>
        <small>{{ $t('sdwan.onlineMembers') }}</small>
      </div>
      <div class="sdwan-stat">
        <span>{{ $t('sdwan.group') }}</span>
        <strong dir="ltr">{{ state.group_tag }}</strong>
        <small>{{ state.active ? $t('sdwan.groupActive') : (state.core_running ? $t('sdwan.groupIdle') : $t('sdwan.coreStopped')) }}</small>
      </div>
    </section>

    <v-card class="mb-4" rounded="lg">
      <v-card-title>{{ $t('sdwan.settings') }}</v-card-title>
      <v-card-text>
        <v-row>
          <v-col cols="12" md="4">
            <v-switch v-model="form.enabled" color="primary" :label="$t('sdwan.enable')" hide-details />
          </v-col>
          <v-col cols="12" md="8">
            <v-select
              v-model="form.entry_inbounds"
              :items="state.inbounds"
              :label="$t('sdwan.entryInbounds')"
              :hint="$t('sdwan.entryInboundsHint')"
              persistent-hint
              multiple
              chips
              closable-chips
            />
          </v-col>
          <v-col cols="12" md="4">
            <v-select v-model="form.protocol" :items="protocolItems" :label="$t('sdwan.protocol')" :hint="protocolHint" persistent-hint />
          </v-col>
          <v-col cols="12" md="8">
            <v-text-field v-model="form.test_url" :label="$t('sdwan.testUrl')" dir="ltr" hide-details />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model.number="form.interval" type="number" min="10" max="1800" :label="$t('sdwan.interval')" :suffix="$t('date.s')" hide-details />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model.number="form.tolerance" type="number" min="1" max="5000" :label="$t('sdwan.tolerance')" :hint="$t('sdwan.toleranceHint')" persistent-hint suffix="ms" />
          </v-col>
          <v-col cols="12" md="4">
            <v-switch v-model="form.include_direct" color="primary" :label="$t('sdwan.includeDirect')" :hint="$t('sdwan.includeDirectHint')" persistent-hint />
          </v-col>
          <v-col cols="12">
            <v-switch v-model="form.rules_first" color="primary" :label="$t('sdwan.rulesFirst')" :hint="$t('sdwan.rulesFirstHint')" persistent-hint />
          </v-col>
        </v-row>
      </v-card-text>
      <v-card-actions class="justify-end">
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-content-save" :loading="busy === 'settings'" :disabled="!state.can_control || !!busy" @click="saveSettings">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>

    <h2 class="sdwan-section adaptive-ink">{{ $t('sdwan.members') }}</h2>
    <v-alert v-if="state.members.length === 0" type="info" variant="tonal" density="compact" class="mb-4">{{ $t('sdwan.noMembers') }}</v-alert>
    <div v-else class="sdwan-grid mb-6">
      <article v-for="member in state.members" :key="member.node_id" class="sdwan-item" :class="{ 'sdwan-item--selected': member.selected }">
        <header>
          <div>
            <strong>
              <span class="sdwan-dot" :class="member.online ? 'sdwan-dot--on' : 'sdwan-dot--off'" />
              {{ member.name }}
            </strong>
            <small dir="ltr">{{ member.protocol }} · {{ member.server }}:{{ member.port }}</small>
          </div>
          <v-chip v-if="member.selected" color="success" size="x-small" variant="flat">{{ $t('sdwan.inUse') }}</v-chip>
        </header>
        <dl>
          <div><dt>{{ $t('sdwan.egressDelay') }}</dt><dd>{{ member.delay != null ? member.delay + ' ms' : '-' }}</dd></div>
          <div><dt>{{ $t('sdwan.controlRtt') }}</dt><dd>{{ formatRtt(member) }}</dd></div>
          <div><dt>{{ $t('sdwan.load') }}</dt><dd>CPU {{ member.cpu_percent.toFixed(0) }}% · ↑{{ formatRate(member.net_sent_rate) }} ↓{{ formatRate(member.net_recv_rate) }}</dd></div>
          <div v-if="member.needs_resync"><dt>{{ $t('sdwan.status') }}</dt><dd class="text-warning">{{ $t('sdwan.needsResync') }}</dd></div>
        </dl>
        <footer>
          <v-btn icon="mdi-refresh" size="small" variant="text" :title="$t('sdwan.redeploy')" :loading="busy === 'add-' + member.node_id" :disabled="!!busy" @click="addMember(member.node_id)" />
          <v-btn icon="mdi-open-in-new" size="small" variant="text" :title="$t('agent.detail')" @click="router.push(`/agents/${member.node_id}`)" />
          <v-btn icon="mdi-link-variant-remove" size="small" variant="text" color="error" :title="$t('sdwan.remove')" :loading="busy === 'remove-' + member.node_id" :disabled="!!busy" @click="removeMember(member.node_id)" />
        </footer>
      </article>
    </div>

    <h2 class="sdwan-section adaptive-ink">{{ $t('sdwan.addServers') }}</h2>
    <v-alert v-if="candidates.length === 0" type="info" variant="tonal" density="compact">{{ $t('sdwan.noCandidates') }}</v-alert>
    <div v-else class="sdwan-grid">
      <article v-for="node in candidates" :key="node.node_id" class="sdwan-item">
        <header>
          <div>
            <strong>
              <span class="sdwan-dot" :class="node.online ? 'sdwan-dot--on' : 'sdwan-dot--off'" />
              {{ node.name }}
            </strong>
            <small dir="ltr">{{ node.public_host || '-' }}</small>
          </div>
        </header>
        <dl>
          <div><dt>{{ $t('sdwan.status') }}</dt><dd>{{ candidateStatus(node) }}</dd></div>
        </dl>
        <footer>
          <v-btn color="primary" variant="text" prepend-icon="mdi-plus" :loading="busy === 'add-' + node.node_id" :disabled="!candidateReady(node) || !!busy" @click="addMember(node.node_id)">{{ $t('sdwan.join') }}</v-btn>
        </footer>
      </article>
    </div>
  </template>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { fetchBackendObject as api } from '@/utils/backend'

interface SdwanSettings {
  enabled: boolean
  protocol: string
  entry_inbounds: string[]
  test_url: string
  interval: number
  tolerance: number
  include_direct: boolean
  rules_first: boolean
}

const router = useRouter()
const loading = ref(false)
const busy = ref('')
const state = ref<any>(null)
const warnings = ref<string[]>([])
const form = reactive<SdwanSettings>({
  enabled: false, protocol: 'shadowsocks', entry_inbounds: [], test_url: '',
  interval: 60, tolerance: 50, include_direct: false, rules_first: false,
})
let timer: ReturnType<typeof setInterval> | undefined

const protocolItems = computed(() => [
  { title: i18n.global.t('sdwan.protocolShadowsocks'), value: 'shadowsocks' },
  { title: i18n.global.t('sdwan.protocolHysteria2'), value: 'hysteria2' },
])
const protocolHint = computed(() => form.protocol === 'hysteria2' ? i18n.global.t('sdwan.protocolHysteria2Hint') : i18n.global.t('sdwan.protocolShadowsocksHint'))
const routingActive = computed(() => Boolean(state.value?.settings?.enabled && state.value?.active && state.value?.settings?.entry_inbounds?.length))
const selectedMember = computed(() => state.value?.members?.find((member: any) => member.selected))
const onlineMembers = computed(() => (state.value?.members ?? []).filter((member: any) => member.online).length)
const candidates = computed(() => (state.value?.nodes ?? []).filter((node: any) => !node.member))

const applyState = (next: any, keepForm = false) => {
  state.value = next
  warnings.value = next?.warnings ?? []
  if (!keepForm && next?.settings) Object.assign(form, { ...next.settings, entry_inbounds: [...(next.settings.entry_inbounds ?? [])] })
}

const load = async (keepForm = false) => {
  if (loading.value) return
  loading.value = true
  try {
    applyState(await api('api/sdwan'), keepForm)
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('failed') })
  } finally {
    loading.value = false
  }
}

const run = async (key: string, path: string, body?: unknown) => {
  if (busy.value) return
  busy.value = key
  try {
    const next = await api(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) })
    applyState(next, key !== 'settings')
    push.success({ message: i18n.global.t('success') })
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('failed') })
  } finally {
    busy.value = ''
  }
}

const saveSettings = () => run('settings', 'api/sdwan/settings', {
  ...form,
  interval: Math.floor(Number(form.interval) || 60),
  tolerance: Math.floor(Number(form.tolerance) || 50),
})
const addMember = (id: number) => run('add-' + id, `api/sdwan/members/${id}`)
const removeMember = (id: number) => run('remove-' + id, `api/sdwan/members/${id}/delete`)
const resync = () => run('resync', 'api/sdwan/resync')
const runTest = () => run('test', 'api/sdwan/test')

const candidateReady = (node: any) => Boolean(node.online && node.managed && node.supported && node.public_host && state.value?.can_control)
const candidateStatus = (node: any) => {
  if (!node.online) return i18n.global.t('sdwan.offline')
  if (!node.managed) return i18n.global.t('sdwan.notManaged')
  if (!node.supported) return i18n.global.t('sdwan.upgradeRequired')
  if (!node.public_host) return i18n.global.t('agent.publicHostRequired')
  return i18n.global.t('sdwan.ready')
}
const formatRtt = (member: any) => {
  const latency = member.latency || {}
  if (latency.last_ms == null) return '-'
  return `${latency.last_ms} ms · ${i18n.global.t('sdwan.loss')} ${Number(latency.loss_pct || 0).toFixed(0)}%`
}
const formatRate = (bytes: number) => {
  const value = Number(bytes) || 0
  if (value >= 1024 * 1024) return (value / 1024 / 1024).toFixed(1) + 'MB/s'
  if (value >= 1024) return (value / 1024).toFixed(0) + 'KB/s'
  return value.toFixed(0) + 'B/s'
}

onMounted(() => {
  load()
  timer = setInterval(() => { if (!document.hidden && !busy.value) load(true) }, 15000)
})
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.sdwan-header { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: center; margin-bottom: 18px; }
.sdwan-title h1 { font-size: 1.25rem; margin: 0; }
.sdwan-title div { opacity: .7; }
.sdwan-actions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.sdwan-overview { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: 16px; }
.sdwan-stat { display: flex; flex-direction: column; gap: 4px; padding: 14px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; background: rgb(var(--v-theme-surface)); min-width: 0; }
.sdwan-stat span { opacity: .65; font-size: .8rem; }
.sdwan-stat strong { font-size: 1.05rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sdwan-stat small { opacity: .65; }
.sdwan-stat .v-chip { align-self: flex-start; }
.sdwan-section { font-size: 1.05rem; margin: 8px 0 12px; }
.sdwan-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 12px; }
.sdwan-item { border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; overflow: hidden; background: rgb(var(--v-theme-surface)); }
.sdwan-item--selected { border-color: rgb(var(--v-theme-success)); }
.sdwan-item > header { display: flex; justify-content: space-between; gap: 10px; padding: 13px 14px 10px; }
.sdwan-item header div { min-width: 0; }
.sdwan-item strong, .sdwan-item small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sdwan-item small { opacity: .65; margin-top: 3px; white-space: normal; overflow-wrap: anywhere; }
.sdwan-item dl { margin: 0; padding: 0 14px 10px; }
.sdwan-item dl div { display: flex; justify-content: space-between; gap: 12px; padding: 5px 0; border-top: 1px solid rgba(var(--v-border-color), .08); }
.sdwan-item dt { opacity: .65; white-space: nowrap; }
.sdwan-item dd { margin: 0; overflow: hidden; text-overflow: ellipsis; text-align: end; }
.sdwan-item footer { display: flex; justify-content: center; border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); min-height: 42px; }
.sdwan-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-inline-end: 6px; vertical-align: middle; }
.sdwan-dot--on { background: rgb(var(--v-theme-success)); }
.sdwan-dot--off { background: rgba(var(--v-border-color), .6); }
@media (max-width: 700px) {
  .sdwan-header { grid-template-columns: 1fr; }
  .sdwan-actions { justify-content: center; }
}
</style>
