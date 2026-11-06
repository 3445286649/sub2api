/**
 * 发票接口的本地假数据，只在 dev + VITE_INVOICE_MOCK=true 时被动态加载，不进生产包。
 * 用户端和管理端共用这一份内存数据：管理端改状态后，用户端刷新即可看到。
 */

import type { InvoiceAPI } from './invoice'
import type { AdminInvoiceAPI } from './admin/invoice'
import type { AddInvoiceExclusionsResult, AdminInvoiceCategory, AdminInvoiceRequest, InvoiceableOrder, InvoiceExclusion, InvoiceRequest } from '@/types/invoice'
import { sumAmounts } from '@/components/invoice/invoiceFormat'

const delay = (ms = 350) => new Promise((resolve) => setTimeout(resolve, ms))
const daysAgo = (days: number, hour = 10) => {
  const d = new Date()
  d.setDate(d.getDate() - days)
  d.setHours(hour, 24, 0, 0)
  return d.toISOString()
}
const mockError = (status: number, code: string, message: string) => ({ status, code, reason: code, message })

/** 假定当前登录用户 */
const ME = { id: 1, email: 'admin@example.com' }

function order(id: number, days: number, pay: number, bonus: number, type: string, method: string): InvoiceableOrder {
  return {
    id,
    out_trade_no: `sub2_${daysAgo(days).slice(0, 10).replace(/-/g, '')}${id.toString(36)}`,
    recharge_code: `PAY-${id}-${id % 100000}`,
    order_type: type,
    payment_type: method,
    pay_amount: pay,
    bonus_amount: bonus,
    invoiceable_amount: pay,
    paid_at: daysAgo(days, 9 + (id % 9)),
    already_invoiced: false,
    invoice_request_id: null,
  }
}

/** 当前用户的订单 */
const orders: InvoiceableOrder[] = [
  order(320118, 1, 500, 25, 'balance', 'alipay'),
  order(320097, 3, 200, 10, 'balance', 'wxpay'),
  order(320054, 6, 99, 0, 'subscription', 'alipay'),
  order(319980, 9, 1000, 50, 'balance', 'alipay'),
  order(319921, 14, 50, 0, 'balance', 'wxpay'),
  order(319870, 21, 2000, 100, 'balance', 'alipay'),
  order(319802, 33, 300, 15, 'balance', 'alipay'),
  order(319755, 41, 100, 5, 'balance', 'wxpay'),
]
/** 其他用户的订单，只在管理端详情里出现 */
const otherOrders: InvoiceableOrder[] = [
  order(320130, 0, 600, 30, 'balance', 'alipay'),
  order(320125, 0, 88, 0, 'subscription', 'wxpay'),
  order(320101, 2, 1200, 60, 'balance', 'alipay'),
  order(320066, 4, 300, 15, 'balance', 'wxpay'),
  order(319990, 8, 150, 0, 'balance', 'alipay'),
]
const allOrders = () => [...orders, ...otherOrders]

/** 申请 → 订单的关联；驳回的申请保留关联但不再占用订单 */
const links = new Map<number, number[]>()

function activeClaim(orderId: number): number | null {
  for (const [reqId, ids] of links) {
    if (!ids.includes(orderId)) continue
    const req = requests.find((r) => r.id === reqId)
    if (req && req.status !== 'rejected') return reqId
  }
  return null
}
/** 管理员手动排除的订单：order_id → 排除记录 */
const exclusions = new Map<number, InvoiceExclusion>()
function view(o: InvoiceableOrder): InvoiceableOrder {
  const claimedBy = activeClaim(o.id)
  const excluded = exclusions.has(o.id)
  return { ...o, already_invoiced: claimedBy !== null || excluded, excluded, invoice_request_id: claimedBy }
}

const requests: AdminInvoiceRequest[] = []
function seed(req: Omit<AdminInvoiceRequest, 'total_amount' | 'order_count'>, orderIds: number[]) {
  links.set(req.id, orderIds)
  const picked = allOrders().filter((o) => orderIds.includes(o.id))
  requests.push({ ...req, total_amount: sumAmounts(picked.map((o) => o.invoiceable_amount)), order_count: picked.length })
}

