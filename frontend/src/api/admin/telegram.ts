import { apiClient } from '../client'

export interface TelegramSettings {
  enabled: boolean
  verify_easypay_orders: boolean
  bot_token_configured: boolean
  chat_id: string
}

export interface UpdateTelegramSettingsRequest {
  enabled: boolean
  verify_easypay_orders: boolean
  bot_token?: string
  chat_id: string
  clear_bot_token?: boolean
}

export async function getTelegramSettings(): Promise<TelegramSettings> {
  const { data } = await apiClient.get<TelegramSettings>('/admin/settings/telegram')
  return data
}

export async function updateTelegramSettings(
  request: UpdateTelegramSettingsRequest
): Promise<TelegramSettings> {
  const { data } = await apiClient.put<TelegramSettings>('/admin/settings/telegram', request)
  return data
}

export async function testTelegramSettings(
  request: UpdateTelegramSettingsRequest
): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/settings/telegram/test', request)
  return data
}
