import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import HeaderQuickLinks from '../HeaderQuickLinks.vue'
import type { HeaderButton } from '@/types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: () => '更多链接' }) }))
let resize: () => void
let available = 520
const buttons: HeaderButton[] = ['充值商城', '使用帮助', '模型雷达', '兜底分组'].map(name => ({ action: 'external', name, url: 'https://example.com', enabled: true }))
beforeEach(() => {
  available = 520
  vi.stubGlobal('ResizeObserver', class { constructor(callback: () => void) { resize = callback } observe() {} disconnect() {} })
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { callback(0); return 1 })
  vi.stubGlobal('cancelAnimationFrame', () => {})
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    return { width: this.dataset.testid === 'header-quick-links' ? available : 100 } as DOMRect
  })
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })
const visibleButtons = (wrapper: ReturnType<typeof mount>) => wrapper.findAll('button').filter(button => !button.element.closest('[inert]'))

describe('responsive header shortcuts', () => {
  it('shows the fourth link when there is room and moves overflow into its own menu', async () => {
    const wrapper = mount(HeaderQuickLinks, { props: { buttons } })
    await nextTick(); await nextTick()
    expect(visibleButtons(wrapper).map(button => button.text())).toEqual(buttons.map(button => button.name))
    available = 320
    resize(); await nextTick()
    expect(visibleButtons(wrapper).map(button => button.text())).toEqual(['充值商城', '使用帮助', '更多链接'])
    await wrapper.get('[aria-haspopup]').trigger('click')
    const fallback = visibleButtons(wrapper).find(button => button.text() === '兜底分组')!
    await fallback.trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([buttons[3]])
    expect(wrapper.get('[aria-haspopup]').attributes('aria-expanded')).toBe('false')
    available = 520
    resize(); await nextTick()
    expect(visibleButtons(wrapper).map(button => button.text())).toEqual(buttons.map(button => button.name))
    wrapper.unmount()
  })
  it('keeps all links reachable on narrow screens and closes with Escape', async () => {
    available = 105
    const wrapper = mount(HeaderQuickLinks, { props: { buttons }, attachTo: document.body })
    await nextTick(); await nextTick()
    expect(visibleButtons(wrapper)).toHaveLength(1)
    await wrapper.get('[aria-haspopup]').trigger('click')
    expect(visibleButtons(wrapper)).toHaveLength(5)
    await wrapper.get('[data-testid="header-quick-links"]').trigger('keydown', { key: 'Escape' })
    expect(wrapper.get('[aria-haspopup]').attributes('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(wrapper.get('[aria-haspopup]').element)
    wrapper.unmount()
  })
})
