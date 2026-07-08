<template>
  <OutboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    :tags="outboundTags"
    @close="closeModal"
  />
  <OutboundBulk
    v-model="bulkModal.visible"
    :visible="bulkModal.visible"
    :outboundTags="outboundTags"
    @close="closeBulkModal"
  />
  <EndpointVue
    v-model="endpointModal.visible"
    :visible="endpointModal.visible"
    :id="endpointModal.id"
    :data="endpointModal.data"
    :tags="endpointTags"
    @close="closeEndpointModal"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <v-row justify="center" align="center">
    <v-col cols="auto">
      <v-btn color="primary" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </v-col>
    <v-col cols="auto">
      <v-btn color="primary" @click="showBulkModal">{{ $t('actions.addbulk') }}</v-btn>
    </v-col>
    <v-col cols="auto">
      <v-btn
        color="info"
        variant="tonal"
        prepend-icon="mdi-cloud-sync"
        :loading="creatingWarp"
        :disabled="creatingWarp"
        @click="createWarpOutbound"
      >
        {{ $t('actions.addWarp') }}
        <v-tooltip activator="parent" location="top" :text="$t('out.warpSafeTip')"></v-tooltip>
      </v-btn>
    </v-col>
    <v-col cols="auto">
      <v-btn
        color="secondary"
        variant="outlined"
        :loading="testingAll"
        append-icon="mdi-speedometer"
        :disabled="testingAll || checkableTags.length === 0"
        @click="checkAllOutbounds"
      >
        {{ $t('actions.testAll') || 'Test all' }}
      </v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>outbounds" :key="item.tag">
      <v-card rounded="xl" elevation="2" min-width="180" :title="item.tag">
        <v-card-subtitle >
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('in.addr') }}</v-col>
            <v-col>
              {{ item.server?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col>
              {{ item.server_port?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.tls') }}</v-col>
            <v-col>
              {{ Object.hasOwn(item,'tls') ? $t(item.tls?.enabled ? 'enable' : 'disable') : '-'  }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('online') }}</v-col>
            <v-col>
              <template v-if="onlines.includes(item.tag)">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('out.delay') }}</v-col>
            <v-col>
              <v-progress-circular
                v-if="checkResults[item.tag]?.loading"
                indeterminate
                size="20"
              />
              <v-icon
                icon="mdi-speedometer"
                v-else
                @click="checkOutbound(item.tag)"
              >
                <v-tooltip activator="parent" location="top" :text="$t('actions.test')"></v-tooltip>
              </v-icon>
              <template v-if="checkResults[item.tag]?.loading == false">
                <template v-if="checkResults[item.tag]">
                  <v-chip
                    v-if="checkResults[item.tag].success"
                    density="compact"
                    size="small"
                    color="success"
                    variant="flat"
                  >
                    {{ checkResults[item.tag].data?.Delay + $t('date.ms') }}
                  </v-chip>
                  <v-tooltip v-else location="top" :text="checkResults[item.tag].errorMessage || $t('failed')">
                    <template v-slot:activator="{ props }">
                      <v-icon v-bind="props" size="small" color="error" icon="mdi-close-circle" />
                    </template>
                  </v-tooltip>
                </template>
              </template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-btn icon="mdi-file-edit" @click="showModal(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove"  color="warning" @click="delOverlay[index] = true">
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
                <v-btn color="error" variant="outlined" @click="delOutbound(item.tag)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-chart-line" @click="showStats(item.tag)" v-if="Data().enableTraffic">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>warpEndpoints" :key="'warp-' + item.tag">
      <v-card rounded="xl" elevation="2" min-width="180" :title="item.tag">
        <v-card-subtitle>
          <v-row>
            <v-col>WARP / {{ $t('objects.outbound') }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('types.wg.localIp') }}</v-col>
            <v-col>
              {{ formatEndpointAddress(item) }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('types.wg.peer') }}</v-col>
            <v-col>
              {{ formatEndpointPeer(item) }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('types.wg.sysIf') }}</v-col>
            <v-col>
              {{ $t(item.system ? 'enable' : 'disable') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('online') }}</v-col>
            <v-col>
              <template v-if="endpointOnlines.includes(item.tag)">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('out.delay') }}</v-col>
            <v-col>
              <v-progress-circular
                v-if="checkResults[item.tag]?.loading"
                indeterminate
                size="20"
              />
              <v-icon
                icon="mdi-speedometer"
                v-else
                @click="checkOutbound(item.tag)"
              >
                <v-tooltip activator="parent" location="top" :text="$t('actions.test')"></v-tooltip>
              </v-icon>
              <template v-if="checkResults[item.tag]?.loading == false">
                <template v-if="checkResults[item.tag]">
                  <v-chip
                    v-if="checkResults[item.tag].success"
                    density="compact"
                    size="small"
                    color="success"
                    variant="flat"
                  >
                    {{ checkResults[item.tag].data?.Delay + $t('date.ms') }}
                  </v-chip>
                  <v-tooltip v-else location="top" :text="checkResults[item.tag].errorMessage || $t('failed')">
                    <template v-slot:activator="{ props }">
                      <v-icon v-bind="props" size="small" color="error" icon="mdi-close-circle" />
                    </template>
                  </v-tooltip>
                </template>
              </template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-btn icon="mdi-file-edit" @click="showEndpointModal(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" color="warning" @click="endpointDelOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="endpointDelOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delEndpoint(item.tag)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="endpointDelOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-chart-line" @click="showStats(item.tag, 'endpoint')" v-if="Data().enableTraffic">
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
import RandomUtil from '@/plugins/randomUtil'
import OutboundVue from '@/layouts/modals/Outbound.vue'
import OutboundBulk from '@/layouts/modals/OutboundBulk.vue'
import EndpointVue from '@/layouts/modals/Endpoint.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Outbound } from '@/types/outbounds'
import { Endpoint, EpTypes, createEndpoint } from '@/types/endpoints'
import { computed, ref } from 'vue'

