import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountProbeSparkline from '../AccountProbeSparkline.vue'
import type { AccountProbeTrend } from '@/types'

function trend(points: AccountProbeTrend['points']): AccountProbeTrend {
  return {
    account_id: 7,
    range: '24h',
    from: '2026-08-25T00:00:00Z',
    to: '2026-08-25T10:00:00Z',
    total: points.length,
    success_count: points.filter(point => point.success_count > 0).length,
    failure_count: points.filter(point => point.failure_count > 0).length,
    success_rate: 0,
    last_result: undefined,
    last_latency_ms: null,
    p50_latency_ms: null,
    p95_latency_ms: null,
    last_probed_at: null,
    last_error_category: '',
    last_error_message: '',
    next_probe_at: null,
    points,
  }
}

describe('AccountProbeSparkline', () => {
  it('uses a fixed 120x28 viewport and positions points by real time', () => {
    const wrapper = mount(AccountProbeSparkline, {
      props: {
        ariaLabel: 'probe trend',
        trend: trend([
          { timestamp: '2026-08-25T01:00:00Z', latency_ms: 100, success_count: 1, failure_count: 0 },
          { timestamp: '2026-08-25T09:00:00Z', latency_ms: 200, success_count: 1, failure_count: 0 },
        ]),
      },
    })

    expect(wrapper.get('svg').attributes('viewBox')).toBe('0 0 120 28')
    expect(wrapper.get('svg').classes()).toContain('w-[120px]')
    const xPositions = wrapper.findAll('circle').map(point => Number(point.attributes('cx')))
    expect(xPositions[0]).toBeCloseTo(13.6)
    expect(xPositions[1]).toBeCloseTo(106.4)
  })

  it('renders a failed probe as a red cross without a zero-latency success point', () => {
    const wrapper = mount(AccountProbeSparkline, {
      props: {
        ariaLabel: 'failed probe',
        trend: trend([
          { timestamp: '2026-08-25T05:00:00Z', latency_ms: null, success_count: 0, failure_count: 1 },
        ]),
      },
    })

    expect(wrapper.findAll('circle')).toHaveLength(0)
    expect(wrapper.findAll('g')).toHaveLength(1)
    expect(wrapper.get('g').findAll('line')).toHaveLength(2)
  })
})