const yunzhan = {
  invoice_type: 'enterprise' as const, title: '杭州云栈科技有限公司', enterprise_tax_number: '91330106MA2KXXXX7Q',
  enterprise_address: '杭州市西湖区文三路 90 号', enterprise_phone: '0571-8888 6666',
  enterprise_bank: '招商银行杭州高新支行', enterprise_bank_account: '571908765410201',
}
seed({ id: 1008, user_id: 7, user_email: 'ops@lingxi.ai', invoice_type: 'enterprise', title: '上海灵犀智能科技有限公司',
  enterprise_tax_number: '91310115MA1HXXXX2B', status: 'pending', receiver_email: 'invoice@lingxi.ai',
  created_at: daysAgo(0, 11) }, [320130, 320125])
seed({ id: 1007, user_id: 9, user_email: 'li.wei@qq.com', invoice_type: 'individual', title: '李伟',
  individual_id_number: '1101**********3312', status: 'pending', receiver_email: 'li.wei@qq.com',
  created_at: daysAgo(1, 21) }, [320066])
seed({ id: 1006, user_id: 12, user_email: 'finance@deepcode.cn', invoice_type: 'enterprise', title: '深圳深码网络有限公司',
  enterprise_tax_number: '91440300MA5GXXXX9K', status: 'processing', receiver_email: 'finance@deepcode.cn',
  admin_notes: '已在税务平台录入', created_at: daysAgo(2, 9), processing_at: daysAgo(1, 15) }, [320101])
seed({ id: 1005, user_id: ME.id, user_email: ME.email, ...yunzhan, status: 'processing', receiver_email: 'finance@example.com',
  created_at: daysAgo(2, 14), processing_at: daysAgo(1, 10) }, [319921, 319802])
seed({ id: 1004, user_id: 15, user_email: 'chen@studio.io', invoice_type: 'individual', title: '陈晨', status: 'rejected',
  receiver_email: 'chen@studio.io', reject_reason: '订单含退款，请退款完成后重新申请',
  created_at: daysAgo(10, 13), rejected_at: daysAgo(9, 11) }, [319990])
seed({ id: 1002, user_id: ME.id, user_email: ME.email, ...yunzhan, status: 'completed', receiver_email: 'finance@example.com',
  invoice_number: '26332000000184527761', has_file: true, invoice_file_name: '发票-26332.pdf', invoice_file_size: 52341,
  admin_notes: '已开具数电普通发票', created_at: daysAgo(18, 11), processing_at: daysAgo(17, 9),
  completed_at: daysAgo(17, 16), email_sent_at: daysAgo(17, 16) }, [319755])
seed({ id: 1001, user_id: ME.id, user_email: ME.email, invoice_type: 'individual', title: '张三',
  individual_id_number: '3301**********1234', status: 'rejected', receiver_email: 'me@example.com',
  reject_reason: '抬头与税号不一致，请核对后重新提交', created_at: daysAgo(45, 20), rejected_at: daysAgo(44, 10) }, [319870])

function withOrders(req: AdminInvoiceRequest): AdminInvoiceRequest {
  const ids = links.get(req.id) ?? []
  return { ...req, orders: allOrders().filter((o) => ids.includes(o.id)).map(view) }
}
function stripAdmin(req: AdminInvoiceRequest): InvoiceRequest {
  const { user_id: _u, user_email: _e, ...rest } = req
  // 与后端一致：开票前管理员上传的文件对用户不可见
  if (rest.status !== 'completed') return { ...rest, has_file: false, invoice_file_name: null, invoice_file_size: null }
  return rest
}
function paginate<T>(list: T[], page = 1, pageSize = 10) {
  return {
    items: list.slice((page - 1) * pageSize, page * pageSize),
    total: list.length,
    page,
    page_size: pageSize,
    pages: Math.max(1, Math.ceil(list.length / pageSize)),
  }
}
function find(id: number) {
  const found = requests.find((r) => r.id === id)
  if (!found) throw mockError(404, 'INVOICE_REQUEST_NOT_FOUND', '发票申请不存在')
  return found
}
const myRequests = () => requests.filter((r) => r.user_id === ME.id)

/** 管理员配置的开票类目；用户端只看到启用的 */
const categories: AdminInvoiceCategory[] = [
  { id: 1, name: '技术服务费', description: '软件技术服务', enabled: true, sort_order: 1, request_count: 0, created_at: daysAgo(30), updated_at: daysAgo(30) },
  { id: 2, name: '软件使用费', description: '', enabled: true, sort_order: 2, request_count: 0, created_at: daysAgo(30), updated_at: daysAgo(30) },
]

