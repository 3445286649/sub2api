import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_FAVICON, updateFavicon } from '@/utils/branding'

describe('updateFavicon', () => {
  beforeEach(() => {
    // 模拟 index.html 里声明的默认图标
    document.head.innerHTML = '<link rel="icon" href="/logo.svg">'
  })

  function link(): HTMLLinkElement {
    return document.querySelector<HTMLLinkElement>('link[rel="icon"]') as HTMLLinkElement
  }

  it('replaces the default favicon with the configured logo', () => {
    updateFavicon('https://example.com/custom-logo.png')

    expect(link().href).toBe('https://example.com/custom-logo.png')
  })

  it('ignores unsafe logo URLs', () => {
    updateFavicon('javascript:alert(1)')

    expect(link().getAttribute('href')).toBe('/logo.svg')
  })

  // 后台清空 Logo 后必须回退，否则旧图标会一直留在标签页上
  it('restores the default favicon when the logo is cleared', () => {
    updateFavicon('https://example.com/custom-logo.png')
    updateFavicon('')

    expect(link().getAttribute('href')).toBe(DEFAULT_FAVICON)
    expect(link().type).toBe('image/svg+xml')
  })

  // MIME 必须跟着实际格式走，否则部分浏览器会拒绝渲染
  it.each([
    ['https://example.com/logo.svg', 'image/svg+xml'],
    ['https://example.com/logo.png', 'image/png'],
    ['https://example.com/logo.webp', 'image/webp'],
    ['https://example.com/logo.jpg', 'image/jpeg'],
    ['https://example.com/logo.jpeg', 'image/jpeg'],
    ['https://example.com/logo.ico', 'image/x-icon'],
    ['https://example.com/logo.svg?v=2', 'image/svg+xml'],
  ])('sets the favicon type for %s', (url, expected) => {
    updateFavicon(url)

    expect(link().type).toBe(expected)
  })

  it('derives the type from data URLs', () => {
    updateFavicon('data:image/png;base64,iVBORw0KGgo=')

    expect(link().type).toBe('image/png')
  })
})
