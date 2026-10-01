import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { Inbound } from '@/types/inbounds'
import { Client } from '@/types/clients'
import { resolveFrontendUrl } from '@/utils/backend'

let pendingLoadData: Promise<void> | null = null
let pendingControllerMode: Promise<boolean> | null = null

const emptyControllerMode = () => ({
  profile: 'client',
  enabled: false,
  monitor_only: false,
  can_control: true,
  configured: 'auto',
  inherited: false,
  can_enable: false,
  agent_count: 0,
  cpu_cores: 0,
  memory_bytes: 0,
  min_cpu_cores: 2,
  min_memory_bytes: 2 * 1024 ** 3,
  lite: false,
  singbox_only: false,
  lite_min_cpu_cores: 1,
  lite_min_memory_bytes: 400 * 1024 ** 2,
})

const Data = defineStore('Data', {
  state: () => ({ 
    lastLoad: 0,
    reloadItems: localStorage.getItem("reloadItems")?.split(',')?? <string[]>[],
    subURI: "",
    enableTraffic: false,
    onlines: {inbound: <string[]>[], outbound: <string[]>[], user: <string[]>[]},
    config: <any>{},
    inbounds: <any[]>[],
    outbounds: <any[]>[],
    services: <any[]>[],
    endpoints: <any[]>[],
    clients: <any>[],
    tlsConfigs: <any[]>[],
    hostRequirements: <any>null,
    // Set when this page is older than the panel and no newer UI is served yet.
    uiOutdated: <{ ui: string, panel: string } | null>null,
    uiVersionChecked: '',
    controllerMode: <any>emptyControllerMode(),
    controllerModeLoaded: false,
    lastCoreLog: "",
  }),
  getters: {
    hostMeetsRequirements(state): boolean {
      // Normal panel: always OK. Cluster control plane: needs ok when applies.
      const r = state.hostRequirements
      if (!r) return true
      if (r.applies !== true) return true
      return r.ok !== false
    },
  },
  actions: {
    assignControllerMode(value: any) {
      const raw = value || {}
      const profile = ['client', 'full', 'monitor'].includes(raw.profile)
        ? raw.profile
        : raw.monitor_only ? 'monitor' : raw.enabled ? 'full' : 'client'
      this.controllerMode = {
        ...emptyControllerMode(),
        ...raw,
        profile,
        enabled: raw.enabled ?? profile !== 'client',
        monitor_only: raw.monitor_only ?? profile === 'monitor',
        can_control: raw.can_control ?? profile !== 'monitor',
      }
      this.controllerModeLoaded = true
    },
    async loadControllerMode(force = false): Promise<boolean> {
      if (this.controllerModeLoaded && !force) return true
      if (pendingControllerMode) return pendingControllerMode
      pendingControllerMode = (async () => {
        const msg = await HttpUtils.get('api/controller-mode')
        if (msg.success && msg.obj) this.assignControllerMode(msg.obj)
        return msg.success
      })()
      try {
        return await pendingControllerMode
      } finally {
        pendingControllerMode = null
      }
    },
    async loadData() {
      if (pendingLoadData) return pendingLoadData
      pendingLoadData = (async () => {
        const msg = await HttpUtils.get('api/load', this.lastLoad > 0 ? { lu: this.lastLoad } : {})
        if (msg.success) {
          if (msg.obj.lastUpdate) this.lastLoad = msg.obj.lastUpdate
          if (msg.obj.panelVersion) this.checkPanelVersion(msg.obj.panelVersion)
          this.onlines = msg.obj.onlines
          if (msg.obj.hostRequirements) {
            this.hostRequirements = msg.obj.hostRequirements
          }
          const lastCoreLog = msg.obj.lastLog || ""
          if (lastCoreLog && lastCoreLog !== this.lastCoreLog) {
            push.error({
              title: i18n.global.t('error.core'),
              duration: 5000,
              message: lastCoreLog
            })
          }
          this.lastCoreLog = lastCoreLog

          if (msg.obj.config) {
            this.setNewData(msg.obj)
          }
        }
      })()
      try {
        await pendingLoadData
      } finally {
        pendingLoadData = null
      }
    },
    // The panel and its UI are released together. A page older than the
    // panel is either cached by the browser, when the server already serves
    // the matching UI (reload once), or waits for the panel to install it.
    async checkPanelVersion(panelVersion: string) {
      const panel = String(panelVersion).replace(/^v/, '')
      const ui = __APP_VERSION__.replace(/^v/, '')
      if (panel === ui) {
        this.uiOutdated = null
        return
      }
      if (this.uiVersionChecked === panel) return
      this.uiVersionChecked = panel
      try {
        const response = await fetch(resolveFrontendUrl('version.json') + '?t=' + Date.now(), { cache: 'no-store' })
        const served = response.ok ? String((await response.json())?.version || '').replace(/^v/, '') : ''
        const reloadKey = 'ui-reloaded-for'
        if (served === panel && sessionStorage.getItem(reloadKey) !== panel) {
          sessionStorage.setItem(reloadKey, panel)
          window.location.reload()
          return
        }
      } catch {
        // version.json is missing on UIs installed before it existed.
      }
      this.uiOutdated = { ui, panel }
    },
    setNewData(data: any) {
      if (data.hostRequirements) this.hostRequirements = data.hostRequirements
      if (data.subURI) this.subURI = data.subURI
      if (data.enableTraffic) this.enableTraffic = data.enableTraffic
      if (data.config) this.config = data.config
      if (Object.hasOwn(data, 'clients')) this.clients = data.clients ?? []
      if (Object.hasOwn(data, 'inbounds')) this.inbounds = data.inbounds ?? []
      if (Object.hasOwn(data, 'outbounds')) this.outbounds = data.outbounds ?? []
      if (Object.hasOwn(data, 'services')) this.services = data.services ?? []
      if (Object.hasOwn(data, 'endpoints')) this.endpoints = data.endpoints ?? []
      if (Object.hasOwn(data, 'tls')) this.tlsConfigs = data.tls ?? []
    },
    async loadInbounds(ids: number[]): Promise<Inbound[]> {
      const options = ids.length > 0 ? {id: ids.join(",")} : {}
      const msg = await HttpUtils.get('api/inbounds', options)
      if(msg.success) {
        return msg.obj.inbounds
      }
      return <Inbound[]>[]
    },
    async loadClients(id: number): Promise<Client> {
      const options = id > 0 ? {id: id} : {}
      const msg = await HttpUtils.get('api/clients', options)
      if(msg.success) {
        return <Client>msg.obj.clients[0]??{}
      }
      return <Client>{}
    },
    async save (object: string, action: string, data: any, initUsers?: number[]): Promise<boolean> {
      let postData = {
        object: object,
        action: action,
        data: JSON.stringify(data, null, 2),
        initUsers: initUsers?.join(',') ?? undefined
      }
      const msg = await HttpUtils.post('api/save', postData)
      if (msg.success) {
        const objectName = ['tls', 'config'].includes(object) ? object : object.substring(0, object.length - 1)
        push.success({
          title: i18n.global.t('success'),
          duration: 5000,
          message: i18n.global.t('actions.' + action) + " " + i18n.global.t('objects.' + objectName)
        })
        this.setNewData(msg.obj)
      }
      return msg.success
    },
    // Check duplicate client name
    checkClientName (id: number, newName: string): boolean {
      const oldName = id > 0 ? this.clients.findLast((i: any) => i.id == id)?.name : null
      if (newName != oldName && this.clients.findIndex((c: any) => c.name == newName) != -1) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
        })
        return true
      }
      return false
    },
    // Check bulk client names
    checkBulkClientNames (names: string[]): boolean {
      const newNames = new Set(names)
      const oldNames = new Set(this.clients.map((c: any) => c.name))
      const allNames = new Set([...oldNames, ...newNames])
      if (newNames.size != names.length || oldNames.size + newNames.size != allNames.size) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
        })
        return true
      }
      return false
    },
    // check duplicate tag
    checkTag (object: string, id: number, tag: string): boolean {
      let objects = <any[]>[]
      switch (object) {
        case 'inbound':
          objects = this.inbounds
          break
        case 'outbound':
          objects = this.outbounds
          break
        case 'service':
          objects = this.services
          break
        case 'endpoint':
          objects = this.endpoints
          break
        default:
          return false
      }
      const oldObject = id > 0 ? objects.findLast((i: any) => i.id == id) : null
      if (tag != oldObject?.tag && objects.findIndex((i: any) => i.tag == tag) != -1) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('objects.tag')
        })
        return true
      }
      return false
    },
  }
})

export default Data
