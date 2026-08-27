export type NaiveQuickAddMode = 'https' | 'quic'

export interface NaiveQuickAddOptions {
  username: string
  password: string
  server: string
  mode: NaiveQuickAddMode
  tls_id: number
  extra_headers_text: string
  udp_over_tcp: boolean
  insecure_concurrency: number
  quic_congestion_control: '' | 'bbr' | 'bbr2' | 'cubic' | 'reno'
}

export function createNaiveQuickAddOptions(server = ''): NaiveQuickAddOptions {
  return {
    username: '',
    password: '',
    server,
    mode: 'quic',
    tls_id: 0,
    extra_headers_text: '',
    udp_over_tcp: true,
    insecure_concurrency: 0,
    quic_congestion_control: 'bbr',
  }
}

export function normalizeNaiveServer(value: string): string {
  let server = String(value || '').trim()
  if (server.startsWith('[')) {
    if (!server.endsWith(']')) throw new Error('invalid NaiveProxy server')
    server = server.slice(1, -1)
  }
  if (!server || server.length > 255 || /[\s\/?#@]/.test(server) || server.includes('://')) {
    throw new Error('invalid NaiveProxy server')
  }
  if ((server.match(/:/g) || []).length === 1) throw new Error('NaiveProxy server must not include a port')
  if (server.includes(':')) {
    try {
      new URL(`http://[${server}]/`)
    } catch {
      throw new Error('invalid NaiveProxy IPv6 server')
    }
  }
  return server
}

export function parseNaiveExtraHeaders(value: string): Record<string, string> {
  const headers: Record<string, string> = {}
  for (const rawLine of String(value || '').split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line) continue
    const separator = line.indexOf(':')
    if (separator <= 0) throw new Error('invalid NaiveProxy extra header')
    const name = line.slice(0, separator).trim()
    const headerValue = line.slice(separator + 1).trim()
    if (!/^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(name) || !headerValue || headerValue.length > 4096) {
      throw new Error('invalid NaiveProxy extra header')
    }
    if (['proxy-authorization', 'padding', 'content-length', 'transfer-encoding', 'connection'].includes(name.toLowerCase())) {
      throw new Error('reserved NaiveProxy extra header')
    }
    headers[name] = headerValue
  }
  if (Object.keys(headers).length > 32) throw new Error('too many NaiveProxy extra headers')
  return headers
}
