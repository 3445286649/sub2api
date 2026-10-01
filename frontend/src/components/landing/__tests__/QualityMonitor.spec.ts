import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import QualityMonitor from '../QualityMonitor.vue'
import { getQualityMonitor, type QualityMonitorResponse } from '@/api/qualityMonitor'
import zh from '@/i18n/locales/zh/landing'

vi.mock('@/api/qualityMonitor', () => ({ getQualityMonitor: vi.fn() }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string, params: Record<string, unknown> = {}) => {
      const message = key.split('.').reduce<unknown>((value, part) =>
        (value as Record<string, unknown>)[part], zh)
      return String(message).replace(/\{(\w+)\}/g, (_, name: string) => String(params[name]))
    },
  }),
}))
enableAutoUnmount(afterEach)
afterEach(() => {
  vi.resetAllMocks()
  vi.useRealTimers()
})

function response(): QualityMonitorResponse {
  return {
    enabled: true,
    available: true,
    data: {
      detail_url: 'https://monitor.example.com',
      updated_at: '2026-10-01T12:00:00Z',
      stale: false,
      groups: [
        {
          id: 'channel-1', name: 'Pro尊享分组', model: 'gpt-6-astra',
          reasoning_effort: 'medium', interval_seconds: 1200,
          next_run_at: '2026-10-01T12:20:00Z',
          pass_rate: 0.9375, passed: 15, failed: 1, errored: 1,
          latest_status: 'failed',
          history: [
            ...Array.from({ length: 15 }, (_, index) => ({ s: 'passed' as const, t: `2026-10-01T11:${String(index).padStart(2, '0')}:00Z` })),
            { s: 'error', t: '2026-10-01T11:30:00Z' },
            { s: 'failed', t: '2026-10-01T12:00:00Z' },
          ],
        },
        {
          id: 'channel-2', name: 'Pro独享分组', model: 'other-model',
          reasoning_effort: 'high', interval_seconds: 600,
          pass_rate: 1, passed: 1, failed: 0, errored: 0,
          latest_status: 'passed',
          history: [{ s: 'passed', t: '2026-10-01T12:00:00Z' }],
        },
      ],
    },
  }
}

function mountMonitor() {
  return mount(QualityMonitor, {
    global: {
      stubs: { Icon: true },
    },
  })
}

