<template>
  <v-main>
    <router-view v-slot="{ Component }">
      <transition name="page" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
  </v-main>
</template>

<script lang="ts" setup>
</script>