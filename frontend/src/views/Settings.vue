<template>
  <v-card :loading="loading">
    <v-tabs
    v-model="tab"
    color="primary"
    align-tabs="center"
    show-arrows
  >
    <v-tab value="t1">{{ $t('setting.interface') }}</v-tab>
    <v-tab value="t2">{{ $t('setting.sub') }}</v-tab>
    <v-tab value="t3">{{ $t('setting.jsonSub') }}</v-tab>
    <v-tab value="t4">{{ $t('setting.clashSub') }}</v-tab>
    <v-tab value="t5">{{ $t('setting.network') }}</v-tab>
  </v-tabs>
  <v-card-text>
    <v-row align="center" justify="center" style="margin-bottom: 10px;">
      <v-col cols="auto">
        <v-btn color="primary" @click="save" :loading="loading" :disabled="!stateChange">
          {{ $t('actions.save') }}
        </v-btn>
      </v-col>
      <v-col cols="auto">
        <v-btn variant="outlined" color="warning" @click="restartApp" :loading="loading" :disabled="stateChange">
          {{ $t('actions.restartApp') }}
        </v-btn>
      </v-col>
    </v-row>
    <v-window v-model="tab">
      <v-window-item value="t1">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model.number="webPort" min="1" type="number" :label="$t('setting.port')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webPath" :label="$t('setting.webPath')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webURI" :label="$t('setting.webUri')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="sessionMaxAge"
              min="0"
              :label="$t('setting.sessionAge')"
              :suffix="$t('date.m')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="trafficAge"
              min="0"
              :label="$t('setting.trafficAge')"
              :suffix="$t('date.d')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="statsBucketSeconds"
              min="1"
              :label="$t('setting.statsBucketSeconds')"
              :suffix="$t('date.s')"
              v-tooltip:top="$t('setting.statsBucketSecondsHint')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="settings.globalReset"
              :label="$t('setting.globalReset')"
              v-tooltip:top="$t('setting.globalResetHint')"
              hide-details
              placeholder="0 0 1 * *"></v-text-field>
          </v-col>
        </v-row>

        <v-divider class="my-4" opacity="40"></v-divider>
        <div class="text-subtitle-2 font-weight-bold mb-3" style="letter-spacing: 0.02em;">{{ $t('setting.uiCustomization') }}</div>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.menuPosition') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in menuPositionOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': menuPositionModel === option.value }"
                  @click="menuPositionModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.uiDensity') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in uiDensityOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': uiDensityModel === option.value }"
                  @click="uiDensityModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.bgPreset') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in bgPresetOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': bgPresetModel === option.value }"
                  @click="bgPresetModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel === 'custom'">
            <v-text-field
              v-model="bgImageModel"
              :label="$t('setting.bgImage')"
              :placeholder="$t('setting.bgImagePlaceholder')"
              hide-details
              clearable
              @click:clear="bgImageModel = ''"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel === 'custom'">
            <v-file-input
              accept="image/png,image/jpeg,image/webp,image/gif"
              prepend-icon="mdi-image-plus"
              :label="$t('setting.bgUpload')"
              hide-details
              clearable
              @update:model-value="handleBgFile"
            ></v-file-input>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="previewBackground">
            <v-img :src="previewBackground" height="76" cover class="rounded-lg app-bg-preview" />
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.bgFit') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in bgFitOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': bgFitModel === option.value }"
                  @click="bgFitModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.bgPosition') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in bgPositionOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': bgPositionModel === option.value }"
                  @click="bgPositionModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-range-field">
              <div class="ui-range-header">
                <span>{{ $t('setting.bgBlur') || 'Background Blur' }}</span>
                <span class="ui-range-value">{{ bgBlurModel }}px</span>
              </div>
              <input
                v-model.number="bgBlurModel"
                class="ui-range"
                type="range"
                min="0"
                max="20"
                step="1"
                :style="rangeStyle(bgBlurModel, 0, 20)"
              />
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-range-field">
              <div class="ui-range-header">
                <span>{{ $t('setting.bgOpacity') || 'Background Opacity' }}</span>
                <span class="ui-range-value">{{ bgOpacityModel }}%</span>
              </div>
              <input
                v-model.number="bgOpacityModel"
                class="ui-range"
                type="range"
                min="5"
                max="100"
                step="1"
                :style="rangeStyle(bgOpacityModel, 5, 100)"
              />
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-range-field">
              <div class="ui-range-header">
                <span>{{ $t('setting.bgSaturate') }}</span>
                <span class="ui-range-value">{{ bgSaturateModel }}%</span>
              </div>
              <input
                v-model.number="bgSaturateModel"
                class="ui-range"
                type="range"
                min="50"
                max="180"
                step="5"
                :style="rangeStyle(bgSaturateModel, 50, 180)"
              />
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-restore" @click="resetUiPrefs">
              {{ $t('setting.resetUi') }}
            </v-btn>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t2">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subEncode" :label="$t('setting.subEncode')" hide-details />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subShowInfo" :label="$t('setting.subInfo')" hide-details />
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subPort"
              min="1"
              :label="$t('setting.port')"
              hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subPath" :label="$t('setting.path')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subUpdates"
              min="0"
              :label="$t('setting.update')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subURI" :label="$t('setting.subUri')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t3">
        <SubJsonExtVue :settings="settings" />
      </v-window-item>

      <v-window-item value="t4">
        <SubClashExtVue :settings="settings" />
      </v-window-item>

      <v-window-item value="t5">
        <v-card variant="tonal" class="mb-4">
          <v-card-title>{{ $t('setting.congestion') }}</v-card-title>
          <v-card-text>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-select
                  v-model="bbrVersion"
                  :label="$t('setting.bbrVersion')"
                  :items="bbrOptions"
                  item-title="title"
                  item-value="value"
                  hide-details
                ></v-select>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-select
                  v-model="qdisc"
                  :label="$t('setting.qdisc')"
                  :items="qdiscOptions"
                  item-title="title"
                  item-value="value"
                  hide-details
                ></v-select>
              </v-col>
            </v-row>
            <v-row>
              <v-col cols="12">
                <v-btn
                  color="primary"
                  variant="tonal"
                  :loading="sysctlLoading"
                  @click="applyCongestion"
                >
                  <v-icon start icon="mdi-cog"></v-icon>
                  {{ $t('setting.applySysctl') }}
                </v-btn>
                <v-chip v-if="sysctlResult" :color="sysctlError ? 'error' : 'success'" class="ml-2" size="small">
                  {{ sysctlResult }}
                </v-chip>
              </v-col>
            </v-row>
            <v-row v-if="sysctlMessages.length > 0">
              <v-col cols="12">
                <v-card variant="outlined" density="compact">
                  <v-card-text>
                    <div v-for="msg in sysctlMessages" :key="msg" class="text-caption">{{ msg }}</div>
                  </v-card-text>
                </v-card>
              </v-col>
            </v-row>
          </v-card-text>
        </v-card>
      </v-window-item>
    </v-window>
  </v-card-text>
