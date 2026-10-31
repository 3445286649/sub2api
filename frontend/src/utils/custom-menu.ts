import type { CustomMenuItem } from '@/types'
import { sanitizeUrl } from '@/utils/url'

/** Generic navigation must never attach the station's authentication credentials. */
export function customMenuUrl(item: Pick<CustomMenuItem, 'url' | 'enabled'>): string {
  if (item.enabled === false) return ''
  const url = sanitizeUrl(item.url)
  if (!url) return ''
  const parsed = new URL(url)
  return parsed.username || parsed.password ? '' : url
}

export function customMenuExternalUrl(item: CustomMenuItem): string | undefined {
  return item.open_mode === 'new_tab' || item.open_mode === 'redirect' ? customMenuUrl(item) || undefined : undefined
}
