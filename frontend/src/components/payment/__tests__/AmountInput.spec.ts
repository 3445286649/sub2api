import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
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

    // 只有到账金额高于面值的档位才显示到账行：100 -> 120 显示，80 -> 80 不显示
    expect(wrapper.text()).toContain('120.00')
    expect(wrapper.text()).not.toContain('80.00')
    // 倍率 9.9 不应参与计算，否则会算出 792.00
    expect(wrapper.text()).not.toContain('792.00')
  })

  it('hides the credited line on tiers without a bonus', () => {
    // 与真实页面一致：门槛以下倍率为 1，达到门槛才有赠送
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        amounts: [50, 100],
        bonusEnabled: true,
        creditMultiplier: 1.2,
        bonusThreshold: 100,
        creditAmountFor: (amount: number) => amount >= 100 ? amount * 1.2 : amount,
        formatCreditAmount: (value: number) => value.toFixed(2),
      },
    })

    // 面值本身照常显示，但门槛以下的档位不该出现到账 50.00 这一行
    expect(wrapper.text()).toContain('¥50')
    expect(wrapper.text()).not.toContain('到账 50.00')
    expect(wrapper.text()).toContain('到账 120.00')
  })
})

enableAutoUnmount(afterEach)

function mountInput(value: number | null = null) {
  return mount(AmountInput, { props: { modelValue: value } })
}

describe('recharge amount input', () => {
  it.each(['10abc', '10.555', '-10', '1e2'])('restores the accepted amount after rejecting %s', async (value) => {
    const wrapper = mountInput(10)
    const input = wrapper.get('input')
    await input.setValue(value)
    expect((input.element as HTMLInputElement).value).toBe('10')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('restores the last typed amount rather than a stale prop', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    await input.setValue('12.50')
    await input.setValue('12.500')
    expect((input.element as HTMLInputElement).value).toBe('12.50')
    expect(wrapper.emitted('update:modelValue')).toEqual([[12.5]])
  })

  it('preserves decimal editing and allows clearing the amount', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    for (const value of ['0', '0.', '0.5', '0.50', '']) await input.setValue(value)
    expect(wrapper.emitted('update:modelValue')).toEqual([[null], [null], [0.5], [0.5], [null]])
    expect((input.element as HTMLInputElement).value).toBe('')
  })
})
