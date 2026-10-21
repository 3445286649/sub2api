import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import KeyUsagePanel from '../KeyUsagePanel.vue'
import Pagination from '@/components/common/Pagination.vue'

const { getAPIKeySummary } = vi.hoisted(() => ({ getAPIKeySummary: vi.fn() }))
vi.mock('@/api/usage', () => ({ usageAPI: { getAPIKeySummary } }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
const totals = { requests: 20, input_tokens: 1000, output_tokens: 200, cache_creation_tokens: 300, cache_read_tokens: 400, total_tokens: 1900, actual_cost: 2.5, subscription_cost: 0.5 }
const fixture = { items: [{ ...totals, api_key_id: 7, name: 'deleted-work-key', deleted: true, group_name: 'new-group' }], summary: totals, total: 31, page: 1, page_size: 20 }
const render = () => mount(KeyUsagePanel, { props: { groups: [], initialKeyId: 7 }, global: { stubs: { Select: true, Pagination: true, LoadingSpinner: true, Teleport: true } } })

describe('KeyUsagePanel', () => {
  beforeEach(() => { getAPIKeySummary.mockReset(); getAPIKeySummary.mockResolvedValue(fixture) })
  it('shows server totals for all pages, deleted history and subscription explanation', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('deleted-work-key')
    expect(wrapper.text()).toContain('usage.errors.keyDeleted')
    expect(wrapper.text()).toContain('usage.keySummary.subscriptionHint')
    expect(wrapper.text()).toContain('31')
    expect(getAPIKeySummary).toHaveBeenCalledWith(expect.objectContaining({ api_key_id: 7, page: 1 }), expect.any(AbortSignal))
    wrapper.findComponent(Pagination).vm.$emit('update:page', 2); await flushPromises()
    expect(getAPIKeySummary).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }), expect.any(AbortSignal))
    const request = getAPIKeySummary.mock.calls[1][0]
    expect(request.start_time).toBe(getAPIKeySummary.mock.calls[0][0].start_time)
    await wrapper.get('[data-testid="ku-details"]').trigger('click')
    expect(wrapper.emitted('details')![0][0]).toMatchObject({ api_key_id: 7, key_name: 'deleted-work-key', key_deleted: true, period: '7d', start_time: request.start_time, end_time: request.end_time })
    wrapper.unmount()
  })
  it('requests exact 24 hours and ignores late responses after filter changes', async () => {
    let resolve!: (value: typeof fixture) => void
    getAPIKeySummary.mockImplementationOnce(() => new Promise(r => { resolve = r }))
    const wrapper = render()
    await wrapper.get('[data-testid="ku-filter-desktop"] [data-period="24h"]').trigger('click'); await flushPromises()
    const params = getAPIKeySummary.mock.lastCall![0]
    expect(new Date(params.end_time).getTime() - new Date(params.start_time).getTime()).toBe(86400000)
    resolve({ ...fixture, items: [{ ...fixture.items[0], name: 'stale-key' }] }); await flushPromises()
    expect(wrapper.text()).not.toContain('stale-key')
    wrapper.unmount()
  })
  it('shows errors with retry, without presenting stale totals as current data', async () => {
    getAPIKeySummary.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('usage.failedToLoad')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('deleted-work-key')
    wrapper.unmount()
  })

  it('opens details with applied dates instead of pending custom inputs', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('[data-testid="ku-filter-desktop"] [data-period="custom"]').trigger('click')
    await flushPromises()
    const applied = getAPIKeySummary.mock.lastCall![0]
    await wrapper.get('#key-usage-start').setValue('2026-01-01')
    await wrapper.get('#key-usage-end').setValue('2026-01-02')
    await wrapper.get('[data-testid="ku-details"]').trigger('click')
    expect(wrapper.emitted('details')![0][0]).toMatchObject({ period: 'custom', start_time: applied.start_time, end_time: applied.end_time })
    wrapper.unmount()
  })

  it('sends comparison and trend params and shows delta, sparkline and share by the filtered total', async () => {
    getAPIKeySummary.mockResolvedValue({
      ...fixture,
      items: [{ ...fixture.items[0], actual_cost: 83.19, group_platform: 'openai' }],
      summary: { ...totals, actual_cost: 238.75 },
      previous: { requests: 10, total_tokens: 1900, actual_cost: 300 },
      trend: [{ bucket: '2026-10-18T00:00:00Z', requests: 1, total_tokens: 1, actual_cost: 1 }, { bucket: '2026-10-19T00:00:00Z', requests: 2, total_tokens: 2, actual_cost: 2 }],
    })
    const wrapper = render(); await flushPromises()
    const params = getAPIKeySummary.mock.lastCall![0]
    expect(params.prev_end_time).toBe(params.start_time)
    expect(params.granularity).toBe('day')
    expect(params.timezone).toBeTruthy()
    const cards = wrapper.findAll('[data-testid="ku-stat"]')
    expect(cards).toHaveLength(4)
    // Matching keys: number only, no comparison and no sparkline.
    expect(cards[0].find('[data-testid="ku-delta"]').exists()).toBe(false)
    expect(cards[0].find('[data-testid="ku-sparkline"]').exists()).toBe(false)
    expect(cards[1].get('[data-testid="ku-delta"]').text()).toContain('▲ +100%')
    expect(cards[2].get('[data-testid="ku-delta"]').text()).toContain('0%')
    expect(cards[3].get('[data-testid="ku-delta"]').text()).toContain('▼ -20%')
    expect(cards[3].find('[data-testid="ku-sparkline"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="ku-bar"]').attributes('style')).toContain('width: 34.8')
    expect(wrapper.get('[data-testid="ku-share"]').text()).toBe('35%')
    expect(wrapper.text()).toContain('$83.19')
    expect(wrapper.find('details').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows the active keys caption only when only-used is on', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).not.toContain('usage.keySummary.activeKeys')
    await wrapper.get('[data-testid="ku-filter-desktop"] [data-testid="ku-only-used"]').trigger('click'); await flushPromises()
    expect(getAPIKeySummary.mock.lastCall![0].only_used).toBe(true)
    expect(wrapper.findAll('[data-testid="ku-stat"]')[0].text()).toContain('usage.keySummary.activeKeys')
    wrapper.unmount()
  })

  it('mobile filter sheet edits a draft, applies on demand and counts non-default filters', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('[data-testid="ku-filter-badge"]').exists()).toBe(false)
    await wrapper.get('[data-testid="ku-filter-open"]').trigger('click')
    const calls = getAPIKeySummary.mock.calls.length
    await wrapper.get('[data-testid="ku-filter-sheet"] [data-period="thisMonth"]').trigger('click')
    await wrapper.get('#key-usage-search-m').setValue('work')
    await flushPromises()
    expect(getAPIKeySummary.mock.calls.length).toBe(calls)
    await wrapper.get('[data-testid="ku-filter-apply"]').trigger('click'); await flushPromises()
    expect(getAPIKeySummary.mock.calls.length).toBe(calls + 1)
    const params = getAPIKeySummary.mock.lastCall![0]
    expect(params.search).toBe('work')
    expect(new Date(params.start_time).getDate()).toBe(1)
    expect(wrapper.get('[data-testid="ku-filter-badge"]').text()).toBe('2')
    expect(wrapper.findAll('[data-testid="ku-filter-mobile"] [aria-pressed="true"]')).toHaveLength(0)
    expect(wrapper.text()).toContain('usage.keySummary.vsLastMonth')
    wrapper.unmount()
  })

  it('opens details from anywhere on a mobile card and by keyboard', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('[data-testid="ku-card"]').trigger('click')
    expect(wrapper.emitted('details')).toHaveLength(1)
    await wrapper.get('[data-testid="ku-card"]').trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('details')).toHaveLength(2)
    wrapper.unmount()
  })
})
