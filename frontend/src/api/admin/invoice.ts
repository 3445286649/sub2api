/**
 * Admin invoice API
 * 与用户端共用 VITE_INVOICE_MOCK 开关：dev 下开启时走 invoiceMock 里同一份内存数据，两端操作互相可见。
 */

import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type {
  AddInvoiceExclusionsResult,
  AdminInvoiceListParams,
  AdminInvoiceRequest,
  AdminUpdateInvoiceStatus,
  InvoiceExclusion,
  InvoiceExclusionListParams,
  InvoiceOrderState,
  InvoiceStatusCounts,
} from '@/types/invoice'

export interface AdminInvoiceAPI {
  getStats(): Promise<InvoiceStatusCounts>
  listRequests(params?: AdminInvoiceListParams): Promise<BasePaginationResponse<AdminInvoiceRequest>>
  getRequest(id: number): Promise<AdminInvoiceRequest>
  updateStatus(id: number, payload: AdminUpdateInvoiceStatus): Promise<AdminInvoiceRequest>
  sendEmail(id: number): Promise<AdminInvoiceRequest>
  getOrderState(orderId: number): Promise<InvoiceOrderState>
  listExclusions(params?: InvoiceExclusionListParams): Promise<BasePaginationResponse<InvoiceExclusion>>
  addExclusions(orderNumbers: string[]): Promise<AddInvoiceExclusionsResult>
  removeExclusion(id: number): Promise<void>
}

const remoteAdminInvoiceAPI: AdminInvoiceAPI = {
  async getStats() {
    const { data } = await apiClient.get<InvoiceStatusCounts>('/admin/invoice/stats')
    return data
  },
  async listRequests(params) {
    const { data } = await apiClient.get<BasePaginationResponse<AdminInvoiceRequest>>('/admin/invoice/requests', { params })
    return data
  },
  async getRequest(id) {
    const { data } = await apiClient.get<AdminInvoiceRequest>(`/admin/invoice/requests/${id}`)
    return data
  },
  async updateStatus(id, payload) {
    const { data } = await apiClient.patch<AdminInvoiceRequest>(`/admin/invoice/requests/${id}/status`, payload)
    return data
  },
  async sendEmail(id) {
    const { data } = await apiClient.post<AdminInvoiceRequest>(`/admin/invoice/requests/${id}/send-email`)
    return data
  },
  async getOrderState(orderId) {
    const { data } = await apiClient.get<InvoiceOrderState>(`/admin/invoice/orders/${orderId}/state`)
    return data
  },
  async listExclusions(params) {
    const { data } = await apiClient.get<BasePaginationResponse<InvoiceExclusion>>('/admin/invoice/exclusions', { params })
    return data
  },
  async addExclusions(orderNumbers) {
    const { data } = await apiClient.post<AddInvoiceExclusionsResult>('/admin/invoice/exclusions', { order_numbers: orderNumbers })
    return data
  },
  async removeExclusion(id) {
    await apiClient.delete(`/admin/invoice/exclusions/${id}`)
  },
}

/** 发票管理页拿到最新统计后派发，detail 为待处理数，侧边栏据此同步角标 */
export const INVOICE_PENDING_EVENT = 'invoice-pending-changed'

const useMock = import.meta.env.DEV && import.meta.env.VITE_INVOICE_MOCK === 'true'

let mockAPI: Promise<AdminInvoiceAPI> | null = null
function resolveAPI(): Promise<AdminInvoiceAPI> {
  if (!useMock) return Promise.resolve(remoteAdminInvoiceAPI)
  mockAPI ??= import('../invoiceMock').then((m) => m.adminInvoiceMockAPI)
  return mockAPI
}

export const adminInvoiceAPI: AdminInvoiceAPI = {
  getStats: () => resolveAPI().then((api) => api.getStats()),
  listRequests: (params) => resolveAPI().then((api) => api.listRequests(params)),
  getRequest: (id) => resolveAPI().then((api) => api.getRequest(id)),
  updateStatus: (id, payload) => resolveAPI().then((api) => api.updateStatus(id, payload)),
  sendEmail: (id) => resolveAPI().then((api) => api.sendEmail(id)),
  getOrderState: (orderId) => resolveAPI().then((api) => api.getOrderState(orderId)),
  listExclusions: (params) => resolveAPI().then((api) => api.listExclusions(params)),
  addExclusions: (orderNumbers) => resolveAPI().then((api) => api.addExclusions(orderNumbers)),
  removeExclusion: (id) => resolveAPI().then((api) => api.removeExclusion(id)),
}

export default adminInvoiceAPI