</v-card>
</template>

<script lang="ts" setup>
import { i18n } from '@/locales'
import { Ref, computed, inject, onMounted, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import SubJsonExtVue from '@/components/SubJsonExt.vue'
import SubClashExtVue from '@/components/SubClashExt.vue'
import { push } from 'notivue'
import bgAsset from '@/assets/bg.jpg'
const tab = ref("t1")

const uiPreferenceEvent = 'ui-preferences-changed'
const notifyUiPrefs = () => window.dispatchEvent(new Event(uiPreferenceEvent))

type UiPrefs = {
  menuPosition: string
  uiDensity: string
  bgPreset: string
  bgImage: string
  bgBlur: string
  bgOpacity: string
  bgSaturate: string
  bgFit: string
  bgPosition: string
}

const uiPrefChoices = {
  menuPosition: ['side', 'top'],
  uiDensity: ['comfortable', 'compact'],
  bgPreset: ['default', 'none', 'custom'],
  bgFit: ['cover', 'contain', 'auto'],
  bgPosition: ['center', 'center top', 'center bottom'],
} as const

const uiPrefValue = (value: unknown) => {
  if (value && typeof value === 'object' && 'value' in value) {
    return String((value as { value?: unknown }).value ?? '')
  }
  return String(value ?? '')
}

const normalizeUiChoice = (value: unknown, fallback: string, choices: readonly string[]) => {
  const next = uiPrefValue(value)
  return choices.includes(next) ? next : fallback
}

const readUiPrefs = (): UiPrefs => ({
  menuPosition: normalizeUiChoice(localStorage.getItem('menuPosition'), 'side', uiPrefChoices.menuPosition),
  uiDensity: normalizeUiChoice(localStorage.getItem('uiDensity'), 'comfortable', uiPrefChoices.uiDensity),
  bgPreset: normalizeUiChoice(localStorage.getItem('bgPreset'), localStorage.getItem('bgImage') ? 'custom' : 'default', uiPrefChoices.bgPreset),
  bgImage: localStorage.getItem('bgImage') || '',
  bgBlur: localStorage.getItem('bgBlur') || '6',
  bgOpacity: localStorage.getItem('bgOpacity') || '40',
  bgSaturate: localStorage.getItem('bgSaturate') || '1.3',
  bgFit: normalizeUiChoice(localStorage.getItem('bgFit'), 'cover', uiPrefChoices.bgFit),
  bgPosition: normalizeUiChoice(localStorage.getItem('bgPosition'), 'center', uiPrefChoices.bgPosition),
})

const uiPrefs = ref<UiPrefs>(readUiPrefs())

const setUiPref = (key: keyof UiPrefs, value: unknown) => {
  const next = uiPrefValue(value)
  uiPrefs.value = { ...uiPrefs.value, [key]: next }
  if (next) localStorage.setItem(key, next)
  else localStorage.removeItem(key)
  notifyUiPrefs()
}

const menuPositionModel = computed({
  get: () => uiPrefs.value.menuPosition,
  set: (v: unknown) => setUiPref('menuPosition', normalizeUiChoice(v, 'side', uiPrefChoices.menuPosition))
})
const menuPositionOptions = [
  { title: i18n.global.t('setting.menuSide'), value: 'side' },
  { title: i18n.global.t('setting.menuTop'), value: 'top' },
]
const uiDensityModel = computed({
  get: () => uiPrefs.value.uiDensity,
  set: (v: unknown) => setUiPref('uiDensity', normalizeUiChoice(v, 'comfortable', uiPrefChoices.uiDensity))
})
const uiDensityOptions = [
  { title: i18n.global.t('setting.uiDensityComfortable'), value: 'comfortable' },
  { title: i18n.global.t('setting.uiDensityCompact'), value: 'compact' },
]
const bgPresetModel = computed({
  get: () => uiPrefs.value.bgPreset,
  set: (v: unknown) => setUiPref('bgPreset', normalizeUiChoice(v, 'default', uiPrefChoices.bgPreset))
})
const bgPresetOptions = [
  { title: i18n.global.t('setting.bgPresetDefault'), value: 'default' },
  { title: i18n.global.t('setting.bgPresetNone'), value: 'none' },
  { title: i18n.global.t('setting.bgPresetCustom'), value: 'custom' },
]
const bgImageModel = computed({
  get: () => uiPrefs.value.bgImage,
  set: (v: string) => {
    setUiPref('bgImage', v)
    if (v) setUiPref('bgPreset', 'custom')
  }
})
const bgBlurModel = computed({
  get: () => parseInt(uiPrefs.value.bgBlur || '6'),
  set: (v: number) => setUiPref('bgBlur', String(v))
})
const bgOpacityModel = computed({
  get: () => parseInt(uiPrefs.value.bgOpacity || '40'),
  set: (v: number) => setUiPref('bgOpacity', String(v))
})
const bgSaturateModel = computed({
  get: () => Math.round(parseFloat(uiPrefs.value.bgSaturate || '1.3') * 100),
  set: (v: number) => setUiPref('bgSaturate', String(v / 100))
})
const bgFitModel = computed({
  get: () => uiPrefs.value.bgFit,
  set: (v: unknown) => setUiPref('bgFit', normalizeUiChoice(v, 'cover', uiPrefChoices.bgFit))
})
const bgFitOptions = [
  { title: i18n.global.t('setting.bgFitCover'), value: 'cover' },
  { title: i18n.global.t('setting.bgFitContain'), value: 'contain' },
  { title: i18n.global.t('setting.bgFitAuto'), value: 'auto' },
]
const bgPositionModel = computed({
  get: () => uiPrefs.value.bgPosition,
  set: (v: unknown) => setUiPref('bgPosition', normalizeUiChoice(v, 'center', uiPrefChoices.bgPosition))
})
const bgPositionOptions = [
  { title: i18n.global.t('setting.bgPositionCenter'), value: 'center' },
  { title: i18n.global.t('setting.bgPositionTop'), value: 'center top' },
  { title: i18n.global.t('setting.bgPositionBottom'), value: 'center bottom' },
]
const previewBackground = computed(() => {
  if (bgPresetModel.value === 'none') return ''
  if (bgPresetModel.value === 'custom') return bgImageModel.value
  return bgAsset
})
const handleBgFile = (value: File | File[] | undefined) => {
  const file = Array.isArray(value) ? value[0] : value
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    bgImageModel.value = String(reader.result || '')
  }
  reader.readAsDataURL(file)
}
const resetUiPrefs = () => {
  ;['menuPosition', 'bgPreset', 'bgImage', 'bgBlur', 'bgOpacity', 'bgSaturate', 'bgFit', 'bgPosition', 'uiDensity'].forEach((key) => {
    localStorage.removeItem(key)
  })
  localStorage.setItem('uiStyle', 'glass')
  uiPrefs.value = readUiPrefs()
  notifyUiPrefs()
}
const rangeStyle = (value: number, min: number, max: number) => {
  const percent = Math.min(100, Math.max(0, ((value - min) / (max - min)) * 100))
  return { '--ui-range-percent': `${percent}%` }
}
const loading:Ref = inject('loading')?? ref(false)
const oldSettings = ref({})

