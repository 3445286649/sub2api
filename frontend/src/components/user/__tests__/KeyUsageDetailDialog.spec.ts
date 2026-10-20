import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KeyUsageDetailDialog from '../KeyUsageDetailDialog.vue'

const { getAPIKeyBreakdown } = vi.hoisted(() => ({ getAPIKeyBreakdown: vi.fn() }))
vi.mock('@/api/usage', () => ({ usageAPI: { getAPIKeyBreakdown } }))
vi.mock('vue-i18n', async () => ({
  ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')),
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key} ${Object.values(params).join(' ')}` : key }),
}))

const totals = { requests: 2, input_tokens: 100, output_tokens: 20, cache_creation_tokens: 30, cache_read_tokens: 40, total_tokens: 190, actual_cost: 2.5, subscription_cost: 0.5 }
const selection = { api_key_id: 7, key_name: 'work-key', start_time: '2026-10-18T15:42:00Z', end_time: '2026-10-19T15:42:00Z', period: '24h' as const, group_id: 3, group_name: 'used-group' }
const fixture = {
  api_key_id: 7, name: 'work-key', deleted: false, start_time: selection.start_time, end_time: selection.end_time, summary: totals,
  groups: [{ ...totals, group_id: 3, group_name: 'used-group', group_deleted: false, models: [{ ...totals, model: 'used-model' }] }],
}
const render = (initial = selection) => mount(KeyUsageDetailDialog, {
  props: { selection: initial },
  global: { stubs: {
    BaseDialog: { props: ['show', 'title', 'width'], emits: ['close'], template: '<section role="dialog"><h2>{{ title }}</h2><button data-testid="dialog-close" @click="$emit(\'close\')">Close</button><slot /></section>' },
    LoadingSpinner: true,
  } },
})

describe('KeyUsageDetailDialog', () => {
  beforeEach(() => { getAPIKeyBreakdown.mockReset(); getAPIKeyBreakdown.mockResolvedValue(fixture) })
  afterEach(() => { vi.useRealTimers() })

  it('inherits the exact applied range and group without recomputing, then clears only its own group filter', async () => {
    const wrapper = render(); await flushPromises()
    expect(getAPIKeyBreakdown).toHaveBeenCalledWith(7, { start_time: selection.start_time, end_time: selection.end_time, group_id: 3 }, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('used-group')
    expect(wrapper.text()).toContain('used-model')
    expect(wrapper.findAll('[data-testid="key-detail-group"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="key-detail-model"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('usage.keySummary.subscriptionHint')
    expect(wrapper.text()).toContain('usage.keySummary.cacheRead')
    await wrapper.get('[data-testid="key-detail-all-groups"]').trigger('click'); await flushPromises()
    expect(getAPIKeyBreakdown).toHaveBeenLastCalledWith(7, { start_time: selection.start_time, end_time: selection.end_time, group_id: undefined }, expect.any(AbortSignal))
    expect(selection.group_id).toBe(3)
    wrapper.unmount()
  })

  it('switches presets locally and keeps exact rolling 24-hour boundaries', async () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-20T07:42:23Z'))
    const wrapper = render(); await flushPromises()
    await wrapper.get('[data-period="7d"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[data-period="7d"]').attributes('aria-pressed')).toBe('true')
    await wrapper.get('[data-period="24h"]').trigger('click'); await flushPromises()
    const params = getAPIKeyBreakdown.mock.lastCall![1]
    expect(params.end_time).toBe('2026-10-20T07:42:23.000Z')
    expect(new Date(params.end_time).getTime() - new Date(params.start_time).getTime()).toBe(86400000)
    expect(selection.start_time).toBe('2026-10-18T15:42:00Z')
    wrapper.unmount()
  })

  it('applies custom dates explicitly, including the final day, and rejects reversed ranges', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('[data-period="custom"]').trigger('click')
    await wrapper.get('#key-detail-start').setValue('2026-10-01')
    await wrapper.get('#key-detail-end').setValue('2026-10-02')
    expect(getAPIKeyBreakdown).toHaveBeenCalledTimes(1)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    const params = getAPIKeyBreakdown.mock.lastCall![1]
    expect(new Date(params.start_time)).toEqual(new Date(2026, 9, 1))
    expect(new Date(params.end_time)).toEqual(new Date(2026, 9, 3))
    await wrapper.get('#key-detail-start').setValue('2026-10-05')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(getAPIKeyBreakdown).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('usage.keySummary.invalidRange')
    wrapper.unmount()
  })

  it('discards cancelled late responses, removes stale data after errors, and retries the applied range', async () => {
    let resolveOld!: (value: typeof fixture) => void
    getAPIKeyBreakdown.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    getAPIKeyBreakdown.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render()
    const firstSignal = getAPIKeyBreakdown.mock.calls[0][2] as AbortSignal
    await wrapper.get('[data-period="7d"]').trigger('click'); await flushPromises()
    expect(firstSignal.aborted).toBe(true)
    resolveOld({ ...fixture, groups: [{ ...fixture.groups[0], group_name: 'stale-group' }] }); await flushPromises()
    expect(wrapper.text()).not.toContain('stale-group')
    expect(wrapper.text()).not.toContain('used-model')
    expect(wrapper.get('[role="alert"]').text()).toContain('usage.failedToLoad')
    const failedParams = getAPIKeyBreakdown.mock.lastCall![1]
    await wrapper.get('[data-testid="key-detail-retry"]').trigger('click'); await flushPromises()
    expect(getAPIKeyBreakdown.mock.lastCall![1]).toEqual(failedParams)
    expect(wrapper.text()).toContain('used-model')
    wrapper.unmount()
  })

  it('keeps real zero-cost calls and labels deleted or unrecorded historical groups', async () => {
    getAPIKeyBreakdown.mockResolvedValueOnce({ ...fixture, deleted: true, groups: [
      { ...totals, group_id: 9, group_name: '', group_deleted: true, models: [{ ...totals, model: 'free-model', actual_cost: 0, total_tokens: 0 }] },
      { ...totals, group_id: null, group_name: '', group_deleted: false, models: [{ ...totals, model: 'historical-model' }] },
    ] })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('free-model')
    expect(wrapper.text()).toContain('usage.keyDetail.groupFallback 9')
    expect(wrapper.text()).toContain('usage.keyDetail.deletedGroup')
    expect(wrapper.text()).toContain('usage.keyDetail.unrecordedGroup')
    expect(wrapper.text()).toContain('usage.errors.keyDeleted')
    wrapper.unmount()
  })

  it('shows an empty state without inventing groups and cancels pending work on close or unmount', async () => {
    getAPIKeyBreakdown.mockResolvedValueOnce({ ...fixture, groups: [] })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('usage.keyDetail.empty')
    expect(wrapper.findAll('[data-testid="key-detail-group"]')).toHaveLength(0)
    getAPIKeyBreakdown.mockImplementationOnce(() => new Promise(() => {}))
    await wrapper.get('[data-period="14d"]').trigger('click')
    const signal = getAPIKeyBreakdown.mock.lastCall![2] as AbortSignal
    await wrapper.get('[data-testid="dialog-close"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(signal.aborted).toBe(true)
    wrapper.unmount()
    getAPIKeyBreakdown.mockImplementationOnce(() => new Promise(() => {}))
    const pending = render()
    const unmountSignal = getAPIKeyBreakdown.mock.lastCall![2] as AbortSignal
    pending.unmount()
    expect(unmountSignal.aborted).toBe(true)
  })

  it('restores keyboard focus to the details trigger before its parent removes the dialog', async () => {
    const trigger = document.createElement('button')
    trigger.textContent = 'View details'
    document.body.append(trigger)
    trigger.focus()
    const wrapper = mount(KeyUsageDetailDialog, {
      attachTo: document.body,
      props: { selection },
      global: { stubs: { Teleport: true, Icon: true, LoadingSpinner: true } },
    })
    try {
      await flushPromises()
      expect(document.activeElement).not.toBe(trigger)
      await wrapper.get('[aria-label="common.close"]').trigger('click')
      await flushPromises()
      expect(wrapper.emitted('close')).toHaveLength(1)
      expect(document.activeElement).toBe(trigger)
    } finally {
      wrapper.unmount()
      trigger.remove()
    }
  })
})
