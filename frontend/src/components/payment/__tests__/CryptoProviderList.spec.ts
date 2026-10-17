import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PaymentProviderList from '../PaymentProviderList.vue'
import ProviderCard from '../ProviderCard.vue'
import PaymentMethodSelector from '../PaymentMethodSelector.vue'
import ToggleSwitch from '../ToggleSwitch.vue'
import type { ProviderInstance } from '@/types/payment'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-draggable-plus', () => ({ VueDraggable: { template: '<div><slot /></div>' } }))

const provider: ProviderInstance = { id: 42, provider_key: 'bepusdt', name: 'USDT Gateway', config: {}, supported_types: ['usdt_gateway'], enabled: false, payment_mode: '', refund_enabled: false, allow_user_refund: false, limits: '', sort_order: 0 }

describe('independent crypto provider controls', () => {
  it('keeps ToggleSwitch stacked by default and opts ProviderCard into mobile inline layout', () => {
    const toggle = mount(ToggleSwitch, { props: { label: 'Enabled', checked: true } })
    expect(toggle.find('label').classes()).toContain('flex-col')

    const wrapper = mount(ProviderCard, {
      props: { provider: { ...provider, provider_key: 'stripe', refund_enabled: true }, enabled: true, availableTypes: [] },
      global: { stubs: { Icon: true } },
    })
    expect(wrapper.findAllComponents(ToggleSwitch).map(item => item.props('layout'))).toEqual(['inline', 'inline', 'inline'])
    wrapper.unmount()
  })

  it('permits gateway controls when the gateway payment type is enabled', () => {
    const wrapper = mount(PaymentProviderList, { props: { providers: [provider], loading: false, canCreate: true, enabledPaymentTypes: ['usdt_gateway'], allPaymentTypes: [], redirectLabel: '' }, global: { stubs: { Icon: true } } })
    expect(wrapper.findComponent(ProviderCard).props('enabled')).toBe(true)
    expect(wrapper.text()).not.toContain('admin.settings.payment.refundEnabled')
    wrapper.unmount()
  })

  it('offers a readonly connection action and disables it while checking', async () => {
    const wrapper = mount(ProviderCard, { props: { provider, enabled: true, availableTypes: [], checking: false }, global: { stubs: { Icon: true } } })
    const button = wrapper.findAll('button').find(item => item.text().includes('admin.settings.payment.gatewayCheck'))!
    await button.trigger('click')
    expect(wrapper.emitted('check')).toHaveLength(1)
    await wrapper.setProps({ checking: true })
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.emitted('toggleField')).toBeUndefined()
    wrapper.unmount()
  })

  it('keeps the old BSC and new gateway as separate user choices', async () => {
    const wrapper = mount(PaymentMethodSelector, { props: { methods: [{ type: 'usdt_bsc', fee_rate: 0, available: true }, { type: 'usdt_gateway', fee_rate: 0, available: true }], selected: 'usdt_gateway' } })
    expect(wrapper.findAll('button')).toHaveLength(2)
    await wrapper.findAll('button')[0].trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual(['usdt_bsc'])
    await wrapper.findAll('button')[1].trigger('click')
    expect(wrapper.emitted('select')?.[1]).toEqual(['usdt_gateway'])
  })
})
