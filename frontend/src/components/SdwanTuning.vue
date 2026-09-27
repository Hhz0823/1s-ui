<template>
  <v-card class="mb-4" rounded="lg">
    <v-card-title class="sdwan-tune-title">
      <v-icon icon="mdi-stethoscope" size="small" />
      <span>{{ $t('sdwan.tune.title') }}</span>
      <v-spacer />
      <v-chip v-if="report && hasNodes" :color="scoreColor(report.score)" size="small" variant="flat">
        {{ $t('sdwan.tune.score') }}
        <template v-if="before">&nbsp;{{ before.score }} → {{ report.score }}</template>
        <template v-else>&nbsp;{{ report.score }}</template>
      </v-chip>
    </v-card-title>
    <v-card-text>
      <p class="sdwan-tune-intro">{{ $t('sdwan.tune.intro') }}</p>
      <div class="sdwan-tune-controls">
        <v-checkbox
          v-model="bandwidth"
          density="compact"
          color="primary"
          :label="$t('sdwan.tune.bandwidth')"
          :hint="$t('sdwan.tune.bandwidthHint')"
          persistent-hint
          :disabled="running"
        />
        <div class="sdwan-tune-buttons">
          <v-btn variant="tonal" prepend-icon="mdi-radar" :loading="starting === 'diagnose'" :disabled="!canControl || running || !!starting" @click="start('diagnose')">
            {{ $t('sdwan.tune.detect') }}
          </v-btn>
          <v-btn color="primary" variant="flat" prepend-icon="mdi-auto-fix" :loading="starting === 'optimize'" :disabled="!canControl || running || !!starting" @click="confirmVisible = true">
            {{ $t('sdwan.tune.optimize') }}
          </v-btn>
        </div>
      </div>

      <template v-if="job">
        <div v-if="running" class="sdwan-tune-progress">
          <div class="sdwan-tune-progress-label">
            <span>{{ jobTitle }} · {{ stageText }}</span>
            <span>{{ job.progress }}%</span>
          </div>
          <v-progress-linear :model-value="job.progress" color="primary" height="8" rounded />
          <ol class="sdwan-log sdwan-log--live">
            <li v-for="(entry, index) in liveLogs" :key="index" :class="'sdwan-log--' + entry.level">{{ logText(entry) }}</li>
          </ol>
        </div>

        <template v-else>
          <v-alert v-if="job.status === 'failed'" type="error" variant="tonal" density="compact" class="mt-4">
            {{ $t('sdwan.tune.failed', { error: job.error || '-' }) }}
          </v-alert>
          <div class="sdwan-tune-meta">
            {{ jobTitle }} · {{ $t('sdwan.tune.finishedAt', { time: formatTime(job.ended_at) }) }}
          </div>

          <template v-if="report">
            <section v-if="job.kind === 'optimize'" class="sdwan-tune-block">
              <h3>{{ $t('sdwan.tune.changes') }}</h3>
              <ul v-if="job.changes.length" class="sdwan-changes">
                <li v-for="(change, index) in job.changes" :key="index">
                  <v-icon icon="mdi-check-circle" color="success" size="small" />
                  <span>{{ logText(change) }}</span>
                </li>
              </ul>
              <p v-else class="sdwan-muted">{{ $t('sdwan.tune.noChanges') }}</p>
            </section>

            <section class="sdwan-tune-block">
              <h3>{{ job.kind === 'optimize' ? $t('sdwan.tune.remaining') : $t('sdwan.tune.findings') }}</h3>
              <p v-if="!report.advice.length" class="sdwan-ok">
                <v-icon icon="mdi-check-decagram" color="success" size="small" />
                {{ $t('sdwan.tune.noFindings') }}
              </p>
              <ul v-else class="sdwan-findings">
                <li v-for="(advice, index) in sortedAdvice" :key="index">
                  <v-icon :icon="severityIcon(advice.severity)" :color="severityColor(advice.severity)" size="small" />
                  <span>{{ adviceText(advice) }}</span>
                  <v-chip v-if="advice.fixable && job.kind === 'diagnose'" size="x-small" color="primary" variant="tonal" label>{{ $t('sdwan.tune.fixable') }}</v-chip>
                </li>
              </ul>
            </section>

            <section v-if="pathRows.length" class="sdwan-tune-block">
              <h3>{{ $t('sdwan.tune.paths') }}</h3>
              <div class="sdwan-table-wrap">
                <v-table density="compact">
                  <thead>
                    <tr>
                      <th>{{ $t('sdwan.tune.colServer') }}</th>
                      <th>{{ $t('sdwan.tune.colProtocol') }}</th>
                      <th class="text-end">{{ $t('sdwan.tune.colLatency') }}</th>
                      <th class="text-end">{{ $t('sdwan.tune.colJitter') }}</th>
                      <th class="text-end">{{ $t('sdwan.tune.colLoss') }}</th>
                      <th v-if="report.bandwidth" class="text-end">{{ $t('sdwan.tune.colBandwidth') }}</th>
                      <th>{{ $t('sdwan.status') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in pathRows" :key="row.path.tag">
                      <td>{{ row.node.name }}</td>
                      <td dir="ltr">{{ protocolName(row.path.protocol) }} <span class="sdwan-muted">:{{ row.path.port }}</span></td>
                      <td class="text-end">{{ row.path.reachable ? row.path.latency + ' ms' : '-' }}</td>
                      <td class="text-end">{{ row.path.reachable ? row.path.jitter + ' ms' : '-' }}</td>
                      <td class="text-end">{{ row.path.measured ? row.path.loss + '%' : '-' }}</td>
                      <td v-if="report.bandwidth" class="text-end">{{ row.path.mbps ? row.path.mbps + ' Mbps' : '-' }}</td>
                      <td>
                        <div class="sdwan-status-chips">
                          <v-chip v-for="status in pathStatuses(row.path)" :key="status.key" :color="status.color" size="x-small" variant="tonal" label>
                            {{ $t('sdwan.pathStatus.' + status.key) }}
                          </v-chip>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </v-table>
              </div>
              <p class="sdwan-muted mt-2">
                {{ $t('sdwan.tune.kernel', { cc: report.controller.congestion_control || '-', qdisc: report.controller.qdisc || '-' }) }}
                <template v-if="report.recommended_tolerance">· {{ $t('sdwan.tune.toleranceNow', { tolerance: report.tolerance, recommended: report.recommended_tolerance }) }}</template>
              </p>
            </section>
          </template>

          <v-expansion-panels v-if="job.logs.length" class="mt-3" variant="accordion">
            <v-expansion-panel elevation="0">
              <v-expansion-panel-title>{{ $t('sdwan.tune.log') }} ({{ job.logs.length }})</v-expansion-panel-title>
              <v-expansion-panel-text>
                <ol class="sdwan-log">
                  <li v-for="(entry, index) in job.logs" :key="index" :class="'sdwan-log--' + entry.level">
                    <time>{{ formatClock(entry.time) }}</time> {{ logText(entry) }}
                  </li>
                </ol>
              </v-expansion-panel-text>
            </v-expansion-panel>
          </v-expansion-panels>
        </template>
      </template>
    </v-card-text>
  </v-card>

  <v-dialog v-model="confirmVisible" width="min(560px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('sdwan.tune.optimize') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <p class="mb-2">{{ $t('sdwan.tune.confirmIntro') }}</p>
        <ul class="sdwan-confirm">
          <li>{{ $t('sdwan.tune.confirmKernel') }}</li>
          <li>{{ $t('sdwan.tune.confirmUpgrade') }}</li>
          <li>{{ $t('sdwan.tune.confirmPaths') }}</li>
          <li>{{ $t('sdwan.tune.confirmRestart') }}</li>
        </ul>
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="confirmVisible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="flat" prepend-icon="mdi-auto-fix" @click="start('optimize')">{{ $t('sdwan.tune.start') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { fetchBackendObject as api } from '@/utils/backend'

interface JobLog {
  time: number
  level: string
  code: string
  params?: Record<string, string>
}

const props = defineProps<{ initialJob?: any, canControl: boolean }>()
const emit = defineEmits<{
  (e: 'running', value: boolean): void
  (e: 'finished', kind: string): void
}>()

const { t, te } = i18n.global
const job = ref<any>(props.initialJob ?? null)
const bandwidth = ref(false)
const starting = ref('')
const confirmVisible = ref(false)
let pollTimer: ReturnType<typeof setTimeout> | undefined
let unmounted = false

const running = computed(() => job.value?.status === 'running')
const report = computed(() => job.value?.report ?? null)
const before = computed(() => job.value?.kind === 'optimize' ? job.value?.before ?? null : null)
const hasNodes = computed(() => (report.value?.nodes ?? []).length > 0)
const liveLogs = computed<JobLog[]>(() => (job.value?.logs ?? []).slice(-8))
const jobTitle = computed(() => job.value?.kind === 'optimize' ? t('sdwan.tune.optimizeJob') : t('sdwan.tune.detectJob'))
const stageText = computed(() => {
  const key = `sdwan.stage.${job.value?.stage}`
  return te(key) ? t(key) : job.value?.stage ?? ''
})
const severityRank: Record<string, number> = { error: 0, warning: 1, info: 2 }
const sortedAdvice = computed(() => [...(report.value?.advice ?? [])].sort((a: any, b: any) => (severityRank[a.severity] ?? 3) - (severityRank[b.severity] ?? 3)))
const pathRows = computed(() => (report.value?.nodes ?? []).flatMap((node: any) => (node.paths ?? []).map((path: any) => ({ node, path }))))

const protocolNames: Record<string, string> = { reality: 'Reality', hysteria2: 'Hysteria2', shadowsocks: 'SS2022' }
const protocolName = (value: string) => protocolNames[value] ?? value
const prettyProtocols = (value?: string) => (value || '').split('+').filter(Boolean).map(protocolName).join(' + ') || '-'

const formatParams = (params: Record<string, string> = {}) => {
  const values: Record<string, string> = { ...params }
  if (values.protocol) values.protocol = protocolName(values.protocol)
  for (const key of ['protocols', 'current', 'desired']) {
    if (key in values) values[key] = prettyProtocols(values[key])
  }
  for (const name of ['reason', 'error']) {
    const key = `sdwan.reasons.${values[name]}`
    if (values[name] && te(key)) values[name] = t(key)
  }
  if ('node' in values && !values.node) values.node = t('sdwan.tune.controller')
  if ('skew' in values && !values.skew) values.skew = '?'
  return values
}
const logText = (entry: JobLog) => {
  const key = `sdwan.log.${entry.code}`
  return te(key) ? t(key, formatParams(entry.params)) : entry.code
}
const adviceText = (advice: any) => {
  const key = `sdwan.advice.${advice.code}`
  return te(key) ? t(key, formatParams(advice.params)) : advice.code
}
const severityIcon = (severity: string) => severity === 'error' ? 'mdi-alert-circle' : severity === 'warning' ? 'mdi-alert' : 'mdi-information'
const severityColor = (severity: string) => severity === 'error' ? 'error' : severity === 'warning' ? 'warning' : 'info'
const scoreColor = (score: number) => score >= 90 ? 'success' : score >= 60 ? 'warning' : 'error'
const pathStatuses = (path: any) => {
  const statuses: { key: string, color: string }[] = []
  if (path.best) statuses.push({ key: 'best', color: 'success' })
  if (path.selected) statuses.push({ key: 'selected', color: 'primary' })
  if (path.disabled) statuses.push({ key: 'disabled', color: 'grey' })
  if (path.measured && !path.reachable) statuses.push({ key: 'unreachable', color: 'error' })
  if (!path.measured) statuses.push({ key: 'notMeasured', color: 'grey' })
  if (path.speed_error) statuses.push({ key: 'speedFailed', color: 'warning' })
  if (!statuses.length) statuses.push({ key: 'ok', color: 'success' })
  return statuses
}
const formatTime = (seconds?: number) => seconds ? new Date(seconds * 1000).toLocaleString() : '-'
const formatClock = (seconds: number) => new Date(seconds * 1000).toLocaleTimeString()

const stopPolling = () => {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = undefined
}
const poll = async () => {
  pollTimer = undefined
  try {
    const next = await api('api/sdwan/job')
    if (next) job.value = next
  } catch {
    // Keep polling; a restarting core can briefly interrupt the API.
  }
  if (unmounted) return
  if (running.value) {
    pollTimer = setTimeout(poll, 1000)
  } else if (job.value) {
    emit('finished', job.value.kind)
  }
}
const schedulePoll = () => {
  if (!pollTimer && !unmounted) pollTimer = setTimeout(poll, 800)
}

const start = async (kind: 'diagnose' | 'optimize') => {
  confirmVisible.value = false
  if (starting.value || running.value) return
  starting.value = kind
  try {
    job.value = await api(`api/sdwan/${kind}`, { method: 'POST', body: JSON.stringify({ bandwidth: bandwidth.value }) })
    schedulePoll()
  } catch (error: any) {
    push.error({ message: error?.message || t('failed') })
  } finally {
    starting.value = ''
  }
}

watch(running, (value) => emit('running', value), { immediate: true })
// Adopt a job started elsewhere (another tab or administrator).
watch(() => props.initialJob, (next) => {
  if (!next || pollTimer) return
  if (!job.value || next.id !== job.value.id || next.status !== job.value.status) {
    job.value = next
    if (next.status === 'running') schedulePoll()
  }
})

onMounted(() => {
  if (running.value) schedulePoll()
})
onBeforeUnmount(() => {
  unmounted = true
  stopPolling()
})
</script>

<style scoped>
.sdwan-tune-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.sdwan-tune-intro { opacity: .75; margin-bottom: 8px; line-height: 1.55; }
.sdwan-tune-controls { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.sdwan-tune-controls .v-checkbox { flex: 1 1 280px; }
.sdwan-tune-buttons { display: flex; gap: 8px; flex-wrap: wrap; padding-top: 6px; }
.sdwan-tune-progress { margin-top: 16px; }
.sdwan-tune-progress-label { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 6px; font-size: .9rem; }
.sdwan-tune-meta { margin-top: 14px; opacity: .65; font-size: .85rem; }
.sdwan-tune-block { margin-top: 14px; }
.sdwan-tune-block h3 { font-size: .95rem; font-weight: 600; margin-bottom: 6px; }
.sdwan-changes, .sdwan-findings, .sdwan-confirm { list-style: none; padding: 0; margin: 0; }
.sdwan-changes li, .sdwan-findings li { display: flex; align-items: flex-start; gap: 8px; padding: 5px 0; border-top: 1px solid rgba(var(--v-border-color), .08); }
.sdwan-changes li:first-child, .sdwan-findings li:first-child { border-top: 0; }
.sdwan-changes .v-icon, .sdwan-findings .v-icon { margin-top: 2px; flex: none; }
.sdwan-findings li span { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.sdwan-findings .v-chip { flex: none; }
.sdwan-confirm { padding-inline-start: 18px; list-style: disc; }
.sdwan-confirm li { margin: 4px 0; }
.sdwan-ok { display: flex; align-items: center; gap: 6px; margin: 0; }
.sdwan-muted { opacity: .65; }
.sdwan-table-wrap { overflow-x: auto; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; }
.sdwan-table-wrap td, .sdwan-table-wrap th { white-space: nowrap; }
.sdwan-status-chips { display: flex; gap: 4px; flex-wrap: wrap; }
.sdwan-log { list-style: none; padding: 0; margin: 0; font-size: .82rem; line-height: 1.6; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; max-height: 320px; overflow: auto; }
.sdwan-log--live { margin-top: 10px; max-height: none; opacity: .85; }
.sdwan-log li { overflow-wrap: anywhere; }
.sdwan-log time { opacity: .55; margin-inline-end: 6px; }
.sdwan-log--warning { color: rgb(var(--v-theme-warning)); }
.sdwan-log--error { color: rgb(var(--v-theme-error)); }
.sdwan-log--success { color: rgb(var(--v-theme-success)); }
</style>
