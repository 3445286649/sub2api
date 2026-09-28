import { sanitizeUrl } from '@/utils/url'

/** index.html 中声明的默认图标；站点未配置 Logo 或配置被清空时回退到它 */
export const DEFAULT_FAVICON = '/logo.svg'

/** 上传的 Logo 可能是 SVG / PNG / JPG / ICO / WebP，MIME 必须跟着实际格式走 */
function faviconType(url: string): string {
  const path = url.split('?')[0]?.split('#')[0]?.toLowerCase() ?? ''
  if (path.endsWith('.svg')) return 'image/svg+xml'
  if (path.endsWith('.png')) return 'image/png'
  if (path.endsWith('.webp')) return 'image/webp'
  if (path.endsWith('.jpg') || path.endsWith('.jpeg')) return 'image/jpeg'
  if (path.endsWith('.ico')) return 'image/x-icon'
  if (path.startsWith('data:image/')) {
    // data URL 形如 data:image/png;base64,...，取第一个分号前的一段
    const head = path.slice('data:'.length)
    const mime = head.split(';')[0]?.trim()
    return mime && mime.includes('/') ? mime : 'image/png'
  }
  return 'image/png'
}

/**
 * 同步浏览器标签图标。传空值时回退到默认图标——
 * 否则后台清空 Logo 后，旧图标会一直留在标签页上直到刷新。
 */
export function updateFavicon(logoUrl: string): void {
  const sanitizedLogoUrl = sanitizeUrl(logoUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  const target = sanitizedLogoUrl || DEFAULT_FAVICON

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = faviconType(target)
  link.href = target
}
