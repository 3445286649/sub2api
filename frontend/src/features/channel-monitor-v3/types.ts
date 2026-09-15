import type { MonitorMatrixRow } from '@/api/channelMonitorV2'

export interface V3CardConfig {
  v3_fusion_enabled?: boolean
  probe_model?: string
  probe_buckets?: ProbeBucket[]
  id: number
  name: string
  provider: string
  display_category: string
  display_order: number
  v3_group_id: number | null
}

export type V3CardRow = MonitorMatrixRow & {
  probe_model?: string
  probe_buckets?: ProbeBucket[]
  monitor_id?: number
  display_category?: string
  display_order?: number
  unlinked?: boolean
}

export interface ProbeBucket {
  bucket_start: string
  success_count: number
  failure_count: number
  last_checked_at: string
}
