import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import UsageRebateTrendChart from '../UsageRebateTrendChart.vue'

const messages: Record<string, string> = {
  'usageRebate.trend.title': 'Recent 30-day rebate trend',
  'usageRebate.trend.period': '2026-06-18 to 2026-07-17',
  'usageRebate.trend.spend': 'Daily spend',
  'usageRebate.trend.reward': 'Daily rebate',
  'usageRebate.trend.noData': 'No trend data',
  'usageRebate.trend.loadFailed': 'Trend temporarily unavailable',
  'usageRebate.rank': 'Rank',
  'usageRebate.rate': 'Rebate rate',
  'usageRebate.status': 'Status',
  'usageRebate.statuses.credited': 'Credited',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'usageRebate.trend.period') {
          return `${params?.start} to ${params?.end}`
        }
        return messages[key] ?? key
      },
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Line: defineComponent({
    name: 'ChartLineStub',
    props: ['data', 'options'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  }),
}))

describe('UsageRebateTrendChart', () => {
  it('fills the complete date range and merges spend with rebate data', () => {
    const wrapper = mount(UsageRebateTrendChart, {
      props: {
        startDate: '2026-06-18',
        endDate: '2026-07-17',
        trendData: [{
          date: '2026-07-16',
          requests: 2,
          input_tokens: 20,
          output_tokens: 10,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 30,
          cost: 12,
          actual_cost: 8.5,
        }],
        rewards: [{
          business_date: '2026-07-16',
          rank: 3,
          spend_amount: '8.5',
          rebate_percent: '6',
          reward_amount: '0.51',
          status: 'credited',
        }],
      },
    })

    const line = wrapper.findComponent({ name: 'ChartLineStub' })
    const data = line.props('data') as any

    expect(data.labels).toHaveLength(30)
    expect(data.labels[0]).toBe('2026-06-18')
    expect(data.labels[29]).toBe('2026-07-17')
    expect(data.datasets[0].yAxisID).toBe('ySpend')
    expect(data.datasets[1].yAxisID).toBe('yReward')
    expect(data.datasets[0].data[28]).toBe(8.5)
    expect(data.datasets[1].data[28]).toBe(0.51)
    expect(data.datasets[0].data[0]).toBe(0)
    expect(data.datasets[1].data[0]).toBe(0)
  })

  it('shows spend, ranking, rate, reward and status in the tooltip', () => {
    const wrapper = mount(UsageRebateTrendChart, {
      props: {
        startDate: '2026-07-16',
        endDate: '2026-07-17',
        trendData: [{
          date: '2026-07-16',
          requests: 2,
          input_tokens: 20,
          output_tokens: 10,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 30,
          cost: 12,
          actual_cost: 8.5,
        }],
        rewards: [{
          business_date: '2026-07-16',
          rank: 3,
          spend_amount: '8.5',
          rebate_percent: '6',
          reward_amount: '0.51',
          status: 'credited',
        }],
      },
    })

    const options = wrapper.findComponent({ name: 'ChartLineStub' }).props('options') as any
    const callbacks = options.plugins.tooltip.callbacks
    const tooltipLines = callbacks.afterBody([{ dataIndex: 0 }])

    expect(callbacks.title([{ dataIndex: 0 }])).toBe('2026-07-16')
    expect(tooltipLines).toEqual([
      'Rank: #3',
      'Rebate rate: 6%',
      'Status: Credited',
    ])
    expect(callbacks.label({ dataset: { yAxisID: 'ySpend' }, raw: 8.5 })).toBe('Daily spend: $8.5')
    expect(callbacks.label({ dataset: { yAxisID: 'yReward' }, raw: 0.51 })).toBe('Daily rebate: $0.51')
  })

  it('does not draw partial data when the spend trend is unavailable', () => {
    const wrapper = mount(UsageRebateTrendChart, {
      props: {
        startDate: '2026-07-16',
        endDate: '2026-07-17',
        trendData: [],
        rewards: [{
          business_date: '2026-07-16',
          rank: 3,
          spend_amount: '8.5',
          rebate_percent: '6',
          reward_amount: '0.51',
          status: 'credited',
        }],
        unavailable: true,
      },
    })

    expect(wrapper.text()).toContain('Trend temporarily unavailable')
    expect(wrapper.findComponent({ name: 'ChartLineStub' }).exists()).toBe(false)
  })
})