describe('QualityMonitor v1 groups in the original landing layout', () => {
  it('renders each group with its own metadata, accurate percentages and real history', async () => {
    vi.mocked(getQualityMonitor).mockResolvedValue(response())
    const wrapper = mountMonitor()
    await flushPromises()
    const rows = wrapper.findAll('.qm-row')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('Pro尊享分组')
    expect(rows[0].text()).toContain('gpt-6-astra')
    expect(rows[0].text()).toContain('medium')
    expect(rows[0].text()).toContain('每 20 分钟检测一次')
    expect(rows[0].text()).toContain('93.7%')
    expect(rows[0].text()).toContain('通过 15 次')
    expect(rows[0].text()).toContain('最新: 疑似降智')
    expect(rows[0].findAll('.qm-bar i')).toHaveLength(48)
    expect(rows[0].findAll('.qm-bar i.is-empty')).toHaveLength(31)
    expect(rows[0].findAll('.qm-bar i').at(-1)?.attributes('title')).toContain('疑似降智')
    expect(rows[1].text()).toContain('Pro独享分组')
    expect(rows[1].text()).toContain('other-model')
    expect(rows[1].text()).toContain('high')
    expect(rows[1].text()).toContain('每 10 分钟检测一次')
    expect(rows[1].text()).toContain('100%')
    expect(rows[1].findAll('.qm-bar i')).toHaveLength(48)
    expect(rows[1].findAll('.qm-bar i.is-empty')).toHaveLength(47)
    expect(wrapper.text()).not.toMatch(/鹈鹕|生成测试/)
    expect(wrapper.emitted('loaded')).toEqual([[true]])
  })

  it('marks cached data as stale and does not invent results or a schedule for an empty group', async () => {
    const data = response()
    data.data!.stale = true
    Object.assign(data.data!.groups[0], {
      history: [], pass_rate: null, passed: 0, failed: 0, errored: 0,
      interval_seconds: 0, latest_status: undefined, next_run_at: undefined,
    })
    vi.mocked(getQualityMonitor).mockResolvedValue(data)
    const wrapper = mountMonitor()
    await flushPromises()
    const row = wrapper.get('.qm-row')
    expect(row.text()).toContain('—')
    expect(row.text()).toContain('监测数据暂不可用')
    expect(row.text()).not.toContain('分钟检测一次')
    expect(row.findAll('.qm-bar i.is-empty')).toHaveLength(48)
    expect(row.findAll('.qm-bar i:not(.is-empty)')).toHaveLength(0)
    expect(wrapper.text()).toContain('同步暂时失败')
  })

  it('keeps only the latest 48 real results in chronological order', async () => {
    const data = response()
    data.data!.groups[0].history = Array.from({ length: 60 }, (_, index) => ({
      s: index < 12 ? 'failed' : 'passed',
      t: new Date(Date.UTC(2026, 9, 1, 10, index)).toISOString(),
    }))
    vi.mocked(getQualityMonitor).mockResolvedValue(data)
    const wrapper = mountMonitor()
    await flushPromises()
    const row = wrapper.get('.qm-row')
    expect(row.findAll('.qm-bar i')).toHaveLength(48)
    expect(row.findAll('.qm-bar i.is-passed')).toHaveLength(48)
    expect(row.findAll('.qm-bar i.is-empty')).toHaveLength(0)
    const localTime = (minute: number) => new Date(Date.UTC(2026, 9, 1, 10, minute))
      .toLocaleTimeString('zh', { hour: '2-digit', minute: '2-digit' })
    expect(row.findAll('.qm-bar i')[0].attributes('title')).toContain(localTime(12))
    expect(row.findAll('.qm-bar i').at(-1)?.attributes('title')).toContain(localTime(59))
  })

  it('refreshes in place, shifts existing results left and preserves them on a temporary failure', async () => {
    vi.useFakeTimers()
    vi.mocked(getQualityMonitor).mockResolvedValue(response())
    const wrapper = mountMonitor()
    await flushPromises()
    const last = wrapper.findAll('.qm-row')[0].findAll('.qm-bar i').at(-1)!
    const element = last.element
    expect(last.attributes('style')).toContain('--qm-slot: 47')
    await vi.advanceTimersByTimeAsync(300_000)
    await flushPromises()
    expect(wrapper.findAll('.qm-row')[0].findAll('.qm-bar i').at(-1)!.element).toBe(element)

    const next = response()
    next.data!.groups[0].history.push({ s: 'passed', t: '2026-10-01T12:20:00Z' })
    vi.mocked(getQualityMonitor).mockResolvedValue(next)
    await vi.advanceTimersByTimeAsync(300_000)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    const cells = wrapper.findAll('.qm-row')[0].findAll('.qm-bar i')
    expect(cells).toHaveLength(48)
    expect(cells.at(-2)!.element).toBe(element)
    expect(cells.at(-2)!.attributes('style')).toContain('--qm-slot: 46')
    expect(cells.at(-1)!.classes()).toContain('is-passed')
    vi.mocked(getQualityMonitor).mockRejectedValue(new Error('temporary outage'))
    await vi.advanceTimersByTimeAsync(300_000)
    await flushPromises()
    expect(wrapper.text()).toContain('同步暂时失败')
    expect(wrapper.findAll('.qm-row')[0].findAll('.qm-bar i.is-empty')).toHaveLength(30)
    wrapper.unmount()
    const calls = vi.mocked(getQualityMonitor).mock.calls.length
    await vi.advanceTimersByTimeAsync(300_000)
    expect(getQualityMonitor).toHaveBeenCalledTimes(calls)
  })

  it('keeps the original loading and unavailable states with a working monitor link', async () => {
    let resolve!: (value: QualityMonitorResponse) => void
    vi.mocked(getQualityMonitor).mockReturnValue(new Promise((done) => { resolve = done }))
    const wrapper = mountMonitor()
    expect(wrapper.find('.qm-board-loading').exists()).toBe(true)
    resolve({ enabled: true, available: false, detail_url: 'https://monitor.example.com' })
    await flushPromises()
    expect(wrapper.text()).toContain('监测数据暂时无法同步')
    expect(wrapper.get('.qm-board-empty a').attributes('href')).toBe('https://monitor.example.com')
    expect(wrapper.findAll('.qm-row')).toHaveLength(0)
  })

  it.each(['disabled', 'request failure'])('hides the section and its navigation for %s', async (state) => {
    if (state === 'disabled') vi.mocked(getQualityMonitor).mockResolvedValue({ enabled: false, available: false })
    else vi.mocked(getQualityMonitor).mockRejectedValue(new Error('network error'))
    const wrapper = mountMonitor()
    await flushPromises()
    expect(wrapper.find('#quality').exists()).toBe(false)
    expect(wrapper.emitted('loaded')).toEqual([[false]])
  })
})
