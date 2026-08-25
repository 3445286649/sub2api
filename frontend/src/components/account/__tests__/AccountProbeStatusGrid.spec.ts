import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountProbeStatusGrid from '../AccountProbeStatusGrid.vue'
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
    last_result: points.at(-1)?.failure_count ? 'failure' : 'success',
    last_latency_ms: points.at(-1)?.latency_ms ?? null,
    p50_latency_ms: null,
    p95_latency_ms: null,
    last_probed_at: points.at(-1)?.timestamp ?? null,
    last_error_category: '',
    last_error_message: '',
    next_probe_at: null,
    points,
  }
}

describe('AccountProbeStatusGrid', () => {
  it('renders ten fixed slots and right-aligns the available probe results', () => {
    const wrapper = mount(AccountProbeStatusGrid, {
      props: {
        ariaLabel: 'probe status',
        successLabel: 'normal',
        failureLabel: 'abnormal',
        emptyLabel: 'no data',
        trend: trend([
          { timestamp: '2026-08-25T08:00:00Z', latency_ms: 100, success_count: 1, failure_count: 0 },
          { timestamp: '2026-08-25T09:00:00Z', latency_ms: null, success_count: 0, failure_count: 1 },
        ]),
      },
    })

    const slots = wrapper.findAll('[data-probe-slot]')
    expect(slots).toHaveLength(10)
    expect(slots.slice(0, 8).every(slot => slot.attributes('data-state') === 'empty')).toBe(true)
    expect(slots[8].attributes('data-state')).toBe('success')
    expect(slots[9].attributes('data-state')).toBe('failure')
    expect(slots[9].attributes('data-latest')).toBe('true')
  })

  it('keeps only the latest ten results in chronological order', () => {
    const points = Array.from({ length: 12 }, (_, index) => ({
      timestamp: `2026-08-25T${String(index).padStart(2, '0')}:00:00Z`,
      latency_ms: 100 + index,
      success_count: 1,
      failure_count: 0,
    }))
    const wrapper = mount(AccountProbeStatusGrid, {
      props: {
        ariaLabel: 'latest probes',
        successLabel: 'normal',
        failureLabel: 'abnormal',
        emptyLabel: 'no data',
        trend: trend(points),
      },
    })

    const slots = wrapper.findAll('[data-probe-slot]')
    expect(slots).toHaveLength(10)
    expect(slots[0].attributes('data-timestamp')).toBe(points[2].timestamp)
    expect(slots[9].attributes('data-timestamp')).toBe(points[11].timestamp)
  })
})
