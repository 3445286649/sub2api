import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { AdminInvoiceRequest } from '@/types/invoice'
import AdminInvoiceProcessDialog from '../AdminInvoiceProcessDialog.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key, te: () => false }),
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))

function request(overrides: Partial<AdminInvoiceRequest> = {}): AdminInvoiceRequest {
  return {
    id: 5, user_id: 7, user_email: 'u@example.com', invoice_type: 'individual', title: '张三',
    total_amount: 100, order_count: 1, status: 'processing', receiver_email: 'u@example.com',
    created_at: '2026-11-01T10:00:00Z', has_file: false,
    ...overrides,
  } as AdminInvoiceRequest
}

function mountDialog(req: AdminInvoiceRequest, props: Record<string, unknown> = {}) {
  return mount(AdminInvoiceProcessDialog, {
    props: { show: true, loading: false, submitting: false, request: req, ...props },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        InvoiceStatusBadge: true,
        InvoiceTimeline: true,
      },
    },
  })
}

async function choose(wrapper: ReturnType<typeof mountDialog>, file: File) {
  const input = wrapper.get('[data-testid=admin-invoice-file-input]')
  Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
  await input.trigger('change')
}

async function submit(wrapper: ReturnType<typeof mountDialog>) {
  await wrapper.get('#adm-inv-number').setValue('26332')
  await wrapper.get('[data-testid=admin-invoice-submit]').trigger('click')
}

const pdf = () => new File(['%PDF-1.7'], '发票.pdf', { type: 'application/pdf' })

describe('AdminInvoiceProcessDialog file upload', () => {
  it('emits the chosen pdf for upload', async () => {
    const wrapper = mountDialog(request())
    const file = pdf()

    await choose(wrapper, file)

    expect(wrapper.emitted('upload-file')).toEqual([[5, file]])
    expect(wrapper.find('[data-testid=admin-invoice-file-error]').exists()).toBe(false)
  })

  it('refuses a non-pdf or empty file without emitting', async () => {
    const wrapper = mountDialog(request())

    await choose(wrapper, new File(['x'], 'photo.png', { type: 'image/png' }))
    expect(wrapper.get('[data-testid=admin-invoice-file-error]').text()).toBe('invoice.admin.file.notPdf')

    await choose(wrapper, new File([], 'empty.pdf'))
    expect(wrapper.get('[data-testid=admin-invoice-file-error]').text()).toBe('invoice.admin.file.empty')
    expect(wrapper.emitted('upload-file')).toBeUndefined()
  })

  it('shows the uploaded file with a replace button instead of the chooser', () => {
    const wrapper = mountDialog(request({ has_file: true, invoice_file_name: '发票.pdf', invoice_file_size: 2048 }))

    const card = wrapper.get('[data-testid=admin-invoice-file-card]')
    expect(card.text()).toContain('发票.pdf')
    expect(card.text()).toContain('2.0 KB')
    expect(wrapper.find('[data-testid=admin-invoice-choose-file]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=admin-invoice-replace-file]').exists()).toBe(true)
  })

  it('blocks completion when there is neither an uploaded file nor a link', async () => {
    const wrapper = mountDialog(request())

    await submit(wrapper)

    expect(wrapper.emitted('update')).toBeUndefined()
    expect(wrapper.text()).toContain('invoice.admin.file.required')
  })

  it('completes with an uploaded file and sends no link', async () => {
    const wrapper = mountDialog(request({ has_file: true, invoice_file_name: 'a.pdf', invoice_file_size: 100 }))

    await submit(wrapper)

    expect(wrapper.emitted('update')).toEqual([
      [5, { status: 'completed', admin_notes: '', invoice_number: '26332', invoice_file_url: undefined, send_email: true }],
    ])
  })

  it('completes with an external link when no file is uploaded', async () => {
    const wrapper = mountDialog(request())
    await wrapper.get('[data-testid=admin-invoice-toggle-link]').trigger('click')
    await wrapper.get('input[placeholder^=http]').setValue('https://a.cn/x.pdf')

    await submit(wrapper)

    expect((wrapper.emitted('update')![0][1] as { invoice_file_url?: string }).invoice_file_url).toBe('https://a.cn/x.pdf')
  })

  it('refuses a malformed link', async () => {
    const wrapper = mountDialog(request())
    await wrapper.get('[data-testid=admin-invoice-toggle-link]').trigger('click')
    await wrapper.get('input[placeholder^=http]').setValue('javascript:alert(1)')

    await submit(wrapper)

    expect(wrapper.emitted('update')).toBeUndefined()
    expect(wrapper.text()).toContain('invoice.admin.fileUrlInvalid')
  })

  it('ignores a link left in the box after switching back to upload', async () => {
    const wrapper = mountDialog(request({ invoice_file_url: 'https://old.example/x.pdf' }))
    expect(wrapper.find('input[placeholder^=http]').exists()).toBe(true)
    await wrapper.get('[data-testid=admin-invoice-toggle-link]').trigger('click')
    expect(wrapper.find('input[placeholder^=http]').exists()).toBe(false)

    await submit(wrapper)

    expect(wrapper.emitted('update')).toBeUndefined()
  })

  it('asks the parent to download the uploaded file for checking', async () => {
    const wrapper = mountDialog(request({ has_file: true, invoice_file_name: 'a.pdf', invoice_file_size: 100 }))

    await wrapper.get('[data-testid=admin-invoice-file-card] button').trigger('click')

    expect(wrapper.emitted('download-file')).toEqual([[5]])
  })

  it('disables the file actions while an upload is running', () => {
    const wrapper = mountDialog(request({ has_file: true, invoice_file_name: 'a.pdf', invoice_file_size: 100 }), { uploading: true })
    expect(wrapper.get('[data-testid=admin-invoice-replace-file]').attributes('disabled')).toBeDefined()
  })
})