export const invoiceMockAPI: InvoiceAPI = {
  async getCategories() {
    await delay(200)
    return categories
      .filter((c) => c.enabled)
      .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
      .map(({ id, name, description }) => ({ id, name, description }))
  },

  async getInvoiceableOrders() {
    await delay()
    const items = orders.map(view)
    const open = items.filter((o) => !o.already_invoiced)
    return {
      items,
      summary: {
        total_invoiceable_amount: sumAmounts(open.map((o) => o.invoiceable_amount)),
        invoiceable_count: open.length,
      },
    }
  },

  async createRequest(payload) {
    await delay(700)
    const picked = orders.filter((o) => payload.order_ids.includes(o.id))
    if (picked.length === 0 || picked.some((o) => activeClaim(o.id) !== null || exclusions.has(o.id))) {
      throw mockError(400, 'ORDER_NOT_INVOICEABLE', '所选订单中有已申请开票的订单，请刷新后重试')
    }
    // 与后端一致：有启用类目时必须选其中一个，没有时保持旧流程
    const enabledCategories = categories.filter((c) => c.enabled)
    const category = enabledCategories.find((c) => c.id === payload.category_id)
    if (enabledCategories.length > 0 && payload.category_id == null) throw mockError(400, 'INVOICE_CATEGORY_REQUIRED', '请选择开票类目')
    if (payload.category_id != null && !category) throw mockError(400, 'INVALID_INVOICE_CATEGORY', '开票类目不可用，请重新选择')
    const isEnterprise = payload.invoice_type === 'enterprise'
    const id = Math.max(...requests.map((r) => r.id)) + 1
    const created: AdminInvoiceRequest = {
      id,
      user_id: ME.id,
      user_email: ME.email,
      invoice_type: payload.invoice_type,
      title: (isEnterprise ? payload.enterprise_name : payload.individual_name) ?? '',
      enterprise_tax_number: isEnterprise ? payload.enterprise_tax_number : null,
      enterprise_address: isEnterprise ? payload.enterprise_address : null,
      enterprise_phone: isEnterprise ? payload.enterprise_phone : null,
      enterprise_bank: isEnterprise ? payload.enterprise_bank : null,
      enterprise_bank_account: isEnterprise ? payload.enterprise_bank_account : null,
      individual_id_number: isEnterprise ? null : payload.individual_id_number,
      total_amount: sumAmounts(picked.map((o) => o.invoiceable_amount)),
      order_count: picked.length,
      status: 'pending',
      receiver_email: payload.receiver_email ?? '',
      category_id: category?.id ?? null,
      category_name: category?.name ?? null,
      created_at: new Date().toISOString(),
    }
    requests.unshift(created)
    links.set(id, picked.map((o) => o.id))
    return stripAdmin(withOrders(created))
  },

  async listRequests(params) {
    await delay()
    const mine = myRequests()
    const filtered = params?.status ? mine.filter((r) => r.status === params.status) : mine
    const result = paginate(filtered, params?.page, params?.page_size)
    return { ...result, items: result.items.map(stripAdmin) }
  },

  async getRequest(id) {
    await delay(250)
    const found = find(id)
    if (found.user_id !== ME.id) throw mockError(404, 'INVOICE_REQUEST_NOT_FOUND', '发票申请不存在')
    return stripAdmin(withOrders(found))
  },

  async downloadFile(id) {
    await delay(300)
    const found = find(id)
    if (found.user_id !== ME.id || found.status !== 'completed' || !found.has_file) {
      throw mockError(404, 'INVOICE_FILE_NOT_FOUND', '发票文件不存在')
    }
    return new Blob(['%PDF-1.4 mock invoice'], { type: 'application/pdf' })
  },
}

const ALLOWED_TRANSITIONS: Record<string, string[]> = {
  pending: ['processing', 'completed', 'rejected'],
  processing: ['completed', 'rejected'],
}

const withCount = (c: AdminInvoiceCategory): AdminInvoiceCategory => ({ ...c, request_count: requests.filter((r) => r.category_id === c.id).length })
const sortedCategories = () => [...categories].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)

