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
import { resolveTheme } from './themes'

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
    defaultTheme: resolveTheme(),
    themes: {
      // Server-panel themes after BaoTa (宝塔) and 1Panel.
      bt: {
        dark: false,
        colors: {
          background: '#F0F2F5',
          surface: '#FFFFFF',
          primary: '#20A53A',
          secondary: '#3E4A56',
          error: '#EF4444',
          success: '#20A53A',
          warning: '#F0AD4E',
          info: '#3598DB',
          'on-surface': '#333333',
          'on-background': '#333333',
        },
      },
      onepanel: {
        dark: false,
        colors: {
          background: '#F2F3F5',
          surface: '#FFFFFF',
          primary: '#005EEB',
          secondary: '#646A73',
          error: '#F54A45',
          success: '#34A853',
          warning: '#FF8800',
          info: '#3370FF',
          'on-surface': '#1F2329',
          'on-background': '#1F2329',
        },
      },
      onepanelDark: {
        dark: true,
        colors: {
          background: '#141517',
          surface: '#1E1F22',
          primary: '#4C8DFF',
          secondary: '#8F959E',
          error: '#F76964',
          success: '#4CC06E',
          warning: '#FFA23A',
          info: '#5B9BFF',
          'on-surface': '#E6E8EB',
          'on-background': '#E6E8EB',
        },
      },
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
      daylight: {
        dark: false,
        colors: {
          background: '#F6F8FB',
          surface: '#FFFFFF',
          primary: '#2563EB',
          secondary: '#F59E0B',
          error: '#DC2626',
          success: '#16A34A',
          warning: '#D97706',
          info: '#0891B2',
          'on-surface': '#243244',
        },
      },
      mint: {
        dark: false,
        colors: {
          background: '#F2FBF7',
          surface: '#FFFFFF',
          primary: '#0F766E',
          secondary: '#8B5CF6',
          error: '#E11D48',
          success: '#059669',
          warning: '#B45309',
          info: '#0284C7',
          'on-surface': '#263B3B',
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
      graphite: {
        dark: true,
        colors: {
          background: '#111318',
          surface: '#1B1F27',
          primary: '#38BDF8',
          secondary: '#F97316',
          error: '#FB7185',
          success: '#34D399',
          warning: '#FBBF24',
          info: '#A78BFA',
          'on-surface': '#E5E7EB',
        },
      },
      // Dark companion of the BaoTa theme.
      btDark: {
        dark: true,
        colors: {
          background: '#17191C',
          surface: '#212429',
          primary: '#2DBE4A',
          secondary: '#8A94A0',
          error: '#F56C6C',
          success: '#2DBE4A',
          warning: '#E6A23C',
          info: '#409EFF',
          'on-surface': '#E3E6EA',
          'on-background': '#E3E6EA',
        },
      },
      // Palettes after well-known editor color schemes.
      githubLight: {
        dark: false,
        colors: {
          background: '#F6F8FA',
          surface: '#FFFFFF',
          primary: '#0969DA',
          secondary: '#8250DF',
          error: '#CF222E',
          success: '#1A7F37',
          warning: '#9A6700',
          info: '#0969DA',
          'on-surface': '#1F2328',
          'on-background': '#1F2328',
        },
      },
      solarizedLight: {
        dark: false,
        colors: {
          background: '#EEE8D5',
          surface: '#FDF6E3',
          primary: '#268BD2',
          secondary: '#2AA198',
          error: '#DC322F',
          success: '#859900',
          warning: '#B58900',
          info: '#6C71C4',
          'on-surface': '#586E75',
          'on-background': '#586E75',
        },
      },
      solarizedDark: {
        dark: true,
        colors: {
          background: '#002B36',
          surface: '#073642',
          primary: '#268BD2',
          secondary: '#2AA198',
          error: '#DC322F',
          success: '#859900',
          warning: '#B58900',
          info: '#6C71C4',
          'on-surface': '#93A1A1',
          'on-background': '#93A1A1',
        },
      },
      latte: {
        dark: false,
        colors: {
          background: '#E6E9EF',
          surface: '#EFF1F5',
          primary: '#8839EF',
          secondary: '#1E66F5',
          error: '#D20F39',
          success: '#40A02B',
          warning: '#DF8E1D',
          info: '#04A5E5',
          'on-surface': '#4C4F69',
          'on-background': '#4C4F69',
        },
      },
      mocha: {
        dark: true,
        colors: {
          background: '#181825',
          surface: '#1E1E2E',
          primary: '#CBA6F7',
          secondary: '#89B4FA',
          error: '#F38BA8',
          success: '#A6E3A1',
          warning: '#F9E2AF',
          info: '#89DCEB',
          'on-surface': '#CDD6F4',
          'on-background': '#CDD6F4',
        },
      },
      tokyoNight: {
        dark: true,
        colors: {
          background: '#16161E',
          surface: '#1A1B26',
          primary: '#7AA2F7',
          secondary: '#BB9AF7',
          error: '#F7768E',
          success: '#9ECE6A',
          warning: '#E0AF68',
          info: '#7DCFFF',
          'on-surface': '#C0CAF5',
          'on-background': '#C0CAF5',
        },
      },
      gruvbox: {
        dark: true,
        colors: {
          background: '#1D2021',
          surface: '#282828',
          primary: '#FE8019',
          secondary: '#83A598',
          error: '#FB4934',
          success: '#B8BB26',
          warning: '#FABD2F',
          info: '#83A598',
          'on-surface': '#EBDBB2',
          'on-background': '#EBDBB2',
        },
      },
      rosePine: {
        dark: true,
        colors: {
          background: '#191724',
          surface: '#1F1D2E',
          primary: '#EBBCBA',
          secondary: '#C4A7E7',
          error: '#EB6F92',
          success: '#9CCFD8',
          warning: '#F6C177',
          info: '#31748F',
          'on-surface': '#E0DEF4',
          'on-background': '#E0DEF4',
        },
      },
      // Warm and soft light themes.
      lavender: {
        dark: false,
        colors: {
          background: '#F5F3FF',
          surface: '#FFFFFF',
          primary: '#7C3AED',
          secondary: '#DB2777',
          error: '#DC2626',
          success: '#16A34A',
          warning: '#D97706',
          info: '#2563EB',
          'on-surface': '#2E2A47',
          'on-background': '#2E2A47',
        },
      },
      coffee: {
        dark: false,
        colors: {
          background: '#F5EFE6',
          surface: '#FFFCF7',
          primary: '#8B5E3C',
          secondary: '#C08552',
          error: '#C0392B',
          success: '#5B8C3A',
          warning: '#D68910',
          info: '#2E86AB',
          'on-surface': '#3E2C23',
          'on-background': '#3E2C23',
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
