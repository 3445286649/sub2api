import type { MonitorFilter, MonitorMatrixGroupBy, MonitorMatrixResponse, MonitorSnapshot } from '@/api/channelMonitorV2'

interface MetricsEntry { snapshot: MonitorSnapshot; matrix: MonitorMatrixResponse }

// Owned by a mounted page, never persisted or shared between sessions.
export function createMonitorPageCache() {
  const entries = new Map<string, { value: MetricsEntry; expires: number }>()
  let version: number | undefined
  return {
    key(userId: number | undefined, admin: boolean, filter: MonitorFilter, groupBy: MonitorMatrixGroupBy) {
      return JSON.stringify([userId, admin, filter.range, [...filter.platforms].sort(), [...filter.groupIds].sort((a, b) => a - b), [...filter.models].sort(), groupBy])
    },
    get(key: string): MetricsEntry | undefined {
      const entry = entries.get(key)
      if (!entry || entry.expires <= Date.now()) { entries.delete(key); return undefined }
      return entry.value
    },
    set(key: string, value: MetricsEntry) {
      if (version !== value.snapshot.config.version) entries.clear()
      version = value.snapshot.config.version
      entries.delete(key)
      if (entries.size >= 8) entries.delete(entries.keys().next().value!)
      const ttl = value.snapshot.coverage.bootstrap?.active ? 10 : Math.min(60, value.snapshot.config.refresh_interval_seconds)
      entries.set(key, { value, expires: Date.now() + ttl * 1000 })
    },
    clear() { entries.clear(); version = undefined },
  }
}
