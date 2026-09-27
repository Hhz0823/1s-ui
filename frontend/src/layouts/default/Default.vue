<template>
  <v-app
    class="app-root"
    :class="[
      `ui-style--${uiStyle}`,
      `ui-density--${uiDensity}`,
      {
        'app-root--side-desktop': menuPosition !== 'top' && !isMobile,
        'app-root--drawer-expanded': drawerExpanded && menuPosition !== 'top' && !isMobile,
        'app-root--drawer-collapsed': !drawerExpanded && menuPosition !== 'top' && !isMobile,
      },
    ]"
  >
    <div
      v-if="bgImage"
      class="app-bg-image"
      :style="{
        backgroundImage: `url(${bgImage})`,
        backgroundSize: bgFit,
        backgroundPosition: bgPosition,
        filter: `blur(${bgBlur}px) saturate(${bgSaturate})`,
        opacity: Number(bgOpacity) / 100,
      }"
    ></div>
    <drawer
      v-if="menuPosition !== 'top' || isMobile"
      :isMobile="isMobile"
      :displayDrawer="drawerOpen"
      :expanded="drawerExpanded"
      @toggleDrawer="toggleDrawer"
      @closeDrawer="closeDrawer"
    />
    <default-bar
      :isMobile="isMobile"
      :menuPosition="menuPosition"
      :menuItems="menuItems"
      :drawerExpanded="drawerExpanded"
      :uiStyle="uiStyle"
      @toggleDrawer="toggleDrawer"
    />
    <default-view />
  </v-app>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import DefaultBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import DefaultView from './View.vue'
import { useDisplay, useTheme } from 'vuetify'
import bgAsset from '@/assets/bg.jpg'
import { useMenuItems } from './menu'

const { smAndDown } = useDisplay()
const theme = useTheme()
const drawerOpen = ref(false)
// Expanded like BaoTa / 1Panel unless the user collapsed it.
const drawerExpanded = ref(localStorage.getItem('drawerExpanded') !== 'false')

const toggleDrawer = () => {
  if (isMobile.value) {
    drawerOpen.value = !drawerOpen.value
    return
  }

  if (menuPosition.value !== 'top') {
    drawerExpanded.value = !drawerExpanded.value
    localStorage.setItem('drawerExpanded', drawerExpanded.value ? 'true' : 'false')
  }
}

const closeDrawer = () => {
  if (isMobile.value) drawerOpen.value = false
}

const isMobile = computed((): boolean => {
  return smAndDown.value
})

const uiPreferenceEvent = 'ui-preferences-changed'
const normalizeUiChoice = (value: string | null, fallback: string, choices: readonly string[]) => {
  return value && choices.includes(value) ? value : fallback
}
const readUiPrefs = () => ({
  menuPosition: normalizeUiChoice(localStorage.getItem('menuPosition'), 'side', ['side', 'top']),
  bgPreset: normalizeUiChoice(localStorage.getItem('bgPreset'), localStorage.getItem('bgImage') ? 'custom' : 'default', ['default', 'none', 'custom']),
  bgImage: localStorage.getItem('bgImage') || '',
  bgBlur: localStorage.getItem('bgBlur') || '6',
  bgOpacity: localStorage.getItem('bgOpacity') || '40',
  bgSaturate: localStorage.getItem('bgSaturate') || '1.3',
  bgFit: normalizeUiChoice(localStorage.getItem('bgFit'), 'cover', ['cover', 'contain', 'auto']),
  bgPosition: normalizeUiChoice(localStorage.getItem('bgPosition'), 'center', ['center', 'center top', 'center bottom']),
  uiStyle: normalizeUiChoice(localStorage.getItem('uiStyle'), 'panel', ['panel', 'glass', 'solid', 'clear']),
  uiDensity: normalizeUiChoice(localStorage.getItem('uiDensity'), 'comfortable', ['comfortable', 'compact']),
})
const uiPrefs = ref(readUiPrefs())
let glassPointerFrame = 0
let glassPointerTarget: HTMLElement | null = null
let glassPointerX = 0
let glassPointerY = 0
let adaptiveInkGeneration = 0

