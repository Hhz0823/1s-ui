// Appearance preferences: color theme, accent color and corner radius.
//
// The palettes live in plugins/vuetify.ts; this file lists them for the
// pickers and applies the saved choices. Every page reads the result through
// Vuetify's CSS variables (rgb(var(--v-theme-primary)) and friends) plus the
// body classes set here:
//   theme-dark / theme-light   whether the current palette is dark
//   ui-radius--sharp / --round  corner override, see the end of settings.scss
// so new pages (such as the server monitoring pages) follow the chosen theme
// without their own theme code.
import { reactive, watch } from 'vue'
import type { ThemeInstance } from 'vuetify'

export const defaultTheme = 'bt'

export type ThemeChoice = { value: string, icon: string, dark: boolean }

export const themeChoices: ThemeChoice[] = [
  { value: 'bt', icon: 'mdi-leaf-circle-outline', dark: false },
  { value: 'onepanel', icon: 'mdi-alpha-p-circle-outline', dark: false },
  { value: 'light', icon: 'mdi-white-balance-sunny', dark: false },
  { value: 'daylight', icon: 'mdi-weather-sunny-alert', dark: false },
  { value: 'githubLight', icon: 'mdi-github', dark: false },
  { value: 'solarizedLight', icon: 'mdi-sun-thermometer-outline', dark: false },
  { value: 'latte', icon: 'mdi-coffee-outline', dark: false },
  { value: 'lavender', icon: 'mdi-flower-tulip-outline', dark: false },
  { value: 'coffee', icon: 'mdi-coffee', dark: false },
  { value: 'sunset', icon: 'mdi-weather-sunset', dark: false },
  { value: 'sakura', icon: 'mdi-flower', dark: false },
  { value: 'mint', icon: 'mdi-leaf', dark: false },
  { value: 'btDark', icon: 'mdi-leaf-circle', dark: true },
  { value: 'onepanelDark', icon: 'mdi-alpha-p-circle', dark: true },
  { value: 'dark', icon: 'mdi-moon-waning-crescent', dark: true },
  { value: 'midnight', icon: 'mdi-weather-night', dark: true },
  { value: 'graphite', icon: 'mdi-circle-slice-8', dark: true },
  { value: 'solarizedDark', icon: 'mdi-sun-thermometer', dark: true },
  { value: 'mocha', icon: 'mdi-cup', dark: true },
  { value: 'tokyoNight', icon: 'mdi-city-variant-outline', dark: true },
  { value: 'gruvbox', icon: 'mdi-campfire', dark: true },
  { value: 'rosePine', icon: 'mdi-pine-tree-variant-outline', dark: true },
  { value: 'ocean', icon: 'mdi-waves', dark: true },
  { value: 'forest', icon: 'mdi-pine-tree', dark: true },
  { value: 'nord', icon: 'mdi-snowflake', dark: true },
  { value: 'dracula', icon: 'mdi-bat', dark: true },
  { value: 'cyberpunk', icon: 'mdi-robot', dark: true },
]

export const systemTheme = 'system'

// Accent presets; '' keeps each theme's own primary color.
export const accentChoices = [
  '',
  '#2563EB',
  '#0891B2',
  '#0D9488',
  '#16A34A',
  '#65A30D',
  '#D97706',
  '#EA580C',
  '#DC2626',
  '#DB2777',
  '#7C3AED',
  '#475569',
]

export const radiusChoices = ['auto', 'sharp', 'round'] as const

type Appearance = {
  theme: string
  themeLight: string
  themeDark: string
  accent: string
  radius: string
}

const themeNames = themeChoices.map(t => t.value)
const lightNames = themeChoices.filter(t => !t.dark).map(t => t.value)
const darkNames = themeChoices.filter(t => t.dark).map(t => t.value)

const pick = (value: string | null, fallback: string, choices: readonly string[]) =>
  value && choices.includes(value) ? value : fallback

const readAppearance = (): Appearance => {
  const accent = localStorage.getItem('themeAccent') ?? ''
  return {
    theme: pick(localStorage.getItem('theme'), defaultTheme, [...themeNames, systemTheme]),
    themeLight: pick(localStorage.getItem('themeLight'), 'light', lightNames),
    themeDark: pick(localStorage.getItem('themeDark'), 'dark', darkNames),
    accent: /^#[0-9a-f]{6}$/i.test(accent) ? accent : '',
    radius: pick(localStorage.getItem('uiRadius'), 'auto', radiusChoices),
  }
}

const storageKeys: Record<keyof Appearance, string> = {
  theme: 'theme',
  themeLight: 'themeLight',
  themeDark: 'themeDark',
  accent: 'themeAccent',
  radius: 'uiRadius',
}

export const appearance = reactive<Appearance>(readAppearance())

export const setAppearance = (key: keyof Appearance, value: string) => {
  appearance[key] = value
  if (value) localStorage.setItem(storageKeys[key], value)
  else localStorage.removeItem(storageKeys[key])
}

const prefersDark = window.matchMedia?.('(prefers-color-scheme: dark)')

// The Vuetify theme to show for a saved choice; "system" follows the OS.
export const resolveTheme = (name = appearance.theme) =>
  name === systemTheme ? (prefersDark?.matches ? appearance.themeDark : appearance.themeLight) : name

export const savedTheme = () => appearance.theme

// Primary colors before any accent override, for the picker swatches.
const basePrimary: Record<string, string> = {}
export const themeSwatch = (theme: ThemeInstance, name: string) => {
  const colors = theme.computedThemes.value[name]?.colors ?? {}
  return {
    background: String(colors.background),
    surface: String(colors.surface),
    primary: basePrimary[name] ?? String(colors.primary),
    secondary: String(colors.secondary),
    ink: String(colors['on-surface']),
  }
}

let started = false

// Applies the saved appearance and keeps it applied; App.vue calls it once.
export const setupAppearance = (theme: ThemeInstance) => {
  if (started) return
  started = true

  for (const [name, def] of Object.entries(theme.themes.value)) {
    basePrimary[name] = String(def.colors.primary)
  }

  const applyTheme = () => {
    const name = resolveTheme()
    if (theme.global.name.value !== name) theme.change(name)
  }
  const applyAccent = () => {
    for (const [name, def] of Object.entries(theme.themes.value)) {
      def.colors.primary = appearance.accent || basePrimary[name]
    }
  }
  const body = document.body.classList
  const applyRadius = () => {
    body.remove(...radiusChoices.map(r => `ui-radius--${r}`))
    body.add(`ui-radius--${appearance.radius}`)
  }
  const applyDark = (dark: boolean) => {
    body.toggle('theme-dark', dark)
    body.toggle('theme-light', !dark)
  }

  watch(() => [appearance.theme, appearance.themeLight, appearance.themeDark], applyTheme, { immediate: true })
  watch(() => appearance.accent, applyAccent, { immediate: true })
  watch(() => appearance.radius, applyRadius, { immediate: true })
  watch(() => theme.current.value.dark, applyDark, { immediate: true })

  prefersDark?.addEventListener('change', applyTheme)
  // Keep other open tabs of the panel in step.
  window.addEventListener('storage', (e) => {
    if (e.key && Object.values(storageKeys).includes(e.key)) Object.assign(appearance, readAppearance())
  })
}
