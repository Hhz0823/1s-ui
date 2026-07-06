/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

// Styles
import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles/main.css'

import colors from 'vuetify/util/colors'
import { fa, en, vi, zhHans, zhHant, ru } from 'vuetify/locale'
import { normalizeLocale } from '@/locales'

// Composables
import { createVuetify } from 'vuetify'

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
  defaults: {
    VRow: { density: 'comfortable' },
    VTextField: {
      variant: 'solo-filled',
      rounded: 'lg',
      density: 'comfortable',
    },
    VSelect: {
      variant: 'solo-filled',
      rounded: 'lg',
      density: 'comfortable',
    },
    VCombobox: {
      variant: 'solo-filled',
      rounded: 'lg',
      density: 'comfortable',
    },
    VTextarea: {
      variant: 'solo-filled',
      rounded: 'lg',
      density: 'comfortable',
    },
    VBtn: {
      rounded: 'lg',
      elevation: 0,
    },
    VCard: {
      rounded: 'xl',
    },
    VSheet: {
      rounded: 'xl',
    },
  },
  theme: {
    defaultTheme: localStorage.getItem('theme') ?? 'system',
    themes: {
      light: {
        colors: {
          error: '#FF5252',
          background: colors.grey.lighten4,
        },
      },
      dark: {
        colors: {
          primary: colors.blue.darken4,
          error: colors.red.accent3,
        },
      },
      midnight: {
        dark: true,
        colors: {
          background: '#0D1117',
          surface: '#161B22',
          primary: '#58A6FF',
          secondary: '#79C0FF',
          error: '#F85149',
          success: '#3FB950',
          warning: '#E3B341',
          info: '#58A6FF',
          'on-surface': '#C9D1D9',
        },
      },
      ocean: {
        dark: true,
        colors: {
          background: '#0A1929',
          surface: '#132F4C',
          primary: '#00ACC1',
          secondary: '#4DD0E1',
          error: '#EF5350',
          success: '#66BB6A',
          warning: '#FFA726',
          info: '#29B6F6',
          'on-surface': '#B3E5FC',
        },
      },
      sunset: {
        dark: false,
        colors: {
          background: '#FFF8E1',
          surface: '#FFFFFF',
          primary: '#FF7043',
          secondary: '#FF8A65',
          error: '#E53935',
          success: '#43A047',
          warning: '#FB8C00',
          info: '#1E88E5',
          'on-surface': '#455A64',
        },
      },
      forest: {
        dark: true,
        colors: {
          background: '#1B2A1B',
          surface: '#2D4A2D',
          primary: '#66BB6A',
          secondary: '#81C784',
          error: '#EF5350',
          success: '#A5D6A7',
          warning: '#FFB74D',
          info: '#4FC3F7',
          'on-surface': '#C8E6C9',
        },
      },
      sakura: {
        dark: false,
        colors: {
          background: '#FFF0F5',
          surface: '#FFFFFF',
          primary: '#EC407A',
          secondary: '#F48FB1',
          error: '#E53935',
          success: '#43A047',
          warning: '#FB8C00',
          info: '#1E88E5',
          'on-surface': '#546E7A',
        },
      },
      cyberpunk: {
        dark: true,
        colors: {
          background: '#0F0E17',
          surface: '#1A1A2E',
          primary: '#FF006E',
          secondary: '#7B2FBE',
          error: '#FF006E',
          success: '#06D6A0',
          warning: '#FFBE0B',
          info: '#3A86FF',
          'on-surface': '#E0E0E0',
        },
      },
      nord: {
        dark: true,
        colors: {
          background: '#2E3440',
          surface: '#3B4252',
          primary: '#88C0D0',
          secondary: '#81A1C1',
          error: '#BF616A',
          success: '#A3BE8C',
          warning: '#EBCB8B',
          info: '#8FBCBB',
          'on-surface': '#D8DEE9',
        },
      },
      dracula: {
        dark: true,
        colors: {
          background: '#282A36',
          surface: '#44475A',
          primary: '#BD93F9',
          secondary: '#FFB86C',
          error: '#FF5555',
          success: '#50FA7B',
          warning: '#F1FA8C',
          info: '#8BE9FD',
          'on-surface': '#F8F8F2',
        },
      },
    },
  },
  locale: {
    locale: normalizeLocale(localStorage.getItem("locale")),
    fallback: 'zhHans',
    messages: { en, fa, vi, zhHans, zhHant, ru },
  },
})
