<template>
  <div class="glass-wrapper" :class="{ 'glass-wrapper--compact': compact }">
    <LiquidGlass
      :displacement-scale="displacementScale"
      :blur-amount="blurAmount"
      :saturation="saturation"
      :aberration-intensity="aberrationIntensity"
      :elasticity="elasticity"
      :corner-radius="cornerRadius"
      :padding="padding"
      class="glass-layer"
    >
      <slot />
    </LiquidGlass>
  </div>
</template>

<script setup lang="ts">
import { LiquidGlass } from '@wxperia/liquid-glass-vue'

withDefaults(defineProps<{
  compact?: boolean
  displacementScale?: number
  blurAmount?: number
  saturation?: number
  aberrationIntensity?: number
  elasticity?: number
  cornerRadius?: number
  padding?: string
}>(), {
  compact: false,
  displacementScale: 40,
  blurAmount: 0.08,
  saturation: 140,
  aberrationIntensity: 1.5,
  elasticity: 0.25,
  cornerRadius: 16,
  padding: '0px',
})
</script>

<style scoped>
.glass-wrapper {
  position: relative;
}

.glass-layer {
  position: relative;
  z-index: 0;
}
</style>
