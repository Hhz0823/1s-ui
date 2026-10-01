<template>
  <v-dialog :model-value="modelValue" max-width="640" scrollable @update:model-value="emit('update:modelValue', $event)">
    <v-card>
      <v-card-title>{{ $t('relaySpeedtest.title', { name: serverName }) }}</v-card-title>
      <v-card-subtitle class="rs-subtitle">{{ $t('relaySpeedtest.hint') }}</v-card-subtitle>
      <v-card-text>
        <v-select v-model="form.relay_id" :items="relayOptions" item-title="title" item-value="value" item-props="props"
          :label="$t('relaySpeedtest.from')" :disabled="running" />
        <div class="rs-label">{{ $t('relaySpeedtest.tests') }}</div>
        <v-chip-group v-model="form.tests" multiple column selected-class="text-primary" :disabled="running">
          <v-chip v-for="test in tests" :key="test" :value="test" size="small" variant="outlined" filter>{{ $t('relaySpeedtest.test.' + test) }}</v-chip>
        </v-chip-group>
        <v-row dense class="mt-1">
          <v-col cols="4">
            <v-select v-model="form.seconds" :items="[5, 10, 15]" :label="$t('relaySpeedtest.seconds')" :disabled="running" density="compact" />
          </v-col>
          <v-col cols="4">
            <v-select v-model="form.streams" :items="[1, 4, 8]" :label="$t('relaySpeedtest.streams')" :disabled="running" density="compact" />
          </v-col>
          <v-col cols="4">
            <v-select v-model="form.udp_mbps" :items="[10, 50, 100, 200, 500]" :label="$t('relaySpeedtest.udpRate')" :disabled="running" density="compact" />
          </v-col>
        </v-row>
        <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mb-3">{{ error }}</v-alert>
        <template v-if="job">
          <div class="rs-route">
            <span>{{ job.relay_id ? job.relay_name || '#' + job.relay_id : $t('proxyMonitor.panelHost') }}</span>
            <v-icon icon="mdi-arrow-right" size="small" />
            <span>{{ serverName }}</span>
            <span v-if="job.host" class="text-medium-emphasis" dir="ltr">({{ job.host }})</span>
          </div>
          <div v-for="test in job.tests" :key="test" class="rs-row">
            <span class="rs-name">{{ $t('relaySpeedtest.test.' + test) }}</span>
            <span v-if="resultOf(test)" class="rs-value" :class="resultOf(test)!.error ? 'text-error' : ''">{{ summary(resultOf(test)!) }}</span>
            <v-progress-circular v-else-if="job.current === test" indeterminate size="18" width="2" color="primary" />
            <span v-else class="text-medium-emphasis">—</span>
          </div>
          <v-alert v-if="job.error" type="error" variant="tonal" density="compact" class="mt-3">{{ job.error }}</v-alert>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="running" :disabled="!form.tests.length" @click="start">{{ $t('relaySpeedtest.start') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { i18n } from '@/locales'
import { fetchBackendObject } from '@/utils/backend'

type Ping = { sent: number, received: number, min_ms: number, avg_ms: number, max_ms: number, jitter_ms: number, loss_pct: number }
type Throughput = { bytes: number, seconds: number, bits_per_second: number, streams: number }
type Udp = { target_mbps: number, sent_packets: number, received_packets: number, bits_per_second: number, jitter_ms: number, loss_pct: number }
type Result = { test: string, host: string, ping?: Ping, throughput?: Throughput, udp?: Udp, error?: string }
type Job = {
  id: string, server_id: number, relay_id: number, relay_name: string, status: string, tests: string[]
  current?: string, host?: string, results: Result[], error?: string
}

const props = defineProps<{ modelValue: boolean, serverId: number, serverName: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()
const t = (key: string, values?: Record<string, unknown>) => i18n.global.t(key, values ?? {})

const tests = ['tcp_ping', 'udp_ping', 'tcp_download', 'tcp_upload', 'udp_download', 'udp_upload']
const form = reactive({ relay_id: 0, tests: [...tests], seconds: 10, streams: 4, udp_mbps: 50 })
const relays = ref<{ id: number, name: string, online: boolean, supported: boolean }[]>([])
const job = ref<Job | null>(null)
const error = ref('')
let timer: number | undefined

const running = computed(() => job.value?.status === 'running')
const relayOptions = computed(() => [
  { value: 0, title: t('proxyMonitor.panelHost'), props: {} },
  ...relays.value.filter(relay => relay.id !== props.serverId).map(relay => ({
    value: relay.id,
    title: relay.name + (!relay.online ? ` (${t('agent.offline')})` : !relay.supported ? ` (${t('relaySpeedtest.needsUpdate')})` : ''),
    props: { disabled: !relay.online || !relay.supported },
  })),
])

const loadRelays = async () => {
  try {
    const nodes = await fetchBackendObject<any[]>('api/agents') || []
    relays.value = nodes.map(node => ({
      id: node.id, name: node.name, online: !!node.online,
      supported: (node.report?.panel?.capabilities || []).includes('speedtest.client.v1'),
    }))
  } catch {
    relays.value = []
  }
}

const resultOf = (test: string) => job.value?.results.find(result => result.test === test)

const mbps = (bits: number) => bits >= 1e9 ? `${(bits / 1e9).toFixed(2)} Gbps` : `${(bits / 1e6).toFixed(1)} Mbps`
const ms = (value: number) => `${value.toFixed(value < 10 ? 1 : 0)} ms`
const summary = (result: Result) => {
  if (result.error) return result.error
  if (result.ping) {
    const ping = result.ping
    if (!ping.received) return t('relaySpeedtest.noReply')
    return `${ms(ping.avg_ms)} (${ms(ping.min_ms)}–${ms(ping.max_ms)}) · ${t('relaySpeedtest.jitter')} ${ms(ping.jitter_ms)} · ${t('relaySpeedtest.loss')} ${ping.loss_pct.toFixed(0)}%`
  }
  if (result.throughput) return `${mbps(result.throughput.bits_per_second)} · ${result.throughput.streams} ${t('relaySpeedtest.streamsShort')}`
  if (result.udp) {
    const udp = result.udp
    return `${mbps(udp.bits_per_second)} / ${udp.target_mbps} Mbps · ${t('relaySpeedtest.loss')} ${udp.loss_pct.toFixed(1)}% · ${t('relaySpeedtest.jitter')} ${ms(udp.jitter_ms)}`
  }
  return ''
}

const stopPolling = () => {
  if (timer) window.clearTimeout(timer)
  timer = undefined
}

const poll = async () => {
  if (!job.value) return
  try {
    job.value = await fetchBackendObject<Job>(`api/relay-speedtests/${job.value.id}`)
  } catch (err: any) {
    error.value = err?.message || t('failed')
    stopPolling()
    return
  }
  if (job.value?.status === 'running' && props.modelValue) timer = window.setTimeout(poll, 1000)
}

const start = async () => {
  error.value = ''
  stopPolling()
  try {
    job.value = await fetchBackendObject<Job>('api/relay-speedtests', {
      method: 'POST', body: JSON.stringify({ server_id: props.serverId, ...form }),
    })
    timer = window.setTimeout(poll, 1000)
  } catch (err: any) {
    error.value = err?.message || t('failed')
  }
}

watch(() => props.modelValue, visible => {
  if (visible) {
    void loadRelays()
    if (running.value) timer = window.setTimeout(poll, 500)
  } else {
    stopPolling()
  }
}, { immediate: true })
onBeforeUnmount(stopPolling)
</script>

<style scoped>
.rs-subtitle { white-space: normal; }
.rs-label { font-size: .85rem; color: rgba(var(--v-theme-on-surface), .7); margin-top: 4px; }
.rs-route { display: flex; align-items: center; gap: 6px; margin: 8px 0; font-weight: 600; flex-wrap: wrap; }
.rs-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.rs-name { white-space: nowrap; }
.rs-value { text-align: end; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
</style>
