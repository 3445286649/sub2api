import { apiClient } from '../client'
import type {
  BasePaginationResponse,
  SupportMessageListFilters,
  SupportTicket,
  SupportTicketMessage,
  UpdateSupportTicketRequest
} from '@/types'

export async function list(
  page = 1,
  pageSize = 20,
  filters?: {
    status?: string
    category?: string
    priority?: string
    search?: string
    unread_only?: boolean
  }
): Promise<BasePaginationResponse<SupportTicket>> {
  const { data } = await apiClient.get<BasePaginationResponse<SupportTicket>>('/admin/support/tickets', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getById(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.get<SupportTicket>(`/admin/support/tickets/${id}`)
  return data
}

export async function listMessages(
  id: number,
  filters?: SupportMessageListFilters
): Promise<SupportTicketMessage[]> {
  const { data } = await apiClient.get<SupportTicketMessage[]>(`/admin/support/tickets/${id}/messages`, {
    params: filters
  })
  return data
}

export async function sendMessage(id: number, content: string): Promise<SupportTicketMessage> {
  const { data } = await apiClient.post<SupportTicketMessage>(`/admin/support/tickets/${id}/messages`, { content })
  return data
}

export async function markRead(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/support/tickets/${id}/read`)
  return data
}

export async function update(id: number, request: UpdateSupportTicketRequest): Promise<SupportTicket> {
  const { data } = await apiClient.put<SupportTicket>(`/admin/support/tickets/${id}`, request)
  return data
}

export async function close(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>(`/admin/support/tickets/${id}/close`)
  return data
}

export async function reopen(
  id: number,
  content = ''
): Promise<{ ticket: SupportTicket; message?: SupportTicketMessage | null }> {
  const { data } = await apiClient.post<{ ticket: SupportTicket; message?: SupportTicketMessage | null }>(
    `/admin/support/tickets/${id}/reopen`,
    { content }
  )
  return data
}

const supportAPI = {
  list,
  getById,
  listMessages,
  sendMessage,
  markRead,
  update,
  close,
  reopen
}

export default supportAPI