interface CheckResult {
  loading?: boolean
  success: boolean
  data?: { OK?: boolean; Delay?: number; Error?: string } | null
  errorMessage?: string
}

const checkResults = ref<Record<string, CheckResult>>({})

const checkOutbound = async (tag: string) => {
  checkResults.value = { ...checkResults.value, [tag]: { loading: true, success: false } }
  const msg = await HttpUtils.get('api/checkOutbound', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  checkResults.value = {
    ...checkResults.value,
    [tag]: { loading: false, success, data: msg.obj ?? null, errorMessage }
  }
}

const testingAll = ref(false)

const checkableTags = computed(() => {
  return [
    ...outbounds.value.map((o) => o.tag),
    ...warpEndpoints.value.map((e: any) => e.tag),
  ].filter(Boolean)
})

const checkAllOutbounds = async () => {
  const tags = checkableTags.value
  if (tags.length === 0) return
  testingAll.value = true
  try {
    await Promise.all(tags.map((tag) => checkOutbound(tag)))
  } finally {
    testingAll.value = false
  }
}

const outbounds = computed((): Outbound[] => {
  return <Outbound[]> Data().outbounds
})

const endpoints = computed((): Endpoint[] => {
  return <Endpoint[]> Data().endpoints
})

const warpEndpoints = computed((): Endpoint[] => {
  return endpoints.value.filter((e: any) => e.type === EpTypes.Warp)
})

const endpointTags = computed((): string[] => {
  return endpoints.value?.map((e: any) => e.tag) ?? []
})

const outboundTags = computed((): string[] => {
  return [...outbounds.value?.map((o: Outbound) => o.tag), ...endpointTags.value]
})

const onlines = computed(() => {
  return Data().onlines.outbound?? []
})

const endpointOnlines = computed(() => {
  return [...(Data().onlines.inbound ?? []), ...(Data().onlines.outbound ?? [])]
})

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

let delOverlay = ref(new Array<boolean>)
let endpointDelOverlay = ref(new Array<boolean>)

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(outbounds.value.findLast(o => o.id == id))
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}

const bulkModal = ref({ visible: false })

const showBulkModal = () => {
  bulkModal.value.visible = true
}

const closeBulkModal = () => {
  bulkModal.value.visible = false
}

const endpointModal = ref({
  visible: false,
  id: 0,
  data: "",
})

const showEndpointModal = (id: number) => {
  endpointModal.value.id = id
  endpointModal.value.data = id == 0 ? '' : JSON.stringify(warpEndpoints.value.findLast((e: any) => e.id == id))
  endpointModal.value.visible = true
}

const closeEndpointModal = () => {
  endpointModal.value.visible = false
}

const creatingWarp = ref(false)

const nextWarpTag = () => {
  let tag = `warp-${RandomUtil.randomSeq(4)}`
  let attempts = 0
  while (outboundTags.value.includes(tag) && attempts < 50) {
    tag = `warp-${RandomUtil.randomSeq(4)}`
    attempts++
  }
  return tag
}

const createWarpOutbound = async () => {
  creatingWarp.value = true
  try {
    const endpoint = createEndpoint(EpTypes.Warp, {
      tag: nextWarpTag(),
      listen_port: 0,
      system: false,
    })
    await Data().save("endpoints", "new", endpoint)
  } finally {
    creatingWarp.value = false
  }
}

const formatEndpointAddress = (endpoint: any) => {
  return endpoint.address?.length > 0 ? endpoint.address.join(', ') : '-'
}

const formatEndpointPeer = (endpoint: any) => {
  const peer = endpoint.peers?.[0]
  if (!peer) return '-'
  return `${peer.address ?? '-'}${peer.port ? ':' + peer.port : ''}`
}

const stats = ref({
  visible: false,
  resource: "outbound",
  tag: "",
})

const delOutbound = async (tag: string) => {
  const index = outbounds.value.findIndex(i => i.tag == tag)
  const success = await Data().save("outbounds", "del", tag)
  if (success) delOverlay.value[index] = false
}

const delEndpoint = async (tag: string) => {
  const index = warpEndpoints.value.findIndex((i: any) => i.tag == tag)
  const success = await Data().save("endpoints", "del", tag)
  if (success && index >= 0) endpointDelOverlay.value[index] = false
}

const showStats = (tag: string, resource = "outbound") => {
  stats.value.resource = resource
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>
