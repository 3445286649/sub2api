/**
 * User invoice API
 * 后端尚未接入时，本地设置 VITE_INVOICE_MOCK=true 走内存假数据预览界面（仅 dev 构建生效）。
 */

import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type {
  CreateInvoiceRequest,
  InvoiceCategoryOption,
  InvoiceableOrdersResponse,
  InvoiceRequest,
  InvoiceRequestStatus,
} from '@/types/invoice'

export interface InvoiceRequestListParams {
  page?: number
  page_size?: number
  status?: InvoiceRequestStatus
}

export interface InvoiceAPI {
  /** 已启用的开票类目，一个都没配置时为空数组 */
  getCategories(): Promise<InvoiceCategoryOption[]>
  getInvoiceableOrders(): Promise<InvoiceableOrdersResponse>
  createRequest(payload: CreateInvoiceRequest): Promise<InvoiceRequest>
  listRequests(params?: InvoiceRequestListParams): Promise<BasePaginationResponse<InvoiceRequest>>
  getRequest(id: number): Promise<InvoiceRequest>
  /** 只有申请人本人、且发票已开具时才能下载 */
  downloadFile(id: number): Promise<Blob>
}

const remoteInvoiceAPI: InvoiceAPI = {
  async getCategories() {
    const { data } = await apiClient.get<InvoiceCategoryOption[]>('/invoice/categories')
    return data
  },
  async getInvoiceableOrders() {
    const { data } = await apiClient.get<InvoiceableOrdersResponse>('/invoice/orders')
    return data
  },
  async createRequest(payload) {
    const { data } = await apiClient.post<InvoiceRequest>('/invoice/requests', payload)
    return data
  },
  async listRequests(params) {
    const { data } = await apiClient.get<BasePaginationResponse<InvoiceRequest>>('/invoice/requests', { params })
    return data
  },
  async getRequest(id) {
    const { data } = await apiClient.get<InvoiceRequest>(`/invoice/requests/${id}`)
    return data
  },
  async downloadFile(id) {
    const { data } = await apiClient.get<Blob>(`/invoice/requests/${id}/file`, { responseType: 'blob' })
    return data
  },
}

const useMock = import.meta.env.DEV && import.meta.env.VITE_INVOICE_MOCK === 'true'

let mockAPI: Promise<InvoiceAPI> | null = null
function resolveAPI(): Promise<InvoiceAPI> {
  if (!useMock) return Promise.resolve(remoteInvoiceAPI)
  mockAPI ??= import('./invoiceMock').then((m) => m.invoiceMockAPI)
  return mockAPI
}

export const invoiceAPI: InvoiceAPI = {
  getCategories: () => resolveAPI().then((api) => api.getCategories()),
  getInvoiceableOrders: () => resolveAPI().then((api) => api.getInvoiceableOrders()),
  createRequest: (payload) => resolveAPI().then((api) => api.createRequest(payload)),
  listRequests: (params) => resolveAPI().then((api) => api.listRequests(params)),
  getRequest: (id) => resolveAPI().then((api) => api.getRequest(id)),
  downloadFile: (id) => resolveAPI().then((api) => api.downloadFile(id)),
}
