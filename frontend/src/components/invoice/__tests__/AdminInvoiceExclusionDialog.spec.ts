import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import AdminInvoiceExclusionDialog from '../AdminInvoiceExclusionDialog.vue'
import type { InvoiceExclusion } from '@/types/invoice'

const api = vi.hoisted(() => ({
  listExclusions: vi.fn(),
  addExclusions: vi.fn(),
  removeExclusion: vi.fn(),
}))
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/invoice', () => ({ adminInvoiceAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/utils/format', () => ({ formatDateTimeToMinute: (value: string) => value }))
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
    props.show ? h('button', { 'data-testid': 'confirm-removal', onClick: () => emit('confirm') }, props.message) : null,
})

const exclusion = (id: number, order: string): InvoiceExclusion => ({
  id,
  order_id: id + 100,
  out_trade_no: order,
  user_email: 'u@example.com',
  pay_amount: 100,
  paid_at: '2026-11-01T00:00:00Z',
  created_at: '2026-11-04T00:00:00Z',
})
const emptyResult = { added: [], already_excluded: [], not_found: [], unpaid: [], ambiguous: [] }
const listOf = (items: InvoiceExclusion[], page = 1, pages = 1) => ({ items, total: items.length, page, page_size: 10, pages })

async function mountDialog() {
  const wrapper = mount(AdminInvoiceExclusionDialog, {
    props: { show: true },
    global: { stubs: { Icon: IconStub, BaseDialog: BaseDialogStub, ConfirmDialog: ConfirmDialogStub } },
  })
  await flushPromises()
  return wrapper
}

describe('AdminInvoiceExclusionDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listExclusions.mockResolvedValue(listOf([exclusion(1, 'sub2_old')]))
    api.addExclusions.mockResolvedValue(emptyResult)
    api.removeExclusion.mockResolvedValue(undefined)
  })

  it('loads the excluded orders when opened', async () => {
    const wrapper = await mountDialog()
    expect(api.listExclusions).toHaveBeenCalledWith({ page: 1, page_size: 10, keyword: undefined })
    expect(wrapper.get('[data-testid="invoice-exclusion-row-1"]').text()).toContain('sub2_old')
  })

  it('keeps submit disabled until an order number is entered', async () => {
    const wrapper = await mountDialog()
    expect(wrapper.get('[data-testid="invoice-exclusion-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('   \n  ')
    expect(wrapper.get('[data-testid="invoice-exclusion-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('sub2_one')
    expect(wrapper.get('[data-testid="invoice-exclusion-submit"]').attributes('disabled')).toBeUndefined()
  })

  it('submits a single typed order number', async () => {
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('sub2_one')
    await wrapper.get('[data-testid="invoice-exclusion-submit"]').trigger('click')
    await flushPromises()
    expect(api.addExclusions).toHaveBeenCalledWith(['sub2_one'])
  })

  it('splits a pasted batch and removes duplicates before submitting', async () => {
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('sub2_a\r\nsub2_b\tsub2_c, sub2_d，sub2_e；sub2_a\n\n')
    expect(wrapper.get('[data-testid="invoice-exclusion-submit"]').text()).toContain('"count":5')
    await wrapper.get('[data-testid="invoice-exclusion-submit"]').trigger('click')
    await flushPromises()
    expect(api.addExclusions).toHaveBeenCalledWith(['sub2_a', 'sub2_b', 'sub2_c', 'sub2_d', 'sub2_e'])
  })

  it('reports every outcome, refreshes the list and leaves only fixable numbers in the box', async () => {
    api.addExclusions.mockResolvedValue({
      added: [exclusion(2, 'sub2_ok')],
      already_excluded: ['sub2_before'],
      not_found: ['sub2_typo'],
      unpaid: ['sub2_unpaid'],
      ambiguous: ['shared'],
    })
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('sub2_ok sub2_before sub2_typo sub2_unpaid shared')
    await wrapper.get('[data-testid="invoice-exclusion-submit"]').trigger('click')
    await flushPromises()

    const result = wrapper.get('[data-testid="invoice-exclusion-result"]')
    expect(result.text()).toContain('invoice.exclusions.resultAdded')
    expect(wrapper.get('[data-testid="invoice-exclusion-not-found"]').text()).toContain('sub2_typo')
    expect(wrapper.get('[data-testid="invoice-exclusion-ambiguous"]').text()).toContain('shared')
    expect(wrapper.get('[data-testid="invoice-exclusion-unpaid"]').text()).toContain('sub2_unpaid')
    expect(wrapper.get('[data-testid="invoice-exclusion-already"]').text()).toContain('sub2_before')
    expect((wrapper.get('[data-testid="invoice-exclusion-input"]').element as HTMLTextAreaElement).value).toBe('sub2_typo\nshared')
    expect(api.listExclusions).toHaveBeenCalledTimes(2)
  })

  it('does not reload the list when nothing was added', async () => {
    api.addExclusions.mockResolvedValue({ ...emptyResult, not_found: ['sub2_typo'] })
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('sub2_typo')
    await wrapper.get('[data-testid="invoice-exclusion-submit"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="invoice-exclusion-result"]').text()).toContain('invoice.exclusions.resultNothingAdded')
    expect(api.listExclusions).toHaveBeenCalledTimes(1)
  })

  it('keeps the input when the request fails', async () => {
    api.addExclusions.mockRejectedValue({ reason: 'INVOICE_EXCLUSION_TOO_MANY', message: 'too many' })
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-input"]').setValue('sub2_a')
    await wrapper.get('[data-testid="invoice-exclusion-submit"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledTimes(1)
    expect((wrapper.get('[data-testid="invoice-exclusion-input"]').element as HTMLTextAreaElement).value).toBe('sub2_a')
  })

  it('asks for confirmation before removing and reloads afterwards', async () => {
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-remove-1"]').trigger('click')
    expect(api.removeExclusion).not.toHaveBeenCalled()

    const confirm = wrapper.get('[data-testid="confirm-removal"]')
    expect(confirm.text()).toContain('sub2_old')
    await confirm.trigger('click')
    await flushPromises()

    expect(api.removeExclusion).toHaveBeenCalledWith(1)
    expect(showSuccess).toHaveBeenCalledTimes(1)
    expect(api.listExclusions).toHaveBeenCalledTimes(2)
  })

  it('steps back a page when the last row of a page is removed', async () => {
    api.listExclusions.mockResolvedValue(listOf([exclusion(1, 'sub2_old')], 2, 2))
    const wrapper = await mountDialog()
    await wrapper.get('[data-testid="invoice-exclusion-remove-1"]').trigger('click')
    await wrapper.get('[data-testid="confirm-removal"]').trigger('click')
    await flushPromises()
    expect(api.listExclusions).toHaveBeenLastCalledWith({ page: 1, page_size: 10, keyword: undefined })
  })
})
