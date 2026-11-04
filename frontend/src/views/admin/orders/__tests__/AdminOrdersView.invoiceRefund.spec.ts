import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

import AdminOrdersView from '../AdminOrdersView.vue'

const api = vi.hoisted(() => ({ getOrders: vi.fn(), refundOrder: vi.fn(), getOrderState: vi.fn() }))

vi.mock('@/api/admin/payment', () => {
  const adminPaymentAPI = { getOrders: api.getOrders, refundOrder: api.refundOrder }
  return { adminPaymentAPI, default: adminPaymentAPI }
})
vi.mock('@/api/admin/invoice', () => ({ adminInvoiceAPI: { getOrderState: api.getOrderState } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params && Object.keys(params).length ? `${key}:${JSON.stringify(params)}` : key),
  }),
}))

const order = { id: 5, status: 'COMPLETED', payment_type: 'alipay', out_trade_no: 'sub2_a', amount: 100, pay_amount: 100, refund_amount: 0 }

const OrderTableStub = defineComponent({
  props: { orders: { type: Array, default: () => [] } },
  setup: (props, { slots }) => () =>
    h('div', (props.orders as { id: number }[]).map((row) => h('div', { 'data-testid': `row-${row.id}` }, slots.actions?.({ row })))),
})
const RefundDialogStub = defineComponent({
  props: { show: Boolean, invoiceNotice: String, invoiceNoticeTitle: String },
  setup: (props) => () =>
    props.show ? h('div', { 'data-testid': 'refund-dialog' }, `${props.invoiceNoticeTitle ?? ''}|${props.invoiceNotice ?? ''}`) : null,
})

async function openRefund() {
  const wrapper = mount(AdminOrdersView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        OrderTable: OrderTableStub,
        AdminRefundDialog: RefundDialogStub,
        BaseDialog: true,
        Pagination: true,
        Select: true,
        Icon: true,
        OrderStatusBadge: true,
      },
    },
  })
  await flushPromises()
  const button = wrapper.get('[data-testid="row-5"]').findAll('button').find((b) => b.text() === 'payment.admin.refund')
  await button!.trigger('click')
  await flushPromises()
  return wrapper
}

describe('AdminOrdersView refund invoice notice', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getOrders.mockResolvedValue({ data: { items: [order], total: 1, page: 1, page_size: 20 } })
  })

  it('opens the dialog without a notice for an order that is not invoiced', async () => {
    api.getOrderState.mockResolvedValue({ invoiced: false, excluded: false })
    const wrapper = await openRefund()
    expect(api.getOrderState).toHaveBeenCalledWith(5)
    expect(wrapper.get('[data-testid="refund-dialog"]').text()).toBe('|')
  })

  it('warns that the order is invoiced and names the invoice', async () => {
    api.getOrderState.mockResolvedValue({ invoiced: true, excluded: false, request_id: 9, request_status: 'completed', invoice_number: '26332' })
    const text = (await openRefund()).get('[data-testid="refund-dialog"]').text()
    expect(text).toContain('invoice.refundNotice.title')
    expect(text).toContain('invoice.refundNotice.issued')
    expect(text).toContain('26332')
  })

  it('warns about manually excluded orders', async () => {
    api.getOrderState.mockResolvedValue({ invoiced: true, excluded: true })
    const text = (await openRefund()).get('[data-testid="refund-dialog"]').text()
    expect(text).toContain('invoice.refundNotice.excluded')
  })

  it('says so when the invoice status cannot be checked, instead of treating it as not invoiced', async () => {
    api.getOrderState.mockRejectedValue(new Error('network'))
    const text = (await openRefund()).get('[data-testid="refund-dialog"]').text()
    expect(text).toContain('invoice.refundNotice.unknownTitle')
    expect(text).toContain('invoice.refundNotice.unknown')
  })
})
