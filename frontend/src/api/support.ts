import { apiClient } from './client'
import type {
  BasePaginationResponse,
  CreateSupportTicketRequest,
  SupportMessageListFilters,
  SupportTicket,
  SupportTicketMessage
} from '@/types'

export async function list(
  page = 1,
  pageSize = 20,
  filters?: { status?: string; unread_only?: boolean }
): Promise<BasePaginationResponse<SupportTicket>> {
  const { data } = await apiClient.get<BasePaginationResponse<SupportTicket>>('/support/tickets', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function create(request: CreateSupportTicketRequest): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>('/support/tickets', request)
  return data
}

export async function getById(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.get<SupportTicket>(`/support/tickets/${id}`)
  return data
}

export async function listMessages(
  id: number,
  filters?: SupportMessageListFilters
): Promise<SupportTicketMessage[]> {
  const { data } = await apiClient.get<SupportTicketMessage[]>(`/support/tickets/${id}/messages`, {
    params: filters
  })
  return data
}

export async function sendMessage(id: number, content: string): Promise<SupportTicketMessage> {
  const { data } = await apiClient.post<SupportTicketMessage>(`/support/tickets/${id}/messages`, { content })
  return data
}

export async function markRead(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/support/tickets/${id}/read`)
  return data
}

export async function close(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>(`/support/tickets/${id}/close`)
  return data
}

export async function reopen(
  id: number,
  content = ''
): Promise<{ ticket: SupportTicket; message?: SupportTicketMessage | null }> {
  const { data } = await apiClient.post<{ ticket: SupportTicket; message?: SupportTicketMessage | null }>(
    `/support/tickets/${id}/reopen`,
    { content }
  )
  return data
}

const supportAPI = {
  list,
  create,
  getById,
  listMessages,
  sendMessage,
  markRead,
  close,
  reopen
}

export default supportAPI
