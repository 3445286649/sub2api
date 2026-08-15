import { apiClient } from '../client'
import type { DailyCheckinConfig, DailyCheckinPage, DailyCheckinStats } from '@/types/dailyCheckin'

export const adminDailyCheckinAPI = {
  getConfig() {
    return apiClient.get<DailyCheckinConfig>('/admin/daily-checkin/config')
  },
  updateConfig(data: DailyCheckinConfig) {
    return apiClient.put<DailyCheckinConfig>('/admin/daily-checkin/config', data)
  },
  getStats() {
    return apiClient.get<DailyCheckinStats>('/admin/daily-checkin/stats')
  },
  getRecords(params?: { page?: number; page_size?: number; date?: string }) {
    return apiClient.get<DailyCheckinPage>('/admin/daily-checkin/records', { params })
  }
}
