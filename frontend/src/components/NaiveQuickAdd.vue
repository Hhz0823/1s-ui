<template>
  <v-col cols="12" class="naive-quick-title">
    <v-divider />
    <div>{{ $t('types.naive.quickOptions') }}</div>
  </v-col>
  <v-col cols="12" sm="6">
    <v-text-field v-model="data.username" :label="$t('types.naive.identity')" hide-details>
      <template #append-inner>
        <v-btn icon="mdi-refresh" size="x-small" variant="text" :title="$t('actions.update')" @click="data.username = 'naive-' + RandomUtil.randomSeq(6)" />
      </template>
    </v-text-field>
  </v-col>
  <v-col cols="12" sm="6">
    <v-text-field v-model="data.password" :label="$t('types.pw')" type="password" hide-details>
      <template #append-inner>
        <v-btn icon="mdi-refresh" size="x-small" variant="text" :title="$t('actions.update')" @click="data.password = RandomUtil.randomShadowsocksPassword(32)" />
      </template>
    </v-text-field>
  </v-col>
  <v-col cols="12" sm="7">
    <v-text-field
      v-model="data.server"
      :label="$t('types.naive.server')"
      :hint="$t('types.naive.serverHint')"
      persistent-hint
      hide-details="auto"
    />
  </v-col>
  <v-col cols="12" sm="5">
    <v-select
      v-model="data.mode"
      :label="$t('types.naive.protocolIdentity')"
      :items="modeOptions"
      item-title="title"
      item-value="value"
      hide-details
    />
  </v-col>
  <v-col cols="12" sm="6">
    <v-select
      v-model.number="data.tls_id"
      :label="$t('types.naive.tlsConfig')"
      :items="tlsOptions"
      item-title="title"
      item-value="value"
      :hint="$t('types.naive.autoTlsHint')"
      persistent-hint
      hide-details="auto"
    />
  </v-col>
  <v-col cols="12" sm="6">
    <v-text-field
      :model-value="$t('types.naive.chromiumFingerprint')"
      :label="$t('types.naive.tlsFingerprint')"
      readonly
      hide-details
      prepend-inner-icon="mdi-fingerprint"
    />
  </v-col>
  <v-col v-if="data.mode === 'quic'" cols="12" sm="6">
    <v-select
      v-model="data.quic_congestion_control"
      :label="$t('types.naive.quicCongestion')"
      :items="congestionOptions"
      hide-details
    />
  </v-col>
  <v-col v-if="data.mode !== 'quic'" cols="12" sm="6">
    <v-text-field
      v-model.number="data.insecure_concurrency"
      :label="$t('types.naive.insecureConcurrency')"
      type="number"
      min="0"
      max="4"
      :hint="$t('types.naive.insecureConcurrencyHint')"
      persistent-hint
      hide-details="auto"
    />
  </v-col>
  <v-col cols="12" sm="6" class="naive-switch-col">
    <v-switch v-model="data.udp_over_tcp" color="primary" :label="$t('types.naive.udpOverTcp')" hide-details />
  </v-col>
  <v-col cols="12">
    <v-textarea
      v-model="data.extra_headers_text"
      :label="$t('types.naive.extraHeaders')"
      :hint="$t('types.naive.extraHeadersHint')"
      rows="2"
      auto-grow
      persistent-hint
      hide-details="auto"
    />
  </v-col>
  <v-col cols="12">
    <v-alert type="info" variant="tonal" density="compact">
      {{ $t('types.naive.tlsFingerprintHint') }}
    </v-alert>
  </v-col>
  <v-col cols="12">
    <v-alert type="warning" variant="tonal" density="compact" :title="$t('quickAdd.compatTitle')">
      {{ $t('quickAdd.compat.naive') }}
    </v-alert>
  </v-col>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import RandomUtil from '@/plugins/randomUtil'
import { i18n } from '@/locales'
import type { NaiveQuickAddOptions } from '@/types/naive'

const props = defineProps<{
  data: NaiveQuickAddOptions
  tlsConfigs: any[]
}>()

const modeOptions = computed(() => [
  { title: i18n.global.t('types.naive.httpsMode'), value: 'https' },
  { title: i18n.global.t('types.naive.quicMode'), value: 'quic' },
])

const tlsOptions = computed(() => [
  { title: i18n.global.t('types.naive.autoTls'), value: 0 },
  ...(props.tlsConfigs || []).map(item => ({ title: item.name, value: Number(item.id) })),
])

const congestionOptions = ['bbr', 'bbr2', 'cubic', 'reno']

watch(() => props.data.mode, mode => {
  if (mode === 'quic') props.data.insecure_concurrency = 0
}, { immediate: true })
</script>

<style scoped>
.naive-quick-title {
  display: grid;
  gap: 10px;
  color: rgb(var(--v-theme-on-surface));
  font-size: 0.875rem;
  font-weight: 600;
}

.naive-switch-col {
  display: flex;
  align-items: center;
}
</style>
