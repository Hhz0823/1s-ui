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
            <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details></v-text-field>
          </v-col>
        </v-row>

        <v-divider class="my-4" opacity="40"></v-divider>
        <div class="text-subtitle-2 font-weight-bold mb-3" style="letter-spacing: 0.02em;">{{ $t('setting.uiCustomization') }}</div>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
              v-model="menuPositionModel"
              :items="menuPositionOptions"
              :label="$t('setting.menuPosition')"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="bgImageModel"
              :label="$t('setting.bgImage')"
              :placeholder="$t('setting.bgImagePlaceholder')"
              hide-details
              clearable
              @click:clear="bgImageModel = ''"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgImageModel">
            <v-img :src="bgImageModel" max-height="60" max-width="120" cover class="rounded-lg" style="border: 1px solid rgba(var(--v-theme-on-surface),0.1);" />
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
const tab = ref("t1")

const menuPositionModel = computed({
  get: () => localStorage.getItem('menuPosition') || 'side',
  set: (v: string) => { localStorage.setItem('menuPosition', v); location.reload() }
})
const menuPositionOptions = [
  { title: i18n.global.t('setting.menuSide'), value: 'side' },
  { title: i18n.global.t('setting.menuTop'), value: 'top' },
]
const bgImageModel = computed({
  get: () => localStorage.getItem('bgImage') || '',
  set: (v: string) => { if (v) localStorage.setItem('bgImage', v); else localStorage.removeItem('bgImage') }
})
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
    sysctlResult.value = msg.msg || 'Failed'
    sysctlError.value = true
  }
}
</script>