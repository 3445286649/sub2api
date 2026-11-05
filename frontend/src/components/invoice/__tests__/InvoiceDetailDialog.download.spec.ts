import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { InvoiceRequest } from '@/types/invoice'
import InvoiceDetailDialog from '../InvoiceDetailDialog.vue'

const api = vi.hoisted(() => ({ downloadFile: vi.fn(), showError: vi.fn(), saveBlob: vi.fn() }))

vi.mock('@/api/invoice', () => ({ invoiceAPI: { downloadFile: api.downloadFile } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: api.showError }) }))
vi.mock('../invoiceFile', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../invoiceFile')>()),
  saveBlob: api.saveBlob,
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

const request = (overrides: Partial<InvoiceRequest> = {}): InvoiceRequest => ({
  id: 5, invoice_type: 'individual', title: '张三', total_amount: 100, order_count: 1, status: 'completed',
  receiver_email: 'u@example.com', created_at: '2026-11-01T10:00:00Z',
  has_file: true, invoice_file_name: '发票.pdf', invoice_file_size: 2048,
  ...overrides,
})

function mountDialog(req: InvoiceRequest) {
  return mount(InvoiceDetailDialog, {
    props: { show: true, loading: false, request: req },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name=footer /></div>' },
        Icon: true,
        InvoiceStatusBadge: true,
        InvoiceTimeline: true,
      },
    },
  })
}

describe('InvoiceDetailDialog download', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches the pdf with the login session and saves it under the invoice name', async () => {
    const blob = new Blob(['%PDF-1.7'], { type: 'application/pdf' })
    api.downloadFile.mockResolvedValue(blob)
    const wrapper = mountDialog(request())

    await wrapper.get('[data-testid=invoice-download]').trigger('click')
    await flushPromises()

    expect(api.downloadFile).toHaveBeenCalledWith(5)
    expect(api.saveBlob).toHaveBeenCalledWith(blob, '发票.pdf')
    expect(api.showError).not.toHaveBeenCalled()
  })

  it('falls back to a generated file name', async () => {
    api.downloadFile.mockResolvedValue(new Blob(['%PDF-1.7']))
    const wrapper = mountDialog(request({ invoice_file_name: null }))

    await wrapper.get('[data-testid=invoice-download]').trigger('click')
    await flushPromises()

    expect(api.saveBlob).toHaveBeenCalledWith(expect.any(Blob), 'invoice-5.pdf')
  })

  it('says the file is gone on a 404', async () => {
    api.downloadFile.mockRejectedValue({ status: 404 })
    const wrapper = mountDialog(request())

    await wrapper.get('[data-testid=invoice-download]').trigger('click')
    await flushPromises()

    expect(api.showError).toHaveBeenCalledWith('invoice.records.fileNotFound')
    expect(api.saveBlob).not.toHaveBeenCalled()
  })

  it('shows a generic message for other failures and re-enables the button', async () => {
    api.downloadFile.mockRejectedValue({ status: 0 })
    const wrapper = mountDialog(request())
    const button = wrapper.get('[data-testid=invoice-download]')

    await button.trigger('click')
    await flushPromises()

    expect(api.showError).toHaveBeenCalledWith('invoice.records.downloadFailed')
    expect(button.attributes('disabled')).toBeUndefined()
  })

  it('ignores a second click while the download is running', async () => {
    let resolve: (blob: Blob) => void = () => {}
    api.downloadFile.mockReturnValue(new Promise<Blob>((r) => { resolve = r }))
    const wrapper = mountDialog(request())
    const button = wrapper.get('[data-testid=invoice-download]')

    await button.trigger('click')
    await button.trigger('click')
    expect(api.downloadFile).toHaveBeenCalledTimes(1)
    expect(button.attributes('disabled')).toBeDefined()

    resolve(new Blob(['%PDF-1.7']))
    await flushPromises()
    expect(api.saveBlob).toHaveBeenCalledTimes(1)
  })

  it('keeps the plain link for older invoices that only have an external url', () => {
    const wrapper = mountDialog(request({ has_file: false, invoice_file_name: null, invoice_file_url: 'https://a.cn/x.pdf' }))

    expect(wrapper.find('[data-testid=invoice-download]').exists()).toBe(false)
    expect(wrapper.get('a[href="https://a.cn/x.pdf"]').exists()).toBe(true)
  })

  it('shows no download before the invoice is issued', () => {
    const wrapper = mountDialog(request({ status: 'processing', has_file: false }))

    expect(wrapper.find('[data-testid=invoice-download]').exists()).toBe(false)
    expect(wrapper.find('a[target=_blank]').exists()).toBe(false)
  })
})
