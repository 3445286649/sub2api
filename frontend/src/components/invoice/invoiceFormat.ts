import type { InvoiceableOrder, InvoiceOrderState, InvoiceRequestStatus, InvoiceTitleForm } from '@/types/invoice'

const cnyFormatter = new Intl.NumberFormat('zh-CN', {
  style: 'currency',
  currency: 'CNY',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})

export function formatCny(value: number | null | undefined): string {
  return cnyFormatter.format(Number.isFinite(value) ? Number(value) : 0)
}

/** 金额按分累加，避免 0.1 + 0.2 这类浮点误差出现在开票总额里 */
export function sumAmounts(values: number[]): number {
  return values.reduce((cents, v) => cents + Math.round(v * 100), 0) / 100
}

/** 展示用订单号：与订单列表一致；早期订单没有 out_trade_no 时退回充值码 */
export function orderNumberOf(order: Pick<InvoiceableOrder, 'out_trade_no' | 'recharge_code'>): string {
  return order.out_trade_no || order.recharge_code
}

/** 管理员退款前要看到的提醒，返回 i18n key 和参数；订单未开票时返回 null */
export function refundInvoiceNotice(state: InvoiceOrderState): { key: string; params: Record<string, string | number> } | null {
  if (!state.invoiced) return null
  if (state.request_id && state.request_status !== 'completed') {
    return { key: 'invoice.refundNotice.inProgress', params: { id: state.request_id } }
  }
  if (state.request_id) {
    return { key: 'invoice.refundNotice.issued', params: { id: state.request_id, number: state.invoice_number ?? '' } }
  }
  return { key: 'invoice.refundNotice.excluded', params: {} }
}

/** 把手输或从表格粘贴的文本拆成去重后的订单号，换行、空格、Tab、中英文逗号分号都算分隔符 */
export function parseOrderNumbers(text: string): string[] {
  return [...new Set(text.split(/[\s,，;；、]+/).filter(Boolean))]
}

// 统一社会信用代码 18 位；旧版税号 15 / 20 位
const TAX_NUMBER_RE = /^(?:[0-9A-Z]{15}|[0-9A-Z]{18}|[0-9A-Z]{20})$/
const ID_NUMBER_RE = /^\d{17}[\dX]$/

export type InvoiceFieldErrors = Partial<Record<keyof InvoiceTitleForm, string>>

/** 返回 i18n key（invoice.validation.*），由调用方翻译 */
export function validateInvoiceTitle(form: InvoiceTitleForm): InvoiceFieldErrors {
  const errors: InvoiceFieldErrors = {}
  if (form.invoice_type === 'enterprise') {
    if (!form.enterprise_name.trim()) errors.enterprise_name = 'enterpriseNameRequired'
    const tax = form.enterprise_tax_number.trim().toUpperCase()
    if (!tax) errors.enterprise_tax_number = 'taxNumberRequired'
    else if (!TAX_NUMBER_RE.test(tax)) errors.enterprise_tax_number = 'taxNumberInvalid'
    if (form.enterprise_bank_account.trim() && !/^\d{8,30}$/.test(form.enterprise_bank_account.replace(/\s/g, ''))) {
      errors.enterprise_bank_account = 'bankAccountInvalid'
    }
  } else {
    if (!form.individual_name.trim()) errors.individual_name = 'individualNameRequired'
    const id = form.individual_id_number.trim().toUpperCase()
    if (id && !ID_NUMBER_RE.test(id)) errors.individual_id_number = 'idNumberInvalid'
  }
  const email = form.receiver_email.trim()
  if (!email) errors.receiver_email = 'emailRequired'
  else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.receiver_email = 'emailInvalid'
  return errors
}

export function maskMiddle(value: string | null | undefined, head = 4, tail = 4): string {
  if (!value) return ''
  if (value.length <= head + tail) return value
  return `${value.slice(0, head)}${'*'.repeat(Math.min(value.length - head - tail, 8))}${value.slice(-tail)}`
}

export interface InvoiceStatusMeta {
  icon: 'clock' | 'sync' | 'checkCircle' | 'xCircle'
  /** 徽标配色，浅色 / 深色都可读 */
  badgeClass: string
  dotClass: string
}

export const INVOICE_STATUS_META: Record<InvoiceRequestStatus, InvoiceStatusMeta> = {
  pending: {
    icon: 'clock',
    badgeClass: 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30',
    dotClass: 'bg-amber-500',
  },
  processing: {
    icon: 'sync',
    badgeClass: 'bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/30',
    dotClass: 'bg-sky-500',
  },
  completed: {
    icon: 'checkCircle',
    badgeClass: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/30',
    dotClass: 'bg-emerald-500',
  },
  rejected: {
    icon: 'xCircle',
    badgeClass: 'bg-red-50 text-red-700 ring-red-200 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-500/30',
    dotClass: 'bg-red-500',
  },
}

export function emptyInvoiceTitleForm(receiverEmail = ''): InvoiceTitleForm {
  return {
    receiver_email: receiverEmail,
    invoice_type: 'enterprise',
    enterprise_name: '',
    enterprise_tax_number: '',
    enterprise_address: '',
    enterprise_phone: '',
    enterprise_bank: '',
    enterprise_bank_account: '',
    individual_name: '',
    individual_id_number: '',
  }
}
