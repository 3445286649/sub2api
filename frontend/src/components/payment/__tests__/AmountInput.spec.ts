import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, unknown>) => {
        if (key === 'payment.quickAmountCredit') return `到账 ${values?.amount}`
        return key
      },
    }),
  }
})

describe('AmountInput recharge bonus preview', () => {
  it('shows credited balance preview on quick amount cards when bonus display is enabled', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        amounts: [100],
        bonusEnabled: true,
        creditMultiplier: 1.1,
        formatCreditAmount: (value: number) => value.toFixed(2),
      },
      global: {
        mocks: {
          $t: (key: string) => key,
        },
      },
    })

    expect(wrapper.text()).toContain('到账')
    expect(wrapper.text()).toContain('110.00')
  })

  it('keeps custom amount input updating the selected amount', async () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        amounts: [100],
        bonusEnabled: true,
        creditMultiplier: 1.1,
      },
    })

    await wrapper.get('input').setValue('88')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([88])
  })

  it('prefers per-amount credited calculator when provided', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        amounts: [80, 100],
        bonusEnabled: true,
        creditMultiplier: 9.9,
        creditAmountFor: (amount: number) => amount >= 100 ? amount * 1.2 : amount,
        formatCreditAmount: (value: number) => value.toFixed(2),
      },
    })

    expect(wrapper.text()).toContain('80.00')
    expect(wrapper.text()).toContain('120.00')
    expect(wrapper.text()).not.toContain('792.00')
  })
})
