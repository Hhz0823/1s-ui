// Choices of the theme pickers in the app bar and on the login page.
export const defaultTheme = 'bt'

export const themeChoices = [
  { value: 'bt', icon: 'mdi-leaf-circle-outline' },
  { value: 'onepanel', icon: 'mdi-alpha-p-circle-outline' },
  { value: 'onepanelDark', icon: 'mdi-alpha-p-circle' },
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'midnight', icon: 'mdi-weather-night' },
  { value: 'ocean', icon: 'mdi-waves' },
  { value: 'sunset', icon: 'mdi-weather-sunset' },
  { value: 'forest', icon: 'mdi-pine-tree' },
  { value: 'sakura', icon: 'mdi-flower' },
  { value: 'daylight', icon: 'mdi-weather-sunny-alert' },
  { value: 'mint', icon: 'mdi-leaf' },
  { value: 'cyberpunk', icon: 'mdi-robot' },
  { value: 'nord', icon: 'mdi-snowflake' },
  { value: 'dracula', icon: 'mdi-bat' },
  { value: 'graphite', icon: 'mdi-circle-slice-8' },
  { value: 'system', icon: 'mdi-laptop' },
]

export const savedTheme = () => localStorage.getItem('theme') ?? defaultTheme
