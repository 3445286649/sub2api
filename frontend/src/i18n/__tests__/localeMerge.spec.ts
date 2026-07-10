import { describe, expect, it } from 'vitest'

import { mergeLocaleFallback } from '../locales/merge'

describe('mergeLocaleFallback', () => {
  it('fills missing nested messages without replacing current translations', () => {
    const current = {
      nav: { current: 'new value', shared: 'new translation' },
    }
    const fallback = {
      nav: { legacy: 'restored value', shared: 'old translation' },
      legacyArea: { title: 'restored title' },
    }

    expect(mergeLocaleFallback(current, fallback)).toEqual({
      nav: {
        current: 'new value',
        shared: 'new translation',
        legacy: 'restored value',
      },
      legacyArea: { title: 'restored title' },
    })
  })
})
