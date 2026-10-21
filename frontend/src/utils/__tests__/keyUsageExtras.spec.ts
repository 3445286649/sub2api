import { describe, expect, it } from 'vitest'
import { formatKeyUsageRange, keyUsageCompareLabel, keyUsageGranularity, keyUsageRange, previousKeyUsageRange } from '../keyUsageRange'
import { formatKeyUsageMoney, formatKeyUsageShare, keyUsageMoneyTitle } from '../keyUsageMoney'
import { keyUsageGroupTone } from '../keyUsageGroupColor'

const local = (iso: string) => new Date(iso)

describe('previousKeyUsageRange', () => {
  it('uses the adjacent equal window for rolling periods', () => {
    const range = keyUsageRange('7d', new Date(2026, 9, 21, 12))
    const prev = previousKeyUsageRange('7d', range)
    expect(prev.prev_end_time).toBe(range.start_time)
    expect(new Date(range.start_time).getTime() - new Date(prev.prev_start_time).getTime()).toBe(7 * 86400000)
  })
  it('compares this month with the same days of last month and clamps short months', () => {
    const range = keyUsageRange('thisMonth', new Date(2026, 2, 31, 10))
    const prev = previousKeyUsageRange('thisMonth', range)
    expect(local(prev.prev_start_time)).toEqual(new Date(2026, 1, 1))
    expect(local(prev.prev_end_time)).toEqual(new Date(2026, 2, 1))
    const oct = previousKeyUsageRange('thisMonth', keyUsageRange('thisMonth', new Date(2026, 9, 21)))
    expect(local(oct.prev_start_time)).toEqual(new Date(2026, 8, 1))
    expect(local(oct.prev_end_time)).toEqual(new Date(2026, 8, 22))
  })
  it('compares last month with the month before', () => {
    const prev = previousKeyUsageRange('lastMonth', keyUsageRange('lastMonth', new Date(2026, 0, 15)))
    expect(local(prev.prev_start_time)).toEqual(new Date(2025, 10, 1))
    expect(local(prev.prev_end_time)).toEqual(new Date(2025, 11, 1))
  })
  it('labels month periods differently', () => {
    expect(keyUsageCompareLabel('thisMonth')).toBe('vsLastMonth')
    expect(keyUsageCompareLabel('24h')).toBe('vsPrevious')
  })
})

describe('granularity and range label', () => {
  it('switches to days after two days', () => {
    expect(keyUsageGranularity(keyUsageRange('24h'))).toBe('hour')
    expect(keyUsageGranularity({ start_time: '2026-10-01T00:00:00Z', end_time: '2026-10-03T00:00:00Z' })).toBe('hour')
    expect(keyUsageGranularity({ start_time: '2026-10-01T00:00:00Z', end_time: '2026-10-03T00:00:01Z' })).toBe('day')
  })
  it('omits the end year within the same year', () => {
    const r = { start_time: new Date(2026, 9, 15).toISOString(), end_time: new Date(2026, 9, 22).toISOString() }
    expect(formatKeyUsageRange(r)).toBe('2026/10/15 00:00 – 10/21 23:59')
    const y = { start_time: new Date(2025, 11, 30).toISOString(), end_time: new Date(2026, 0, 2).toISOString() }
    expect(formatKeyUsageRange(y)).toBe('2025/12/30 00:00 – 2026/01/01 23:59')
  })
})

describe('money, share and tone', () => {
  it('formats money to cents with thresholds', () => {
    expect(formatKeyUsageMoney(0)).toBe('$0.00')
    expect(formatKeyUsageMoney(0.004)).toBe('<$0.01')
    expect(formatKeyUsageMoney(1234.567)).toBe('$1,234.57')
    expect(keyUsageMoneyTitle(0.004)).toBe('$0.0040')
  })
  it('formats shares', () => {
    expect(formatKeyUsageShare(83.19, 238.75)).toBe('35%')
    expect(formatKeyUsageShare(0.1, 100)).toBe('<1%')
    expect(formatKeyUsageShare(1, 0)).toBe('—')
  })
  it('maps platforms to tones', () => {
    expect(keyUsageGroupTone('openai')).toBe('openai')
    expect(keyUsageGroupTone('anthropic')).toBe('claude')
    expect(keyUsageGroupTone('gemini')).toBe('gemini')
    expect(keyUsageGroupTone('composite')).toBe('other')
    expect(keyUsageGroupTone(undefined)).toBe('other')
  })
})
