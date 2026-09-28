import { describe, expect, it } from 'vitest'

import { progressToStep, sectionProgress } from '../useScrollProgress'

describe('progressToStep', () => {
  it('splits progress evenly across steps', () => {
    expect(progressToStep(0, 5)).toBe(0)
    expect(progressToStep(0.19, 5)).toBe(0)
    expect(progressToStep(0.2, 5)).toBe(1)
    expect(progressToStep(0.79, 5)).toBe(3)
    expect(progressToStep(0.8, 5)).toBe(4)
  })

  it('keeps the last step at full progress', () => {
    expect(progressToStep(1, 5)).toBe(4)
  })

  it('clamps out-of-range and invalid input', () => {
    expect(progressToStep(-0.4, 5)).toBe(0)
    expect(progressToStep(3, 5)).toBe(4)
    expect(progressToStep(Number.NaN, 5)).toBe(0)
    expect(progressToStep(0.5, 0)).toBe(0)
  })
})

describe('sectionProgress', () => {
  it('is 0 before the section reaches the top of the viewport', () => {
    expect(sectionProgress(120, 3000, 1000)).toBe(0)
  })

  it('grows as the section scrolls past the viewport top', () => {
    expect(sectionProgress(-1000, 3000, 1000)).toBe(0.5)
  })

  it('is 1 once the section bottom reaches the viewport bottom', () => {
    expect(sectionProgress(-2000, 3000, 1000)).toBe(1)
    expect(sectionProgress(-2600, 3000, 1000)).toBe(1)
  })

  it('returns 0 when the section is not taller than the viewport', () => {
    expect(sectionProgress(-50, 800, 1000)).toBe(0)
  })
})
