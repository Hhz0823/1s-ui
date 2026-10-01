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
import { backendBaseUrl } from '@/utils/backend'
import { copyText } from '@/utils/clipboard'

const status = ref<{ enabled: boolean, created_at: number }>({ enabled: false, created_at: 0 })
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

const load = async () => {
  const msg = await HttpUtils.get('api/monitor-key')
  if (msg.success) status.value = msg.obj
}

const rotate = async () => {
  if (status.value.enabled && !confirm(i18n.global.t('monitorApp.rotateConfirm'))) return
  loading.value = true
  const msg = await HttpUtils.post('api/monitor-key', {})
  loading.value = false
  if (!msg.success) return
  key.value = msg.obj.key
  bindCode.value = encodeBindCode(panelUrl, msg.obj.key)
  status.value = { enabled: true, created_at: msg.obj.created_at }
}

const disable = async () => {
  if (!confirm(i18n.global.t('monitorApp.disableConfirm'))) return
  loading.value = true
  const msg = await HttpUtils.post('api/monitor-key/delete', {})
  loading.value = false
  if (!msg.success) return
  key.value = ''
  bindCode.value = ''
  status.value = { enabled: false, created_at: 0 }
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
