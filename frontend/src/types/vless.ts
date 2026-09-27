export type VlessQuickAddVariant = 'reality-vision' | 'reality-xhttp' | 'reality-xhttp-vision' | 'enc-vision' | 'enc-xhttp' | 'tls'

export interface VlessQuickAddOptions {
  variant: VlessQuickAddVariant
  reality_server: string
}

// Variants the panel only builds on Xray-core.
export const vlessXrayOnlyVariants: VlessQuickAddVariant[] = ['reality-xhttp', 'reality-xhttp-vision', 'enc-vision', 'enc-xhttp']

export const vlessQuickAddVariants: { value: VlessQuickAddVariant, key: string }[] = [
  { value: 'reality-vision', key: 'realityVision' },
  { value: 'reality-xhttp', key: 'realityXhttp' },
  { value: 'reality-xhttp-vision', key: 'realityXhttpVision' },
  { value: 'enc-vision', key: 'encVision' },
  { value: 'enc-xhttp', key: 'encXhttp' },
  { value: 'tls', key: 'tls' },
]

export function createVlessQuickAddOptions(): VlessQuickAddOptions {
  return { variant: 'reality-vision', reality_server: '' }
}

export function vlessVariantIsReality(variant: VlessQuickAddVariant): boolean {
  return variant === 'reality-vision' || variant === 'reality-xhttp' || variant === 'reality-xhttp-vision'
}

// Mirrors the panel's REALITY target check: a bare domain name.
export function normalizeRealityServer(value: string): string {
  const server = String(value || '').trim().toLowerCase()
  if (!server) return ''
  if (server.length > 253 || /[\s\/?#@:\[\]]/.test(server) || !server.includes('.') || /^[\d.]+$/.test(server)) {
    throw new Error('invalid REALITY target')
  }
  return server
}

// Targets Xray-core warns about (infra/conf/transport_security.go): imitating
// them makes the GFW more likely to block the server's IP.
export function realityTargetDiscouraged(value: string): boolean {
  const server = String(value || '').trim().toLowerCase()
  if (!server) return false
  return ['.cn', '.ru', '.ir'].some(suffix => server.endsWith(suffix))
    || ['apple', 'icloud', 'microsoft'].some(name => server.includes(name))
}