export const adminInvoiceMockAPI: AdminInvoiceAPI = {
  async getStats() {
    await delay(200)
    const today = new Date().toDateString()
    const count = (s: string) => requests.filter((r) => r.status === s).length
    return {
      pending: count('pending'),
      processing: count('processing'),
      completed: count('completed'),
      rejected: count('rejected'),
      open_amount: sumAmounts(requests.filter((r) => r.status === 'pending' || r.status === 'processing').map((r) => r.total_amount)),
      completed_amount: sumAmounts(requests.filter((r) => r.status === 'completed').map((r) => r.total_amount)),
      today_new: requests.filter((r) => new Date(r.created_at).toDateString() === today).length,
    }
  },

  async listRequests(params) {
    await delay()
    const kw = params?.keyword?.trim().toLowerCase()
    const filtered = requests.filter((r) => {
      if (params?.status && r.status !== params.status) return false
      if (params?.invoice_type && r.invoice_type !== params.invoice_type) return false
      if (params?.category_id && r.category_id !== params.category_id) return false
      if (kw) {
        const hay = [String(r.id), r.user_email, r.title, r.enterprise_tax_number ?? ''].join(' ').toLowerCase()
        if (!hay.includes(kw)) return false
      }
      return true
    })
    const result = paginate(filtered, params?.page, params?.page_size ?? 20)
    return { ...result, items: result.items.map((r) => ({ ...r })) }
  },

  async getRequest(id) {
    await delay(250)
    return withOrders(find(id))
  },

  async updateStatus(id, payload) {
    await delay(600)
    const req = find(id)
    if (!ALLOWED_TRANSITIONS[req.status]?.includes(payload.status)) {
      throw mockError(400, 'INVALID_STATUS_TRANSITION', '当前状态不能执行该操作')
    }
    const now = new Date().toISOString()
    if (payload.admin_notes !== undefined) req.admin_notes = payload.admin_notes || null
    if (payload.status === 'processing') {
      req.processing_at = now
    } else if (payload.status === 'completed') {
      if (!payload.invoice_number || (!payload.invoice_file_url && !req.has_file)) {
        throw mockError(400, 'MISSING_REQUIRED_FIELDS', '请填写发票号码并上传发票文件')
      }
      req.processing_at ??= now
      req.completed_at = now
      req.invoice_number = payload.invoice_number
      if (payload.invoice_file_url) req.invoice_file_url = payload.invoice_file_url
      if (payload.send_email) req.email_sent_at = now
    } else if (payload.status === 'rejected') {
      if (!payload.reject_reason) throw mockError(400, 'MISSING_REQUIRED_FIELDS', '请填写驳回原因')
      req.rejected_at = now
      req.reject_reason = payload.reject_reason
    }
    req.status = payload.status
    return withOrders(req)
  },

  async sendEmail(id) {
    await delay(500)
    const req = find(id)
    if (req.status !== 'completed') throw mockError(400, 'CANNOT_SEND_EMAIL', '发票未开具，不能发送邮件')
    req.email_sent_at = new Date().toISOString()
    return withOrders(req)
  },

  async uploadFile(id, file) {
    await delay(500)
    const req = find(id)
    if (req.status === 'rejected') throw mockError(409, 'INVOICE_FILE_STATUS', '已驳回的申请不能上传发票文件')
    if (file.size === 0) throw mockError(400, 'INVOICE_FILE_REQUIRED', '请选择发票文件')
    if (file.size > 10 * 1024 * 1024) throw mockError(400, 'INVOICE_FILE_TOO_LARGE', '发票文件不能超过 10 MB')
    if (!/\.pdf$/i.test(file.name)) throw mockError(400, 'INVOICE_FILE_NOT_PDF', '只支持 PDF 格式的发票文件')
    req.has_file = true
    req.invoice_file_name = file.name
    req.invoice_file_size = file.size
    return withOrders(req)
  },

  async downloadFile(id) {
    await delay(300)
    const req = find(id)
    if (!req.has_file) throw mockError(404, 'INVOICE_FILE_NOT_FOUND', '发票文件不存在')
    return new Blob(['%PDF-1.4 mock invoice'], { type: 'application/pdf' })
  },

  async getOrderState(orderId) {
    await delay(150)
    const requestId = activeClaim(orderId)
    const req = requests.find((r) => r.id === requestId)
    const excluded = exclusions.has(orderId)
    return {
      invoiced: excluded || requestId !== null,
      excluded,
      request_id: requestId,
      request_status: req?.status ?? null,
      invoice_number: req?.invoice_number ?? null,
    }
  },

  async listExclusions(params) {
    await delay(200)
    const kw = params?.keyword?.trim().toLowerCase()
    const list = [...exclusions.values()].reverse().filter((e) => !kw || `${e.out_trade_no} ${e.user_email}`.toLowerCase().includes(kw))
    return paginate(list, params?.page, params?.page_size ?? 10)
  },

  async addExclusions(orderNumbers) {
    await delay(400)
    const result: AddInvoiceExclusionsResult = { added: [], already_excluded: [], not_found: [], unpaid: [], ambiguous: [] }
    for (const number of orderNumbers) {
      const hit = allOrders().find((o) => o.out_trade_no === number || o.recharge_code === number || String(o.id) === number)
      if (!hit) {
        result.not_found.push(number)
      } else if (exclusions.has(hit.id)) {
        result.already_excluded.push(number)
      } else {
        const entry: InvoiceExclusion = {
          id: exclusions.size + 1,
          order_id: hit.id,
          out_trade_no: hit.out_trade_no,
          user_email: orders.includes(hit) ? ME.email : 'other@example.com',
          pay_amount: hit.pay_amount,
          paid_at: hit.paid_at,
          created_at: new Date().toISOString(),
        }
        exclusions.set(hit.id, entry)
        result.added.push(entry)
      }
    }
    return result
  },

  async removeExclusion(id) {
    await delay(200)
    for (const [orderId, entry] of exclusions) {
      if (entry.id === id) {
        exclusions.delete(orderId)
        return
      }
    }
    throw mockError(404, 'INVOICE_EXCLUSION_NOT_FOUND', '这条排除记录已不存在')
  },

  async listCategories() {
    await delay(200)
    return sortedCategories().map(withCount)
  },

  async createCategory(input) {
    await delay(300)
    const name = (input.name ?? '').trim()
    if (!name) throw mockError(400, 'INVOICE_CATEGORY_NAME_REQUIRED', '请填写类目名称')
    if (categories.some((c) => c.name === name)) throw mockError(409, 'INVOICE_CATEGORY_DUPLICATE', '已经有同名的类目')
    const now = new Date().toISOString()
    const created: AdminInvoiceCategory = {
      id: Math.max(0, ...categories.map((c) => c.id)) + 1,
      name,
      description: (input.description ?? '').trim(),
      enabled: input.enabled ?? true,
      sort_order: input.sort_order ?? Math.max(0, ...categories.map((c) => c.sort_order)) + 1,
      request_count: 0,
      created_at: now,
      updated_at: now,
    }
    categories.push(created)
    return withCount(created)
  },

  async updateCategory(id, input) {
    await delay(300)
    const category = categories.find((c) => c.id === id)
    if (!category) throw mockError(404, 'INVOICE_CATEGORY_NOT_FOUND', '类目不存在')
    const name = input.name === undefined ? category.name : input.name.trim()
    if (!name) throw mockError(400, 'INVOICE_CATEGORY_NAME_REQUIRED', '请填写类目名称')
    if (categories.some((c) => c.id !== id && c.name === name)) throw mockError(409, 'INVOICE_CATEGORY_DUPLICATE', '已经有同名的类目')
    Object.assign(category, {
      name,
      description: input.description === undefined ? category.description : input.description.trim(),
      enabled: input.enabled ?? category.enabled,
      sort_order: input.sort_order ?? category.sort_order,
      updated_at: new Date().toISOString(),
    })
    return withCount(category)
  },

  async deleteCategory(id) {
    await delay(200)
    const index = categories.findIndex((c) => c.id === id)
    if (index < 0) throw mockError(404, 'INVOICE_CATEGORY_NOT_FOUND', '类目不存在')
    if (requests.some((r) => r.category_id === id)) throw mockError(409, 'INVOICE_CATEGORY_IN_USE', '这个类目已有开票申请使用，只能停用，不能删除')
    categories.splice(index, 1)
  },

  async reorderCategories(ids) {
    await delay(200)
    if (ids.length === 0 || new Set(ids).size !== ids.length) throw mockError(400, 'INVOICE_CATEGORY_REORDER_INVALID', '类目排序无效')
    if (ids.some((id) => !categories.some((c) => c.id === id))) throw mockError(404, 'INVOICE_CATEGORY_NOT_FOUND', '类目不存在')
    ids.forEach((id, index) => {
      categories.find((c) => c.id === id)!.sort_order = index + 1
    })
    return sortedCategories().map(withCount)
  },
}
