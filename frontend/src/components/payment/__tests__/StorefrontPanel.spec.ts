import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import StorefrontPanel from '../StorefrontPanel.vue'

const redeem = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const fetchActiveSubscriptions = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showWarning = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }),
}))

vi.mock('@/api', () => ({ redeemAPI: { redeem } }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showSuccess, showWarning }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ fetchActiveSubscriptions }) }))

const IconStub = defineComponent({
  props: { name: String },
  setup(props) {
    return () => h('span', { 'data-icon': props.name })
  },
})

const channels = [
  { id: 'backup-1', name: '备用一', url: 'https://shop.example.com/', enabled: true, sort_order: 1 },
  { id: 'backup-2', name: '备用二', url: 'https://pay.example.com/shop/2', enabled: true, sort_order: 2 },
]

function mountPanel(props: Record<string, unknown> = {}) {
  return mount(StorefrontPanel, {
    props: { channels, ...props },
    global: { stubs: { Icon: IconStub } },
  })
}

describe('StorefrontPanel', () => {
  beforeEach(() => {
    redeem.mockReset()
    refreshUser.mockReset().mockResolvedValue(undefined)
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    showSuccess.mockReset()
    showWarning.mockReset()
  })

  it('keeps every channel iframe mounted and shows only the active one', () => {
    const wrapper = mountPanel({ channelId: 'backup-2' })
    const frames = wrapper.findAll('iframe')
    expect(frames.map((frame) => frame.attributes('src'))).toEqual(channels.map((channel) => channel.url))
    const containers = wrapper.findAll('.sf-frame-container')
    expect(containers[0].attributes('style')).toContain('display: none')
    expect(containers[1].attributes('style') ?? '').not.toContain('display: none')
  })

  it('emits the selected channel id from the chips', async () => {
    const wrapper = mountPanel({ channelId: 'backup-1' })
    await wrapper.findAll('.sf-chip')[1].trigger('click')
    expect(wrapper.emitted('update:channelId')?.[0]).toEqual(['backup-2'])
  })

  it('hides the chips when only one channel is configured', () => {
    const wrapper = mountPanel({ channels: [channels[0]] })
    expect(wrapper.findAll('.sf-chip')).toHaveLength(0)
    expect(wrapper.text()).toContain('备用一')
  })

  it('redeems a code, refreshes the user and clears the input', async () => {
    redeem.mockResolvedValue({ message: 'ok', type: 'balance', value: 100, new_balance: 200 })
    const wrapper = mountPanel()
    await wrapper.get('[data-testid="storefront-code-input"]').setValue('  CARD-123  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(redeem).toHaveBeenCalledWith('CARD-123')
    expect(refreshUser).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()
    expect((wrapper.get('[data-testid="storefront-code-input"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-testid="storefront-redeem-result"]').text()).toContain('payment.storefront.redeemedBalance')
  })

  it('shows the server error inline when redemption fails', async () => {
    redeem.mockRejectedValue({ response: { data: { detail: '卡密已被使用' } } })
    const wrapper = mountPanel()
    await wrapper.get('[data-testid="storefront-code-input"]').setValue('USED')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(refreshUser).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="storefront-redeem-result"]').text()).toContain('卡密已被使用')
    expect((wrapper.get('[data-testid="storefront-code-input"]').element as HTMLInputElement).value).toBe('USED')
  })
})
