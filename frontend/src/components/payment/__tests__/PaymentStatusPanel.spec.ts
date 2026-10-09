import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const pollOrderStatus = vi.hoisted(() => vi.fn())
const cancelOrder = vi.hoisted(() => vi.fn())
const verifyOrder = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const toCanvas = vi.hoisted(() => vi.fn())
const copyToClipboard = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    pollOrderStatus,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    cachedPublicSettings: null,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    cancelOrder,
    verifyOrder,
  },
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: false,
    copyToClipboard,
  }),
}))

vi.mock('qrcode', () => ({
  default: {
    toCanvas,
  },
}))

import PaymentStatusPanel from '../PaymentStatusPanel.vue'

const orderFactory = (status: string, overrides: Record<string, unknown> = {}) => ({
  id: 42,
  user_id: 9,
  amount: 88,
  pay_amount: 88,
  fee_rate: 0,
  payment_type: 'alipay',
  out_trade_no: 'sub2_20260420abcd1234',
  status,
  order_type: 'balance',
  created_at: '2026-04-20T12:00:00Z',
  expires_at: '2099-01-01T12:30:00Z',
  refund_amount: 0,
  ...overrides,
})

const usdtStubs = { Icon: true, RouterLink: true, ConfirmDialog: true }

const usdtProps = (overrides: Record<string, unknown> = {}) => ({
  orderId: 42,
  qrCode: '0x3b210bdc924c685fDd10Ae96b7f95D0E14536106',
  expiresAt: '2099-01-01T12:30:00Z',
  paymentType: 'usdt_bsc',
  orderType: 'balance',
  cryptoAmount: '10.000123',
  cryptoCurrency: 'USDT',
  receiveAddress: '0x3b210bdc924c685fDd10Ae96b7f95D0E14536106',
  outTradeNo: 'sub2_20260420abcd1234',
  ...overrides,
})

const gatewayCheckoutStub = {
  name: 'CryptoGatewayCheckout',
  props: ['cryptoRate', 'totalSeconds', 'remainingSeconds', 'cryptoAmount', 'receiveAddress', 'orderStatus', 'gatewayStatus', 'queryFailed'],
  emits: ['check'],
  template: '<div data-test="gateway-checkout"><button @click="$emit(\'check\')" /></div>',
}

const gatewayQuote = (overrides: Record<string, unknown> = {}) => ({
  payment_type: 'usdt_gateway',
  create_stage: 'ready',
  crypto_network: 'bsc_mainnet',
  crypto_amount: '0.142859',
  receive_address: '0x3b210bdc924c685fDd10Ae96b7f95D0E14536106',
  gateway_status: 'WAITING',
  ...overrides,
})

const gatewayPanel = (overrides: Record<string, unknown> = {}) => mount(PaymentStatusPanel, {
  props: { ...usdtProps(), paymentType: 'usdt_gateway', cryptoNetwork: 'bsc_mainnet', cryptoRate: '7.00', ...overrides },
  global: { stubs: { ...usdtStubs, CryptoGatewayCheckout: gatewayCheckoutStub } },
})

