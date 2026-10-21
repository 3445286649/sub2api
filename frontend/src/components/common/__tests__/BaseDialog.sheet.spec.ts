import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import BaseDialog from '../BaseDialog.vue'

function setMobile(mobile: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('max-width: 767.98px') ? mobile : false,
    media: query, addEventListener: vi.fn(), removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia
}

const render = (props: Record<string, unknown> = {}) => mount(BaseDialog, {
  attachTo: document.body,
  props: { show: true, title: 'Sheet', mobileSheet: true, closeOnClickOutside: true, ...props },
  slots: { default: '<div style="height:2000px" data-testid="content"><button>inside</button></div>' },
  global: { stubs: { Teleport: true, Transition: false } },
})

let now = 0
function drag(panel: Element, from: Element, dy: number, ms: number) {
  now = 1000
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  const fire = (type: string, y: number) => from.dispatchEvent(Object.assign(new MouseEvent(type, { bubbles: true, clientY: y }), { pointerId: 1 }))
  fire('pointerdown', 100)
  fire('pointermove', 100 + dy / 2)
  fire('pointermove', 100 + dy)
  now += ms
  fire('pointerup', 100 + dy)
}

describe('BaseDialog mobile sheet', () => {
  afterEach(() => { vi.restoreAllMocks(); document.body.innerHTML = '' })

  it('renders as a bottom sheet with a drag handle only on mobile', async () => {
    setMobile(true)
    const w = render(); await nextTick()
    expect(w.find('.modal-overlay--sheet').exists()).toBe(true)
    expect(w.find('.modal-content--sheet').exists()).toBe(true)
    expect(w.find('[data-sheet-handle]').attributes('aria-hidden')).toBe('true')
    w.unmount()
    setMobile(false)
    const desktop = render({ width: 'detail' }); await nextTick()
    expect(desktop.find('.modal-content--sheet').exists()).toBe(false)
    expect(desktop.get('[data-testid="base-dialog-panel"]').classes()).toContain('md:max-w-[880px]')
    desktop.unmount()
  })

  it('closes on a long or fast drag and springs back otherwise', async () => {
    setMobile(true)
    const w = render(); await nextTick()
    const panel = w.get('[data-testid="base-dialog-panel"]').element
    panel.getBoundingClientRect = () => ({ height: 800 } as DOMRect)
    const handle = w.get('[data-sheet-handle]').element
    drag(panel, handle, 60, 1000) // slow and short
    expect(w.emitted('close')).toBeUndefined()
    drag(panel, handle, 130, 1000) // > min(120, 25%)
    expect(w.emitted('close')).toHaveLength(1)
    drag(panel, handle, 60, 50) // 1.2px/ms > 0.5
    expect(w.emitted('close')).toHaveLength(2)
    w.unmount()
  })

  it('uses the smaller 25% threshold on short sheets', async () => {
    setMobile(true)
    const w = render(); await nextTick()
    const panel = w.get('[data-testid="base-dialog-panel"]').element
    panel.getBoundingClientRect = () => ({ height: 300 } as DOMRect)
    drag(panel, w.get('[data-sheet-handle]').element, 80, 1000) // > 75
    expect(w.emitted('close')).toHaveLength(1)
    w.unmount()
  })

  it('only drags from content when it is scrolled to the top', async () => {
    setMobile(true)
    const w = render(); await nextTick()
    const panel = w.get('[data-testid="base-dialog-panel"]').element
    panel.getBoundingClientRect = () => ({ height: 800 } as DOMRect)
    const body = w.get('.modal-body').element as HTMLElement
    const content = w.get('[data-testid="content"]').element
    body.scrollTop = 50
    Object.defineProperty(body, 'scrollTop', { value: 50, configurable: true })
    drag(panel, content, 200, 1000)
    expect(w.emitted('close')).toBeUndefined()
    Object.defineProperty(body, 'scrollTop', { value: 0, configurable: true })
    drag(panel, content, 200, 1000)
    expect(w.emitted('close')).toHaveLength(1)
    w.unmount()
  })

  it('closes on overlay click and Escape', async () => {
    setMobile(true)
    const w = render(); await nextTick()
    await w.get('.modal-overlay').trigger('click')
    expect(w.emitted('close')).toHaveLength(1)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(w.emitted('close')).toHaveLength(2)
    w.unmount()
  })
})
