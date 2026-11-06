import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import AdminInvoiceCategoryDialog from '../AdminInvoiceCategoryDialog.vue'
import type { AdminInvoiceCategory } from '@/types/invoice'

const api = vi.hoisted(() => ({
  listCategories: vi.fn(),
  createCategory: vi.fn(),
  updateCategory: vi.fn(),
  deleteCategory: vi.fn(),
  reorderCategories: vi.fn(),
}))
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/invoice', () => ({ adminInvoiceAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }),
}))

const IconStub = defineComponent({ setup: () => () => h('i') })
const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  setup: (props, { slots }) => () => (props.show ? h('div', [slots.default?.(), slots.footer?.()]) : null),
})
const ConfirmDialogStub = defineComponent({
  props: { show: Boolean, message: String },
  emits: ['confirm', 'cancel'],
  setup: (props, { emit }) => () =>
    props.show ? h('button', { 'data-testid': 'confirm-delete', onClick: () => emit('confirm') }, props.message) : null,
})

const category = (id: number, name: string, extra: Partial<AdminInvoiceCategory> = {}): AdminInvoiceCategory => ({
  id,
  name,
  description: '',
  enabled: true,
  sort_order: id,
  request_count: 0,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
  ...extra,
})

async function mountDialog() {
  const wrapper = mount(AdminInvoiceCategoryDialog, {
    props: { show: true },
    global: { stubs: { Icon: IconStub, BaseDialog: BaseDialogStub, ConfirmDialog: ConfirmDialogStub } },
  })
  await flushPromises()
  return wrapper
}

const nameInput = (w: Awaited<ReturnType<typeof mountDialog>>) => w.get('[data-testid=invoice-category-name]')
const descInput = (w: Awaited<ReturnType<typeof mountDialog>>) => w.get('[data-testid=invoice-category-description]')
const saveButton = (w: Awaited<ReturnType<typeof mountDialog>>) => w.get('[data-testid=invoice-category-save]')

describe('AdminInvoiceCategoryDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listCategories.mockResolvedValue([
      category(1, '技术服务费', { description: '软件技术服务' }),
      category(2, '软件使用费', { enabled: false, request_count: 3 }),
    ])
    api.createCategory.mockResolvedValue(category(9, 'new'))
    api.updateCategory.mockResolvedValue(category(1, 'x'))
    api.deleteCategory.mockResolvedValue(undefined)
    api.reorderCategories.mockResolvedValue([category(2, '软件使用费'), category(1, '技术服务费')])
  })

  it('lists every category, including disabled ones and how often each is used', async () => {
    const wrapper = await mountDialog()
    expect(api.listCategories).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid=invoice-category-row-1]').text()).toContain('技术服务费')
    expect(wrapper.get('[data-testid=invoice-category-row-1]').text()).toContain('软件技术服务')
    const second = wrapper.get('[data-testid=invoice-category-row-2]').text()
    expect(second).toContain('invoice.categories.disabled')
    expect(second).toContain('"count":3')
  })

  it('shows an empty state when nothing is configured', async () => {
    api.listCategories.mockResolvedValue([])
    const wrapper = await mountDialog()
    expect(wrapper.find('[data-testid=invoice-category-empty]').exists()).toBe(true)
  })

  it('refuses a blank name without calling the API', async () => {
    const wrapper = await mountDialog()
    await nameInput(wrapper).setValue('   ')
    await saveButton(wrapper).trigger('click')
    await flushPromises()
    expect(api.createCategory).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('invoice.errors.INVOICE_CATEGORY_NAME_REQUIRED')
  })

  it('creates a category with trimmed fields, reloads the list and tells the page', async () => {
    const wrapper = await mountDialog()
    await nameInput(wrapper).setValue('  新类目  ')
    await descInput(wrapper).setValue('  新说明 ')
    await saveButton(wrapper).trigger('click')
    await flushPromises()
    expect(api.createCategory).toHaveBeenCalledWith({ name: '新类目', description: '新说明' })
    expect(api.listCategories).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect((nameInput(wrapper).element as HTMLInputElement).value).toBe('')
    expect(showSuccess).toHaveBeenCalledTimes(1)
  })

  it('keeps the draft and does not notify the page when saving fails', async () => {
    api.createCategory.mockRejectedValue({ reason: 'INVOICE_CATEGORY_DUPLICATE', message: 'dup' })
    const wrapper = await mountDialog()
    await nameInput(wrapper).setValue('技术服务费')
    await saveButton(wrapper).trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledTimes(1)
    expect((nameInput(wrapper).element as HTMLInputElement).value).toBe('技术服务费')
    expect(wrapper.emitted('changed')).toBeUndefined()
  })

  it('edits an existing category through the same form', async () => {
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid=invoice-category-edit-1]').trigger('click')
    expect((nameInput(wrapper).element as HTMLInputElement).value).toBe('技术服务费')
    expect((descInput(wrapper).element as HTMLInputElement).value).toBe('软件技术服务')
    await nameInput(wrapper).setValue('技术服务')
    await saveButton(wrapper).trigger('click')
    await flushPromises()
    expect(api.updateCategory).toHaveBeenCalledWith(1, { name: '技术服务', description: '软件技术服务' })
    expect(api.createCategory).not.toHaveBeenCalled()
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('toggles a category with only the enabled flag and reloads', async () => {
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid=invoice-category-toggle-1]').trigger('click')
    await flushPromises()
    expect(api.updateCategory).toHaveBeenCalledWith(1, { enabled: false })
    expect(api.listCategories).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('reloads the list after a failed toggle so the screen matches the server', async () => {
    api.updateCategory.mockRejectedValue({ reason: 'INVOICE_CATEGORY_NOT_FOUND', message: 'gone' })
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid=invoice-category-toggle-1]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledTimes(1)
    expect(api.listCategories).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('changed')).toBeUndefined()
  })

  it('reorders by swapping neighbours and disables the moves that go off either end', async () => {
    const wrapper = await mountDialog()
    expect(wrapper.get('[data-testid=invoice-category-up-1]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid=invoice-category-down-2]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid=invoice-category-down-1]').trigger('click')
    await flushPromises()
    expect(api.reorderCategories).toHaveBeenCalledWith([2, 1])
    expect(wrapper.emitted('changed')).toHaveLength(1)
    const rows = wrapper.findAll('[data-testid^=invoice-category-row-]')
    expect(rows[0].text()).toContain('软件使用费')
  })

  it('blocks deleting a category that requests use, and deletes an unused one after confirmation', async () => {
    const wrapper = await mountDialog()
    expect(wrapper.get('[data-testid=invoice-category-delete-2]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid=invoice-category-delete-1]').trigger('click')
    expect(api.deleteCategory).not.toHaveBeenCalled()
    const confirm = wrapper.get('[data-testid=confirm-delete]')
    expect(confirm.text()).toContain('技术服务费')
    await confirm.trigger('click')
    await flushPromises()

    expect(api.deleteCategory).toHaveBeenCalledWith(1)
    expect(showSuccess).toHaveBeenCalledTimes(1)
    expect(api.listCategories).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })
})
