import type { UsageLog } from '@/types'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_VIDEO,
  getDisplayBillingMode,
} from '@/utils/billingMode'

type UsageTokenSpeedRow = Pick<
  UsageLog,
  'billing_mode' | 'duration_ms' | 'first_token_ms' | 'image_count' | 'output_tokens' | 'stream'
>

export const calculateUsageTokenSpeed = (row: UsageTokenSpeedRow): number | null => {
  const billingMode = getDisplayBillingMode(row)
  if (billingMode === BILLING_MODE_IMAGE || billingMode === BILLING_MODE_VIDEO) return null

  if (!Number.isFinite(row.output_tokens) || row.output_tokens <= 0) return null
  if (!Number.isFinite(row.duration_ms) || row.duration_ms == null || row.duration_ms <= 0) return null

  let generationDurationMs = row.duration_ms
  if (row.stream) {
    if (!Number.isFinite(row.first_token_ms) || row.first_token_ms == null) return null
    generationDurationMs -= row.first_token_ms
  }

  if (generationDurationMs <= 0) return null

  const speed = row.output_tokens / (generationDurationMs / 1000)
  return Number.isFinite(speed) && speed > 0 ? speed : null
}

export const formatUsageTokenSpeed = (row: UsageTokenSpeedRow): string => {
  const speed = calculateUsageTokenSpeed(row)
  if (speed == null) return '-'

  const rounded = speed < 10 ? Math.round(speed * 10) / 10 : Math.round(speed)
  return `${rounded.toLocaleString()} Token/s`
}
