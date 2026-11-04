import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

import UserOrdersView from '../UserOrdersView.vue'

const api = vi.hoisted(() => ({
  getMyOrders: vi.fn(),
  getRefundEligibleProviders: vi.fn(),
  requestRefund: vi.fn(),
  getInvoiceableOrders: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getMyOrders: api.getMyOrders,
    getRefundEligibleProviders: api.getRefundEligibleProviders,
    cancelOrder: vi.fn(),
    requestRefund: api.requestRefund,
  },
}))
vi.mock('@/api/invoice', () => ({ invoiceAPI: { getInvoiceableOrders: api.getInvoiceableOrders } }))
vi.mock('@/api/points', () => ({ pointsAPI: { getOrders: vi.fn() } }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError: api.showError, showSuccess: api.showSuccess }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

const order = (id: number) => ({ id, status: 'COMPLETED', payment_type: 'alipay', provider_instance_id: '7', amount: 100, pay_amount: 100 })

const OrderTableStub = defineComponent({
  props: { orders: { type: Array, default: () => [] } },
  setup: (props, { slots }) => () =>
    h('div', (props.orders as { id: number }[]).map((row) => h('div', { 'data-testid': `row-${row.id}` }, slots.actions?.({ row })))),
})
const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  setup: (props, { slots }) => () => (props.show ? h('div', { 'data-testid': 'refund-dialog' }, [slots.default?.(), slots.footer?.()]) : null),
})

async function mountView() {
  const wrapper = mount(UserOrdersView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        OrderTable: OrderTableStub,
        BaseDialog: BaseDialogStub,
        Pagination: true,
        Select: true,
        DataTable: true,
        Icon: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

const refundButton = (wrapper: Awaited<ReturnType<typeof mountView>>, id: number) =>
  wrapper.get(`[data-testid="row-${id}"]`).findAll('button').find((b) => b.text() === 'payment.orders.requestRefund')

describe('UserOrdersView invoice refund block', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getMyOrders.mockResolvedValue({ data: { items: [order(1), order(2)], total: 2, page: 1, page_size: 20 } })
    api.getRefundEligibleProviders.mockResolvedValue({ data: { provider_instance_ids: ['7'] } })
    api.getInvoiceableOrders.mockResolvedValue({
      items: [{ id: 1, already_invoiced: true }, { id: 2, already_invoiced: false }],
      summary: { total_invoiceable_amount: 100, invoiceable_count: 1 },
    })
  })

  it('replaces the refund button with a notice on invoiced orders only', async () => {
    const wrapper = await mountView()

    expect(refundButton(wrapper, 1)).toBeUndefined()
    expect(wrapper.get('[data-testid="row-1"]').text()).toContain('payment.orders.invoicedNoRefund')
    expect(refundButton(wrapper, 2)).toBeDefined()
    expect(wrapper.get('[data-testid="row-2"]').text()).not.toContain('payment.orders.invoicedNoRefund')
  })

  it('keeps the button when the invoice lookup fails, leaving the server to refuse', async () => {
    api.getInvoiceableOrders.mockRejectedValue(new Error('network'))
    const wrapper = await mountView()
    expect(refundButton(wrapper, 1)).toBeDefined()
    expect(api.showError).not.toHaveBeenCalled()
  })

  async function submitRefundFor(wrapper: Awaited<ReturnType<typeof mountView>>, id: number) {
    await refundButton(wrapper, id)!.trigger('click')
    await wrapper.get('[data-testid="refund-dialog"] textarea').setValue('不用了')
    await wrapper.get('[data-testid="refund-dialog"] .btn-primary').trigger('click')
    await flushPromises()
  }

  it('closes the dialog and swaps the button when the server reports the order was just invoiced', async () => {
    const wrapper = await mountView()
    api.requestRefund.mockRejectedValue({ reason: 'ORDER_INVOICED_REFUND_BLOCKED', message: 'blocked' })
    api.getInvoiceableOrders.mockResolvedValue({
      items: [{ id: 1, already_invoiced: true }, { id: 2, already_invoiced: true }],
      summary: { total_invoiceable_amount: 0, invoiceable_count: 0 },
    })

    await submitRefundFor(wrapper, 2)

    expect(api.showError).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="refund-dialog"]').exists()).toBe(false)
    expect(refundButton(wrapper, 2)).toBeUndefined()
  })

  it('keeps the dialog and the typed reason on other errors', async () => {
    const wrapper = await mountView()
    api.requestRefund.mockRejectedValue({ reason: 'BALANCE_NOT_ENOUGH', message: 'no balance' })

    await submitRefundFor(wrapper, 2)

    expect(api.showError).toHaveBeenCalledTimes(1)
    const textarea = wrapper.get('[data-testid="refund-dialog"] textarea').element as HTMLTextAreaElement
    expect(textarea.value).toBe('不用了')
  })
})