const settings = ref({
	webListen: "",
	webDomain: "",
	webPort: "2095",
	webCertFile: "",
	webKeyFile: "",
  webPath: "/app/",
  webURI: "",
	sessionMaxAge: "0",
  trafficAge: "30",
  statsBucketSeconds: "60",
	timeLocation: "Asia/Shanghai",
  subListen: "",
	subPort: "2096",
	subPath: "/sub/",
	subDomain: "",
	subCertFile: "",
	subKeyFile: "",
	subUpdates: "12",
	subEncode: "true",
	subShowInfo: "false",
	subURI: "",
  subJsonExt: "",
  subClashExt: "",
  subClashNoDefGrp: "false",
  subClashSprtAll: "false",
  globalReset: "",
  congestionAlgo: "",
  qdisc: "",
})

onMounted(async () => {
  loading.value = true
  await loadData()
  loading.value = false
})

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/settings')
  loading.value = false
  if (msg.success) {
    setData(msg.obj)
  }
}

const setData = (data: any) => {
  settings.value = data
  oldSettings.value = { ...data }
}

const save = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/save', { object: 'settings', action: 'set', data: JSON.stringify(settings.value) })
  if (msg.success) {
    push.success({
      title: i18n.global.t('success'),
      duration: 5000,
      message: i18n.global.t('actions.set') + " " + i18n.global.t('pages.settings')
    })
    setData(msg.obj.settings)
  }
  loading.value = false
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const restartApp = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/restartApp',{})
  if (msg.success) {
    let url = settings.value.webURI
    if (url !== "") {
      const isTLS = settings.value.webCertFile !== "" || settings.value.webKeyFile !== ""
      url = buildURL(settings.value.webDomain,settings.value.webPort.toString(),isTLS, settings.value.webPath)
    }
    await sleep(3000)
    window.location.replace(url)
  }
  loading.value = false
}

