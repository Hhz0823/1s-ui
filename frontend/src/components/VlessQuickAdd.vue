<template>
  <v-col cols="12" class="vless-quick-title">
    <v-divider />
    <div>{{ $t('quickAdd.vlessOptions') }}</div>
  </v-col>
  <v-col cols="12">
    <v-select
      v-model="data.variant"
      :label="$t('quickAdd.vlessVariant')"
      :items="variantItems"
      item-title="title"
      item-value="value"
      item-props
      hide-details
    />
  </v-col>
  <v-col v-if="isReality" cols="12">
    <v-text-field
      v-model="data.reality_server"
      :label="$t('quickAdd.realityServer')"
      :placeholder="$t('quickAdd.realityAuto')"
      :hint="$t('quickAdd.realityServerHint')"
      persistent-hint
      hide-details="auto"
      prepend-inner-icon="mdi-web"
    />
  </v-col>
  <v-col v-if="isReality && riskyTarget" cols="12">
    <v-alert type="warning" variant="tonal" density="compact">{{ $t('quickAdd.realityTargetRisky') }}</v-alert>
  </v-col>
  <v-col v-if="isReality && port && Number(port) !== 443" cols="12">
    <v-alert type="info" variant="text" density="compact" icon="mdi-lightbulb-on-outline">{{ $t('quickAdd.realityPort443') }}</v-alert>
  </v-col>
  <v-col v-if="cdnAllowed" cols="12">
    <v-switch
      v-model="data.cdn_enabled"
      :label="$t('quickAdd.cdnDownlink')"
      color="primary"
      density="compact"
      inset
      hide-details
    />
  </v-col>
  <template v-if="cdnAllowed && data.cdn_enabled">
    <v-col cols="12" sm="7">
      <v-text-field
        v-model="data.cdn_domain"
        :label="$t('quickAdd.cdnDomain')"
        placeholder="cdn.example.com"
        :hint="$t('quickAdd.cdnDomainHint')"
        persistent-hint
        hide-details="auto"
        prepend-inner-icon="mdi-cloud-outline"
      />
    </v-col>
    <v-col cols="12" sm="5">
      <v-select
        v-model="data.cdn_port"
        :label="$t('quickAdd.cdnPort')"
        :items="cdnPortItems"
        item-title="title"
        item-value="value"
        hide-details
      />
    </v-col>
    <v-col cols="12">
      <v-alert type="info" variant="tonal" density="compact" icon="mdi-cloud-download-outline">{{ $t('quickAdd.cdnHelp') }}</v-alert>
    </v-col>
  </template>
  <v-col cols="12">
    <v-alert type="info" variant="tonal" density="compact" :title="$t('quickAdd.compatTitle')">
      {{ compatText }}
    </v-alert>
  </v-col>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { i18n } from '@/locales'
import { CoreTypes } from '@/types/inbounds'
import {
  type VlessQuickAddOptions,
  type VlessQuickAddVariant,
  cloudflareHttpsPorts,
  realityTargetDiscouraged,
  vlessCdnVariants,
  vlessQuickAddVariants,
  vlessVariantIsReality,
  vlessXrayOnlyVariants,
} from '@/types/vless'

const props = withDefaults(defineProps<{
  data: VlessQuickAddOptions
  coreType: string
  // Whether Xray-core can be chosen on the target server.
  xrayAvailable: boolean
  // Whether this server's sing-box build can run REALITY (every release
  // build can). Vue casts an absent boolean prop to false, hence the default.
  singBoxReality?: boolean
  port?: number
  // Variants the target server's panel is too old to build.
  unsupportedVariants?: VlessQuickAddVariant[]
  // Whether the target server's panel can send downloads through a CDN.
  cdnSupported?: boolean
}>(), { singBoxReality: true, port: 0, unsupportedVariants: () => [], cdnSupported: true })

const emit = defineEmits<{ (event: 'update:coreType', value: string): void }>()

const isXray = computed(() => props.coreType === CoreTypes.Xray)
const isReality = computed(() => vlessVariantIsReality(props.data.variant))
const riskyTarget = computed(() => realityTargetDiscouraged(props.data.reality_server))
const cdnAllowed = computed(() => props.cdnSupported && vlessCdnVariants.includes(props.data.variant))
const cdnPortItems = computed(() => [
  { title: i18n.global.t('quickAdd.cdnPortAuto'), value: 0 },
  ...cloudflareHttpsPorts.map(port => ({ title: String(port), value: port })),
])

// REALITY runs on sing-box too unless the server's build lacks it.
const needsXray = (variant: VlessQuickAddVariant): boolean =>
  vlessXrayOnlyVariants.includes(variant) || (vlessVariantIsReality(variant) && props.singBoxReality === false)
const variantAllowed = (variant: VlessQuickAddVariant): boolean =>
  !props.unsupportedVariants.includes(variant) && (!needsXray(variant) || props.xrayAvailable)
const singBoxDefault = (): VlessQuickAddVariant => props.singBoxReality === false ? 'tls' : 'reality-vision'

const variantItems = computed(() => vlessQuickAddVariants.map(item => ({
  value: item.value,
  title: i18n.global.t(`quickAdd.variants.${item.key}`),
  subtitle: i18n.global.t(props.unsupportedVariants.includes(item.value)
    ? 'quickAdd.needsPanelUpgrade'
    : needsXray(item.value) ? 'quickAdd.xrayOnly' : 'quickAdd.anyCore'),
  disabled: !variantAllowed(item.value),
})))

const compatText = computed(() => {
  switch (props.data.variant) {
    case 'reality-vision':
      return i18n.global.t(isXray.value ? 'quickAdd.compat.realityVisionXray' : 'quickAdd.compat.realityVision')
    case 'reality-xhttp':
      return i18n.global.t('quickAdd.compat.realityXhttp')
    case 'reality-xhttp-vision':
      return i18n.global.t('quickAdd.compat.realityXhttpVision')
    case 'enc-vision':
      return i18n.global.t('quickAdd.compat.encryption') + ' ' + i18n.global.t('quickAdd.compat.passwallSubscription')
    case 'enc-xhttp':
      return i18n.global.t('quickAdd.compat.encryption')
    default:
      return i18n.global.t('quickAdd.compat.tls')
  }
})

// Choosing an Xray-only variant moves the node to Xray-core; moving the node
// back to sing-box falls back to a variant sing-box can run.
watch(() => props.data.variant, (variant) => {
  if (!variantAllowed(variant)) {
    props.data.variant = singBoxDefault()
    return
  }
  if (needsXray(variant) && !isXray.value) emit('update:coreType', CoreTypes.Xray)
}, { immediate: true })

watch(() => props.coreType, (core) => {
  if (core !== CoreTypes.Xray && needsXray(props.data.variant)) props.data.variant = singBoxDefault()
})
</script>

<style scoped>
.vless-quick-title {
  display: grid;
  gap: 10px;
  color: rgb(var(--v-theme-on-surface));
  font-size: 0.875rem;
  font-weight: 600;
}
</style>
