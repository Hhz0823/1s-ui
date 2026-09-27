<template>
  <v-dialog :model-value="modelValue" width="min(420px, calc(100vw - 24px))" @update:model-value="$emit('update:modelValue', $event)">
    <v-card :title="$t('actions.del')" rounded="lg">
      <v-divider />
      <v-card-text>
        {{ $t('confirm') }}
        <div v-if="target" class="delete-target-tag">{{ target }}</div>
      </v-card-text>
      <v-card-actions class="justify-end">
        <v-btn color="secondary" variant="outlined" :disabled="loading" @click="$emit('update:modelValue', false)">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="loading" @click="$emit('confirm')">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
defineProps<{
  modelValue: boolean
  target?: string
  loading?: boolean
}>()

defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'confirm'): void
}>()
</script>
