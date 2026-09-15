import type { MonitorMatrixBucket } from '@/api/channelMonitorV2'
import type { ProbeBucket } from './types'

export type CardTimelineBucket = MonitorMatrixBucket & { missing?: boolean; incomplete?: boolean; bucket_end?: string; probe?: ProbeBucket; probeModel?: string }

export const cardHealthStyles = {
  healthy: { bar: 'bg-emerald-500', badge: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300', height: 100 },
  warning: { bar: 'bg-amber-500', badge: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300', height: 65 },
  critical: { bar: 'bg-red-500', badge: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300', height: 35 },
  noTraffic: { bar: 'bg-gray-300 dark:bg-dark-600', badge: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300', height: 35 },
  unknown: { bar: 'bg-gray-300 dark:bg-dark-600', badge: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300', height: 35 },
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

export function errorColor(rate: number): string {
  return rate > 0.8 ? '#ef4444' : rate > 0.55 ? '#f59e0b' : '#10b981'
}

export const metricClasses: Record<CardState, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  warning: 'text-amber-600 dark:text-amber-400',
  critical: 'text-red-600 dark:text-red-400',
  unknown: 'text-gray-500', noTraffic: 'text-gray-500',
}
