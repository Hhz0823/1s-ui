import type { AgentNode, AgentUsage } from '@/types/agents'

// Formatting and display helpers shared by the server monitor pages.

export const usagePercent = (value?: AgentUsage) => value?.total ? Number(value.used || 0) * 100 / value.total : undefined

export const clampPercent = (value?: number) => Math.max(0, Math.min(100, Number(value) || 0))

export const percent = (value?: number) => {
  if (value == null || Number.isNaN(value)) return '-'
  return `${value.toFixed(value > 0 && value < 1 ? 2 : 1)}%`
}

const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

// bytes uses 0 decimals for B and KB, 2 for MB and 1 above, and none once a
// value reaches three digits.
export const bytes = (value?: number) => {
  let size = Math.max(0, Number(value) || 0)
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  const digits = size >= 100 || unit < 2 ? 0 : unit === 2 ? 2 : 1
  return `${size.toFixed(digits)} ${units[unit]}`
}

export const rate = (value?: number) => value == null ? '-' : `${bytes(value)}/s`

export const usageText = (value?: AgentUsage) => value?.total ? `${bytes(value.used)} / ${bytes(value.total)}` : '-'

// usageTone follows the monitor's thresholds: red from 80%, orange from 60%.
export const usageTone = (value?: number) => {
  const current = Number(value) || 0
  return current >= 80 ? 'error' : current >= 60 ? 'warning' : 'success'
}

export const uptimeText = (seconds: number | undefined, t: (key: string, values?: Record<string, unknown>) => string) => {
  if (!seconds) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const parts = []
  if (days) parts.push(t('monitor.days', { n: days }))
  if (hours || days) parts.push(t('monitor.hours', { n: hours }))
  parts.push(t('monitor.minutes', { n: minutes }))
  return parts.join(' ')
}

export const flagEmoji = (region?: string) => {
  const code = String(region || '').toUpperCase()
  if (!/^[A-Z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...[...code].map(char => 0x1f1e6 + char.charCodeAt(0) - 65))
}

const osIcons: [RegExp, string][] = [
  [/ubuntu/i, 'mdi-ubuntu'],
  [/debian/i, 'mdi-debian'],
  [/centos/i, 'mdi-centos'],
  [/red ?hat|rhel|rocky|alma/i, 'mdi-redhat'],
  [/fedora/i, 'mdi-fedora'],
  [/arch/i, 'mdi-arch'],
  [/gentoo/i, 'mdi-gentoo'],
  [/opensuse|suse/i, 'mdi-linux'],
  [/windows/i, 'mdi-microsoft-windows'],
  [/darwin|mac/i, 'mdi-apple'],
  [/freebsd/i, 'mdi-freebsd'],
]

export const osName = (node: AgentNode) => node.report.platform || node.report.os || ''

export const osIcon = (node: AgentNode) => {
  const name = `${node.report.platform || ''} ${node.report.os || ''}`
  return osIcons.find(([pattern]) => pattern.test(name))?.[1] || (node.report.os ? 'mdi-linux' : 'mdi-help-circle-outline')
}

export const nodeOnline = (node: AgentNode) => Boolean(node.online)

// trafficUsed picks the counted traffic for a limit type.
export const trafficUsed = (node: AgentNode) => {
  const sent = Number(node.traffic?.sent || 0)
  const recv = Number(node.traffic?.recv || 0)
  switch (node.traffic_limit_type) {
    case 'max': return Math.max(sent, recv)
    case 'min': return Math.min(sent, recv)
    case 'up': return sent
    case 'down': return recv
    default: return sent + recv
  }
}

export const trafficPercent = (node: AgentNode) => node.traffic_limit ? trafficUsed(node) * 100 / node.traffic_limit : undefined

export const daysLeft = (expireAt?: number) => expireAt ? Math.ceil((expireAt * 1000 - Date.now()) / 86400000) : undefined

// Tag colors, cycled for tags that do not name one.
export const tagPalette: Record<string, string> = {
  ruby: '#e54666', gray: '#8d8d8d', gold: '#978365', bronze: '#a18072', brown: '#ad7f58', yellow: '#d5ae39',
  amber: '#ffc53d', orange: '#f76b15', tomato: '#e54d2e', red: '#e5484d', crimson: '#e93d82', pink: '#d6409f',
  plum: '#ab4aba', purple: '#8e4ec6', violet: '#6e56cf', iris: '#5b5bd6', indigo: '#3e63dd', blue: '#0090ff',
  cyan: '#00a2c7', teal: '#12a594', jade: '#29a383', green: '#30a46c', grass: '#46a758', lime: '#bdee63',
  mint: '#86ead4', sky: '#7ce2fe',
}
const tagColors = Object.keys(tagPalette)

// parseTags splits "CN2<red>;IPv6" into labelled, colored tags.
export const parseTags = (value?: string) => String(value || '')
  .split(';')
  .map(part => part.trim())
  .filter(Boolean)
  .map((part, index) => {
    const match = part.match(/^(.*)<([a-z]+)>$/i)
    const name = match?.[2]?.toLowerCase()
    const label = (match ? match[1] : part).trim()
    const color = name && tagPalette[name] ? name : tagColors[index % tagColors.length]
    return { label, color: tagPalette[color] }
  })
  .filter(tag => tag.label)

export const billingCycles = [30, 92, 182, 365, 730, 1095, 1825, -1]

export const cycleKey = (days?: number) => {
  const value = Number(days || 0)
  if (value === -1) return 'once'
  if (value >= 27 && value <= 32) return 'monthly'
  if (value >= 87 && value <= 95) return 'quarterly'
  if (value >= 175 && value <= 185) return 'semiAnnual'
  if (value >= 360 && value <= 370) return 'annual'
  if (value >= 720 && value <= 750) return 'biennial'
  if (value >= 1080 && value <= 1150) return 'triennial'
  if (value >= 1800 && value <= 1850) return 'quinquennial'
  return ''
}

// ewma smooths a series; null gaps are kept.
export const ewma = (values: (number | null)[], alpha = 0.3) => {
  let last: number | null = null
  return values.map(value => {
    if (value == null) return null
    last = last == null ? value : alpha * value + (1 - alpha) * last
    return last
  })
}