const buildURL = (host: string, port: string, isTLS: boolean, path: string) => {
  if (!host || host.length == 0) host = window.location.hostname
  if (!port || port.length == 0) port = window.location.port

  const protocol = isTLS ? "https:" : "http:"

  if (port === "" || (isTLS && port === "443") || (!isTLS && port === "80")) {
      port = ""
  } else {
      port = `:${port}`
  }

  return `${protocol}//${host}${port}${path}settings`
}

const subEncode = computed({
  get: () => { return settings.value.subEncode == "true" },
  set: (v:boolean) => { settings.value.subEncode = v ? "true" : "false" }
})

const subShowInfo = computed({
  get: () => { return settings.value.subShowInfo == "true" },
  set: (v:boolean) => { settings.value.subShowInfo = v ? "true" : "false" }
})

const webPort = computed({
  get: () => { return settings.value.webPort.length>0 ? parseInt(settings.value.webPort) : 2095 },
  set: (v:number) => { settings.value.webPort = v>0 ? v.toString() : "2095" }
})

const sessionMaxAge = computed({
  get: () => { return settings.value.sessionMaxAge.length>0 ? parseInt(settings.value.sessionMaxAge) : 0 },
  set: (v:number) => { settings.value.sessionMaxAge = v>0 ? v.toString() : "0" }
})

