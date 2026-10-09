import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TelegramPaymentSettings from '../TelegramPaymentSettings.vue'

const api = vi.hoisted(() => ({
  getTelegramSettings: vi.fn(),
  updateTelegramSettings: vi.fn(),
  testTelegramSettings: vi.fn()
}))
const notifications = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin/telegram', () => api)
vi.mock('@/stores', () => ({ useAppStore: () => notifications }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

function mountSettings() {
  return mount(TelegramPaymentSettings, {
    global: { stubs: { Icon: true, ConfirmDialog: true } }
  })
}

function state(overrides = {}) {
  return { enabled: true, verify_easypay_orders: false, bot_token_configured: true, chat_id: '-10012', ...overrides }
}

describe('TelegramPaymentSettings', () => {
  beforeEach(() => {
    Object.values(api).forEach(mock => mock.mockReset())
    Object.values(notifications).forEach(mock => mock.mockReset())
    api.getTelegramSettings.mockResolvedValue(state())
    api.updateTelegramSettings.mockImplementation(async request => state({
      enabled: request.enabled,
      verify_easypay_orders: request.verify_easypay_orders,
      chat_id: request.chat_id,
      bot_token_configured: !request.clear_bot_token
    }))
    api.testTelegramSettings.mockResolvedValue({ message: 'sent' })
  })

  it('shows saved configuration status without filling a token and preserves it on ordinary save', async () => {
    const wrapper = mountSettings()
    await flushPromises()

    const token = wrapper.get('#telegram-bot-token')
    expect(token.attributes('type')).toBe('password')
    expect((token.element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-testid="telegram-token-status"]').text()).toContain('configured')
    await wrapper.get('#telegram-chat-id').setValue('  -10034  ')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    await flushPromises()

    expect(api.updateTelegramSettings).toHaveBeenCalledWith({ enabled: true, verify_easypay_orders: false, chat_id: '-10034' })
    expect(api.updateTelegramSettings.mock.calls[0][0]).not.toHaveProperty('bot_token')
    expect(api.updateTelegramSettings.mock.calls[0][0]).not.toHaveProperty('clear_bot_token')
  })

  it('replaces the token only when entered and clears it after a successful save', async () => {
    const wrapper = mountSettings()
    await flushPromises()
    await wrapper.get('#telegram-bot-token').setValue('  123:write-only-test-token  ')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    await flushPromises()

    expect(api.updateTelegramSettings).toHaveBeenCalledWith({
      enabled: true, verify_easypay_orders: false, chat_id: '-10012', bot_token: '123:write-only-test-token'
    })
    expect((wrapper.get('#telegram-bot-token').element as HTMLInputElement).value).toBe('')
    expect(wrapper.html()).not.toContain('write-only-test-token')
  })

  it('sends a test using unsaved inputs without saving or discarding the draft', async () => {
    const wrapper = mountSettings()
    await flushPromises()
    await wrapper.get('#telegram-bot-token').setValue('123:unsaved-test-token')
    await wrapper.get('#telegram-chat-id').setValue('-10056')
    await wrapper.get('[data-testid="telegram-enabled"]').trigger('click')
    await wrapper.get('[data-testid="telegram-test"]').trigger('click')
    await flushPromises()

    expect(api.testTelegramSettings).toHaveBeenCalledWith({
      enabled: false, verify_easypay_orders: false, chat_id: '-10056', bot_token: '123:unsaved-test-token'
    })
    expect(api.updateTelegramSettings).not.toHaveBeenCalled()
    expect((wrapper.get('#telegram-bot-token').element as HTMLInputElement).value).toBe('123:unsaved-test-token')
    expect(notifications.showSuccess).toHaveBeenCalledWith('admin.settings.telegram.testSent')
  })

  it('requires delivery credentials when enabling or testing, but permits saving disabled empty settings', async () => {
    api.getTelegramSettings.mockResolvedValue(state({ enabled: false, bot_token_configured: false, chat_id: '' }))
    const wrapper = mountSettings()
    await flushPromises()
    await wrapper.get('[data-testid="telegram-test"]').trigger('click')
    expect(api.testTelegramSettings).not.toHaveBeenCalled()
    expect(notifications.showError).toHaveBeenCalledWith('admin.settings.telegram.tokenRequired')

    await wrapper.get('[data-testid="telegram-enabled"]').trigger('click')
    await wrapper.get('#telegram-bot-token').setValue('123:test-token')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    expect(api.updateTelegramSettings).not.toHaveBeenCalled()
    expect(notifications.showError).toHaveBeenLastCalledWith('admin.settings.telegram.chatIdRequired')

    await wrapper.get('[data-testid="telegram-enabled"]').trigger('click')
    await wrapper.get('#telegram-bot-token').setValue('')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    await flushPromises()
    expect(api.updateTelegramSettings).toHaveBeenCalledWith({ enabled: false, verify_easypay_orders: false, chat_id: '' })
  })

  it('prevents writing unloaded defaults after a failed GET and can reload saved values', async () => {
    api.getTelegramSettings.mockRejectedValueOnce(new Error('temporarily unavailable'))
    const wrapper = mountSettings()
    await flushPromises()
    expect(wrapper.find('[data-testid="telegram-save"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="telegram-test"]').exists()).toBe(false)
    expect(api.updateTelegramSettings).not.toHaveBeenCalled()

    api.getTelegramSettings.mockResolvedValueOnce(state({ chat_id: '-10098' }))
    await wrapper.get('[data-testid="telegram-retry"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('#telegram-chat-id').element as HTMLInputElement).value).toBe('-10098')
    expect(wrapper.get('[data-testid="telegram-enabled"]').attributes('aria-checked')).toBe('true')
  })

  it('clears a saved token only after confirmation and disables notifications', async () => {
    api.getTelegramSettings.mockResolvedValue(state({ verify_easypay_orders: true }))
    const wrapper = mountSettings()
    await flushPromises()
    await wrapper.get('#telegram-chat-id').setValue('-100-unsaved')
    await wrapper.get('[data-testid="telegram-clear-token"]').trigger('click')
    const confirm = wrapper.getComponent({ name: 'ConfirmDialog' })
    expect(confirm.props('show')).toBe(true)
    expect(api.updateTelegramSettings).not.toHaveBeenCalled()
    confirm.vm.$emit('cancel')
    await flushPromises()
    expect(confirm.props('show')).toBe(false)
    expect(api.updateTelegramSettings).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="telegram-clear-token"]').trigger('click')
    confirm.vm.$emit('confirm')
    await flushPromises()
    expect(api.updateTelegramSettings).toHaveBeenCalledWith({
      enabled: false, verify_easypay_orders: true, chat_id: '-10012', clear_bot_token: true
    })
    expect(wrapper.find('[data-testid="telegram-clear-token"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="telegram-enabled"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[data-testid="telegram-verify-easypay"]').attributes('aria-checked')).toBe('true')
  })

  it('saves and preserves provider verification independently from Telegram delivery', async () => {
    api.getTelegramSettings.mockResolvedValue(state({ enabled: false, bot_token_configured: false, chat_id: '' }))
    const wrapper = mountSettings()
    await flushPromises()
    expect(wrapper.get('[data-testid="telegram-verify-easypay"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[data-testid="telegram-verify-easypay"]').trigger('click')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    await flushPromises()
    expect(api.updateTelegramSettings).toHaveBeenCalledWith({
      enabled: false, verify_easypay_orders: true, chat_id: ''
    })
    expect(wrapper.get('[data-testid="telegram-verify-easypay"]').attributes('aria-checked')).toBe('true')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    await flushPromises()
    expect(api.updateTelegramSettings).toHaveBeenLastCalledWith({
      enabled: false, verify_easypay_orders: true, chat_id: ''
    })
  })

  it('blocks competing actions while saving and reports failures without exposing a raw error or discarding input', async () => {
    let rejectSave!: (reason: Error) => void
    api.updateTelegramSettings.mockImplementation(() => new Promise((_resolve, reject) => { rejectSave = reject }))
    const wrapper = mountSettings()
    await flushPromises()
    await wrapper.get('#telegram-bot-token').setValue('123:private-draft-token')
    await wrapper.get('[data-testid="telegram-save"]').trigger('click')
    expect(wrapper.get('[data-testid="telegram-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="telegram-test"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="telegram-enabled"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="telegram-test"]').trigger('click')
    expect(api.testTelegramSettings).not.toHaveBeenCalled()

    rejectSave(new Error('raw error includes 123:private-draft-token'))
    await flushPromises()
    expect(notifications.showError).toHaveBeenCalledWith('admin.settings.telegram.saveFailed')
    expect((wrapper.get('#telegram-bot-token').element as HTMLInputElement).value).toBe('123:private-draft-token')
    expect(wrapper.get('[data-testid="telegram-save"]').attributes('disabled')).toBeUndefined()
  })

  it('uses explicit button actions inside the outer settings form', async () => {
    const wrapper = mountSettings()
    await flushPromises()
    for (const testId of ['telegram-save', 'telegram-test', 'telegram-clear-token']) {
      expect(wrapper.get(`[data-testid="${testId}"]`).attributes('type')).toBe('button')
    }
  })
})
