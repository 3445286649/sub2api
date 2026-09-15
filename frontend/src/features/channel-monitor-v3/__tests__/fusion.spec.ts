import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import MonitorCardGrid from '../MonitorCardGrid.vue'
import type { V3CardRow } from '../types'
import type { MonitorCoverage } from '@/api/channelMonitorV2'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key, locale: ref('en-US') }) }))
const coverage = { requested_start: '2026-09-15T12:00:00Z', requested_end: '2026-09-15T12:15:00Z', coverage_start: '2026-09-15T12:00:00Z', data_through: '2026-09-15T12:15:00Z', bucket_seconds: 300 } as MonitorCoverage
function fixture(): V3CardRow {
  return {
    monitor_id: 1, group_id: 1, platform: 'openai', group_name: 'Fusion card', probe_model: 'primary-model',
    metrics: { traffic_state: 'no_traffic', error_rate: 0, cache_rate: 0, ttft: { p50_ms: null } },
    health: { error_rate: 'unknown', cache: 'unknown' }, buckets: [],
    probe_buckets: [{ bucket_start: '2026-09-15T12:00:00Z', success_count: 1, failure_count: 0, last_checked_at: '2026-09-15T12:02:00Z' }],
  } as V3CardRow
}
function make(item = fixture()) {
  return mount(MonitorCardGrid, { props: { rows: [item], coverage }, global: { stubs: { ProviderIcon: true, EmptyState: true } } })
}
describe('V3 fusion timeline', () => {
  it('fills a successful probe green only in its own interval without changing real summaries', async () => {
    const w = make()
    const bars = w.findAll('.v3-soft-glass-bar')
    expect(bars[0].classes()).toContain('bg-emerald-500')
    expect(bars[0].classes()).toContain('is-probe')
    expect(bars[1].classes()).toContain('is-empty')
    expect(bars[1].attributes('style')).toContain('height: 100%')
    expect(w.find('[data-testid="channel-status-health-badge"]').text()).toContain('noTraffic')
    expect(w.findAll('article .text-lg').map(e => e.text())).toEqual(['-', '-', '-'])
    await w.findAll('.v3-bar-hitbox')[0].trigger('click')
    expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('probeSource')
    expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('primary-model')
    expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('100.0%')
    w.unmount()
  })
  it.each([[9, 11, 'bg-emerald-500'], [4, 16, 'bg-amber-500'], [3, 17, 'bg-red-500'], [0, 1, 'bg-red-500']])('colors probe samples %s/%s correctly', (success, failure, color) => {
    const row = fixture()
    Object.assign(row.probe_buckets![0], { success_count: success, failure_count: failure })
    const w = make(row)
    expect(w.find('.v3-soft-glass-bar').classes()).toContain(color)
    w.unmount()
  })
  it('never lets a successful probe overwrite failed traffic or unresolved aggregation', async () => {
    const row = fixture()
    row.metrics.traffic_state = 'traffic'
    row.metrics.error_rate = 1
    row.buckets = [{ bucket_start: coverage.requested_start, metrics: row.metrics, health: row.health }]
    const w = make(row)
    expect(w.find('.v3-soft-glass-bar').classes()).toContain('bg-red-500')
    expect(w.find('.is-probe').exists()).toBe(false)
    await w.setProps({ rows: [fixture()], coverage: { ...coverage, data_through: coverage.requested_start } })
    expect(w.find('.is-probe').exists()).toBe(false)
    expect(w.findAll('.v3-bar-slot')).toHaveLength(0)
    w.unmount()
  })
  it('preserves slots and their entrance delay on refresh; only new intervals have zero delay', async () => {
    const row = fixture()
    const w = make(row)
    const slots = w.findAll('.v3-bar-slot')
    expect(slots[0].attributes('style')).toContain('0ms')
    expect(slots[2].attributes('style')).toContain('300ms')
    await w.setProps({ rows: [{ ...row, probe_buckets: [{ ...row.probe_buckets![0], failure_count: 1 }] }] })
    expect(w.findAll('.v3-bar-slot')[0].element).toBe(slots[0].element)
    await w.setProps({ coverage: { ...coverage, requested_start: '2026-09-15T12:05:00Z', requested_end: '2026-09-15T12:20:00Z', data_through: '2026-09-15T12:20:00Z' } })
    expect(w.findAll('.v3-bar-slot')[0].element).toBe(slots[1].element)
    expect(w.findAll('.v3-bar-slot')[0].attributes('style')).toContain('150ms')
    expect(w.findAll('.v3-bar-slot')[2].attributes('style')).toContain('0ms')
    w.unmount()
  })
  it('hides trailing pending intervals and appends new data on the right when coverage advances', async () => {
    const w = make()
    await w.setProps({ coverage: { ...coverage, data_through: '2026-09-15T12:11:00Z' } })
    expect(w.findAll('.v3-bar-slot')).toHaveLength(2)
    expect(w.find('.is-pending').exists()).toBe(false)
    const first = w.findAll('.v3-bar-slot')[0].element
    const second = w.findAll('.v3-bar-slot')[1].element
    await w.setProps({ coverage })
    expect(w.findAll('.v3-bar-slot')).toHaveLength(3)
    expect(w.findAll('.v3-bar-slot')[0].element).toBe(first)
    expect(w.findAll('.v3-bar-slot')[1].element).toBe(second)
    expect(w.findAll('.v3-bar-slot')[2].attributes('style')).toContain('0ms')
    expect(w.findAll('.is-empty')).toHaveLength(2)
    w.unmount()
  })
})
