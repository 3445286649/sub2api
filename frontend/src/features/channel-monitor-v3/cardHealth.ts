import type { MonitorMatrixBucket } from '@/api/channelMonitorV2'
import type { ProbeBucket } from './types'

export type CardTimelineBucket = MonitorMatrixBucket & { missing?: boolean; incomplete?: boolean; bucket_end?: string; probe?: ProbeBucket; probeModel?: string }

export const cardHealthStyles = {
  healthy: { bar: 'bg-emerald-500', height: 100 },
  warning: { bar: 'bg-amber-500', height: 65 },
  critical: { bar: 'bg-red-500', height: 35 },
  noTraffic: { bar: 'bg-gray-300 dark:bg-dark-600', height: 100 },
  unknown: { bar: 'bg-gray-300 dark:bg-dark-600', height: 100 },
} as const

export type CardState = 'healthy' | 'warning' | 'critical' | 'unknown' | 'noTraffic'

export function cardState(bucket?: CardTimelineBucket): CardState {
  if (!bucket || bucket.incomplete || bucket.missing) return 'unknown'
  if (bucket.metrics.traffic_state !== 'traffic' && bucket.probe) {
    const { success_count: success, failure_count: failure } = bucket.probe
    if (success >= 0 && failure >= 0 && success + failure > 0) {
      const rate = failure / (success + failure)
      return rate > 0.8 ? 'critical' : rate > 0.55 ? 'warning' : 'healthy'
    }
  }
  if (bucket.metrics.traffic_state === 'no_traffic') return 'noTraffic'
  if (bucket.metrics.traffic_state !== 'traffic' || !Number.isFinite(bucket.metrics.error_rate)) return 'unknown'
  return bucket.metrics.error_rate > 0.8 ? 'critical' : bucket.metrics.error_rate > 0.55 ? 'warning' : 'healthy'
}

export const metricClasses: Record<CardState, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  warning: 'text-amber-600 dark:text-amber-400',
  critical: 'text-red-600 dark:text-red-400',
  unknown: 'text-gray-500', noTraffic: 'text-gray-500',
}
