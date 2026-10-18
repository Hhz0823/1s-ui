<template>
  <v-dialog transition="dialog-bottom-transition" width="400">
    <v-card class="rounded-lg" id="qrcode-modal" :loading="loading">
      <v-card-title>
        <v-row>
          <v-col>QrCode</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close-box" @click="$emit('close')" /></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-skeleton-loader
          class="mx-auto border"
          width="80%"
          type="text, image, divider, text, image"
          v-if="loading"
        ></v-skeleton-loader>
      <v-card-text style="overflow-y: auto; padding: 0" :hidden="loading">
        <v-tabs
          v-model="tab"
          density="compact"
          fixed-tabs
          align-tabs="center"
        >
          <v-tab value="sub">{{ $t('setting.sub') }}</v-tab>
          <v-tab value="link">{{ $t('client.links') }}</v-tab>
        </v-tabs>
        <v-window v-model="tab" style="margin-top: 10px;">
          <v-window-item value="sub">
            <v-row v-for="item in subscriptions" :key="item.label">
              <v-col style="text-align: center;">
                <v-chip>{{ item.label }}</v-chip>
                <div class="qr-hint">{{ item.hint }}</div>
                <QrcodeVue :value="item.value" :size="size" @click="copyToClipboard(item.value)" :margin="1" style="border-radius: 1rem; cursor: copy;" />
              </v-col>
            </v-row>
            <v-row>
              <v-col style="text-align: center;">
                <v-chip>{{ $t('client.singboxImport') }}</v-chip>
                <div class="qr-hint">{{ $t('client.singboxImportHint') }}</div>
                <QrcodeVue :value="singbox" :size="size" @click="copyToClipboard(singbox)" :margin="1" style="border-radius: .8rem; cursor: copy;" />
                <div><v-btn :href="singbox" variant="tonal" size="small" prepend-icon="mdi-open-in-app" class="mt-2">{{ $t('client.openInSingbox') }}</v-btn></div>
              </v-col>
            </v-row>
          </v-window-item>
          <v-window-item value="link">
            <v-row v-for="l in clientLinks">
              <v-col style="text-align: center;">
                <v-chip>{{ linkLabel(l) }}</v-chip><br />
                <QrcodeVue :value="l.uri" :size="size" @click="copyToClipboard(l.uri)" :margin="1" style="border-radius: .5rem; cursor: copy;" />
              </v-col>
            </v-row>
          </v-window-item>
        </v-window>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import QrcodeVue from 'qrcode.vue'
import Data from '@/store/modules/data'
import Clipboard from 'clipboard'
import { i18n } from '@/locales'
import { push } from 'notivue'

export default {
  props: ['id', 'visible'],
  data() {
    return {
      tab: "sub",
      client: <any>{},
      loading: false,
    }
  },
  methods: {
    async load() {
      this.loading = true
      const newData = await Data().loadClients(this.$props.id)
      this.client = newData
      this.loading = false
    },
    copyToClipboard(txt:string) {
      const hiddenButton = document.createElement('button')
      hiddenButton.className = 'clipboard-btn'
      document.body.appendChild(hiddenButton)

      const clipboard = new Clipboard('.clipboard-btn', {
        text: () => txt,
        container: document.getElementById('qrcode-modal')?? undefined
      });

      clipboard.on('success', () => {
        clipboard.destroy()
        push.success({
          message: i18n.global.t('success') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      clipboard.on('error', () => {
        clipboard.destroy()
        push.error({
          message: i18n.global.t('failed') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      // Perform click on hidden button to trigger copy
      hiddenButton.click()
      document.body.removeChild(hiddenButton)
    },
    linkLabel(link:any) {
      const remark = link.remark ?? i18n.global.t('client.' + link.type)
      if (String(link.uri || '').startsWith('v2rayn://naive/')) return `v2rayN · ${remark}`
      if (String(link.uri || '').startsWith('naive+')) return `Naive URL · ${remark}`
      return remark
    }
  },
  computed: {
    clientSub() {
      return Data().subURI + encodeURIComponent(this.client.name ?? '')
    },
    subscriptions() {
      return [
        { label: i18n.global.t('setting.sub'), hint: i18n.global.t('client.subForLinks'), value: this.clientSub },
        { label: i18n.global.t('setting.jsonSub'), hint: i18n.global.t('client.subForJson'), value: this.clientSub + '?format=json' },
        { label: i18n.global.t('setting.clashSub'), hint: i18n.global.t('client.subForClash'), value: this.clientSub + '?format=clash' },
      ]
    },
    // sing-box apps open this link (the desktop app registers sing-box://)
    // and add the JSON subscription as a remote profile.
    singbox() {
      const url = this.clientSub + "?format=json"
      return "sing-box://import-remote-profile?url=" + encodeURIComponent(url) + "#" + encodeURIComponent(this.client.name ?? '')
    },
    clientLinks() {
      return this.client.links?? []
    },
    size() {
      if (window.innerWidth > 380) return 300
      if (window.innerWidth > 330) return 280
      return 250
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.tab = "sub"
        this.load()
      }
    },
  },
  components: { QrcodeVue }
}
</script>

<style scoped>
.qr-hint {
  font-size: 0.75rem;
  opacity: 0.7;
  margin: 4px auto 6px;
  max-width: 300px;
  line-height: 1.35;
}
</style>
