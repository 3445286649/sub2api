import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import PaymentProviderDialog from '@/components/payment/PaymentProviderDialog.vue'
import { STRIPE_SDK_API_VERSION } from '@/components/payment/providerConfig'
import type { ProviderInstance } from '@/types/payment'

const messages: Record<string, string> = {
  'admin.settings.payment.providerConfig': 'Credentials',
  'admin.settings.payment.easypayCustomMethods': 'Custom EasyPay methods',
  'admin.settings.payment.easypayCustomMethodsHint': 'Add provider-specific EasyPay type values.',
  'admin.settings.payment.addCustomMethod': 'Add method',
  'admin.settings.payment.customMethodType': 'Payment type',
  'admin.settings.payment.customMethodUpstreamType': 'Upstream type',
  'admin.settings.payment.customMethodDisplayName': 'Display name',
  'admin.settings.payment.customMethodDisplayNamePlaceholder': '信用卡',
  'admin.settings.payment.paymentGuideTrigger': 'View payment guide',
  'admin.settings.payment.alipayGuideSummary': 'Desktop prefers QR precreate and falls back to cashier; mobile prefers WAP checkout.',
  'admin.settings.payment.wxpayGuideSummary': 'Desktop prefers Native QR; mobile routes to JSAPI or H5 based on browser context.',
  'admin.settings.payment.airwallexGuideSummary': 'Use Payment Acceptance read/write only.',
  'admin.settings.payment.stripeWebhookHint': 'Configure Stripe webhook.',
  'admin.settings.payment.stripeWebhookApiVersionHint': 'Use Stripe API version {version}.',
  'admin.settings.payment.airwallexWebhookHint': 'Select payment_intent.succeeded and use the latest stable API version.',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      const message = messages[key] ?? key
      if (!params) return message
      return Object.entries(params).reduce(
        (value, [name, replacement]) => value.replaceAll(`{${name}}`, replacement),
        message,
      )
    },
  }),
}))

function providerFactory(overrides: Partial<ProviderInstance> = {}): ProviderInstance {
  return {
    id: 1,
    provider_key: 'airwallex',
    name: 'Airwallex',
    config: {},
    supported_types: ['airwallex'],
    enabled: true,
    payment_mode: '',
    refund_enabled: false,
    allow_user_refund: false,
    limits: '',
    sort_order: 0,
    ...overrides,
  }
}

function mountDialog(options: { editing?: ProviderInstance | null } = {}) {
  return mount(PaymentProviderDialog, {
    props: {
      show: true,
      saving: false,
      editing: options.editing ?? null,
      allKeyOptions: [
        { value: 'easypay', label: 'EasyPay' },
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' },
        { value: 'stripe', label: 'Stripe' },
        { value: 'airwallex', label: 'Airwallex' },
      ],
      enabledKeyOptions: [
        { value: 'easypay', label: 'EasyPay' },
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' },
        { value: 'airwallex', label: 'Airwallex' },
      ],
      allPaymentTypes: [
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' },
      ],
      redirectLabel: 'Redirect',
    },
    global: {
      stubs: {
        BaseDialog: {
          template: '<div><slot /><slot name="footer" /></div>',
        },
        Select: {
          props: ['modelValue', 'options', 'disabled'],
          template: '<div />',
        },
        ToggleSwitch: {
          props: ['label', 'checked'],
          emits: ['toggle'],
          template: '<button type="button" role="switch" :aria-checked="checked" @click="$emit(\'toggle\')">{{ label }}</button>',
        },
      },
    },
  })
}

