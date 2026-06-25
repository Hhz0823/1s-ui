<template>
  <LiquidGlass
    :displacement-scale="20"
    :blur-amount="0.1"
    :saturation="160"
    :aberration-intensity="1"
    :elasticity="0.2"
    :corner-radius="14"
    padding="4px"
    class="glass-menu"
  >
    <slot />
  </LiquidGlass>
</template>

<script setup lang="ts">
import { LiquidGlass } from '@wxperia/liquid-glass-vue'
</script>

<style scoped>
.glass-menu {
  min-width: 140px;
}
</style>
