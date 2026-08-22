<template>
  <v-dialog transition="dialog-bottom-transition" width="90%" max-width="500">
    <v-card class="rounded-lg">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('main.backup.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto">
            <v-icon icon="mdi-close" @click="control.visible = false" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclStats')" value="stats" hide-details></v-checkbox>
          </v-col>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclChanges')" value="changes" hide-details></v-checkbox>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="backup()" hide-details>{{ $t('main.backup.backup') }}</v-btn>
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="restore()" hide-details>{{ $t('main.backup.restore') }}</v-btn>
          </v-col>
        </v-row>
        <v-row>
          <v-divider></v-divider>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="config()" hide-details>{{ $t('main.backup.sbConfig') }}</v-btn>
          </v-col>
          <v-col cols="auto" align-self="center" v-if="!isOpenWrtLite">
            <v-btn color="primary" variant="tonal" @click="xrayConfig()" hide-details>{{ $t('main.backup.xrayConfig') }}</v-btn>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { downloadBackendFile } from '@/utils/backend'
export default {
  props: ['control', 'visible'],
  data() {
    return {
      exclude: ["stats", "changes"],
      isOpenWrtLite: import.meta.env.VITE_OPENWRT_LITE === 'true',
    }
  },
  methods: {
    async backup() {
      const excludeOption = this.exclude.length>0 ? '?exclude=' +this.exclude.join(',') : ''
      await this.download('api/getdb' + excludeOption, '1s-ui-backup.db')
    },
    async config() {
      await this.download('api/singbox-config', 'sing-box-config.json')
    },
    async xrayConfig() {
      await this.download('api/xray-config', 'xray-config.json')
    },
    async download(path: string, fallbackName: string) {
      try {
        await downloadBackendFile(path, fallbackName)
      } catch (error: any) {
        push.error({ message: error?.message || i18n.global.t('failed') })
      }
    },
    restore() {
      const fileInput = document.createElement('input')
      fileInput.type = 'file'
      fileInput.accept = '.db'

      fileInput.addEventListener('change', async (event: Event) => {
        const inputElement = event.target as HTMLInputElement
        const dbFile = inputElement.files ? inputElement.files[0] : null

        if (dbFile) {
          const formData = new FormData()
          formData.append('db', dbFile)

          this.control.visible = false

          const uploadMsg = await HttpUtils.post('api/importdb', formData, {
              headers: {
                  'Content-Type': 'multipart/form-data',
              },
          })

          if (uploadMsg.success) {
            await new Promise(resolve => setTimeout(resolve, 1000))
            location.reload()
          }
        }
    })

    fileInput.click()
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.exclude = ["stats", "changes"]
      }
    },
  },
}
</script>
