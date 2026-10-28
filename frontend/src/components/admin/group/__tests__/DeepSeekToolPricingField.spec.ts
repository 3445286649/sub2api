import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h, reactive } from 'vue'

import DeepSeekToolPricingField from '../DeepSeekToolPricingField.vue'
import type { DeepSeekToolPricing } from '@/types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

function mountField() {
  const state = reactive<{ pricing: DeepSeekToolPricing }>({
    pricing: {
      search: { mode: 'fixed', price_per_call: 0.01 },
      fetch: { mode: 'group', price_per_call: 0.01 },
    },
  })
  const host = defineComponent({
    setup: () => () => h(DeepSeekToolPricingField, {
      value: state.pricing,
      groupRate: 1,
      idPrefix: 'test',
      'onUpdate:value': (value: DeepSeekToolPricing) => { state.pricing = value },
    }),
  })
  return { wrapper: mount(host), state }
}

describe('DeepSeekToolPricingField', () => {
  it.each(['search', 'fetch'] as const)('keeps an unfinished decimal in the %s price', async (tool) => {
    const { wrapper, state } = mountField()
    const input = wrapper.get<HTMLInputElement>(`#test-${tool}-price`)

    for (const text of ['0', '0.', '0.0', '0.00', '0.007']) {
      await input.setValue(text)
      expect(input.element.value).toBe(text)
    }
    expect(state.pricing[tool].price_per_call).toBe(0.007)
    await input.trigger('blur')
    expect(input.element.value).toBe('0.007')
  })

  it('keeps a pasted decimal and rejects invalid input without turning it into zero', async () => {
    const { wrapper, state } = mountField()
    const input = wrapper.get<HTMLInputElement>('#test-search-price')

    await input.setValue('0.007')
    expect(input.element.value).toBe('0.007')
    expect(state.pricing.search.price_per_call).toBe(0.007)

    await input.setValue('not a price')
    expect(input.element.value).toBe('not a price')
    expect(state.pricing.search.price_per_call).toBeNull()

    await input.setValue('')
    expect(state.pricing.search.price_per_call).toBeNull()
    await input.setValue('0')
    expect(state.pricing.search.price_per_call).toBe(0)
  })

  it('preserves fractional independent multipliers', async () => {
    const { wrapper, state } = mountField()
    await wrapper.get('#test-search-mode').setValue('independent')
    const input = wrapper.get<HTMLInputElement>('#test-search-rate')

    await input.setValue('0.')
    expect(input.element.value).toBe('0.')
    await input.setValue('0.33')
    expect(input.element.value).toBe('0.33')
    expect(state.pricing.search.rate_multiplier).toBe(0.33)

    await input.setValue('')
    expect(input.element.value).toBe('')
    expect(state.pricing.search.rate_multiplier).toBeNull()
  })
})
