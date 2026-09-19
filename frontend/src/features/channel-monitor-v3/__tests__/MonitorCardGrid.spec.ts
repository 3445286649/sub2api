import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import MonitorCardGrid from '@/features/channel-monitor-v3/MonitorCardGrid.vue'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, values?: Record<string, unknown>) => values ? `${key} ${JSON.stringify(values)}` : key, locale: ref('en-US') }),
}))
const coverage = { coverage_start: '2026-09-14T00:00:00Z', requested_start: '2026-09-14T00:00:00Z', requested_end: '2026-09-14T00:15:00Z', data_through: '2026-09-14T00:15:00Z', bucket_seconds: 300 } as MonitorCoverage
const row = (platform = 'openai', group_id = 1): MonitorMatrixRow => ({
  platform, group_id, group_name: `Group ${group_id}`,
  metrics: { traffic_state: 'traffic', error_rate: 0.1, cache_rate: 0.5, request_count: 0, ttft: { p50_ms: 300 } },
  health: { overall: 'healthy', error_rate: 'healthy', cache: 'healthy', ttft: 'healthy', minimum_sample: 10 }, buckets: [],
} as unknown as MonitorMatrixRow)
const make = (rows: MonitorMatrixRow[]) => mount(MonitorCardGrid, { props: { rows, coverage }, global: { stubs: { ProviderIcon: true, EmptyState: true } } })

