import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import UserOrdersView from '../UserOrdersView.vue'

const { getMyOrders, getRefundEligibleProviders, getPointsOrders } = vi.hoisted(() => ({
  getMyOrders: vi.fn(),
  getRefundEligibleProviders: vi.fn(),
  getPointsOrders: vi.fn(),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getMyOrders,
    getRefundEligibleProviders,
    cancelOrder: vi.fn(),
    requestRefund: vi.fn(),
  },
}))

vi.mock('@/api/points', () => ({
  pointsAPI: {
    getOrders: getPointsOrders,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('UserOrdersView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getMyOrders.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 20 } })
    getRefundEligibleProviders.mockResolvedValue({ data: { provider_instance_ids: [] } })
    getPointsOrders.mockResolvedValue({
      data: {
        items: [{
          id: 1,
          order_no: 'PS123',
          user_id: 1,
          product_name: '1 积分兑换余额',
          product_type: 'balance',
          points_price: 1,
          balance_amount: 1,
          status: 'completed',
          balance_after: 10,
          created_at: '2026-08-14T00:00:00Z',
          completed_at: '2026-08-14T00:00:00Z',
        }],
        total: 1,
        page: 1,
        page_size: 20,
      },
    })
  })

  it('switches from payment orders to the current user points redemption orders', async () => {
    const wrapper = mount(UserOrdersView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          OrderTable: { template: '<div data-test="payment-orders" />' },
          DataTable: {
            props: ['data'],
            template: '<div data-test="points-orders"><span v-for="row in data" :key="row.id">{{ row.product_name }}</span></div>',
          },
          Pagination: true,
          BaseDialog: true,
          Select: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    expect(getMyOrders).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="payment-orders"]').exists()).toBe(true)

    const pointsTab = wrapper.findAll('button').find((button) => button.text() === 'points.userOrders.redemptionTab')
    expect(pointsTab).toBeDefined()
    await pointsTab!.trigger('click')
    await flushPromises()

    expect(getPointsOrders).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(wrapper.find('[data-test="points-orders"]').text()).toContain('1 积分兑换余额')
    expect(wrapper.find('[data-test="payment-orders"]').exists()).toBe(false)
  })
})
