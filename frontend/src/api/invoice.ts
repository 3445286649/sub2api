/**
 * User invoice API
 * 后端尚未接入时，本地设置 VITE_INVOICE_MOCK=true 走内存假数据预览界面（仅 dev 构建生效）。
 */

import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type {
  CreateInvoiceRequest,
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
  getInvoiceableOrders(): Promise<InvoiceableOrdersResponse>
  createRequest(payload: CreateInvoiceRequest): Promise<InvoiceRequest>
  listRequests(params?: InvoiceRequestListParams): Promise<BasePaginationResponse<InvoiceRequest>>
  getRequest(id: number): Promise<InvoiceRequest>
}

const remoteInvoiceAPI: InvoiceAPI = {
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
}

const useMock = import.meta.env.DEV && import.meta.env.VITE_INVOICE_MOCK === 'true'

let mockAPI: Promise<InvoiceAPI> | null = null
function resolveAPI(): Promise<InvoiceAPI> {
  if (!useMock) return Promise.resolve(remoteInvoiceAPI)
  mockAPI ??= import('./invoiceMock').then((m) => m.invoiceMockAPI)
  return mockAPI
}

export const invoiceAPI: InvoiceAPI = {
  getInvoiceableOrders: () => resolveAPI().then((api) => api.getInvoiceableOrders()),
  createRequest: (payload) => resolveAPI().then((api) => api.createRequest(payload)),
  listRequests: (params) => resolveAPI().then((api) => api.listRequests(params)),
  getRequest: (id) => resolveAPI().then((api) => api.getRequest(id)),
}
