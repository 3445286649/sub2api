import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import ChannelStatusV3View from '../ChannelStatusV3View.vue'
import * as api from '@/api/channelMonitorV2'
import { apiClient } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, te: () => true, locale: ref('en-US') }),
}))
vi.mock('@/api/groups', () => ({ default: { getAvailable: vi.fn().mockResolvedValue([]), getUserGroupRates: vi.fn().mockResolvedValue({}) } }))
vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn().mockResolvedValue({ data: [{ id: 12, name: 'Configured card', provider: 'openai', display_category: '', display_order: 0, v3_group_id: 1 }] }) } }))
vi.mock('@/api/channelMonitorV2', () => ({ getSnapshot: vi.fn(), getMatrix: vi.fn(), getDimensions: vi.fn(), getModels: vi.fn(), getErrors: vi.fn(), getUsers: vi.fn() }))
const latency = { sample_count: 30, p50_ms: 300, p95_ms: 500, avg_ms: 350 }
const metrics = { success_requests: 0, error_requests: 0, request_count: 0, token_count: 0, rpm: 0, tpm: 0, error_rate: 0.1, cache_rate: .5, cache_rate_numerator: 0, cache_rate_denominator: 0, ttft: latency, duration: latency }
const health = { overall: 'healthy', error_rate: 'healthy', ttft: 'healthy', cache: 'healthy', minimum_sample: 10 } as const
const coverage = { requested_start: '2026-09-14T00:00:00Z', requested_end: '2026-09-14T00:15:00Z', coverage_start: '2026-09-14T00:00:00Z', data_through: '2026-09-14T00:15:00Z', computed_at: '2026-09-14T00:15:00Z', aggregation_lag_seconds: 0, coverage_complete: true, bucket_seconds: 300 }
let wrapper: ReturnType<typeof mount> | undefined
afterEach(() => { wrapper?.unmount(); vi.useRealTimers(); vi.clearAllMocks() })
describe('independent v3 page loading', () => {
  it('defaults to cards, reuses cached range, forces manual refresh and clears on identity change', async () => {
    vi.useFakeTimers()
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.$patch({ user: { id: 1, role: 'user' } })
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }] })
    await router.push('/')
    vi.mocked(api.getSnapshot).mockResolvedValue({ metrics, health, coverage, trend: [], config: { version: 1, enabled: true, refresh_interval_seconds: 300, platforms: [], group_ids: [], health_thresholds: {} } } as unknown as Awaited<ReturnType<typeof api.getSnapshot>>)
    vi.mocked(api.getMatrix).mockResolvedValue({ group_by: 'platform_group', coverage, items: [{ platform: 'openai', group_id: 1, group_name: 'Test group', metrics, health, buckets: [] }] })
    vi.mocked(api.getDimensions).mockResolvedValue({ platforms: [], groups: [], models: [] })
    vi.mocked(api.getModels).mockResolvedValue({ coverage, items: [] })
    wrapper = mount(ChannelStatusV3View, { global: { plugins: [pinia, router], stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true, ProviderIcon: true, MonitorTrendChart: true, RelayPulseMatrix: true, MonitorRankBadge: true } } })
    await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(api.getMatrix).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Configured card')
    const article = wrapper.find('article').element
    await vi.advanceTimersByTimeAsync(59999)
    expect(api.getMatrix).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(api.getMatrix).toHaveBeenCalledTimes(2)
    expect(wrapper.find('article').element).toBe(article)
    expect(api.getMatrix).toHaveBeenLastCalledWith(expect.objectContaining({ view: 'v3' }), 'platform_group', false, expect.any(AbortSignal))
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    await vi.advanceTimersByTimeAsync(60000)
    expect(api.getMatrix).toHaveBeenCalledTimes(2)
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(api.getMatrix).toHaveBeenCalledTimes(3)
    vi.mocked(api.getMatrix).mockClear()
    const click = async (label: string) => {
      await wrapper!.findAll('button').find(button => button.text() === label)!.trigger('click')
      await flushPromises()
    }
    await click('channelMonitorV2.ranges.24h')
    expect(api.getMatrix).toHaveBeenCalledTimes(1)
    await click('channelMonitorV2.ranges.90m')
    expect(api.getMatrix).toHaveBeenCalledTimes(1)
    await wrapper.find('button[title="common.refresh"]').trigger('click')
    await flushPromises()
    expect(api.getMatrix).toHaveBeenCalledTimes(2)
    vi.mocked(api.getMatrix).mockRejectedValueOnce(new Error('temporary outage'))
    await wrapper.find('button[title="common.refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.find('[role="alert"]').text()).toContain('refreshFailed')
    auth.$patch({ user: { id: 2, role: 'user' } })
    await flushPromises()
    expect(api.getMatrix).toHaveBeenCalledTimes(4)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: [
      { id: 12, name: 'First configured card', provider: 'openai', display_category: 'Custom', display_order: 2, v3_group_id: 1 },
      { id: 13, name: 'Second configured card', provider: 'openai', display_category: 'Custom', display_order: 1, v3_group_id: 1 },
      { id: 14, name: 'Unlinked card', provider: 'openai', display_category: 'Custom', display_order: 3, v3_group_id: null },
    ] })
    await wrapper.find('button[title="common.refresh"]').trigger('click')
    await flushPromises()
    const cards = wrapper.findAll('article')
    expect(cards).toHaveLength(3)
    expect(cards[0].text()).toContain('Second configured card')
    expect(cards[1].text()).toContain('First configured card')
    expect(cards[2].find('[data-testid="channel-status-health-badge"]').exists()).toBe(false)
    expect(cards[2].findAll('.text-lg').map(node => node.text())).toEqual(['-', '-', '-'])
  })
})
