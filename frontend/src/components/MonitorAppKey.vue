<template>
  <v-card variant="tonal" class="mt-4">
    <v-card-title>{{ $t('monitorApp.title') }}</v-card-title>
    <v-card-subtitle style="white-space: normal">{{ $t('monitorApp.hint') }}</v-card-subtitle>
    <v-card-text>
      <div class="mb-3">
        <v-chip :color="status.enabled ? 'success' : undefined" size="small">
          {{ status.enabled ? $t('monitorApp.enabled') : $t('monitorApp.disabled') }}
        </v-chip>
        <span v-if="status.enabled && status.created_at" class="text-caption ms-2">
          {{ $t('monitorApp.createdAt') }} {{ new Date(status.created_at * 1000).toLocaleString() }}
        </span>
      </div>
      <template v-if="bindCode">
        <v-alert type="warning" variant="tonal" density="compact" class="mb-3">{{ $t('monitorApp.showOnce') }}</v-alert>
        <v-row>
          <v-col cols="12" md="auto" class="text-center">
            <QrcodeVue :value="bindCode" :size="220" :margin="1" style="border-radius: .75rem" />
          </v-col>
          <v-col cols="12" md>
            <v-text-field :model-value="panelUrl" :label="$t('monitorApp.panelUrl')" readonly hide-details class="mb-3"
              append-inner-icon="mdi-content-copy" @click:append-inner="copy(panelUrl)" />
            <v-text-field :model-value="key" :label="$t('monitorApp.key')" readonly hide-details class="mb-3"
              append-inner-icon="mdi-content-copy" @click:append-inner="copy(key)" />
            <v-text-field :model-value="bindCode" :label="$t('monitorApp.bindCode')" readonly hide-details
              append-inner-icon="mdi-content-copy" @click:append-inner="copy(bindCode)" />
          </v-col>
        </v-row>
      </template>
      <v-divider class="my-3" />
      <div class="text-subtitle-2">{{ $t('monitorApp.permissions') }}</div>
      <div class="text-caption text-medium-emphasis mb-1" style="white-space: normal">{{ $t('monitorApp.permissionsHint') }}</div>
      <v-switch v-model="appSettings.proxies" :label="$t('monitorApp.allowProxies')" color="primary" density="compact" hide-details
        :loading="savingSettings" @update:model-value="saveSettings" />
      <v-switch v-model="appSettings.speedtest" :label="$t('monitorApp.allowSpeedtest')" color="primary" density="compact" hide-details
        :loading="savingSettings" @update:model-value="saveSettings" />
      <v-switch v-model="appSettings.client" :label="$t('monitorApp.allowClient')" color="primary" density="compact" hide-details
        :loading="savingSettings" @update:model-value="saveSettings" />
      <v-text-field v-model.number="appSettings.speedtest_port" type="number" min="1" max="65535" :label="$t('monitorApp.speedtestPort')"
        :hint="$t('monitorApp.speedtestPortHint')" persistent-hint class="mt-3" style="max-width: 360px" dir="ltr" @change="saveSettings" />
    </v-card-text>
    <v-card-actions>
      <v-btn color="primary" variant="flat" :loading="loading" @click="rotate">
        {{ status.enabled ? $t('monitorApp.rotate') : $t('monitorApp.generate') }}
      </v-btn>
      <v-btn v-if="status.enabled" color="error" variant="text" :loading="loading" @click="disable">
        {{ $t('monitorApp.disable') }}
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import QrcodeVue from 'qrcode.vue'
import { push } from 'notivue'
import HttpUtils from '@/plugins/httputil'
import { i18n } from '@/locales'
import { backendBaseUrl, fetchBackendObject } from '@/utils/backend'
import { copyText } from '@/utils/clipboard'

type KeyStatus = { enabled: boolean, created_at: number, proxies: boolean, speedtest: boolean, speedtest_port: number, client: boolean }
const status = ref<KeyStatus>({ enabled: false, created_at: 0, proxies: true, speedtest: true, speedtest_port: 5201, client: true })
// What the key may do besides reading: manage proxy monitors, start speed tests.
const appSettings = ref({ proxies: true, speedtest: true, speedtest_port: 5201, client: true })
const savingSettings = ref(false)
const loading = ref(false)
const key = ref('')
const bindCode = ref('')
const panelUrl = backendBaseUrl()

// The app reads "1sui-monitor:" + base64url(JSON) from the QR code or clipboard.
const encodeBindCode = (url: string, value: string) => {
  const json = JSON.stringify({ v: 1, url, key: value })
  const bytes = new TextEncoder().encode(json)
  let binary = ''
  bytes.forEach(b => { binary += String.fromCharCode(b) })
  return '1sui-monitor:' + btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

const applyStatus = (value: KeyStatus) => {
  status.value = value
  appSettings.value = { proxies: value.proxies, speedtest: value.speedtest, speedtest_port: value.speedtest_port, client: value.client !== false }
}

const load = async () => {
  const msg = await HttpUtils.get('api/monitor-key')
  if (msg.success) applyStatus(msg.obj)
}

const saveSettings = async () => {
  const port = Number(appSettings.value.speedtest_port)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    push.error({ message: i18n.global.t('monitorApp.speedtestPortInvalid'), duration: 3000 })
    return
  }
  savingSettings.value = true
  try {
    applyStatus(await fetchBackendObject<KeyStatus>('api/monitor-key/settings', {
      method: 'POST',
      body: JSON.stringify({ ...appSettings.value, speedtest_port: port }),
    }))
    push.success({ message: i18n.global.t('success'), duration: 2000 })
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('failed'), duration: 4000 })
    await load()
  } finally {
    savingSettings.value = false
  }
}

const rotate = async () => {
  if (status.value.enabled && !confirm(i18n.global.t('monitorApp.rotateConfirm'))) return
  loading.value = true
  const msg = await HttpUtils.post('api/monitor-key', {})
  loading.value = false
  if (!msg.success) return
  key.value = msg.obj.key
  bindCode.value = encodeBindCode(panelUrl, msg.obj.key)
  status.value = { ...status.value, enabled: true, created_at: msg.obj.created_at }
}

const disable = async () => {
  if (!confirm(i18n.global.t('monitorApp.disableConfirm'))) return
  loading.value = true
  const msg = await HttpUtils.post('api/monitor-key/delete', {})
  loading.value = false
  if (!msg.success) return
  key.value = ''
  bindCode.value = ''
  status.value = { ...status.value, enabled: false, created_at: 0 }
}

const copy = async (value: string) => {
  try {
    await copyText(value)
    push.success({ message: i18n.global.t('success') + ': ' + i18n.global.t('copyToClipboard'), duration: 3000 })
  } catch {
    push.error({ message: i18n.global.t('failed') + ': ' + i18n.global.t('copyToClipboard'), duration: 3000 })
  }
}

onMounted(load)
</script>
