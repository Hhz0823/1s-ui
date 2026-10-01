// A panel update replaces the UI files, so a tab opened before it can ask for
// page chunks that no longer exist. Reload such a tab once to pick up the new
// UI; the guard stops a reload loop when the files are really missing.
const reloadKey = 'sui-chunk-reload'

export const isChunkLoadError = (error: unknown): boolean => {
  const message = String((error as any)?.message ?? error ?? '')
  return /dynamically imported module|Importing a module script failed|Unable to preload CSS|error loading dynamically imported/i.test(message)
}

export const reloadForNewChunks = (href?: string): boolean => {
  let reloadedAt = 0
  try { reloadedAt = Number(sessionStorage.getItem(reloadKey) || 0) } catch {}
  if (Date.now() - reloadedAt < 30000) return false
  try { sessionStorage.setItem(reloadKey, String(Date.now())) } catch {}
  if (href) window.location.assign(href)
  else window.location.reload()
  return true
}