const themeBackground = () => {
  const value = String(theme.current.value.colors.background || '#ffffff')
  const hex = value.replace('#', '')
  if (/^[0-9a-f]{6}$/i.test(hex)) {
    return [0, 2, 4].map(offset => Number.parseInt(hex.slice(offset, offset + 2), 16))
  }
  const numbers = value.match(/[\d.]+/g)?.map(Number)
  return numbers?.length === 3 ? numbers : [255, 255, 255]
}

const luminance = (red: number, green: number, blue: number) => {
  const linear = [red, green, blue].map(value => {
    const channel = value / 255
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2]
}

const applyAdaptiveInkFallback = () => {
  const [red, green, blue] = themeBackground()
  document.body.style.setProperty('--adaptive-ink-fallback', luminance(red, green, blue) > 0.5 ? '#111111' : '#ffffff')
  document.body.style.setProperty('--adaptive-ink-map', 'none')
}

const refreshAdaptiveInk = async () => {
  const generation = ++adaptiveInkGeneration
  const source = bgImage.value
  if (uiStyle.value === 'solid' || !source) {
    applyAdaptiveInkFallback()
    return
  }

  const image = new Image()
  if (/^https?:/i.test(source)) image.crossOrigin = 'anonymous'
  try {
    await new Promise<void>((resolve, reject) => {
      image.onload = () => resolve()
      image.onerror = () => reject(new Error('background image could not be sampled'))
      image.src = source
    })
    if (generation !== adaptiveInkGeneration) return

    const scale = Math.min(1, 64 / image.naturalWidth, 48 / image.naturalHeight)
    const width = Math.max(1, Math.round(image.naturalWidth * scale))
    const height = Math.max(1, Math.round(image.naturalHeight * scale))
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const context = canvas.getContext('2d', { willReadFrequently: true })
    if (!context) throw new Error('canvas is unavailable')
    context.drawImage(image, 0, 0, width, height)
    const pixels = context.getImageData(0, 0, width, height)
    const [baseRed, baseGreen, baseBlue] = themeBackground()
    const opacity = Math.max(0, Math.min(1, Number(bgOpacity.value) / 100))
    let totalLuminance = 0
    for (let index = 0; index < pixels.data.length; index += 4) {
      const sourceOpacity = (pixels.data[index + 3] / 255) * opacity
      const red = pixels.data[index] * sourceOpacity + baseRed * (1 - sourceOpacity)
      const green = pixels.data[index + 1] * sourceOpacity + baseGreen * (1 - sourceOpacity)
      const blue = pixels.data[index + 2] * sourceOpacity + baseBlue * (1 - sourceOpacity)
      const light = luminance(red, green, blue)
      totalLuminance += light
      // Black has better contrast above the WCAG black/white crossover.
      const ink = light > 0.179 ? 17 : 255
      pixels.data[index] = ink
      pixels.data[index + 1] = ink
      pixels.data[index + 2] = ink
      pixels.data[index + 3] = 255
    }
    context.putImageData(pixels, 0, 0)
    const average = totalLuminance / (pixels.data.length / 4)
    document.body.style.setProperty('--adaptive-ink-fallback', average > 0.5 ? '#111111' : '#ffffff')
    document.body.style.setProperty('--adaptive-ink-map', `url("${canvas.toDataURL('image/png')}")`)
    document.body.style.setProperty('--adaptive-ink-position', bgPosition.value)
    document.body.style.setProperty('--adaptive-ink-size', bgFit.value === 'auto' ? `${image.naturalWidth}px ${image.naturalHeight}px` : bgFit.value)
  } catch {
    if (generation === adaptiveInkGeneration) applyAdaptiveInkFallback()
  }
}

const updateGlassPointer = (event: PointerEvent) => {
  if (!document.body.classList.contains('ui-style--glass')) return
  const target = (event.target as Element | null)?.closest<HTMLElement>(
    '.v-btn, .ui-choice-button, .theme-chip, .v-card, .v-field, .v-list-item, .v-expansion-panel, .v-tab, .menu-card, .app-drawer, .app-bar',
  )
  if (!target) return
  glassPointerTarget = target
  glassPointerX = event.clientX
  glassPointerY = event.clientY
  if (glassPointerFrame) return
  glassPointerFrame = window.requestAnimationFrame(() => {
    glassPointerFrame = 0
    if (!glassPointerTarget?.isConnected) return
    const rect = glassPointerTarget.getBoundingClientRect()
    glassPointerTarget.style.setProperty('--glass-pointer-x', `${Math.max(0, Math.min(rect.width, glassPointerX - rect.left))}px`)
    glassPointerTarget.style.setProperty('--glass-pointer-y', `${Math.max(0, Math.min(rect.height, glassPointerY - rect.top))}px`)
  })
}

const refreshUiPrefs = () => {
  uiPrefs.value = readUiPrefs()
}
const documentUiClasses = [
  'ui-style--panel',
  'ui-style--glass',
  'ui-style--solid',
  'ui-style--clear',
  'ui-density--comfortable',
  'ui-density--compact',
]
const syncDocumentUiClasses = () => {
  document.body.classList.remove(...documentUiClasses)
  document.body.classList.add(`ui-style--${uiStyle.value}`, `ui-density--${uiDensity.value}`)
}

onMounted(() => {
  window.addEventListener(uiPreferenceEvent, refreshUiPrefs)
  window.addEventListener('storage', refreshUiPrefs)
  document.addEventListener('pointermove', updateGlassPointer, { passive: true })
  void refreshAdaptiveInk()
})
onBeforeUnmount(() => {
  window.removeEventListener(uiPreferenceEvent, refreshUiPrefs)
  window.removeEventListener('storage', refreshUiPrefs)
  document.removeEventListener('pointermove', updateGlassPointer)
  if (glassPointerFrame) window.cancelAnimationFrame(glassPointerFrame)
  adaptiveInkGeneration++
  document.body.style.removeProperty('--adaptive-ink-fallback')
  document.body.style.removeProperty('--adaptive-ink-map')
  document.body.style.removeProperty('--adaptive-ink-position')
  document.body.style.removeProperty('--adaptive-ink-size')
  document.body.classList.remove(...documentUiClasses)
})

const bgImage = computed(() => {
  if (uiPrefs.value.uiStyle === 'panel' || uiPrefs.value.bgPreset === 'none') return ''
  if (uiPrefs.value.bgPreset === 'custom') return uiPrefs.value.bgImage
  return bgAsset
})
const bgBlur = computed(() => uiPrefs.value.bgBlur)
const bgOpacity = computed(() => uiPrefs.value.bgOpacity)
const bgSaturate = computed(() => uiPrefs.value.bgSaturate)
const bgFit = computed(() => uiPrefs.value.bgFit)
const bgPosition = computed(() => uiPrefs.value.bgPosition)
const uiStyle = computed(() => uiPrefs.value.uiStyle)
const uiDensity = computed(() => uiPrefs.value.uiDensity)
const menuPosition = computed(() => uiPrefs.value.menuPosition)

watch([uiStyle, uiDensity], syncDocumentUiClasses, { immediate: true })
watch([bgImage, bgOpacity, bgFit, bgPosition, uiStyle, () => theme.current.value.colors.background], () => { void refreshAdaptiveInk() })

watch([smAndDown, menuPosition], ([mobile, position]) => {
  drawerOpen.value = !mobile && position !== 'top'
}, { immediate: true })

const menuItems = useMenuItems()
</script>

<style>
.app-root {
  background: rgb(var(--v-theme-background));
}

.v-card-subtitle {
  text-align: center;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  min-height: 20px;
}

.v-switch.v-input {
  padding-inline-start: 0.6rem;
}
</style>
