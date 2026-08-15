import { apiClient } from './client'
import type { DailyCheckinPage, DailyCheckinRecord, DailyCheckinStatus } from '@/types/dailyCheckin'

export const dailyCheckinAPI = {
  getStatus() {
    return apiClient.get<DailyCheckinStatus>('/daily-checkin/status')
  },
  claim() {
    return apiClient.post<DailyCheckinRecord>('/daily-checkin/claim')
  },
  getHistory(params?: { page?: number; page_size?: number }) {
    return apiClient.get<DailyCheckinPage>('/daily-checkin/history', { params })
  }
}
