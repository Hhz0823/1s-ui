<template>
  <v-overlay
    :model-value="loading"
    persistent
    content-class="text-center"
    class="align-center justify-center"
  >
    <v-progress-circular
      indeterminate
      size="64"
    ></v-progress-circular>
    <br />
    {{ $t('loading') }}
  </v-overlay>
  <Message />
  <router-view />
</template>

<script lang="ts" setup>
import Message from '@/components/message.vue'
import { inject, ref, Ref } from 'vue'
import { useTheme } from 'vuetify'
import { setupAppearance } from '@/plugins/themes'

const loading:Ref = inject('loading')?? ref(false)

setupAppearance(useTheme())

// Change page title
document.title = "1S-UI " + document.location.hostname
</script>

<style>
.v-overlay .v-list-item,
.v-field__input {
  direction: ltr;
}
</style>