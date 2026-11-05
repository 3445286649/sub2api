/** 发票文件（PDF）的前端校验和保存。大小上限与后端 InvoiceFileMaxBytes 保持一致。 */
export const INVOICE_FILE_MAX_BYTES = 10 * 1024 * 1024

export type InvoiceFileCheck = 'ok' | 'empty' | 'tooLarge' | 'notPdf'

export function formatFileSize(bytes?: number | null): string {
  if (!bytes || bytes <= 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

/** 只做提前提示，真正的判断以后端对文件头的校验为准。 */
export function checkInvoiceFile(file: File): InvoiceFileCheck {
  if (file.size === 0) return 'empty'
  if (file.size > INVOICE_FILE_MAX_BYTES) return 'tooLarge'
  if (!/\.pdf$/i.test(file.name) && file.type !== 'application/pdf') return 'notPdf'
  return 'ok'
}

/** 登录态在请求头里，下载只能先取 Blob 再触发保存，不能直接用 <a href>。 */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
