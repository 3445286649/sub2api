import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PointsProductCard from '../PointsProductCard.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const product = {
  id: 1,
  product_type: 'balance' as const,
  name: 'Balance 5',
  description: 'Referral reward',
  points_price: 10,
  original_points_price: 12,
  balance_amount: 5,
  stock_total: 20,
  stock_redeemed: 3,
  per_user_limit: 1,
  features: 'Instant credit\nNo expiry',
  sort_order: 0,
  for_sale: true,
  created_at: '2026-08-14T00:00:00Z',
  updated_at: '2026-08-14T00:00:00Z',
}

describe('PointsProductCard', () => {
  it('shows balance reward and emits redemption for sufficient points', async () => {
    const wrapper = mount(PointsProductCard, { props: { product, availablePoints: 10 } })
    expect(wrapper.text()).toContain('$5.00')
    expect(wrapper.text()).toContain('Instant credit')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('redeem')?.[0]).toEqual([product])
  })

  it('blocks redemption when points are insufficient', () => {
    const wrapper = mount(PointsProductCard, { props: { product, availablePoints: 9 } })
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  })
})
