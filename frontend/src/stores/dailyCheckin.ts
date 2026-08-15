import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { dailyCheckinAPI } from '@/api/dailyCheckin'
import type { DailyCheckinRecord, DailyCheckinStatus } from '@/types/dailyCheckin'

export const useDailyCheckinStore = defineStore('dailyCheckin', () => {
  const open = ref(false)
  const loading = ref(false)
  const claiming = ref(false)
  const statusError = ref(false)
  const status = ref<DailyCheckinStatus | null>(null)
  const history = ref<DailyCheckinRecord[]>([])

  const enabled = computed(() => status.value?.config.enabled === true)
  const hasUnreadClaim = computed(() => enabled.value && status.value?.today_claimed === false)

  async function fetchStatus(): Promise<DailyCheckinStatus | null> {
    try {
      const response = await dailyCheckinAPI.getStatus()
      status.value = response.data
      statusError.value = false
      return status.value
    } catch {
      status.value = null
      statusError.value = true
      return null
    }
  }

  async function fetchHistory(): Promise<DailyCheckinRecord[]> {
    try {
      const response = await dailyCheckinAPI.getHistory({ page: 1, page_size: 100 })
      history.value = response.data.items || []
      return history.value
    } catch {
      history.value = []
      return []
    }
  }

  async function show(): Promise<void> {
    open.value = true
    loading.value = true
    try {
      await Promise.all([fetchStatus(), fetchHistory()])
    } finally {
      loading.value = false
    }
  }

  function close(): void {
    open.value = false
  }

  async function claim(): Promise<DailyCheckinRecord> {
    claiming.value = true
    try {
      const response = await dailyCheckinAPI.claim()
      await Promise.all([fetchStatus(), fetchHistory()])
      return response.data
    } finally {
      claiming.value = false
    }
  }

  return { open, loading, claiming, statusError, status, history, enabled, hasUnreadClaim, fetchStatus, fetchHistory, show, close, claim }
})
