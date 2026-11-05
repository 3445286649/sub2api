/**
 * Invoice types
 * 金额统一为人民币元；只有实付金额可开票，赠送额度不开票。
 */

export type InvoiceType = 'enterprise' | 'individual'

export type InvoiceRequestStatus = 'pending' | 'processing' | 'completed' | 'rejected'

/** 可开票订单（来自 payment_order，已完成支付的充值/订阅订单） */
export interface InvoiceableOrder {
  id: number
  /** 后台和用户订单列表里显示的订单号 */
  out_trade_no: string
  recharge_code: string
  order_type: string
  payment_type: string
  /** 实付金额（含手续费），扣除退款后即可开票金额 */
  pay_amount: number
  /** 赠送额度，不开票 */
  bonus_amount: number
  invoiceable_amount: number
  paid_at: string
  /** 已被未驳回的申请占用，或被管理员手动排除 */
  already_invoiced: boolean
  /** 管理员登记为已在系统外开过票 */
  excluded?: boolean
  invoice_request_id?: number | null
}

export interface InvoiceableSummary {
  total_invoiceable_amount: number
  invoiceable_count: number
}

export interface InvoiceableOrdersResponse {
  items: InvoiceableOrder[]
  summary: InvoiceableSummary
}

export interface InvoiceTitleForm {
  invoice_type: InvoiceType
  enterprise_name: string
  enterprise_tax_number: string
  enterprise_address: string
  enterprise_phone: string
  enterprise_bank: string
  enterprise_bank_account: string
  individual_name: string
  individual_id_number: string
  /** 电子发票接收邮箱 */
  receiver_email: string
}

export interface CreateInvoiceRequest extends Partial<InvoiceTitleForm> {
  invoice_type: InvoiceType
  order_ids: number[]
}

export interface InvoiceRequest {
  id: number
  invoice_type: InvoiceType
  /** 抬头展示名：企业名称或个人姓名 */
  title: string
  enterprise_tax_number?: string | null
  enterprise_address?: string | null
  enterprise_phone?: string | null
  enterprise_bank?: string | null
  enterprise_bank_account?: string | null
  /** 后端返回时已打码 */
  individual_id_number?: string | null
  total_amount: number
  order_count: number
  status: InvoiceRequestStatus
  receiver_email: string
  invoice_number?: string | null
  invoice_file_url?: string | null
  /** 管理员上传了 PDF；用户端只在已开具后为 true，管理端任何状态都可见 */
  has_file?: boolean
  invoice_file_name?: string | null
  /** 字节 */
  invoice_file_size?: number | null
  email_sent_at?: string | null
  reject_reason?: string | null
  created_at: string
  processing_at?: string | null
  completed_at?: string | null
  rejected_at?: string | null
  /** 仅详情接口返回 */
  orders?: InvoiceableOrder[]
}

export interface InvoiceStatusCounts {
  pending: number
  processing: number
  completed: number
  rejected: number
  /** 待处理 + 开具中的申请金额合计 */
  open_amount: number
  completed_amount: number
  today_new: number
}

// ---- 管理端 ----

export interface AdminInvoiceRequest extends InvoiceRequest {
  user_id: number
  user_email: string
  /** 仅管理员可见，用户接口不返回 */
  admin_notes?: string | null
}

export interface AdminInvoiceListParams {
  page?: number
  page_size?: number
  status?: InvoiceRequestStatus
  invoice_type?: InvoiceType
  /** 匹配用户邮箱、抬头、税号或申请编号 */
  keyword?: string
}

/**
 * 状态流转：pending → processing / completed / rejected；processing → completed / rejected。
 * completed 时 invoice_number、invoice_file_url 必填；rejected 时 reject_reason 必填。
 */
export interface AdminUpdateInvoiceStatus {
  status: Exclude<InvoiceRequestStatus, 'pending'>
  admin_notes?: string
  invoice_number?: string
  invoice_file_url?: string
  reject_reason?: string
  /** 开具后立即发送邮件 */
  send_email?: boolean
}

/** 订单当前的开票情况，管理员退款前查看 */
export interface InvoiceOrderState {
  invoiced: boolean
  excluded: boolean
  request_id?: number | null
  request_status?: InvoiceRequestStatus | null
  invoice_number?: string | null
}

/** 已在系统外开过票、不允许用户再申请的订单 */
export interface InvoiceExclusion {
  id: number
  order_id: number
  out_trade_no: string
  user_email: string
  pay_amount: number
  paid_at?: string | null
  created_at: string
}

export interface InvoiceExclusionListParams {
  page?: number
  page_size?: number
  /** 匹配订单号、充值码或用户邮箱 */
  keyword?: string
}

/** 每个粘贴的号都有去向，错号不会被静默吞掉 */
export interface AddInvoiceExclusionsResult {
  added: InvoiceExclusion[]
  already_excluded: string[]
  not_found: string[]
  /** 订单未支付，排除没有意义 */
  unpaid: string[]
  /** 同一个号命中多笔订单，需要换更精确的号 */
  ambiguous: string[]
}