describe('passive monitor cards', () => {
  it('keeps the latest 18 minutes including empty slots and replaces the oldest on refresh', async () => {
    const item = row()
    item.buckets = [{ bucket_start: '2026-09-14T01:28:00Z', metrics: item.metrics, health: item.health }]
    const minuteCoverage = { ...coverage, bucket_seconds: 60, requested_end: '2026-09-14T01:30:00Z', data_through: '2026-09-14T01:30:00Z' }
    const w = make([item])
    await w.setProps({ coverage: minuteCoverage, completedWindow: true })
    const slots = w.findAll('.v3-bar-slot')
    expect(slots).toHaveLength(18)
    expect(w.findAll('.is-empty')).toHaveLength(17)
    expect(w.findAll('.bg-emerald-500')).toHaveLength(1)
    expect(slots[0].attributes('aria-label')).toContain(new Date('2026-09-14T01:12:00Z').toLocaleString('en-US'))
    const article = w.find('article').element
    const retained = slots[1].element
    await w.setProps({ coverage: { ...minuteCoverage, requested_start: '2026-09-14T00:01:00Z', requested_end: '2026-09-14T01:31:00Z', data_through: '2026-09-14T01:31:00Z' } })
    expect(w.findAll('.v3-bar-slot')).toHaveLength(18)
    expect(w.find('article').element).toBe(article)
    expect(w.find('.v3-bar-slot').element).toBe(retained)
    expect(w.findAll('.v3-bar-slot')[17].attributes('aria-label')).toContain(new Date('2026-09-14T01:30:00Z').toLocaleString('en-US'))
    expect(w.findAll('.is-empty')).toHaveLength(17)
    w.unmount()
  })
  it.each([
    [0.55, 'healthy', '100%'], [0.5501, 'warning', '65%'],
    [0.8, 'warning', '65%'], [0.8001, 'critical', '35%'],
  ])('uses availability boundaries for error rate %s', (errorRate, _state, height) => {
    const item = row()
    item.metrics.error_rate = errorRate as number
    item.buckets = [{ bucket_start: '2026-09-14T00:10:00Z', metrics: item.metrics, health: item.health }]
    const w = make([item])
    expect(w.find('[data-testid="channel-status-health-badge"]').exists()).toBe(false)
    expect(w.findAll('.v3-soft-glass-bar')[2].attributes('style')).toContain(`height: ${height}`)
    w.unmount()
  })
  it('uses TTFT independently and preserves card and bucket DOM on new data', async () => {
    const item = row()
    item.metrics.ttft.p50_ms = 12900
    item.buckets = [{ bucket_start: '2026-09-14T00:10:00Z', metrics: item.metrics, health: item.health }]
    const w = make([item])
    const article = w.find('article').element
    const bar = w.findAll('.v3-bar-hitbox')[2].element
    expect(w.find('[data-testid="channel-status-health-badge"]').exists()).toBe(false)
    expect(w.findAll('article .text-lg')[1].classes()).toContain('text-emerald-600')
    expect(w.findAll('article .text-lg')[2].classes()).toContain('text-gray-900')
    await w.setProps({ rows: [{ ...item, metrics: { ...item.metrics, error_rate: .2 } }], coverage: { ...coverage, requested_start: '2026-09-14T00:05:00Z', requested_end: '2026-09-14T00:20:00Z', data_through: '2026-09-14T00:20:00Z' } })
    expect(w.find('article').element).toBe(article)
    expect(w.findAll('.v3-bar-hitbox')[1].element).toBe(bar)
    expect(w.text()).not.toContain('nextUpdateIn')
    w.unmount()
  })
  it('groups and orders cards, filters locally and resets a removed category', async () => {
    const w = make([row('anthropic', 3), row('openai', 2), row('openai', 1)])
    expect(w.findAll('article .line-clamp-2').map(node => node.text())).toEqual(['Group 1', 'Group 2', 'Group 3'])
    expect(w.findAll('nav button')[0].text()).toContain('(3)')
    await w.findAll('nav button')[2].trigger('click')
    expect(w.findAll('article')).toHaveLength(1)
    await w.setProps({ rows: [row()] })
    expect(w.findAll('nav button')[0].attributes('aria-pressed')).toBe('true')
    w.unmount()
  })
  it('keeps range summaries without hover, click or focus popups', async () => {
    const item = row()
    item.buckets = [{ bucket_start: '2026-09-14T00:10:00Z', metrics: { ...item.metrics, error_rate: 0 }, health: item.health }]
    const w = make([item])
    expect(w.findAll('article .text-lg').map(node => node.text())).toEqual(['50.0%', '90.0%', '300ms'])
    const bars = w.findAll('.v3-bar-hitbox')
    expect(bars).toHaveLength(3)
    expect(w.findAll('.v3-soft-glass-bar')[0].classes()).toContain('bg-gray-300')
    await w.findAll('.v3-bar-slot')[2].trigger('mouseenter')
    expect(document.querySelector('[role="tooltip"]')).toBeNull()
    await w.find('.v3-timeline-bars').trigger('mouseleave')
    await bars[2].trigger('focus')
    expect(document.querySelector('[role="tooltip"]')).toBeNull()
    w.unmount()
  })
  it('does not report no samples as successful or render names as HTML', () => {
    const item = row()
    item.group_name = '<img src=x onerror=alert(1)>'
    item.metrics.traffic_state = undefined
    item.health = { ...item.health, overall: 'unknown', error_rate: 'unknown', cache: 'unknown' }
    const w = make([item])
    expect(w.find('img').exists()).toBe(false)
    expect(w.findAll('article .text-lg').map(node => node.text())).toEqual(['-', '-', '-'])
    expect(w.findAll('.is-empty')).toHaveLength(3)
    w.unmount()
  })
  it('keeps usable traffic green and distinguishes complete empty intervals', async () => {
    const item = row()
    item.health = { ...item.health, overall: 'critical' }
    item.buckets = [{ bucket_start: '2026-09-14T00:10:00Z', metrics: item.metrics, health: item.health }]
    const w = make([item])
    expect(w.find('[data-testid="channel-status-health-badge"]').exists()).toBe(false)
    expect(w.findAll('.v3-soft-glass-bar')[2].classes()).toContain('bg-emerald-500')
    await w.findAll('.v3-bar-hitbox')[0].trigger('click')
    expect(document.querySelector('[role="tooltip"]')).toBeNull()
    w.unmount()
  })

  it('shows success as green despite low cache health, and pending coverage separately from no traffic', async () => {
    const item = row()
    item.health = { ...item.health, overall: 'critical', error_rate: 'unknown' }
    item.buckets = [{ bucket_start: '2026-09-14T00:10:00Z', metrics: { ...item.metrics, error_rate: 0 }, health: item.health }]
    const w = make([item])
    expect(w.find('[data-testid="channel-status-health-badge"]').exists()).toBe(false)
    expect(w.findAll('.v3-soft-glass-bar')[2].classes()).toContain('bg-emerald-500')
    await w.findAll('.v3-bar-hitbox')[2].trigger('click')
    expect(document.querySelector('[role="tooltip"]')).toBeNull()
    await w.setProps({ coverage: { ...coverage, data_through: '2026-09-14T00:05:00Z' } })
    expect(w.findAll('.v3-bar-hitbox')).toHaveLength(1)
    expect(w.find('.is-pending').exists()).toBe(false)
    w.unmount()
  })
})
