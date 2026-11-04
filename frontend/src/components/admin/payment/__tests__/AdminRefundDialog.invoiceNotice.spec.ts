import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { PaymentOrder } from '@/types/payment'
import AdminRefundDialog from '../AdminRefundDialog.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const order: PaymentOrder = {
  id: 1, user_id: 10, amount: 100, pay_amount: 100, fee_rate: 0,
  payment_type: 'alipay', out_trade_no: 'order-1', status: 'COMPLETED',
  order_type: 'balance', created_at: '2026-09-01', expires_at: '2026-09-02',
  refund_amount: 0,
} as PaymentOrder

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(AdminRefundDialog, {
    props: { show: true, order, ...props },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } },
  })
}

describe('AdminRefundDialog invoice notice', () => {
  it('shows nothing for an order without an invoice', () => {
    expect(mountDialog().find('[data-testid="refund-invoice-notice"]').exists()).toBe(false)
  })

  it('warns above the form and still lets the admin refund', () => {
    const wrapper = mountDialog({ invoiceNotice: '请到税务平台冲红', invoiceNoticeTitle: '这笔订单已开票' })

    const notice = wrapper.get('[data-testid="refund-invoice-notice"]')
    expect(notice.text()).toContain('这笔订单已开票')
    expect(notice.text()).toContain('请到税务平台冲红')
    expect(wrapper.get('input[type="number"]').attributes('disabled')).toBeUndefined()
  })
})