describe('PaymentProviderDialog callback URLs', () => {
  it('defaults JianPay to Alipay only and preserves stored secrets when editing', async () => {
    const wrapper = mountDialog()
    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset('jianpay')
    await nextTick()
    expect(wrapper.findAll('button').filter(button => button.classes().includes('bg-primary-500')).map(button => button.text())).toEqual(['Alipay'])
    expect(wrapper.text()).not.toContain('admin.settings.payment.refundEnabled')
    const provider = providerFactory({ provider_key: 'jianpay', name: '简付', supported_types: ['alipay'], config: {
      clientNo: 'JP-test', merchantKey: 'must-not-populate-secret',
      notifyUrl: 'https://site.test/api/v1/payment/webhook/jianpay', returnUrl: 'https://site.test/payment/result',
    } })
    await wrapper.setProps({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (value: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    expect(wrapper.findAll('input[type="password"]').map(input => (input.element as HTMLInputElement).value)).toEqual([''])
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/jianpay')
    await wrapper.get('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as ProviderInstance
    expect(payload.provider_key).toBe('jianpay')
    expect(payload.supported_types).toEqual(['alipay'])
    expect(payload.config.apiBase).toBe('https://jpay.hzjianban.com')
    expect(payload.config.verifyAfterCallback).toBe('true')
    expect(payload.config).not.toHaveProperty('merchantKey')
    expect(payload.refund_enabled).toBe(false)
    expect(payload.allow_user_refund).toBe(false)
    wrapper.unmount()
  })
  it('saves and restores the independent JianPay auto-query switch', async () => {
    const provider = providerFactory({ provider_key: 'jianpay', name: '简付', supported_types: ['alipay'], config: {
      clientNo: 'JP-test', verifyAfterCallback: 'false',
      notifyUrl: 'https://site.test/api/v1/payment/webhook/jianpay', returnUrl: 'https://site.test/payment/result',
    } })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (value: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    const toggle = wrapper.get('[data-test="jianpay-auto-query-switch"]')
    expect(toggle.attributes('aria-checked')).toBe('false')
    expect(wrapper.find('[data-test="fm-auto-query-switch"]').exists()).toBe(false)
    await toggle.trigger('click')
    expect(toggle.attributes('aria-checked')).toBe('true')
    await wrapper.get('form').trigger('submit')
    expect((wrapper.emitted('save')?.[0]?.[0] as ProviderInstance).config.verifyAfterCallback).toBe('true')
    await toggle.trigger('click')
    await wrapper.get('form').trigger('submit')
    expect((wrapper.emitted('save')?.[1]?.[0] as ProviderInstance).config.verifyAfterCallback).toBe('false')
    wrapper.unmount()
  })
  it('saves the FM auto-query switch in the provider config and restores it when editing', async () => {
    const provider = providerFactory({ provider_key: 'zhifufm', name: 'FM', config: {
      pid: '123', apiBase: 'https://fm.example/epay', queryBase: 'https://fm.example/api/pay', verifyAfterCallback: 'false',
      notifyUrl: 'https://codeplan.codes/api/v1/payment/webhook/zhifufm', returnUrl: 'https://codeplan.codes/payment/result',
    } })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (value: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    const toggle = wrapper.get('[data-test="fm-auto-query-switch"]')
    expect(toggle.attributes('aria-checked')).toBe('false')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-checked')).toBe('true')
    await wrapper.get('form').trigger('submit')
    expect((wrapper.emitted('save')?.[0]?.[0] as ProviderInstance).config.verifyAfterCallback).toBe('true')
    await toggle.trigger('click')
    await wrapper.get('form').trigger('submit')
    expect((wrapper.emitted('save')?.[1]?.[0] as ProviderInstance).config.verifyAfterCallback).toBe('false')
    wrapper.unmount()
  })
  it('configures FM independently with native query, visible methods and no refunds', async () => {
    const provider = providerFactory({ provider_key: 'zhifufm', name: 'FM', supported_types: ['alipay', 'wxpay'], config: {
      pid: '123', pkey: 'must-not-populate-secret', apiBase: 'https://fm.example/epay', queryBase: 'https://fm.example/api/pay',
      notifyUrl: 'https://codeplan.codes/api/v1/payment/webhook/zhifufm', returnUrl: 'https://codeplan.codes/payment/result',
    } })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (value: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    expect(wrapper.findAll('input[type="password"]').map(input => (input.element as HTMLInputElement).value)).toEqual([''])
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/zhifufm')
    expect(wrapper.text()).not.toContain('admin.settings.payment.refundEnabled')
    await wrapper.get('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as ProviderInstance
    expect(payload.provider_key).toBe('zhifufm')
    expect(payload.supported_types).toEqual(['alipay', 'wxpay'])
    expect(payload.config.queryBase).toBe('https://fm.example/api/pay')
    expect(payload.config.verifyAfterCallback).toBe('true')
    expect(payload.config).not.toHaveProperty('pkey')
    expect(payload.refund_enabled).toBe(false)
    wrapper.unmount()
  })
  it('edits gateway networks without exposing stored secrets or enabling refunds', async () => {
    const provider = providerFactory({ provider_key: 'bepusdt', name: 'Gateway', supported_types: ['usdt_gateway'], refund_enabled: true, allow_user_refund: true, config: {
      apiUrl: 'https://gateway.example.com', merchantId: 'merchant', keyId: 'key', allowedNetworks: 'tron_mainnet', apiSecret: 'must-never-fill-api-secret', callbackSecret: 'must-never-fill-notify-secret',
    } })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (value: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    expect(wrapper.findAll('input[type="password"]').map(input => (input.element as HTMLInputElement).value)).toEqual(['', ''])
    expect(wrapper.text()).not.toContain('must-never-fill')
    expect(wrapper.text()).not.toContain('admin.settings.payment.refundEnabled')
    await wrapper.get('[data-network="bsc_mainnet"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as { config: Record<string, string>; refund_enabled: boolean; allow_user_refund: boolean }
    expect(payload.config.allowedNetworks).toBe('bsc_mainnet,tron_mainnet')
    expect(payload.config).not.toHaveProperty('apiSecret')
    expect(payload.config).not.toHaveProperty('callbackSecret')
    expect(payload.refund_enabled).toBe(false)
    expect(payload.allow_user_refund).toBe(false)
    wrapper.unmount()
  })
  it.each([
    ['https://notify.example.com/', 'https://return.example.com///', 'https://notify.example.com', 'https://return.example.com'],
    [' https://notify.example.com/sub/ ', ' https://return.example.com/site/ ', 'https://notify.example.com/sub', 'https://return.example.com/site'],
    ['https://notify.example.com', 'https://return.example.com', 'https://notify.example.com', 'https://return.example.com'],
    ['', '', window.location.origin, window.location.origin],
  ])('joins callback paths to %s and %s', async (notify, returnUrl, expectedNotify, expectedReturn) => {
    const provider = providerFactory({
      provider_key: 'easypay', name: 'EasyPay',
      config: { pid: 'pid-1', apiBase: 'https://pay.example.com' },
      supported_types: ['alipay'], payment_mode: 'qrcode',
    })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    const bases = wrapper.findAll('input').filter(input => input.classes().includes('!rounded-r-none'))
    await bases[0].setValue(notify)
    await bases[1].setValue(returnUrl)
    await wrapper.find('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as { config: Record<string, string> }
    expect(payload.config.notifyUrl).toBe(expectedNotify + '/api/v1/payment/webhook/easypay')
    expect(payload.config.returnUrl).toBe(expectedReturn + '/payment/result')
    wrapper.unmount()
  })
})

describe('PaymentProviderDialog payment guide', () => {
  it('shows no payment guide for providers without a flow guide', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).not.toContain(messages['admin.settings.payment.alipayGuideSummary'])
    expect(wrapper.text()).not.toContain(messages['admin.settings.payment.wxpayGuideSummary'])
    expect(wrapper.find('button[title="View payment guide"]').exists()).toBe(false)
  })

  it.each([
    ['alipay', 'admin.settings.payment.alipayGuideSummary'],
    ['wxpay', 'admin.settings.payment.wxpayGuideSummary'],
    ['airwallex', 'admin.settings.payment.airwallexGuideSummary'],
  ])('shows the payment guide summary for %s', async (providerKey, summaryKey) => {
    const wrapper = mountDialog()

    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset(providerKey)
    await nextTick()

    expect(wrapper.text()).toContain(messages[summaryKey])
    expect(wrapper.find('button[title="View payment guide"]').exists()).toBe(true)
  })

  it('shows Airwallex webhook event and API version guidance with the webhook URL', async () => {
    const wrapper = mountDialog()

    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset('airwallex')
    await nextTick()

    expect(wrapper.text()).toContain(messages['admin.settings.payment.airwallexWebhookHint'])
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/airwallex')
  })

  it('shows Stripe webhook API version guidance with the integrated SDK version', async () => {
    const wrapper = mountDialog()

    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset('stripe')
    await nextTick()

    expect(wrapper.text()).toContain(messages['admin.settings.payment.stripeWebhookHint'])
    expect(wrapper.text()).toContain(`Use Stripe API version ${STRIPE_SDK_API_VERSION}.`)
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/stripe')
  })

  it('emits an empty Airwallex accountId when the admin clears it', async () => {
    const provider = providerFactory({
      config: {
        clientId: 'cid_123',
        apiBase: 'https://api.airwallex.com/api/v1',
        countryCode: 'CN',
        currency: 'CNY',
        accountId: 'acct_123',
      },
    })
    const wrapper = mountDialog({ editing: provider })

    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()

    const accountIdInput = wrapper
      .findAll('input[type="text"]')
      .find(input => (input.element as HTMLInputElement).value === 'acct_123')
    if (!accountIdInput) throw new Error('accountId input not found')

    await accountIdInput.setValue('')
    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as { config: Record<string, string> }
    expect(payload.config.accountId).toBe('')
  })

  it.each(['epay', 'usdt.trc20'])('serializes EasyPay upstream type %s and adds the local type to supported_types', async (upstreamType) => {
    const provider = providerFactory({
      provider_key: 'easypay',
      name: 'EasyPay',
      config: {
        pid: 'pid-1',
        apiBase: 'https://pay.example.com',
        notifyUrl: 'https://example.com/api/v1/payment/webhook/easypay',
        returnUrl: 'https://example.com/payment/result',
      },
      supported_types: ['alipay', 'wxpay'],
      payment_mode: 'qrcode',
    })
    const wrapper = mountDialog({ editing: provider })

    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()

    await wrapper.find('button.btn-sm').trigger('click')
    await nextTick()

    const inputs = wrapper.findAll('input[type="text"]')
    const customTypeInputs = inputs.filter(input => (input.element as HTMLInputElement).placeholder === 'credit_card')
    const ldcTypeInput = customTypeInputs[0]
    const upstreamTypeInput = customTypeInputs[1]
    const displayNameInput = inputs.find(input => (input.element as HTMLInputElement).placeholder === '信用卡')
    if (!ldcTypeInput || !upstreamTypeInput || !displayNameInput) {
      throw new Error('custom method inputs not found')
    }

    await ldcTypeInput.setValue('ldc')
    await upstreamTypeInput.setValue(upstreamType)
    await displayNameInput.setValue('LDC')
    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as {
      config: Record<string, string>
      supported_types: string[]
    }
    expect(JSON.parse(payload.config.customMethods)).toEqual([{ type: 'ldc', upstreamType, displayName: 'LDC' }])
    expect(payload.supported_types).toEqual(['alipay', 'wxpay', 'ldc'])
  })

  it.each([
    ['alipay_hk', 'hkpay'],
    ['usdt.trc20', 'usdt.trc20'],
    ['usdt_trc20', 'usdt/trc20'],
  ])('rejects invalid EasyPay mapping %s to %s', async (type, upstreamType) => {
    const provider = providerFactory({
      provider_key: 'easypay',
      name: 'EasyPay',
      config: {
        pid: 'pid-1',
        apiBase: 'https://pay.example.com',
        notifyUrl: 'https://example.com/api/v1/payment/webhook/easypay',
        returnUrl: 'https://example.com/payment/result',
      },
      supported_types: ['alipay', 'wxpay'],
      payment_mode: 'qrcode',
    })
    const wrapper = mountDialog({ editing: provider })

    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()

    await wrapper.find('button.btn-sm').trigger('click')
    await nextTick()

    const inputs = wrapper.findAll('input[type="text"]')
    const customTypeInputs = inputs.filter(input => (input.element as HTMLInputElement).placeholder === 'credit_card')
    const typeInput = customTypeInputs[0]
    const upstreamTypeInput = customTypeInputs[1]
    const displayNameInput = inputs.find(input => (input.element as HTMLInputElement).placeholder === '信用卡')
    if (!typeInput || !upstreamTypeInput || !displayNameInput) {
      throw new Error('custom method inputs not found')
    }

    await typeInput.setValue(type)
    await upstreamTypeInput.setValue(upstreamType)
    await displayNameInput.setValue('Custom payment')
    await wrapper.find('form').trigger('submit.prevent')

    expect(wrapper.emitted('save')).toBeUndefined()
  })
})
