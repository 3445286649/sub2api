import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMonitorPageCache } from '../monitorPageCache'
import type { MonitorFilter, MonitorMatrixResponse, MonitorSnapshot } from '@/api/channelMonitorV2'

const filter: MonitorFilter = { range: '90m', platforms: ['openai', 'anthropic'], groupIds: [2, 1], models: [] }
const value = (version = 1, bootstrap = false) => ({
  snapshot: { config: { version, refresh_interval_seconds: 300 }, coverage: { bootstrap: { active: bootstrap } } } as MonitorSnapshot,
  matrix: { items: [] } as unknown as MonitorMatrixResponse,
})
afterEach(() => vi.useRealTimers())
describe('monitor page cache', () => {
  it('reuses normalized filters and isolates viewers, roles, ranges and dimensions', () => {
    const cache = createMonitorPageCache()
    const key = cache.key(1, false, filter, 'platform_group')
    cache.set(key, value())
    expect(cache.get(cache.key(1, false, { ...filter, platforms: [...filter.platforms].reverse(), groupIds: [1, 2] }, 'platform_group'))).toBeDefined()
    expect(cache.get(cache.key(2, false, filter, 'platform_group'))).toBeUndefined()
    expect(cache.get(cache.key(1, true, filter, 'platform_group'))).toBeUndefined()
    expect(cache.get(cache.key(1, false, { ...filter, range: '24h' }, 'platform_group'))).toBeUndefined()
    expect(cache.get(cache.key(1, false, filter, 'platform_model'))).toBeUndefined()
  })
  it('expires in 60 seconds, or 10 seconds during bootstrap', () => {
    vi.useFakeTimers()
    const cache = createMonitorPageCache()
    cache.set('normal', value())
    cache.set('bootstrap', value(1, true))
    vi.advanceTimersByTime(10_000)
    expect(cache.get('bootstrap')).toBeUndefined()
    expect(cache.get('normal')).toBeDefined()
    vi.advanceTimersByTime(50_000)
    expect(cache.get('normal')).toBeUndefined()
  })
  it('bounds memory and invalidates old config versions and cleared sessions', () => {
    const cache = createMonitorPageCache()
    for (let i = 0; i < 9; i++) cache.set(String(i), value())
    expect(cache.get('0')).toBeUndefined()
    expect(cache.get('1')).toBeDefined()
    cache.set('new', value(2))
    expect(cache.get('1')).toBeUndefined()
    expect(cache.get('new')).toBeDefined()
    cache.clear()
    expect(cache.get('new')).toBeUndefined()
  })
})
