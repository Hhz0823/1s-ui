import { createI18n } from 'vue-i18n'
import en from './en'
import fa from './fa'
import vi from './vi'
import zhcn from './zhcn'
import zhtw from './zhtw'
import ru from './ru'

export const normalizeLocale = (value: string | null | undefined) => {
  switch ((value || '').toLowerCase()) {
    case 'zhcn':
    case 'zh-cn':
    case 'zh_hans':
    case 'zh-hans':
    case 'zhhans':
      return 'zhHans'
    case 'zhtw':
    case 'zh-tw':
    case 'zh_hant':
    case 'zh-hant':
    case 'zhhant':
      return 'zhHant'
    case 'en':
    case 'fa':
    case 'vi':
    case 'ru':
      return (value || '').toLowerCase()
    default:
      return 'zhHans'
  }
}

const initialLocale = normalizeLocale(localStorage.getItem("locale"))
if (localStorage.getItem("locale") !== initialLocale) {
  localStorage.setItem("locale", initialLocale)
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'zhHans',
  messages: {
    en: en,
    fa: fa,
    vi: vi,
    zhHans: zhcn,
    zhHant: zhtw,
    ru: ru
  },
})

export const locale = (() => {
  const l = i18n.global.locale.value
  switch (l) {
    case "zhHans":
      return "zh-cn"
    case "zhHant":
      return "zh-tw"
    default:
      return l
  }
})()

export const languages = [
  { title: 'English', value: 'en' },
  { title: 'فارسی', value: 'fa' },
  { title: 'Tiếng Việt', value: 'vi' },
  { title: '简体中文', value: 'zhHans' },
  { title: '繁體中文', value: 'zhHant' },
  { title: 'Русский', value: 'ru' },
]
