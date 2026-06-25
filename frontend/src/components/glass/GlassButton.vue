<template>
  <LiquidGlass
    :displacement-scale="30"
    :blur-amount="0.06"
    :saturation="150"
    :aberration-intensity="1"
    :elasticity="0.3"
    :corner-radius="10"
    padding="6px 16px"
    class="glass-btn"
    @click="$emit('click', $event)"
  >
    <span class="glass-btn-inner">
      <slot />
    </span>
  </LiquidGlass>
</template>

<script setup lang="ts">
import { LiquidGlass } from '@wxperia/liquid-glass-vue'
defineEmits(['click'])
</script>

<style scoped>
.glass-btn {
  cursor: pointer;
  display: inline-flex;
}

.glass-btn-inner {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
}
</style>
