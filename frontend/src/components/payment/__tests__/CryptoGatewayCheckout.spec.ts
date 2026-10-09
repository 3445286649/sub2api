import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { toCanvas, copyToClipboard } = vi.hoisted(() => ({ toCanvas: vi.fn(), copyToClipboard: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, args?: Record<string, unknown>) => args ? `${key} ${JSON.stringify(args)}` : key }) }))
vi.mock('qrcode', () => ({ default: { toCanvas } }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard }) }))

import CryptoGatewayCheckout from '../CryptoGatewayCheckout.vue'

const address = '0x3b210bdc924c685fDd10Ae96b7f95D0E14536106'
const ConfirmDialog = {
  props: ['show'], emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-test="confirm-dialog"><button data-test="confirm-ok" @click="$emit(\'confirm\')" /></div>',
}
function checkout(overrides: Record<string, unknown> = {}) {
  return mount(CryptoGatewayCheckout, {
    props: {
      network: 'bsc_mainnet', cryptoAmount: '0.142859', cryptoRate: '7.00', receiveAddress: address,
      orderNo: 'sub2_gateway_fixture', orderAmount: 'CNY 1.00', remainingSeconds: 1790, totalSeconds: 1800,
      gatewayStatus: 'WAITING', createStage: 'ready', showSupport: true, ...overrides,
    },
    global: { stubs: { Icon: true, RouterLink: { template: '<a><slot /></a>' }, ConfirmDialog } },
  })
}

describe('CryptoGatewayCheckout', () => {
  beforeEach(() => {
    toCanvas.mockReset().mockResolvedValue(undefined)
    copyToClipboard.mockReset().mockResolvedValue(true)
  })

  it('uses the legacy amount emphasis, raw copy values, locked rate and mobile QR interaction', async () => {
    const wrapper = checkout()
    await flushPromises()
    expect(wrapper.get('[data-test="gateway-amount-tail"]').text()).toBe('2859')
    expect(wrapper.text()).toContain('payment.crypto.exchangeRateValue')
    expect(toCanvas).toHaveBeenCalledWith(expect.any(HTMLCanvasElement), address, expect.objectContaining({ margin: 1 }))
    await wrapper.get('[data-test="gateway-copy-amount"]').trigger('click')
    await wrapper.get('[data-test="gateway-copy-address"]').trigger('click')
    expect(copyToClipboard.mock.calls.map(call => call[0])).toEqual(['0.142859', address])
    const toggle = wrapper.get('[data-test="gateway-toggle-qr"]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('[data-test="gateway-address"]').text().replace(/\s/g, '')).toBe(address)
  })

  it('does not expose an old quote while an unknown create result is being recovered', () => {
    const wrapper = checkout({ createStage: 'unknown' })
    expect(wrapper.text()).toContain('payment.gateway.creating')
    expect(wrapper.find('[data-test="gateway-copy-address"]').exists()).toBe(false)
    expect(wrapper.find('canvas').exists()).toBe(false)
  })
})

describe('gateway state boundaries', () => {
  beforeEach(() => {
    toCanvas.mockReset().mockResolvedValue(undefined)
    copyToClipboard.mockReset().mockResolvedValue(true)
  })

  it.each([
    ['CONFIRMING', 'confirming'], ['PAID', 'accepted'], ['REVIEW', 'review'],
    ['EXPIRED', 'expired'], ['CANCELLED', 'cancelled'],
  ])('renders %s as a dedicated non-payable state', (gatewayStatus, label) => {
    const wrapper = checkout({ gatewayStatus, txHash: '0x' + 'ab'.repeat(32), confirmations: 8, requiredConfirmations: 20 })
    expect(wrapper.get('[data-test="gateway-status"]').text()).toContain(`payment.gateway.${label}`)
    expect(wrapper.find('canvas').exists()).toBe(false)
    expect(wrapper.find('[data-test="gateway-copy-amount"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="gateway-copy-address"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="gateway-cancel"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="crypto-tx-link"]').attributes('href')).toContain('https://bscscan.com/tx/')
    if (gatewayStatus === 'CONFIRMING') expect(wrapper.get('[data-test="gateway-confirmations"]').text()).toContain('"current":8,"required":20')
  })

  it('adapts TRON network warnings, original address and explorer links', async () => {
    const tronAddress = 'TYrRGMhFiCVJ3D9s1VsFxbbxFtq5eER3qJ'
    const wrapper = checkout({ network: 'tron_mainnet', receiveAddress: tronAddress })
    expect(wrapper.text()).toContain('TRON · TRC20')
    expect(wrapper.text()).toContain('payment.gateway.tronFeeTitle')
    expect(wrapper.text()).not.toContain('payment.crypto.ruleGasTitle')
    await wrapper.get('[data-test="gateway-copy-address"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(tronAddress, 'payment.crypto.addressCopied')
    await wrapper.setProps({ gatewayStatus: 'CONFIRMING', txHash: 'ab'.repeat(32) })
    expect(wrapper.get('[data-test="crypto-tx-link"]').attributes('href')).toBe('https://tronscan.org/#/transaction/' + 'ab'.repeat(32))
    expect(wrapper.get('[data-test="crypto-tx-link"]').attributes('title')).toContain('TronScan')
  })

  it('copies every decimal digit without rounding and rejects invalid quotes', async () => {
    const precise = '99999.123456789012345678'
    const wrapper = checkout({ cryptoAmount: precise })
    await wrapper.get('[data-test="gateway-copy-amount"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(precise, 'payment.crypto.amountCopied')
    await wrapper.setProps({ network: 'tron_mainnet', receiveAddress: 'TYrRGMhFiCVJ3D9s1VsFxbbxFtq5eER3qJ' })
    expect(wrapper.find('[data-test="gateway-copy-amount"]').exists()).toBe(false)
    await wrapper.setProps({ cryptoAmount: '0' })
    expect(wrapper.find('canvas').exists()).toBe(false)
  })

  it('preserves the quote on query failures, while a locally ended window hides payment controls', async () => {
    const wrapper = checkout({ queryFailed: true })
    expect(wrapper.get('[data-test="gateway-query-failed"]').text()).toContain('payment.gateway.queryFailed')
    expect(wrapper.find('[data-test="gateway-copy-amount"]').exists()).toBe(true)
    await wrapper.setProps({ remainingSeconds: 0 })
    expect(wrapper.get('[data-test="gateway-status"]').text()).toContain('payment.gateway.checkingExpiry')
    expect(wrapper.find('canvas').exists()).toBe(false)
    expect(wrapper.emitted('done')).toBeUndefined()
  })

  it('requires confirmation to cancel and closes stale dialogs when the payment state changes', async () => {
    const wrapper = checkout()
    await wrapper.get('[data-test="gateway-cancel"]').trigger('click')
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)
    await wrapper.setProps({ gatewayStatus: 'CONFIRMING' })
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(false)
    expect(wrapper.emitted('cancel')).toBeUndefined()
    await wrapper.setProps({ gatewayStatus: 'WAITING' })
    await wrapper.get('[data-test="gateway-cancel"]').trigger('click')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
  })

  it('keeps an already paid order in fulfillment when the application reports FAILED', () => {
    const wrapper = checkout({ orderStatus: 'FAILED', gatewayStatus: 'PAID' })
    expect(wrapper.get('[data-test="gateway-status"]').text()).toBe('payment.gateway.accepted')
    expect(wrapper.text()).not.toContain('payment.gateway.rejected')
    expect(wrapper.find('[data-test="gateway-copy-amount"]').exists()).toBe(false)
  })
})
