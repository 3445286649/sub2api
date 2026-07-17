import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import UsageRebateView from '../UsageRebateView.vue'

const getOverview = vi.hoisted(() => vi.fn())
const getDashboardTrend = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('@/api/usageRebate', () => ({
  default: { getOverview },
}))

vi.mock('@/api/usage', () => ({
  default: { getDashboardTrend },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'usageRebate.rankValue') return `#${params?.rank}/${params?.total}`
        if (key === 'usageRebate.gapToTop20') return `${params?.amount}`
        return key
      },
    }),
  }
})

describe('UsageRebateView', () => {
  beforeEach(() => {
    getOverview.mockReset()
    getDashboardTrend.mockReset()
    showError.mockReset()
    getOverview.mockResolvedValue({
      enabled: true,
      business_date: '2026-07-17',
      timezone: 'Asia/Shanghai',
      settlement_time: '00:15',
      rates: [{ rank: 1, percent: '10' }],
      my_position: {
        rank: 21, participant_count: 25, requests: 8, tokens: 900,
        spend_amount: '36', rebate_percent: '0', estimated_reward: '0', eligible: false,
        previous_rank: 20, gap_to_previous: '1.5', gap_to_top20: '4.25',
      },
      my_rewards: [{
        business_date: '2026-07-16', rank: 1, spend_amount: '20',
        rebate_percent: '10', reward_amount: '2', status: 'credited',
      }],
    })
    getDashboardTrend.mockResolvedValue({
      trend: [],
      start_date: '2026-06-18',
      end_date: '2026-07-17',
      granularity: 'day',
    })
  })

  it('renders own-spend rebate data without any mutation controls', async () => {
    const wrapper = mount(UsageRebateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          UsageRebateTrendChart: {
            name: 'UsageRebateTrendChart',
            props: ['trendData', 'rewards', 'startDate', 'endDate', 'loading', 'unavailable'],
            template: '<div class="rebate-trend-stub" />',
          },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('$36')
    expect(wrapper.get('[data-testid="usage-rebate-rank"]').text()).toContain('21')
    expect(wrapper.get('[data-testid="usage-rebate-gap-previous"]').text()).toContain('$1.5')
    expect(wrapper.get('[data-testid="usage-rebate-gap-top20"]').text()).toContain('$4.25')
    expect(wrapper.get('[data-testid="usage-rebate-estimated"]').text()).toContain('$0')
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.findAll('textarea')).toHaveLength(0)
    expect(wrapper.findAll('button')).toHaveLength(0)
    expect(getDashboardTrend).toHaveBeenCalledWith({
      start_date: '2026-06-18',
      end_date: '2026-07-17',
      granularity: 'day',
      billing_type: 0,
      timezone: 'Asia/Shanghai',
    })

    const trend = wrapper.findComponent({ name: 'UsageRebateTrendChart' })
    expect(trend.props('startDate')).toBe('2026-06-18')
    expect(trend.props('endDate')).toBe('2026-07-17')
    expect(trend.props('rewards')).toHaveLength(1)
  })

  it('keeps rebate content available when trend loading fails', async () => {
    getDashboardTrend.mockRejectedValue(new Error('trend unavailable'))

    const wrapper = mount(UsageRebateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          UsageRebateTrendChart: {
            name: 'UsageRebateTrendChart',
            props: ['trendData', 'rewards', 'startDate', 'endDate', 'loading', 'unavailable'],
            template: '<div class="rebate-trend-stub" />',
          },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="usage-rebate-rank"]').text()).toContain('21')
    const trend = wrapper.findComponent({ name: 'UsageRebateTrendChart' })
    expect(trend.props('trendData')).toEqual([])
    expect(trend.props('unavailable')).toBe(true)
    expect(showError).not.toHaveBeenCalled()
  })

  it('renders the leader state without a previous-rank gap', async () => {
    getOverview.mockResolvedValue({
      enabled: true,
      business_date: '2026-07-17',
      timezone: 'Asia/Shanghai',
      settlement_time: '00:15',
      rates: [{ rank: 1, percent: '10' }],
      my_position: {
        rank: 1, participant_count: 25, requests: 8, tokens: 900,
        spend_amount: '100', rebate_percent: '10', estimated_reward: '10', eligible: true,
        previous_rank: null, gap_to_previous: null, gap_to_top20: null,
      },
      my_rewards: [],
    })

    const wrapper = mount(UsageRebateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          UsageRebateTrendChart: { template: '<div />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="usage-rebate-rank"]').text()).toContain('1')
    expect(wrapper.get('[data-testid="usage-rebate-gap-previous"]').text()).toContain('usageRebate.currentLeader')
    expect(wrapper.get('[data-testid="usage-rebate-estimated"]').text()).toContain('$10')
    expect(wrapper.find('[data-testid="usage-rebate-gap-top20"]').exists()).toBe(false)
  })

  it('renders no rank for a user without eligible spend', async () => {
    getOverview.mockResolvedValue({
      enabled: true,
      business_date: '2026-07-17',
      timezone: 'Asia/Shanghai',
      settlement_time: '00:15',
      rates: [],
      my_position: {
        rank: null, participant_count: 25, requests: 0, tokens: 0,
        spend_amount: '0', rebate_percent: '0', estimated_reward: '0', eligible: false,
        previous_rank: null, gap_to_previous: null, gap_to_top20: null,
      },
      my_rewards: [],
    })

    const wrapper = mount(UsageRebateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          UsageRebateTrendChart: { template: '<div />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="usage-rebate-rank"]').text()).toContain('--')
    expect(wrapper.text()).toContain('usageRebate.empty')
    expect(wrapper.find('[data-testid="usage-rebate-gap-top20"]').exists()).toBe(false)
  })
})
