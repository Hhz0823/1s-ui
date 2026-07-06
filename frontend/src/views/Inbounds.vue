<template>
  <v-dialog v-model="quickAdd.visible" transition="dialog-bottom-transition" width="min(560px, calc(100vw - 24px))">
    <v-card class="rounded-lg">
      <v-card-title>{{ $t('pages.quickAddNode') }}</v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="12">
            <v-select
              v-model="quickAdd.core_type"
              label="Core"
              :items="coreOptions"
              item-title="title"
              item-value="value"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12">
            <v-select
              v-model="quickAdd.protocol"
              :label="$t('pages.selectProtocol')"
              :items="protocolOptions"
              item-title="title"
              item-value="value"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12">
            <v-text-field
              v-model="quickAdd.tag"
              :label="$t('objects.tag')"
              hide-details
              readonly
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="regenerateQuickAdd" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12">
            <v-text-field
              v-model.number="quickAdd.port"
              :label="$t('in.port')"
              type="number"
              hide-details
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="quickAdd.port = RandomUtil.randomIntRange(10000, 60000)" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasPassword">
            <v-text-field
              v-model="quickAdd.password"
              :label="$t('types.pw')"
              hide-details
              readonly
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="quickAdd.password = randomPasswordForMethod(quickAdd.method)" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasMethod">
            <v-select
              v-model="quickAdd.method"
              :label="$t('in.ssMethod')"
              :items="shadowsocksMethods"
              @update:model-value="quickAdd.password = randomPasswordForMethod($event)"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasObfs">
            <v-text-field
              v-model="quickAdd.obfsPassword"
              :label="$t('types.hy.obfs')"
              hide-details
              readonly
            >
              <template v-slot:append-inner>
                <v-icon icon="mdi-refresh" @click="quickAdd.obfsPassword = RandomUtil.randomShadowsocksPassword(16)" style="cursor: pointer;" />
              </template>
            </v-text-field>
          </v-col>
          <v-col cols="12" v-if="quickAdd.hasHandshake">
            <v-text-field
              v-model="quickAdd.handshakeServer"
              :label="$t('types.shdwTls.hs')"
              hide-details
            ></v-text-field>
          </v-col>
        </v-row>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn color="primary" variant="outlined" @click="quickAdd.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="quickAdd.loading" @click="createQuickNode">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <InboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :inTags="inTags"
    :tlsConfigs="tlsConfigs"
    @close="closeModal"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <v-row class="page-toolbar" align="center" justify="start">
    <v-col cols="auto" class="page-toolbar__actions">
      <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
      <v-btn color="primary" variant="tonal" class="ml-2" @click="openQuickAdd">
        <v-icon start icon="mdi-lightning-bolt"></v-icon>
        {{ $t('pages.quickAddNode') }}
      </v-btn>
    </v-col>
  </v-row>
  <v-row class="resource-grid">
    <v-col cols="12" sm="6" md="4" lg="3" xl="2" v-for="(item, index) in <any[]>inbounds" :key="item.tag" class="resource-col">
      <v-card rounded="lg" elevation="1" :title="item.tag" class="resource-card">
        <v-card-subtitle>{{ item.core_type || 'sing-box' }} / {{ item.type }}</v-card-subtitle>
        <v-card-text class="resource-card__body">
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('in.addr') }}</v-col>
            <v-col cols="7" class="resource-value">
              {{ item.listen }}
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('in.port') }}</v-col>
            <v-col cols="7" class="resource-value">
              {{ item.listen_port }}
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('objects.tls') }}</v-col>
            <v-col cols="7" class="resource-value">
              {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('pages.clients') }}</v-col>
            <v-col cols="7" class="resource-value">
              <template v-if="item.users">
                <v-tooltip activator="parent" dir="ltr" location="bottom" v-if="item.users.length > 0">
                  <span v-for="u in item.users" :key="u">{{ u }}<br /></span>
                </v-tooltip>
                {{ item.users.length }}
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row class="resource-row" no-gutters>
            <v-col cols="5" class="resource-label">{{ $t('online') }}</v-col>
            <v-col cols="7" class="resource-value">
              <template v-if="onlines.includes(item.tag)">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions class="resource-actions">
          <v-btn icon="mdi-file-edit" size="small" variant="text" @click="showModal(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" size="small" variant="text" color="warning" @click="delOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delInbound(item.id)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-content-duplicate" size="small" variant="text" :loading="cloneLoading" @click="clone(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.clone')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-chart-line" size="small" variant="text" @click="showStats(item.tag)" v-if="Data().enableTraffic">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config } from '@/types/config'
