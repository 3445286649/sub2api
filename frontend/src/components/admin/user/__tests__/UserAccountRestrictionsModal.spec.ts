import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { AdminUser } from '@/types'
import UserAccountRestrictionsModal from '../UserAccountRestrictionsModal.vue'

const api = vi.hoisted(() => ({
  rules: vi.fn(), save: vi.fn(), groups: vi.fn(), accounts: vi.fn(), success: vi.fn(), error: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  users: { getAccountRestrictions: api.rules, updateAccountRestrictions: api.save },
  groups: { list: api.groups }, accounts: { list: api.accounts }
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: api.success, showError: api.error }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const stubs = {
  BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
  Select: {
    props: ['modelValue', 'options', 'disabled'], emits: ['update:modelValue'],
    template: '<select :value="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', Number($event.target.value))"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>'
  }
}
async function open() {
  const wrapper = mount(UserAccountRestrictionsModal, {
    props: { show: true, user: { id: 7, email: 'user@example.com' } as AdminUser },
    global: { stubs }
  })
  await flushPromises()
  return wrapper
}
function button(wrapper: Awaited<ReturnType<typeof open>>, key: string) {
  return wrapper.findAll('button').find(b => b.text() === key)!
}
beforeEach(() => {
  vi.clearAllMocks()
  api.rules.mockResolvedValue({ restrictions: { 1: [11], 2: [22] } })
  api.groups.mockResolvedValue({ items: [{ id: 1, name: 'A', platform: 'openai' }, { id: 2, name: 'B', platform: 'openai' }], total: 2 })
  api.accounts.mockImplementation((_page: number, _size: number, filters: { group: string }) => Promise.resolve({
    items: filters.group === '1'
      ? [{ id: 11, name: 'OAuth account', type: 'oauth' }, { id: 12, name: 'API key account', type: 'apikey' }]
      : [{ id: 22, name: 'Group B account', type: 'apikey' }],
    total: 2
  }))
  api.save.mockImplementation((_user: number, _group: number, ids: number[]) => Promise.resolve({ account_ids: ids }))
})

describe('UserAccountRestrictionsModal', () => {
  it('saves only the selected group and retains the other group rules', async () => {
    const wrapper = await open()
    const checkboxes = wrapper.findAll('input[type=checkbox]')
    expect((checkboxes[0].element as HTMLInputElement).checked).toBe(true)
    await checkboxes[1].setValue(true)
    expect(wrapper.find('select').attributes('disabled')).toBeDefined()
    await button(wrapper, 'common.save').trigger('click')
    await flushPromises()
    expect(api.save).toHaveBeenCalledWith(7, 1, [11, 12])
    await wrapper.find('select').setValue('2')
    await flushPromises()
    expect((wrapper.find('input[type=checkbox]').element as HTMLInputElement).checked).toBe(true)
    expect(api.save).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('clears restrictions with an explicit empty array and preserves edits on failure', async () => {
    const wrapper = await open()
    await button(wrapper, 'admin.users.accountRestrictions.clear').trigger('click')
    api.save.mockRejectedValueOnce(new Error('offline'))
    await button(wrapper, 'common.save').trigger('click')
    await flushPromises()
    expect(api.save).toHaveBeenCalledWith(7, 1, [])
    expect(api.error).toHaveBeenCalled()
    expect(wrapper.find('select').attributes('disabled')).toBeDefined()
    await button(wrapper, 'admin.users.accountRestrictions.undo').trigger('click')
    expect((wrapper.find('input[type=checkbox]').element as HTMLInputElement).checked).toBe(true)
    wrapper.unmount()
  })

  it('cannot save after initial loading fails', async () => {
    api.rules.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await open()
    expect(wrapper.find('[role=alert]').exists()).toBe(true)
    expect(button(wrapper, 'common.save').attributes('disabled')).toBeDefined()
    expect(api.save).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('preserves selected IDs when searching another page of accounts', async () => {
    const wrapper = await open()
    api.accounts.mockResolvedValueOnce({ items: [{ id: 13, name: 'Filtered account', type: 'apikey' }], total: 1 })
    await wrapper.find('input:not([type=checkbox])').setValue('Filtered')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    await wrapper.find('input[type=checkbox]').setValue(true)
    await button(wrapper, 'common.save').trigger('click')
    await flushPromises()
    expect(api.save).toHaveBeenCalledWith(7, 1, [11, 13])
    wrapper.unmount()
  })
})
