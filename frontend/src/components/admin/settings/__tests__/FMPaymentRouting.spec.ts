import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import FMPaymentRouting from '../FMPaymentRouting.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { ProviderInstance } from '@/types/payment'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: 'zh' }) }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getConfig: vi.fn(), updateConfig: vi.fn() } }))
vi.mock('@/api/client', () => ({ apiClient: {} }))

describe('FM payment routing', () => {
  it('offers JianPay without FM and keeps unavailable WeChat hidden', async () => {
    vi.mocked(adminPaymentAPI.getConfig).mockResolvedValue({ data: { payment_visible_method_alipay_source: 'jianpay_alipay', payment_visible_method_wxpay_source: '' } } as never)
    vi.mocked(adminPaymentAPI.updateConfig).mockResolvedValue({} as never)
    const providers = [{ provider_key: 'jianpay', enabled: true, supported_types: ['alipay'] }] as ProviderInstance[]
    const wrapper = mount(FMPaymentRouting, { props: { providers } })
    await flushPromises()
    expect(wrapper.get('#fm-source-alipay').text()).toContain('简付 支付宝')
    expect(wrapper.get('#fm-source-wxpay').text()).not.toContain('简付 微信')
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.updateConfig).toHaveBeenLastCalledWith(expect.objectContaining({ payment_visible_method_alipay_source: 'jianpay_alipay' }))
    await wrapper.setProps({ providers: [{ ...providers[0], supported_types: ['alipay', 'wxpay'] }] as ProviderInstance[] })
    expect(wrapper.get('#fm-source-wxpay').text()).toContain('简付 微信')
    wrapper.unmount()
    vi.clearAllMocks()
  })
  it('loads without changing routing and saves only selected payment sources', async () => {
    vi.mocked(adminPaymentAPI.getConfig).mockResolvedValue({ data: { payment_visible_method_alipay_source: 'easypay_alipay', payment_visible_method_wxpay_source: 'zhifufm_wxpay' } } as never)
    vi.mocked(adminPaymentAPI.updateConfig).mockResolvedValue({} as never)
    const providers = [
      { provider_key: 'easypay', enabled: true, supported_types: ['alipay', 'wxpay'] },
      { provider_key: 'zhifufm', enabled: true, supported_types: ['alipay', 'wxpay'] },
    ] as ProviderInstance[]
    const wrapper = mount(FMPaymentRouting, { props: { providers } })
    await flushPromises()
    expect(adminPaymentAPI.updateConfig).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('支付FM 支付宝')
    await wrapper.get('#fm-source-alipay').setValue('zhifufm_alipay')
    await wrapper.get('form').trigger('submit')
    expect(adminPaymentAPI.updateConfig).toHaveBeenCalledWith({
      payment_visible_method_alipay_source: 'zhifufm_alipay', payment_visible_method_wxpay_source: 'zhifufm_wxpay',
      payment_visible_method_alipay_enabled: true, payment_visible_method_wxpay_enabled: true,
    })
    wrapper.unmount()
  })
})
