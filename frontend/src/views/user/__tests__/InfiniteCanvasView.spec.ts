import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { ApiKey } from '@/types'
import InfiniteCanvasView from '../InfiniteCanvasView.vue'
import { CANVAS_PROTOCOL, CANVAS_PROTOCOL_VERSION, CANVAS_SETTINGS_REFRESH_MS, CANVAS_CONNECT_TIMEOUT_MS } from '@/utils/infiniteCanvas'

const mocks = vi.hoisted(() => ({ list: vi.fn(), getById: vi.fn(), fetchSettings: vi.fn(), fetch: vi.fn(),
  app: {} as any, auth: {} as any }))
vi.mock('@/api/keys', () => ({ keysAPI: { list: mocks.list, getById: mocks.getById } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh-CN' } }) }))

const ownKey = { id: 1, user_id: 1, name: 'My image key', key: 'sk-canvas-fixture', status: 'active',
  quota: 0, quota_used: 0, expires_at: null, group: { name: 'Images', platform: 'openai', allow_image_generation: true } } as ApiKey
let wrapper: VueWrapper | undefined

function view() {
  wrapper = mount(InfiniteCanvasView, { attachTo: document.body, global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Teleport: true, RouterLink: { template: '<a><slot /></a>' } } } })
  return wrapper
}
async function open() {
  const w = view()
  await flushPromises()
  await w.get('#canvas-image-key').setValue('1')
  await flushPromises()
  await w.findAll('button').find(button => button.text() === 'infiniteCanvas.enter')!.trigger('click')
  await flushPromises()
  const post = vi.spyOn((w.get('iframe').element as HTMLIFrameElement).contentWindow!, 'postMessage').mockImplementation(() => {})
  message(w, 'READY')
  await flushPromises()
  const init = post.mock.calls.find(([data]) => data.type === 'INIT')![0]
  message(w, 'ACK', { sessionId: init.sessionId, revision: init.revision })
  await flushPromises()
  return w
}
function message(w: VueWrapper, type: string, extra = {}, origin = window.location.origin) {
  window.dispatchEvent(new MessageEvent('message', { origin, source: (w.get('iframe').element as HTMLIFrameElement).contentWindow,
    data: { protocol: CANVAS_PROTOCOL, version: CANVAS_PROTOCOL_VERSION, type, ...extra } }))
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  localStorage.setItem('auth_token', 'login-fixture')
  mocks.auth = reactive({ isAuthenticated: true, user: { id: 1 }, token: 'login-fixture' })
  mocks.app = reactive({ cachedPublicSettings: { infinite_canvas_enabled: true }, fetchPublicSettings: mocks.fetchSettings })
  mocks.fetchSettings.mockResolvedValue({ infinite_canvas_enabled: true })
  mocks.list.mockResolvedValue({ items: [ownKey], total: 1, page_size: 100 })
  mocks.getById.mockResolvedValue(ownKey)
  mocks.fetch.mockResolvedValue({ ok: true, json: async () => ({ data: [{ id: 'gpt-image-2' }] }) })
  vi.stubGlobal('fetch', mocks.fetch)
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

describe('Infinite Canvas multi-connection host', () => {
  it('lists Gemini image keys and sends their platform through the acknowledged handshake', async () => {
    const gemini = { ...ownKey, group: { ...ownKey.group, platform: 'gemini' } } as ApiKey
    mocks.list.mockResolvedValue({ items: [gemini], total: 1, page_size: 100 })
    mocks.getById.mockResolvedValue(gemini)
    const w = await open()
    const post = vi.mocked((w.get('iframe').element as HTMLIFrameElement).contentWindow!.postMessage)
    const init = post.mock.calls.find(([data]) => data.type === 'INIT')![0]
    expect(init.config.connections).toEqual([expect.objectContaining({ capability: 'image', platform: 'gemini', apiKeyId: 1 })])
    expect(JSON.parse(localStorage.getItem('sub2api.canvas.connections.1')!).selection).toEqual({ image: [1], text: [], video: [] })
    expect(JSON.stringify(localStorage)).not.toContain(gemini.key)
  })
  it('disconnects a Gemini image connection when its group revokes image permission', async () => {
    vi.useFakeTimers()
    const gemini = { ...ownKey, group: { ...ownKey.group, platform: 'gemini' } } as ApiKey
    mocks.list.mockResolvedValue({ items: [gemini], total: 1, page_size: 100 })
    mocks.getById.mockResolvedValue(gemini)
    const w = await open()
    mocks.getById.mockResolvedValue({ ...gemini, group: { ...gemini.group, allow_image_generation: false } })
    await vi.advanceTimersByTimeAsync(CANVAS_SETTINGS_REFRESH_MS)
    await flushPromises()
    expect(w.find('iframe').exists()).toBe(false)
    expect(localStorage.getItem('sub2api.canvas.connections.1')).toBeNull()
  })
  it('starts empty with three optional purposes and never renders secrets', async () => {
    const w = view(); await flushPromises()
    expect(w.findAll('fieldset')).toHaveLength(3)
    expect(w.find('iframe').exists()).toBe(false)
    expect(w.html()).not.toContain(ownKey.key)
    expect(w.get('#canvas-video-key').findAll('option')).toHaveLength(1)
  })
  it('keeps the remove action separate from a long group label', async () => {
    const longName = 'Very long image generation group '.repeat(12)
    mocks.list.mockResolvedValue({ items: [{ ...ownKey, group: { ...ownKey.group, name: longName } }], total: 1, page_size: 100 })
    const w = view(); await flushPromises()
    await w.get('#canvas-image-key').setValue('1')
    const label = w.get(`span[title*="${longName.slice(0, 30)}"]`)
    expect(label.classes()).toContain('truncate')
    expect(label.element.parentElement?.className).toContain('grid-cols-[auto_minmax(0,1fr)_auto_auto]')
    const remove = w.findAll('button').find(b => b.text() === 'infiniteCanvas.remove')!
    expect(remove.classes()).toContain('shrink-0')
    await remove.trigger('click')
    expect(w.findAll('button').some(b => b.text() === 'infiniteCanvas.remove')).toBe(false)
  })
  it('persists only IDs after acknowledged initialization and restores with ownership validation', async () => {
    const w = await open()
    expect(JSON.parse(localStorage.getItem('sub2api.canvas.connections.1')!).selection.image).toEqual([1])
    expect(JSON.stringify(localStorage)).not.toContain(ownKey.key)
    w.unmount(); mocks.getById.mockClear()
    const restored = view(); await flushPromises()
    expect(mocks.getById).toHaveBeenCalledWith(1)
    expect(restored.find('iframe').exists()).toBe(true)
  })
  it('migrates the old image selection without choosing text or video keys', async () => {
    localStorage.setItem('sub2api.canvas.key.1', '1')
    const w = view(); await flushPromises()
    const post = vi.spyOn((w.get('iframe').element as HTMLIFrameElement).contentWindow!, 'postMessage').mockImplementation(() => {})
    message(w, 'READY'); await flushPromises()
    expect(post.mock.calls[0]![0].config).toMatchObject({ legacyImageKeyId: 1, connections: [{ id: 'image-1', capability: 'image' }] })
  })
  it('updates multiple connections only on matching ACK without replacing the frame', async () => {
    const textKey = { ...ownKey, id: 2, key: 'text-fixture', group: { ...ownKey.group, allow_image_generation: false } }
    mocks.list.mockResolvedValue({ items: [ownKey, textKey], total: 2, page_size: 100 })
    mocks.getById.mockImplementation(async (id: number) => id === 2 ? textKey : ownKey)
    const w = await open(); const frame = w.get('iframe').element as HTMLIFrameElement
    const post = vi.mocked(frame.contentWindow!.postMessage)
    await w.get('[data-testid="canvas-settings"]').trigger('click'); await flushPromises()
    await w.get('#canvas-text-key').setValue('2')
    await w.findAll('button').find(b => b.text() === 'infiniteCanvas.apply')!.trigger('click'); await flushPromises()
    const update = post.mock.calls.find(([data]) => data.type === 'UPDATE')![0]
    expect(update.config.connections.map((c: any) => c.id)).toEqual(['image-1', 'text-2'])
    message(w, 'ACK', { sessionId: update.sessionId, revision: update.revision - 1 }); await flushPromises()
    expect(JSON.parse(localStorage.getItem('sub2api.canvas.connections.1')!).selection.text).toEqual([])
    message(w, 'ACK', { sessionId: update.sessionId, revision: update.revision }); await flushPromises()
    expect(JSON.parse(localStorage.getItem('sub2api.canvas.connections.1')!).selection.text).toEqual([2])
    expect(w.get('iframe').element).toBe(frame)
    expect(post.mock.calls.some(([data]) => data.type === 'DISPOSE')).toBe(false)
    expect(JSON.stringify(localStorage)).not.toContain('text-fixture')
  })
  it('cancelled drafts do not alter active connections', async () => {
    const w = await open(); const original = w.get('iframe').element
    await w.get('[data-testid="canvas-settings"]').trigger('click'); await flushPromises()
    await w.get('#canvas-text-key').setValue('1')
    await w.findAll('button').find(b => b.text() === 'common.cancel')!.trigger('click')
    expect(w.get('iframe').element).toBe(original)
    expect(JSON.parse(localStorage.getItem('sub2api.canvas.connections.1')!).selection.text).toEqual([])
  })
  it('rejects pasted keys not owned by this user and clears the input', async () => {
    const w = view(); await flushPromises()
    await w.get('input[type=password]').setValue('foreign-fixture')
    await w.findAll('button').find(b => b.text() === 'infiniteCanvas.add')!.trigger('click')
    expect(w.text()).toContain('infiniteCanvas.invalidPastedKey')
    expect((w.get('input[type=password]').element as HTMLInputElement).value).toBe('')
    expect(w.find('iframe').exists()).toBe(false)
  })
  it('discards late ownership responses after cancelling or changing identity', async () => {
    const w = view(); await flushPromises()
    let resolve!: (value: ApiKey) => void
    mocks.getById.mockReturnValue(new Promise<ApiKey>(done => { resolve = done }))
    await w.get('#canvas-image-key').setValue('1')
    await w.findAll('button').find(b => b.text() === 'infiniteCanvas.enter')!.trigger('click'); await flushPromises()
    mocks.auth.user = { id: 2 }; await flushPromises(); resolve(ownKey); await flushPromises()
    expect(w.find('iframe').exists()).toBe(false)
    expect(JSON.stringify(localStorage)).not.toContain('sub2api.canvas.connections')
  })
  it('does not list or initialize when disabled', async () => {
    mocks.app.cachedPublicSettings.infinite_canvas_enabled = false
    const w = view(); await flushPromises()
    expect(mocks.list).not.toHaveBeenCalled(); expect(w.find('iframe').exists()).toBe(false)
  })
  it('paginates and excludes foreign, inactive and unsupported keys', async () => {
    mocks.list.mockResolvedValueOnce({ items: [ownKey, { ...ownKey, id: 2, user_id: 2 }], total: 3, page_size: 2 })
      .mockResolvedValueOnce({ items: [{ ...ownKey, id: 3 }], total: 3, page_size: 2 })
    const w = view(); await flushPromises()
    expect(mocks.list).toHaveBeenCalledTimes(2)
    expect(w.findAll('#canvas-image-key option').map(o => o.attributes('value'))).toEqual(['', '1', '3'])
  })
  it('blocks untrusted READY and does not send a key that failed a fresh ownership check', async () => {
    const w = view(); await flushPromises()
    await w.get('#canvas-image-key').setValue('1')
    await w.findAll('button').find(b => b.text() === 'infiniteCanvas.enter')!.trigger('click'); await flushPromises()
    const post = vi.spyOn((w.get('iframe').element as HTMLIFrameElement).contentWindow!, 'postMessage').mockImplementation(() => {})
    message(w, 'READY', {}, 'https://other.test'); await flushPromises(); expect(post).not.toHaveBeenCalled()
    mocks.getById.mockResolvedValue({ ...ownKey, user_id: 2 })
    message(w, 'READY'); await flushPromises()
    expect(post.mock.calls.some(([data]) => data.type === 'INIT')).toBe(false)
    expect(w.find('iframe').exists()).toBe(false)
  })
  it('disposes on revoked keys, feature disable and cross-tab logout', async () => {
    vi.useFakeTimers()
    const w = await open()
    mocks.getById.mockResolvedValue({ ...ownKey, status: 'revoked' })
    await vi.advanceTimersByTimeAsync(CANVAS_SETTINGS_REFRESH_MS); await flushPromises()
    expect(w.find('iframe').exists()).toBe(false)
    expect(localStorage.getItem('sub2api.canvas.connections.1')).toBeNull()
  })
  it('disposes when the feature is switched off', async () => {
    const w = await open(); mocks.app.cachedPublicSettings.infinite_canvas_enabled = false; await flushPromises()
    expect(w.find('iframe').exists()).toBe(false)
  })
  it('disposes on cross-tab logout but preserves same-user profile refresh', async () => {
    const w = await open(); mocks.auth.user = { id: 1, balance: 9 }; await flushPromises()
    expect(w.find('iframe').exists()).toBe(true)
    window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token', newValue: null })); await flushPromises()
    expect(w.find('iframe').exists()).toBe(false)
  })
  it('offers recovery for an incompatible canvas protocol without persisting an unacknowledged selection', async () => {
    vi.useFakeTimers()
    const w = view(); await flushPromises()
    await w.get('#canvas-image-key').setValue('1')
    await w.findAll('button').find(b => b.text() === 'infiniteCanvas.enter')!.trigger('click'); await flushPromises()
    await vi.advanceTimersByTimeAsync(CANVAS_CONNECT_TIMEOUT_MS)
    expect(w.find('iframe').exists()).toBe(false)
    expect(w.text()).toContain('infiniteCanvas.notReady')
    expect(localStorage.getItem('sub2api.canvas.connections.1')).toBeNull()
  })
})
