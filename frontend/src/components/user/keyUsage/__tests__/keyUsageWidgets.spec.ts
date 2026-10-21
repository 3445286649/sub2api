import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import KeyUsageDeltaBadge from '../KeyUsageDeltaBadge.vue'
import KeyUsageGroupPill from '../KeyUsageGroupPill.vue'
import KeyUsageSparkline from '../KeyUsageSparkline.vue'
import KeyUsageTokenBreakdown from '../KeyUsageTokenBreakdown.vue'

vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))

function setViewport(desktop: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('min-width: 768px') ? desktop : !desktop,
    media: query, addEventListener: vi.fn(), removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia
}

describe('KeyUsageDeltaBadge', () => {
  const text = (current: number, previous?: number | null) => mount(KeyUsageDeltaBadge, { props: { current, previous } }).text()
  it('rounds to integers with direction and tone', () => {
    expect(text(112, 100)).toBe('▲ +12%')
    expect(text(80, 100)).toBe('▼ -20%')
    expect(text(100.4, 100)).toBe('▲ +<1%')
    expect(text(100, 100)).toBe('0%')
    const up = mount(KeyUsageDeltaBadge, { props: { current: 2, previous: 1 } })
    expect(up.find('.text-ku-up').exists()).toBe(true)
    const down = mount(KeyUsageDeltaBadge, { props: { current: 1, previous: 2 } })
    expect(down.find('.text-ku-down').exists()).toBe(true)
  })
  it('shows a dash when there is nothing to compare', () => {
    expect(text(5, 0)).toBe('—')
    expect(text(5, null)).toBe('—')
    expect(text(5)).toBe('—')
  })
})

describe('KeyUsageGroupPill', () => {
  it('colors by platform and keeps the full name in title', () => {
    const w = mount(KeyUsageGroupPill, { props: { name: 'ChatGpt-Pro尊享', platform: 'openai', title: 'full' } })
    expect(w.attributes('data-tone')).toBe('openai')
    expect(w.attributes('title')).toBe('full')
    expect(w.attributes('style')).toContain('var(--ku-group-openai)')
    expect(w.classes()).toContain('truncate')
    expect(mount(KeyUsageGroupPill, { props: { name: 'x', platform: 'composite' } }).attributes('data-tone')).toBe('other')
  })
})

describe('KeyUsageSparkline', () => {
  it('snaps hover to the nearest bucket with guide line and tooltip, ignores touch', async () => {
    const w = mount(KeyUsageSparkline, { props: { values: [1, 5, 3], labels: ['10/16 · $1.00', '10/17 · $5.00', '10/18 · $3.00'] } })
    const root = w.get('[data-testid="ku-sparkline"]')
    expect(root.attributes('aria-hidden')).toBe('true')
    root.element.getBoundingClientRect = () => ({ left: 0, width: 80, top: 0, height: 28, right: 80, bottom: 28, x: 0, y: 0, toJSON: () => ({}) })
    await root.trigger('pointermove', { clientX: 78, pointerType: 'mouse' })
    expect(w.get('[data-testid="ku-sparkline-tip"]').text()).toBe('10/18 · $3.00')
    expect(w.find('line').exists()).toBe(true)
    await root.trigger('pointerleave')
    expect(w.find('[data-testid="ku-sparkline-tip"]').exists()).toBe(false)
    await root.trigger('pointermove', { clientX: 40, pointerType: 'touch' })
    expect(w.find('[data-testid="ku-sparkline-tip"]').exists()).toBe(false)
  })
})

describe('KeyUsageTokenBreakdown', () => {
  const totals = { input_tokens: 600, output_tokens: 200, cache_read_tokens: 150, cache_creation_tokens: 50, total_tokens: 1000 }
  it('mobile: tap toggles the four rows inline without bubbling', async () => {
    setViewport(false)
    const parentClick = vi.fn()
    const w = mount({ components: { KeyUsageTokenBreakdown }, template: '<div @click="onClick"><KeyUsageTokenBreakdown :totals="totals" text="1K" /></div>', setup: () => ({ totals, onClick: parentClick }) })
    const btn = w.get('[data-testid="ku-token-toggle"]')
    expect(btn.attributes('aria-expanded')).toBe('false')
    await btn.trigger('click')
    expect(parentClick).not.toHaveBeenCalled()
    const rows = w.get('[data-testid="ku-token-rows"]').text()
    expect(rows).toContain('admin.usage.inputTokens')
    expect(rows).toContain('60.0%')
    expect(rows).toContain('usage.keySummary.cacheWrite')
    await btn.trigger('click')
    expect(w.find('[data-testid="ku-token-rows"]').exists()).toBe(false)
  })
  it('omits missing fields, hides shares on zero totals, and is static when nothing is known', async () => {
    setViewport(false)
    const partial = mount(KeyUsageTokenBreakdown, { props: { totals: { input_tokens: 0, output_tokens: 0, total_tokens: 0 }, text: '0' } })
    await partial.get('[data-testid="ku-token-toggle"]').trigger('click')
    const rows = partial.get('[data-testid="ku-token-rows"]').text()
    expect(rows).not.toContain('usage.keySummary.cacheRead')
    expect(rows).not.toContain('%')
    const none = mount(KeyUsageTokenBreakdown, { props: { totals: { total_tokens: 5 }, text: '5' } })
    expect(none.find('button').exists()).toBe(false)
    expect(none.text()).toBe('5')
  })
  it('desktop: renders a focusable hover tooltip trigger', () => {
    setViewport(true)
    const w = mount(KeyUsageTokenBreakdown, { props: { totals, text: '1K' }, attachTo: document.body })
    expect(w.find('[tabindex="0"]').text()).toBe('1K')
    expect(document.body.textContent).toContain('usage.keySummary.cacheRead')
    w.unmount()
  })
})
