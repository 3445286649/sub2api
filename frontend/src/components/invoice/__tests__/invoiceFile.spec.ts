import { describe, expect, it } from 'vitest'
import { INVOICE_FILE_MAX_BYTES, checkInvoiceFile, formatFileSize } from '../invoiceFile'

function fileOf(name: string, size: number, type = ''): File {
  const file = new File([], name, { type })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

describe('invoiceFile', () => {
  it('formats sizes and stays empty for missing values', () => {
    expect(formatFileSize(512)).toBe('512 B')
    expect(formatFileSize(52341)).toBe('51.1 KB')
    expect(formatFileSize(3 * 1024 * 1024)).toBe('3.0 MB')
    expect(formatFileSize(null)).toBe('')
    expect(formatFileSize(0)).toBe('')
  })

  it('accepts a pdf by extension or by type', () => {
    expect(checkInvoiceFile(fileOf('invoice.pdf', 100))).toBe('ok')
    expect(checkInvoiceFile(fileOf('INVOICE.PDF', 100))).toBe('ok')
    expect(checkInvoiceFile(fileOf('download', 100, 'application/pdf'))).toBe('ok')
  })

  it('refuses empty, oversized and non-pdf files before uploading', () => {
    expect(checkInvoiceFile(fileOf('a.pdf', 0))).toBe('empty')
    expect(checkInvoiceFile(fileOf('a.pdf', INVOICE_FILE_MAX_BYTES))).toBe('ok')
    expect(checkInvoiceFile(fileOf('a.pdf', INVOICE_FILE_MAX_BYTES + 1))).toBe('tooLarge')
    expect(checkInvoiceFile(fileOf('a.png', 100, 'image/png'))).toBe('notPdf')
  })
})