describe('PaymentStatusPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    pollOrderStatus.mockReset()
    cancelOrder.mockReset()
    verifyOrder.mockReset()
    showError.mockReset()
    copyToClipboard.mockReset().mockResolvedValue(true)
    toCanvas.mockReset().mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('passes the fallback rate and then synchronizes the gateway quote rate and server lifetime', async () => {
    vi.setSystemTime(new Date('2026-10-09T11:00:00Z'))
    const firstOrder = orderFactory('PENDING', gatewayQuote({
      created_at: '2026-10-09T10:55:00Z', expires_at: '2026-10-09T11:25:00Z',
    }))
    pollOrderStatus.mockResolvedValue(firstOrder)
    verifyOrder.mockResolvedValue({ data: firstOrder })
    const wrapper = gatewayPanel({ expiresAt: '2026-10-09T11:30:00Z' })
    await flushPromises()

    const checkout = wrapper.findComponent({ name: 'CryptoGatewayCheckout' })
    expect(checkout.props('cryptoRate')).toBe('7.00')
    expect(checkout.props('totalSeconds')).toBe(1800)
    expect(checkout.props('remainingSeconds')).toBe(1500)

    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', gatewayQuote({
      crypto_rate: '7.10', created_at: '2026-10-09T10:55:00Z', expires_at: '2026-10-09T11:30:00Z',
    })))
    await vi.advanceTimersByTimeAsync(5000)
    expect(checkout.props('cryptoRate')).toBe('7.10')
    expect(checkout.props('totalSeconds')).toBe(2100)
    expect(checkout.props('remainingSeconds')).toBe(1795)
    wrapper.unmount()
  })

  it.each(['PAID', 'RECHARGING'])('keeps gateway %s in the fulfillment state without announcing success', async (status) => {
    pollOrderStatus.mockResolvedValue(orderFactory(status, gatewayQuote({ gateway_status: 'PAID', paid_at: '2026-10-09T11:00:00Z' })))
    const wrapper = gatewayPanel()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'CryptoGatewayCheckout' }).props('orderStatus')).toBe(status)
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.text()).not.toContain('payment.result.success')
    wrapper.unmount()
  })

  it.each(['EXPIRED', 'CANCELLED'])('continues checking a gateway %s order until fulfillment completes', async (status) => {
    const closedOrder = orderFactory(status, gatewayQuote({ gateway_status: status }))
    pollOrderStatus.mockResolvedValue(closedOrder)
    verifyOrder.mockResolvedValue({ data: closedOrder })
    const wrapper = gatewayPanel()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'CryptoGatewayCheckout' }).props('orderStatus')).toBe(status)
    expect(wrapper.emitted('success')).toBeUndefined()

    pollOrderStatus.mockResolvedValue(orderFactory('COMPLETED', gatewayQuote({ gateway_status: 'PAID', paid_at: '2026-10-09T11:00:00Z' })))
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(pollOrderStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('success')).toHaveLength(1)
    expect(wrapper.emitted('settled')).toEqual([['success']])
    expect(wrapper.findComponent({ name: 'CryptoGatewayCheckout' }).exists()).toBe(false)
    wrapper.unmount()
  })

  it('preserves a gateway quote through failed poll and verify requests, clearing the notice after recovery', async () => {
    const order = orderFactory('PENDING', gatewayQuote())
    pollOrderStatus.mockResolvedValue(order)
    verifyOrder.mockResolvedValue({ data: order })
    const wrapper = gatewayPanel()
    await flushPromises()
    const checkout = wrapper.findComponent({ name: 'CryptoGatewayCheckout' })
    expect(checkout.props('queryFailed')).toBe(false)

    pollOrderStatus.mockResolvedValueOnce(null)
    await vi.advanceTimersByTimeAsync(5000)
    expect(checkout.props('queryFailed')).toBe(true)
    expect(checkout.props('cryptoAmount')).toBe('0.142859')
    expect(checkout.props('receiveAddress')).toBe(order.receive_address)

    pollOrderStatus.mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(5000)
    expect(checkout.props('queryFailed')).toBe(true)
    await vi.advanceTimersByTimeAsync(5000)
    expect(checkout.props('queryFailed')).toBe(false)

    verifyOrder.mockRejectedValueOnce(new Error('verify unavailable'))
    await vi.advanceTimersByTimeAsync(5000)
    expect(checkout.props('queryFailed')).toBe(true)
    expect(checkout.props('cryptoAmount')).toBe('0.142859')
    await vi.advanceTimersByTimeAsync(5000)
    expect(checkout.props('queryFailed')).toBe(false)
    expect(wrapper.emitted('success')).toBeUndefined()
    wrapper.unmount()
  })

  it('ignores a gateway response that arrives after the panel is unmounted', async () => {
    let resolvePoll!: (order: ReturnType<typeof orderFactory>) => void
    pollOrderStatus.mockReturnValue(new Promise(resolve => { resolvePoll = resolve }))
    const wrapper = gatewayPanel()
    wrapper.unmount()
    resolvePoll(orderFactory('COMPLETED', gatewayQuote({ gateway_status: 'PAID', paid_at: '2026-10-09T11:00:00Z' })))
    await flushPromises()
    expect(verifyOrder).not.toHaveBeenCalled()
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('settled')).toBeUndefined()
    await vi.advanceTimersByTimeAsync(5000)
    expect(pollOrderStatus).toHaveBeenCalledTimes(1)
  })

  it('recovers gateway quotes, shows confirmations/review and waits for fulfillment before success', async () => {
    const props = { ...usdtProps(), paymentType: 'usdt_gateway', qrCode: '', cryptoAmount: '', receiveAddress: '', cryptoNetwork: 'tron_mainnet', createStage: 'unknown' }
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', { payment_type: 'usdt_gateway', create_stage: 'unknown', crypto_network: 'tron_mainnet' }))
    verifyOrder.mockResolvedValue({ data: orderFactory('PENDING', { payment_type: 'usdt_gateway', create_stage: 'unknown', crypto_network: 'tron_mainnet' }) })
    const wrapper = mount(PaymentStatusPanel, { props, global: { stubs: usdtStubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('payment.gateway.creating')
    const quote = { payment_type: 'usdt_gateway', create_stage: 'ready', crypto_network: 'tron_mainnet', crypto_amount: '10.000123', receive_address: 'T' + 'A'.repeat(33), gateway_status: 'WAITING', confirmations: 0, required_confirmations: 30 }
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', quote))
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(wrapper.get('[data-test="gateway-amount"]').text()).toContain('10.000123')
    expect(toCanvas).toHaveBeenCalledWith(expect.any(HTMLCanvasElement), quote.receive_address, expect.any(Object))
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', { ...quote, gateway_status: 'CONFIRMING', confirmations: 8 }))
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.text()).toContain('payment.gateway.confirming')
    expect(wrapper.find('[data-test="gateway-copy-address"]').exists()).toBe(false)
    pollOrderStatus.mockResolvedValue(orderFactory('PAID', { ...quote, gateway_status: 'PAID', paid_at: '2026-10-09T00:00:00Z' }))
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.text()).toContain('payment.gateway.accepted')
    expect(wrapper.emitted('success')).toBeUndefined()
    pollOrderStatus.mockResolvedValue(orderFactory('COMPLETED', { ...quote, paid_at: '2026-10-09T00:00:00Z' }))
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.emitted('success')).toHaveLength(1)
    wrapper.unmount()
  })

  it('does not show payable details for a gateway REVIEW state', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', { payment_type: 'usdt_gateway', gateway_status: 'REVIEW', crypto_network: 'bsc_mainnet' }))
    verifyOrder.mockResolvedValue({ data: orderFactory('PENDING', { payment_type: 'usdt_gateway', gateway_status: 'REVIEW', crypto_network: 'bsc_mainnet' }) })
    const wrapper = mount(PaymentStatusPanel, { props: { ...usdtProps(), paymentType: 'usdt_gateway', cryptoNetwork: 'bsc_mainnet' }, global: { stubs: usdtStubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('payment.gateway.review')
    expect(wrapper.find('[data-test="gateway-copy-address"]').exists()).toBe(false)
    expect(wrapper.emitted('success')).toBeUndefined()
    wrapper.unmount()
  })

  it('treats RECHARGING as a successful terminal state', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('RECHARGING'))

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
          RouterLink: true,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(pollOrderStatus).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('payment.result.success')
    expect(wrapper.emitted('success')).toHaveLength(1)
  })

  it('shows reopen button in QR mode when payUrl is also available', async () => {
    const openSpy = vi.spyOn(window, 'open').mockReturnValue({ closed: false } as Window)

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        payUrl: 'https://pay.example.com/session/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
          RouterLink: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('payment.qr.openPayWindow')

    await wrapper.get('button.btn.btn-secondary.text-sm').trigger('click')
    expect(openSpy).toHaveBeenCalledWith(
      'https://pay.example.com/session/42',
      'paymentPopup',
      expect.any(String),
    )

    openSpy.mockRestore()
  })

  it('renders the USDT-BSC checkout and copies the raw amount and address', async () => {
    const address = '0x3b210bdc924c685fDd10Ae96b7f95D0E14536106'

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: address,
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'usdt_bsc',
        orderType: 'balance',
        cryptoAmount: '10.000123',
        cryptoCurrency: 'USDT',
        cryptoNetwork: 'BSC',
        cryptoRate: '6.770000',
        receiveAddress: address,
      },
      global: {
        stubs: usdtStubs,
      },
    })

    await flushPromises()

    expect(wrapper.findComponent({ name: 'UsdtBscCheckout' }).exists()).toBe(true)
    expect(wrapper.get('[data-test="usdt-amount"]').text()).toBe('10.000123USDT')
    expect(wrapper.text()).toContain('payment.crypto.exchangeRateValue')
    expect(toCanvas).toHaveBeenCalledWith(expect.anything(), address, expect.any(Object))

    await wrapper.get('[data-test="copy-amount"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('10.000123', 'payment.crypto.amountCopied')

    await wrapper.get('button.wallet-address-copy').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(address, 'payment.crypto.addressCopied')
  })

  it('verifies a pending USDT-BSC order on a fixed interval without an attempt cap', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', { payment_type: 'usdt_bsc' }))
    verifyOrder.mockResolvedValue({ data: orderFactory('PENDING', { payment_type: 'usdt_bsc' }) })

    const wrapper = mount(PaymentStatusPanel, {
      props: usdtProps(),
      global: { stubs: usdtStubs },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(verifyOrder).toHaveBeenCalledTimes(1)

    // 6 次上限按 15 秒算约 90 秒就用完；USDT 应该 3 分钟后还在核单
    await vi.advanceTimersByTimeAsync(180_000)
    await flushPromises()
    expect(verifyOrder.mock.calls.length).toBeGreaterThan(6)

    wrapper.unmount()
  })

  it('keeps watching an expired USDT-BSC order and switches to success when it is credited', async () => {
    const expiresAt = new Date(Date.now() + 2000).toISOString()
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', { payment_type: 'usdt_bsc' }))
    verifyOrder.mockResolvedValue({ data: orderFactory('PENDING', { payment_type: 'usdt_bsc' }) })

    const wrapper = mount(PaymentStatusPanel, {
      props: usdtProps({ expiresAt }),
      global: { stubs: usdtStubs },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(wrapper.text()).toContain('payment.qr.expired')
    expect(wrapper.find('[data-test="usdt-watch"]').text()).toContain('payment.crypto.watching')

    const txHash = '0x9c3e5b1f2a7d4e6c8b0a1f3d5e7c9b2a4d6f8e0c1b3a5d7e9f1c3b5a7d9e1f3a'
    pollOrderStatus.mockResolvedValue(orderFactory('COMPLETED', {
      payment_type: 'usdt_bsc',
      crypto_tx_hash: txHash,
      crypto_amount: '10.000124',
      crypto_currency: 'USDT',
    }))
    await vi.advanceTimersByTimeAsync(15_000)
    await flushPromises()

    expect(wrapper.text()).toContain('payment.result.success')
    expect(wrapper.get('[data-test="crypto-paid-amount"]').text()).toBe('10.000124 USDT')
    expect(wrapper.get('[data-test="crypto-tx-link"]').attributes('href')).toBe(`https://bscscan.com/tx/${txHash}`)
    expect(wrapper.emitted('success')).toHaveLength(1)
    expect(wrapper.emitted('settled')).toEqual([['expired'], ['success']])

    wrapper.unmount()
  })

  it('stops watching a cancelled USDT-BSC order after the grace window', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING', { payment_type: 'usdt_bsc' }))
    verifyOrder.mockResolvedValue({ data: orderFactory('PENDING', { payment_type: 'usdt_bsc' }) })
    cancelOrder.mockResolvedValue({})

    const wrapper = mount(PaymentStatusPanel, {
      props: usdtProps(),
      global: { stubs: usdtStubs },
    })
    await flushPromises()

    wrapper.findComponent({ name: 'UsdtBscCheckout' }).vm.$emit('cancel')
    await flushPromises()
    expect(cancelOrder).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('payment.qr.cancelled')

    pollOrderStatus.mockClear()
    pollOrderStatus.mockResolvedValue(orderFactory('CANCELLED', { payment_type: 'usdt_bsc' }))
    await vi.advanceTimersByTimeAsync(6 * 60 * 1000)
    await flushPromises()

    expect(wrapper.text()).toContain('payment.crypto.watchEnded')
    const callsAtEnd = pollOrderStatus.mock.calls.length
    expect(callsAtEnd).toBeGreaterThan(20)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(pollOrderStatus.mock.calls.length).toBe(callsAtEnd)

    wrapper.unmount()
  })

  it('actively verifies a stuck pending order and settles it when upstream confirms payment', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    verifyOrder.mockResolvedValue({
      data: orderFactory('COMPLETED'),
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'wxpay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
          RouterLink: true,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(pollOrderStatus).toHaveBeenCalledWith(42)
    expect(verifyOrder).toHaveBeenCalledWith('sub2_20260420abcd1234')
    expect(wrapper.text()).toContain('payment.result.success')
    expect(wrapper.emitted('success')).toHaveLength(1)
  })

  it('actively verifies a pending mobile Alipay precreate order', async () => {
    const originalLocation = window.location
    const originalHidden = Object.getOwnPropertyDescriptor(document, 'hidden')
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { assign: vi.fn() },
    })
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => false,
    })
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    verifyOrder.mockResolvedValue({ data: orderFactory('COMPLETED') })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/dynamic-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
        mobileAlipayDeepLink: true,
      },
      global: { stubs: { Icon: true, RouterLink: true } },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(verifyOrder).toHaveBeenCalledWith('sub2_20260420abcd1234')
    expect(wrapper.emitted('success')).toHaveLength(1)

    wrapper.unmount()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    if (originalHidden) Object.defineProperty(document, 'hidden', originalHidden)
  })

  it('actively verifies a pending desktop Alipay order', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    verifyOrder.mockResolvedValue({ data: orderFactory('COMPLETED') })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/desktop-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
      },
      global: { stubs: { Icon: true, RouterLink: true } },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(verifyOrder).toHaveBeenCalledWith('sub2_20260420abcd1234')
    expect(wrapper.emitted('success')).toHaveLength(1)

    wrapper.unmount()
  })

  it('keeps the QR fallback hidden until the Alipay app launch times out', async () => {
    const originalLocation = window.location
    const originalHidden = Object.getOwnPropertyDescriptor(document, 'hidden')
    const assign = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { assign },
    })
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => false,
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/dynamic-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
        mobileAlipayDeepLink: true,
      },
      global: { stubs: { Icon: true, RouterLink: true } },
    })

    await flushPromises()
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('alipays://platformapi/startapp?saId=10000007&qrcode='))
    expect(wrapper.find('[data-test="alipay-qr-fallback"]').exists()).toBe(false)

    await vi.advanceTimersByTimeAsync(2200)
    await flushPromises()

    expect(wrapper.find('[data-test="alipay-qr-fallback"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('payment.qr.saveQRCode')
    expect(wrapper.text()).toContain('sub2_20260420abcd1234')
    expect(toCanvas).toHaveBeenCalledWith(expect.any(HTMLCanvasElement), 'https://qr.alipay.com/dynamic-order-42', expect.any(Object))

    wrapper.unmount()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    if (originalHidden) Object.defineProperty(document, 'hidden', originalHidden)
  })

  it('does not show the QR fallback after the page enters the background', async () => {
    const originalLocation = window.location
    const originalHidden = Object.getOwnPropertyDescriptor(document, 'hidden')
    let hidden = false
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { assign: vi.fn() },
    })
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => hidden,
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/dynamic-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
        mobileAlipayDeepLink: true,
      },
      global: { stubs: { Icon: true, RouterLink: true } },
    })

    await flushPromises()
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(2200)
    await flushPromises()

    expect(wrapper.find('[data-test="alipay-qr-fallback"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('payment.qr.alipayContinueInApp')

    wrapper.unmount()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    if (originalHidden) Object.defineProperty(document, 'hidden', originalHidden)
  })
})
