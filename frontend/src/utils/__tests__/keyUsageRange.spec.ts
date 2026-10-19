import { describe, expect, it } from 'vitest'
import { keyUsageRange, keyUsageDisplayDates } from '../keyUsageRange'

describe('keyUsageRange', () => {
  const now = new Date(2026, 9, 19, 15, 42, 23)
  it('preserves exact rolling 24 hours instead of expanding to two days', () => {
    const range = keyUsageRange('24h', now)
    expect(new Date(range.end_time).getTime() - new Date(range.start_time).getTime()).toBe(86400000)
    expect(new Date(range.start_time).getMinutes()).toBe(42)
    expect(new Date(range.end_time)).toEqual(now)
  })
  it('uses calendar days and last-month boundaries', () => {
    const range = keyUsageRange('7d', now)
    expect(keyUsageDisplayDates(range)).toEqual({ startDate: '2026-10-13', endDate: '2026-10-19' })
    expect(keyUsageDisplayDates(keyUsageRange('lastMonth', now))).toEqual({ startDate: '2026-09-01', endDate: '2026-09-30' })
    expect(keyUsageDisplayDates(keyUsageRange('thisMonth', now))).toEqual({ startDate: '2026-10-01', endDate: '2026-10-19' })
    expect(keyUsageDisplayDates(keyUsageRange('lastMonth', new Date(2026, 0, 15)))).toEqual({ startDate: '2025-12-01', endDate: '2025-12-31' })
  })
  it('includes all of the final custom day and rejects reversed or excessive ranges', () => {
    expect(keyUsageDisplayDates(keyUsageRange('custom', now, '2026-10-01', '2026-10-01'))).toEqual({ startDate: '2026-10-01', endDate: '2026-10-01' })
    expect(() => keyUsageRange('custom', now, '2026-10-02', '2026-10-01')).toThrow()
    expect(() => keyUsageRange('custom', now, '2024-10-02', '2026-10-01')).toThrow()
  })
})
