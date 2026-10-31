import { describe, expect, it } from 'vitest'
import { customMenuExternalUrl, customMenuUrl } from '../custom-menu'
import type { CustomMenuItem } from '@/types'

const menu: CustomMenuItem = { id: 'docs', label: 'Docs', url: 'https://docs.example.com/guide?q=video', icon_svg: '', visibility: 'user', sort_order: 0 }

describe('custom navigation', () => {
  it('keeps existing menus embedded and selects direct links explicitly', () => {
    expect(customMenuExternalUrl(menu)).toBeUndefined()
    expect(customMenuExternalUrl({ ...menu, open_mode: 'embed' })).toBeUndefined()
    for (const open_mode of ['new_tab', 'redirect'] as const) {
      expect(customMenuExternalUrl({ ...menu, open_mode })).toBe(menu.url)
    }
  })
  it('does not forward identity or credentials', () => {
    const url = new URL(customMenuUrl(menu))
    expect(url.searchParams.toString()).toBe('q=video')
    expect(url.searchParams.has('token')).toBe(false)
    expect(url.searchParams.has('user_id')).toBe(false)
  })
  it('rejects disabled entries, executable URLs, credentials and protocol-relative URLs', () => {
    expect(customMenuUrl({ ...menu, enabled: false })).toBe('')
    for (const url of ['javascript:alert(1)', '//example.com', 'data:text/html,hello', 'https://user:pass@example.com/']) {
      expect(customMenuUrl({ ...menu, url })).toBe('')
    }
  })
})