import { computed, ref, watch } from 'vue'
import { CoreTypes, createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import { i18n } from '@/locales'
import { push } from 'notivue'

const isOpenWrtLite = import.meta.env.VITE_OPENWRT_LITE === 'true'

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> Data().inbounds
})

const tlsConfigs = computed((): any[] => {
  return <any[]> Data().tlsConfigs
})

const inTags = computed((): string[] => {
  return [...inbounds.value?.map(i => i.tag), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

const onlines = computed(() => {
  return Data().onlines.inbound?? []
})

const modal = ref({
  visible: false,
  id: 0,
})

let delOverlay = ref(new Array<boolean>)

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const quickAdd = ref({
  visible: false,
  core_type: CoreTypes.SingBox,
  protocol: 'mixed',
  tag: '',
  port: RandomUtil.randomIntRange(10000, 60000),
  password: '',
  method: '2022-blake3-aes-256-gcm',
  obfsPassword: '',
  handshakeServer: 'www.microsoft.com',
  hasPassword: false,
  hasMethod: false,
  hasObfs: false,
  hasHandshake: false,
  loading: false,
})

const coreOptions = computed(() => {
  const items = [{ title: 'sing-box', value: CoreTypes.SingBox }]
  if (!isOpenWrtLite) items.push({ title: 'Xray-core', value: CoreTypes.Xray })
  return items
})

watch(() => quickAdd.value.protocol, (val) => {
  quickAdd.value.hasPassword = val === 'shadowsocks'
  quickAdd.value.hasMethod = val === 'shadowsocks'
  quickAdd.value.hasObfs = val === 'hysteria2'
  quickAdd.value.hasHandshake = val === 'shadowtls'
  regenerateQuickAdd()
})

watch(() => quickAdd.value.core_type, (val) => {
  if (isOpenWrtLite && val !== CoreTypes.SingBox) {
    quickAdd.value.core_type = CoreTypes.SingBox
    return
  }
  if (val === CoreTypes.Xray) {
    quickAdd.value.protocol = 'vless'
  }
  regenerateQuickAdd()
})

const closeModal = () => {
  modal.value.visible = false
}

const delInbound = async (id: number) => {
  const index = inbounds.value.findIndex(i => i.id == id)
  const tag = inbounds.value[index].tag

  const success = await Data().save("inbounds", "del", tag)
  if (success) delOverlay.value[index] = false
}

let cloneLoading = ref(false)

const clone = async (id: number) => {
  cloneLoading.value = true
  const inboundArray = await Data().loadInbounds([id])
  const inbound = inboundArray[0]
  let newTag = inbound.type + "-" + RandomUtil.randomSeq(3)
  const newInbound = createInbound(inbound.type, { ...inbound,
    id: 0,
    tag: newTag,
    listen_port: RandomUtil.randomIntRange(10000, 60000),
  })
  await Data().save("inbounds", "new", newInbound)
  cloneLoading.value = false
}



const singBoxProtocolOptions = [
  { title: 'Mixed', value: 'mixed' },
  { title: 'SOCKS', value: 'socks' },
  { title: 'HTTP', value: 'http' },
  { title: 'Shadowsocks', value: 'shadowsocks' },
  { title: 'VMess', value: 'vmess' },
  { title: 'Trojan', value: 'trojan' },
  { title: 'VLESS', value: 'vless' },
  { title: 'Hysteria2', value: 'hysteria2' },
  { title: 'ShadowTLS', value: 'shadowtls' },
  { title: 'TUIC', value: 'tuic' },
  { title: 'Naive', value: 'naive' },
  { title: 'AnyTLS', value: 'anytls' },
  { title: 'Direct', value: 'direct' },
]

const protocolOptions = computed(() => {
  if (quickAdd.value.core_type === CoreTypes.Xray) {
    return [{ title: 'VLESS', value: 'vless' }]
  }
  return singBoxProtocolOptions
})

const shadowsocksMethods = [
  'aes-128-gcm',
  'aes-192-gcm',
  'aes-256-gcm',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm',
  '2022-blake3-aes-256-gcm',
  '2022-blake3-chacha20-poly1305',
]

const randomPasswordForMethod = (method: string): string => {
  if (method === '2022-blake3-aes-128-gcm') return RandomUtil.randomShadowsocksPassword(16)
  if (method.startsWith('2022')) return RandomUtil.randomShadowsocksPassword(32)
  return RandomUtil.randomSeq(16)
}

const regenerateQuickAdd = () => {
  const port = RandomUtil.randomIntRange(10000, 60000)
  quickAdd.value.tag = quickAdd.value.protocol + '-' + port
  quickAdd.value.port = port
  quickAdd.value.password = randomPasswordForMethod(quickAdd.value.method)
}

const openQuickAdd = () => {
  regenerateQuickAdd()
  quickAdd.value.visible = true
}

const needsTls = ['vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'naive', 'anytls']

const pinnedSha256FromCertificate = async (certificate: string[]): Promise<string[]> => {
  try {
    const resp = await fetch('api/pinnedSha256', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cert: certificate.join('\n') }),
      credentials: 'include',
    })
    const msg = await resp.json()
    if (msg.success && Array.isArray(msg.obj)) return msg.obj
  } catch (e) {
    console.error('pinnedSha256FromCertificate error:', e)
  }
  return []
}

const genSelfSignedTls = async (serverName: string): Promise<number> => {
  let tlsName = 'auto-' + quickAdd.value.tag
  while (Data().tlsConfigs.find((t: any) => t.name === tlsName)) {
    tlsName += '-copy'
  }
  const cleanServerName = (serverName || quickAdd.value.tag).replace(/[^a-zA-Z0-9.-]/g, '-')
  try {
    const keyMsg = await HttpUtils.get('api/keypairs', { k: 'tls', o: cleanServerName })
    if (!keyMsg.success || !keyMsg.obj || !keyMsg.obj.length) return 0
    const lines: string[] = keyMsg.obj.filter((l: string) => l && l.trim())
    if (lines.length < 4) return 0
    const privateKey: string[] = []
    const publicKey: string[] = []
    let inKey = false, inCert = false
    for (const line of lines) {
      const t = line.trim()
      if (!t) continue
      if (t === '-----BEGIN PRIVATE KEY-----') { inKey = true; inCert = false; privateKey.push(t) }
      else if (t === '-----END PRIVATE KEY-----') { inKey = false; privateKey.push(t) }
      else if (t === '-----BEGIN CERTIFICATE-----') { inCert = true; inKey = false; publicKey.push(t) }
      else if (t === '-----END CERTIFICATE-----') { inCert = false; publicKey.push(t) }
      else { if (inKey) privateKey.push(t); if (inCert) publicKey.push(t) }
    }
    if (!privateKey.length || !publicKey.length) return 0
    const pinnedSha256 = await pinnedSha256FromCertificate(publicKey)
    if (!pinnedSha256.length) return 0
    const tlsConfig = {
      id: 0,
      name: tlsName,
      server: {
        enabled: true,
        server_name: cleanServerName,
        alpn: ['h3', 'h2', 'http/1.1'],
        min_version: '1.2',
        max_version: '1.3',
        key: privateKey,
        certificate: publicKey,
      },
      client: {
        enabled: true,
        server_name: cleanServerName,
        pinned_peer_certificate_sha256: pinnedSha256,
        alpn: ['h3', 'h2', 'http/1.1'],
        min_version: '1.2',
        max_version: '1.3',
        utls: {
          enabled: true,
          fingerprint: 'chrome',
        },
      }
    }
    const success = await Data().save('tls', 'new', tlsConfig)
    if (success) {
      const saved = Data().tlsConfigs.find((t: any) => t.name === tlsName)
      if (saved && saved.id) return saved.id
    }
  } catch (e) {
    console.error('genSelfSignedTls error:', e)
  }
  return 0
}

const createQuickNode = async () => {
  quickAdd.value.loading = true
  const port = quickAdd.value.port
  const proto = quickAdd.value.protocol
  const clientName = 'user-' + RandomUtil.randomSeq(6)
  const password = RandomUtil.randomSeq(10)
  const uuid = RandomUtil.randomUUID()

  let tlsId = 0
  if (needsTls.includes(proto)) {
    tlsId = await genSelfSignedTls(quickAdd.value.tag)
    if (tlsId === 0) {
      quickAdd.value.loading = false
      push.error('TLS generation failed. Please create TLS certificate in TLS Settings first.')
      return
    }
  }
  const inbound = createInbound(proto, {
    id: 0,
    core_type: quickAdd.value.core_type,
    tag: quickAdd.value.tag,
    listen: '::',
    listen_port: port,
  } as any)

  switch (proto) {
    case 'shadowsocks':
      ;(inbound as any).method = quickAdd.value.method || '2022-blake3-aes-256-gcm'
      ;(inbound as any).password = quickAdd.value.password
      inbound.addrs = []
      inbound.out_json = {}
      break
    case 'vmess':
      ;(inbound as any).tls_id = tlsId
      ;(inbound as any).transport = { type: 'http' }
      inbound.addrs = []
      inbound.out_json = {}
      break
    case 'vless':
      ;(inbound as any).tls_id = tlsId
      ;(inbound as any).transport = quickAdd.value.core_type === CoreTypes.Xray
        ? { type: 'xhttp', path: '/xhttp', host: location.hostname, mode: 'auto' }
        : { type: 'http' }
      inbound.addrs = []
      inbound.out_json = {}
      break
    case 'trojan':
      ;(inbound as any).tls_id = tlsId
      ;(inbound as any).transport = { type: 'http' }
      inbound.addrs = []
      inbound.out_json = {}
      break
    case 'shadowtls':
      ;(inbound as any).version = 3
      ;(inbound as any).password = RandomUtil.randomShadowsocksPassword(16)
      ;(inbound as any).handshake = { server: quickAdd.value.handshakeServer || 'www.microsoft.com', server_port: 443 }
      break
    case 'hysteria2':
      ;(inbound as any).tls_id = tlsId
      ;(inbound as any).obfs = { type: 'salamander', password: quickAdd.value.obfsPassword || RandomUtil.randomShadowsocksPassword(16) }
      break
    case 'tuic':
      ;(inbound as any).tls_id = tlsId
      ;(inbound as any).congestion_control = 'cubic'
      break
    case 'naive':
      ;(inbound as any).tls_id = tlsId
      break
    case 'anytls':
      ;(inbound as any).tls_id = tlsId
      ;(inbound as any).padding_scheme = [
        'stop=8',
        '0=30-30',
        '1=100-400',
        '2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000',
        '3=9-9,500-1000',
        '4=500-1000',
        '5=500-1000',
        '6=500-1000',
        '7=500-1000'
      ]
      break
    case 'mixed':
    case 'socks':
    case 'http':
      inbound.addrs = []
      inbound.out_json = {}
      break
    case 'direct':
      break
  }

  // Create a default client for protocols that need users
  const needsClient = ['shadowsocks', 'vmess', 'vless', 'trojan', 'naive', 'hysteria2', 'tuic', 'anytls', 'shadowtls']
  let initUsers: number[] | undefined = undefined
  if (needsClient.includes(proto)) {
    const protoConfig: any = {}
    switch (proto) {
      case 'shadowsocks':
        protoConfig.shadowsocks = { name: clientName, password: RandomUtil.randomShadowsocksPassword(32) }
        break
      case 'vmess':
        protoConfig.vmess = { name: clientName, uuid: uuid, alterId: 0 }
        break
      case 'vless':
        protoConfig.vless = { name: clientName, uuid: uuid, flow: quickAdd.value.core_type === CoreTypes.Xray ? '' : 'xtls-rprx-vision' }
        break
      case 'trojan':
        protoConfig.trojan = { name: clientName, password: password }
        break
      case 'naive':
        protoConfig.naive = { username: clientName, password: password }
        break
      case 'hysteria2':
        protoConfig.hysteria2 = { name: clientName, password: password }
        break
      case 'tuic':
        protoConfig.tuic = { name: clientName, uuid: uuid, password: password }
        break
      case 'anytls':
        protoConfig.anytls = { name: clientName, password: password }
        break
      case 'shadowtls':
        protoConfig.shadowtls = { name: clientName, password: RandomUtil.randomShadowsocksPassword(32) }
        break
    }
    const client = {
      enable: true,
      name: clientName,
      config: protoConfig,
      inbounds: [],
      links: [],
      volume: 0,
      expiry: 0,
      up: 0,
      down: 0,
      desc: '',
      group: '',
    }
    const clientBody = new URLSearchParams()
    clientBody.append('object', 'clients')
    clientBody.append('action', 'new')
    clientBody.append('data', JSON.stringify(client))
    try {
      const clientResp = await fetch('api/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: clientBody.toString(),
        credentials: 'include',
      })
      const clientMsg = await clientResp.json()
      if (clientMsg.success && clientMsg.obj && clientMsg.obj.clients) {
        const savedClient = clientMsg.obj.clients.find((c: any) => c.name === clientName)
        if (savedClient && savedClient.id) {
          initUsers = [savedClient.id]
        }
      }
    } catch (e) {
      console.error('Quick add client creation error:', e)
    }
  }

  const success = await Data().save('inbounds', 'new', inbound, initUsers)
  quickAdd.value.loading = false
  if (success) {
    quickAdd.value.visible = false
  }
}

const stats = ref({
  visible: false,
  resource: "inbound",
  tag: "",
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>
