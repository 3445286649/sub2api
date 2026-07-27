import { describe, expect, it } from 'vitest'

import { calculateUsageTokenSpeed, formatUsageTokenSpeed } from '../usageTokenSpeed'

const baseRow = {
  billing_mode: 'token',
  duration_ms: 5_500,
  first_token_ms: 500,
  image_count: 0,
  output_tokens: 100,
  stream: true,
}

describe('usage token speed', () => {
  it('uses generation time after the first token for streaming requests', () => {
    expect(calculateUsageTokenSpeed(baseRow)).toBe(20)
    expect(formatUsageTokenSpeed(baseRow)).toBe('20 Token/s')
  })

  it('uses total duration for non-streaming requests', () => {
    expect(calculateUsageTokenSpeed({ ...baseRow, stream: false })).toBeCloseTo(18.1818, 4)
  })

  it.each([
    ['zero output tokens', { output_tokens: 0 }],
    ['missing duration', { duration_ms: null }],
    ['missing streaming first token time', { first_token_ms: null }],
    ['non-positive generation time', { duration_ms: 500 }],
    ['image billing', { billing_mode: 'image' }],
    ['legacy image billing', { billing_mode: null, image_count: 1 }],
    ['video billing', { billing_mode: 'video', image_count: 1 }],
  ])('returns no speed for %s', (_label, overrides) => {
    const row = { ...baseRow, ...overrides }
    expect(calculateUsageTokenSpeed(row)).toBeNull()
    expect(formatUsageTokenSpeed(row)).toBe('-')
  })
})
