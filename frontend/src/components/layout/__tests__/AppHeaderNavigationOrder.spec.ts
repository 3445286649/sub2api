import { describe, expect, it } from 'vitest'
import { resolveHeaderButtons, isHeaderButtonURLSafe } from '@/utils/headerButtons'

describe('header navigation configuration', () => {
  it('preserves legacy order and targets until configured', () => {
    const buttons = resolveHeaderButtons({ support_group_link_url: 'https://chat.example.com', support_group_enabled: true }, key => key)
    expect(buttons.map(button => button.action)).toEqual(['recharge', 'rebate', 'help', 'radar', 'support', 'pixmo'])
    expect(buttons[4]).toMatchObject({ url: 'https://chat.example.com', enabled: true })
  })
  it('respects an empty list and custom order', () => {
    expect(resolveHeaderButtons({ header_buttons: [] }, key => key)).toEqual([])
    const buttons = [{ action: 'external' as const, name: 'Status', url: 'https://example.com', enabled: true }]
    expect(resolveHeaderButtons({ header_buttons: buttons }, key => key)).toEqual(buttons)
  })
  it('blocks executable URLs and credentials', () => {
    for (const url of ['javascript:alert(1)', '//example.com', 'data:text/html,x', 'https://user:pass@example.com']) {
      expect(isHeaderButtonURLSafe(url)).toBe(false)
    }
    expect(isHeaderButtonURLSafe('https://example.com/path?q=1#section')).toBe(true)
  })
})
