import { describe, expect, it, vi } from 'vitest'
import { clearCryptoPaymentIntent, cryptoPaymentRequestId, cryptoTransactionUrl, gatewaySubscriptionPrice, supportedCryptoNetworks } from '../cryptoGateway'
import { buildCreateOrderPayload, decidePaymentLaunch } from '../paymentFlow'

describe('crypto gateway boundaries', () => {
  it('generates a cryptographic UUID v4 on HTTP without randomUUID and reuses it on retry', () => {
    const storage = window.localStorage
    storage.clear()
    const getRandomValues = vi.fn((bytes: Uint8Array) => {
      bytes.set([0, 1, 2, 3, 4, 5, 255, 7, 255, 9, 10, 11, 12, 13, 14, 15])
      return bytes
    })
    vi.stubGlobal('crypto', { randomUUID: undefined, getRandomValues })
    try {
      const intent = { amount: 12, orderType: 'balance', network: 'bsc_mainnet' as const }
      const first = cryptoPaymentRequestId(storage, 8, intent)
      expect(first).toBe('00010203-0405-4f07-bf09-0a0b0c0d0e0f')
      expect(cryptoPaymentRequestId(storage, 8, intent)).toBe(first)
      expect(getRandomValues).toHaveBeenCalledTimes(1)
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('converts only USD subscriptions using an explicit rate and preserves CNY prices', () => {
    expect(gatewaySubscriptionPrice(12, 'USD', 7.2)).toBeCloseTo(86.4)
    expect(gatewaySubscriptionPrice(12, '', 7.2)).toBeCloseTo(86.4)
    expect(gatewaySubscriptionPrice(12, 'CNY')).toBe(12)
    expect(gatewaySubscriptionPrice(12, 'USD')).toBeNull()
    expect(gatewaySubscriptionPrice(12, 'EUR', 7.2)).toBeNull()
    expect(gatewaySubscriptionPrice(12, 'USD', Infinity)).toBeNull()
  })
  it('only exposes known networks from server capabilities', () => {
    expect(supportedCryptoNetworks(['tron_mainnet', 'evil', 'bsc_mainnet', 'tron_mainnet'])).toEqual(['bsc_mainnet', 'tron_mainnet'])
    expect(supportedCryptoNetworks(undefined)).toEqual([])
  })

  it('preserves a per-user request ID across retries and rotates for a new intent', () => {
    const storage = window.localStorage
    storage.clear()
    const generate = () => '11111111-1111-4111-8111-111111111111'
    const intent = { amount: 12, orderType: 'balance', network: 'tron_mainnet' as const }
    const first = cryptoPaymentRequestId(storage, 8, intent, generate)
    expect(cryptoPaymentRequestId(storage, 8, intent, () => { throw new Error('must reuse') })).toBe(first)
    expect(cryptoPaymentRequestId(storage, 9, intent, () => '22222222-2222-4222-8222-222222222222')).not.toBe(first)
    expect(cryptoPaymentRequestId(storage, 8, { ...intent, amount: 13 }, () => '33333333-3333-4333-8333-333333333333')).not.toBe(first)
    clearCryptoPaymentIntent(storage, 8)
    expect(storage.getItem('payment.crypto.intent.8')).toBeNull()
  })

  it('sends network and request ID only for the gateway and enters recovery without a QR response', () => {
    const input = { amount: 12, paymentType: 'usdt_gateway', orderType: 'balance' as const, isMobile: false, isWechatBrowser: false, cryptoNetwork: 'tron_mainnet' as const, clientRequestId: '11111111-1111-4111-8111-111111111111' }
    expect(buildCreateOrderPayload(input)).toMatchObject({ payment_type: 'usdt_gateway', crypto_network: 'tron_mainnet', client_request_id: input.clientRequestId })
    expect(buildCreateOrderPayload({ ...input, paymentType: 'usdt_bsc' })).not.toHaveProperty('crypto_network')
    const result = decidePaymentLaunch({ order_id: 8, amount: 12, pay_amount: 12, fee_rate: 0, expires_at: '2099-01-01T00:00:00Z', create_stage: 'unknown' }, { visibleMethod: 'usdt_gateway', orderType: 'balance', isMobile: false })
    expect(result.kind).toBe('qr_waiting')
    expect(result.paymentState.createStage).toBe('unknown')
  })

  it('creates explorer URLs only for validated network-specific hashes', () => {
    expect(cryptoTransactionUrl('a'.repeat(64), 'tron_mainnet')).toBe(`https://tronscan.org/#/transaction/${'a'.repeat(64)}`)
    expect(cryptoTransactionUrl(`0x${'a'.repeat(64)}`, 'bsc_mainnet')).toContain('bscscan.com')
    expect(cryptoTransactionUrl('javascript:alert(1)', 'tron_mainnet')).toBe('')
    expect(cryptoTransactionUrl('a'.repeat(64), 'bsc_mainnet')).toBe('')
  })
})