const trafficAge = computed({
  get: () => { return settings.value.trafficAge.length>0 ? parseInt(settings.value.trafficAge) : 0 },
  set: (v:number) => { settings.value.trafficAge = v>0 ? v.toString() : "0" }
})

const statsBucketSeconds = computed({
  get: () => { return settings.value.statsBucketSeconds.length>0 ? parseInt(settings.value.statsBucketSeconds) : 60 },
  set: (v:number) => { settings.value.statsBucketSeconds = v>0 ? v.toString() : "60" }
})

const subPort = computed({
  get: () => { return settings.value.subPort.length>0 ? parseInt(settings.value.subPort) : 2096 },
  set: (v:number) => { settings.value.subPort = v>0 ? v.toString() : "2096" }
})

const subUpdates = computed({
  get: () => { return settings.value.subUpdates.length>0 ? parseInt(settings.value.subUpdates) : 12 },
  set: (v:number) => { settings.value.subUpdates = v>0 ? v.toString() : "12" }
})

const stateChange = computed(() => {
  return !FindDiff.deepCompare(settings.value,oldSettings.value)
})

const sysctlLoading = ref(false)
const sysctlResult = ref('')
const sysctlError = ref(false)
const sysctlMessages = ref<string[]>([])

const bbrOptions = [
  { title: 'BBR v1', value: 'bbr' },
  { title: 'BBR v2', value: 'bbr2' },
  { title: 'BBR v3', value: 'bbr3' },
  { title: 'BBR v2 Plus', value: 'bbr2plus' },
  { title: 'BBR Plus', value: 'bbrplus' },
  { title: 'Cubic (默认)', value: 'cubic' },
]

const qdiscOptions = [
  { title: 'FQ (Fair Queue)', value: 'fq' },
  { title: 'CAKE', value: 'cake' },
  { title: '默认 (pfifo_fast)', value: '' },
]

const bbrVersion = computed({
  get: () => settings.value.congestionAlgo ?? 'bbr',
  set: (v: string) => { settings.value.congestionAlgo = v }
})

