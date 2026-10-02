import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

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

import UsdtBscCheckout from '../UsdtBscCheckout.vue'
import { bscScanTxUrl, formatCountdown, formatUsdtRate, groupWalletAddress, isCryptoTxHash, shortenTxHash, splitUsdtAmount } from '../usdtBsc'

const address = '0x3b210bdc924c685fDd10Ae96b7f95D0E14536106'

const ConfirmDialogStub = {
  name: 'ConfirmDialog',
  props: ['show'],
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-test="confirm-dialog"><button data-test="confirm-ok" @click="$emit(\'confirm\')" /><button data-test="confirm-back" @click="$emit(\'cancel\')" /></div>',
}

function mountCheckout(overrides: Record<string, unknown> = {}) {
  return mount(UsdtBscCheckout, {
    props: {
      cryptoAmount: '10.056873',
      cryptoCurrency: 'USDT',
      cryptoRate: '7.160000',
      orderAmount: '¥72.00',
      receiveAddress: address,
      orderNo: 'sub2_20260420abcd1234',
      remainingSeconds: 1721,
      totalSeconds: 1800,
      showSupport: true,
      ...overrides,
    },
    global: {
      stubs: { Icon: true, RouterLink: { template: '<a data-test="support-link"><slot /></a>' }, ConfirmDialog: ConfirmDialogStub },
    },
  })
}

describe('usdtBsc helpers', () => {
  it('splits the amount after the second decimal', () => {
    expect(splitUsdtAmount('10.056873')).toEqual({ head: '10.05', tail: '6873' })
    expect(splitUsdtAmount(' 10.05 ')).toEqual({ head: '10.05', tail: '' })
    expect(splitUsdtAmount('10')).toEqual({ head: '10', tail: '' })
  })

  it('groups the address in fours and keeps the 0x prefix on the first group', () => {
    const groups = groupWalletAddress(address)
    expect(groups[0]).toBe('0x3b21')
    expect(groups.at(-1)).toBe('6106')
    expect(groups).toHaveLength(10)
    expect(groups.join('')).toBe(address)
  })

  it('formats countdown and rate', () => {
    expect(formatCountdown(1721)).toBe('28:41')
    expect(formatCountdown(-3)).toBe('00:00')
    expect(formatUsdtRate('7.160000')).toBe('7.16')
    expect(formatUsdtRate('')).toBe('')
    expect(formatUsdtRate('abc')).toBe('abc')
  })

  it('validates, shortens and links transaction hashes', () => {
    const hash = `0x${'ab'.repeat(30)}cdef`
    expect(isCryptoTxHash(hash)).toBe(true)
    expect(isCryptoTxHash(` ${hash} `)).toBe(true)
    expect(isCryptoTxHash('sub2_x|72.00|10.056873|7.16|0x3b21|1759300000')).toBe(false)
    expect(isCryptoTxHash(undefined)).toBe(false)
    expect(shortenTxHash(hash)).toBe('0xabababab…ababcdef')
    expect(bscScanTxUrl(hash)).toBe(`https://bscscan.com/tx/${hash}`)
  })
})

describe('UsdtBscCheckout', () => {
  beforeEach(() => {
    toCanvas.mockReset().mockResolvedValue(undefined)
    copyToClipboard.mockReset().mockResolvedValue(true)
  })

  it('highlights the amount tail and draws the address QR code', async () => {
    const wrapper = mountCheckout()
    await flushPromises()

    expect(wrapper.get('[data-test="usdt-amount-tail"]').text()).toBe('6873')
    expect(wrapper.get('[data-test="usdt-address"]').findAll('span')).toHaveLength(10)
    expect(wrapper.text()).toContain('28:41')
    expect(toCanvas).toHaveBeenCalledWith(expect.any(HTMLCanvasElement), address, expect.any(Object))
  })

  it('omits the highlighted tail when the amount has no extra decimals', () => {
    const wrapper = mountCheckout({ cryptoAmount: '10' })
    expect(wrapper.find('[data-test="usdt-amount-tail"]').exists()).toBe(false)
  })

  it('copies raw values and ticks the matching steps', async () => {
    const wrapper = mountCheckout()

    await wrapper.get('[data-test="copy-amount"]').trigger('click')
    await flushPromises()
    expect(copyToClipboard).toHaveBeenCalledWith('10.056873', 'payment.crypto.amountCopied')

    await wrapper.get('button.wallet-address-copy').trigger('click')
    await flushPromises()
    expect(copyToClipboard).toHaveBeenCalledWith(address, 'payment.crypto.addressCopied')

    const stepNums = wrapper.findAll('span.rounded-full.border-\\[1\\.5px\\]')
    expect(stepNums[0].classes()).toContain('bg-primary-500')
    expect(stepNums[1].classes()).toContain('bg-primary-500')
    expect(stepNums[2].classes()).toContain('ring-4')
  })

  it('does not tick a step when copying fails', async () => {
    copyToClipboard.mockResolvedValue(false)
    const wrapper = mountCheckout()

    await wrapper.get('[data-test="copy-amount"]').trigger('click')
    await flushPromises()

    const stepNums = wrapper.findAll('span.rounded-full.border-\\[1\\.5px\\]')
    expect(stepNums[0].classes()).not.toContain('bg-primary-500')
  })

  it('toggles the mobile QR code with aria-expanded', async () => {
    const wrapper = mountCheckout()
    const toggle = wrapper.get('[data-test="toggle-qr"]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('#usdt-qr').classes()).toContain('hidden')

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('#usdt-qr').classes()).not.toContain('hidden')
  })

  it('shows the urgent hint in the last five minutes', () => {
    expect(mountCheckout().find('[data-test="usdt-urgent"]').exists()).toBe(false)
    expect(mountCheckout({ remainingSeconds: 299 }).find('[data-test="usdt-urgent"]').exists()).toBe(true)
  })

  it('asks for confirmation before emitting cancel', async () => {
    const wrapper = mountCheckout()

    await wrapper.get('[data-test="usdt-cancel"]').trigger('click')
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)

    await wrapper.get('[data-test="confirm-back"]').trigger('click')
    expect(wrapper.emitted('cancel')).toBeUndefined()
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-test="usdt-cancel"]').trigger('click')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
  })

  it('hides the support link when support tickets are disabled', () => {
    expect(mountCheckout().find('[data-test="support-link"]').exists()).toBe(true)
    expect(mountCheckout({ showSupport: false }).find('[data-test="support-link"]').exists()).toBe(false)
  })
})