const qdisc = computed({
  get: () => settings.value.qdisc ?? '',
  set: (v: string) => { settings.value.qdisc = v }
})

const applyCongestion = async () => {
  sysctlLoading.value = true
  sysctlResult.value = ''
  sysctlError.value = false
  sysctlMessages.value = []

  const algo = bbrVersion.value || 'bbr'
  const qdiscVal = qdisc.value
  const msg = await HttpUtils.post('api/setSysctl', { congestionAlgo: algo, qdisc: qdiscVal })

  sysctlLoading.value = false
  if (msg.success) {
    sysctlResult.value = i18n.global.t('success')
    sysctlError.value = false
    sysctlMessages.value = msg.obj ?? []
    settings.value.congestionAlgo = algo
    settings.value.qdisc = qdiscVal
  } else {
    sysctlResult.value = i18n.global.t('failed') || 'Failed'
    sysctlError.value = true
    sysctlMessages.value = Array.isArray(msg.obj) ? msg.obj : (msg.msg ? msg.msg.split('\n') : [])
  }
}
</script>

<style scoped>
.ui-choice-field,
.ui-range-field {
  min-height: 64px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  border-radius: 10px;
  background: rgba(var(--v-theme-surface), 0.72);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  padding: 8px;
}

.ui-choice-label,
.ui-range-header {
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 12px;
  line-height: 1.2;
  margin-bottom: 7px;
}

.ui-choice-group {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(0, 1fr);
  gap: 4px;
}

.ui-choice-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
  height: 32px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: rgba(var(--v-theme-on-surface), 0.74);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.1;
  padding: 0 8px;
  transition: background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
  white-space: nowrap;
}

.ui-choice-button:hover {
  background: rgba(var(--v-theme-primary), 0.08);
}

.ui-choice-button.is-active {
  background: rgba(var(--v-theme-primary), 0.92);
  color: rgb(var(--v-theme-on-primary));
  box-shadow: 0 6px 16px rgba(var(--v-theme-primary), 0.22);
}

.ui-range-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 9px;
}

.ui-range-value {
  flex: 0 0 auto;
  min-width: 48px;
  border-radius: 999px;
  background: rgba(var(--v-theme-primary), 0.14);
  color: rgb(var(--v-theme-primary));
  font-size: 12px;
  font-weight: 700;
  padding: 3px 8px;
  text-align: center;
}

.ui-range {
  --ui-range-percent: 0%;
  display: block;
  width: 100%;
  height: 24px;
  margin: 0;
  appearance: none;
  background: transparent;
  cursor: pointer;
}

.ui-range::-webkit-slider-runnable-track {
  height: 6px;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--v-theme-primary)) 0 var(--ui-range-percent),
    rgba(var(--v-theme-on-surface), 0.16) var(--ui-range-percent) 100%
  );
}

.ui-range::-webkit-slider-thumb {
  width: 18px;
  height: 18px;
  border: 3px solid rgb(var(--v-theme-surface));
  border-radius: 50%;
  appearance: none;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  margin-top: -6px;
}

.ui-range::-moz-range-track {
  height: 6px;
  border-radius: 999px;
  background: rgba(var(--v-theme-on-surface), 0.16);
}

.ui-range::-moz-range-progress {
  height: 6px;
  border-radius: 999px;
  background: rgb(var(--v-theme-primary));
}

.ui-range::-moz-range-thumb {
  width: 14px;
  height: 14px;
  border: 3px solid rgb(var(--v-theme-surface));
  border-radius: 50%;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.ui-range:focus-visible {
  outline: 2px solid rgba(var(--v-theme-primary), 0.35);
  outline-offset: 4px;
}

@media (max-width: 560px) {
  .ui-choice-group {
    grid-auto-flow: row;
    grid-auto-columns: unset;
  }

  .ui-choice-button {
    justify-content: center;
    white-space: normal;
  }
}
</style>
