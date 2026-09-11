import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import MonitorCardGrid from '../monitor/MonitorCardGrid.vue'
import type { UserMonitorView } from '@/api/channelMonitor'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, args?: { count: number }) => args ? `${args.count} channels` : key }),
}))
const item = (id: number, provider: UserMonitorView['provider'], display_category = '') => ({
  id, provider, display_category, name: `Channel ${id}`, primary_model: 'model', availability_7d: 99,
}) as UserMonitorView
const make = (items: UserMonitorView[]) => mount(MonitorCardGrid, {
  props: { items, loading: false, window: '7d', countdownSeconds: 30, detailCache: {} },
  global: { stubs: {
    MonitorCard: { props: ['item'], template: '<button @click="$emit(\'click\')">{{ item.name }}</button>' },
    EmptyState: true,
  } },
})

describe('monitor display categories', () => {
  it('groups legacy providers and custom labels with stable order and counts', () => {
    const w = make([item(9, 'openai', '国产聚合'), item(5, 'anthropic'), item(3, 'openai'), item(1, 'openai')])
    expect(w.findAll('section').map(s => s.attributes('aria-label'))).toEqual(['OpenAI', 'Claude', '国产聚合'])
    expect(w.findAll('section')[0]!.findAll('button').map(b => b.text())).toEqual(['Channel 1', 'Channel 3'])
    expect(w.find('button').text()).toContain('(4)')
    expect(w.text()).not.toContain('Gemini')
    w.unmount()
  })
  it('preserves selection across refresh, falls back when removed, and forwards detail clicks', async () => {
    const w = make([item(1, 'openai'), item(2, 'openai', '智谱 GLM')])
    await w.findAll('button').find(b => b.text().includes('智谱 GLM'))!.trigger('click')
    expect(w.findAll('section')).toHaveLength(1)
    await w.setProps({ items: [item(3, 'openai'), item(2, 'openai', '智谱 GLM')] })
    expect(w.find('section').attributes('aria-label')).toBe('智谱 GLM')
    await w.find('section button').trigger('click')
    expect(w.emitted('cardClick')?.[0]?.[0]).toMatchObject({ id: 2 })
    await w.setProps({ items: [item(3, 'openai')] })
    expect(w.find('button').attributes('aria-pressed')).toBe('true')
    expect(w.find('section').attributes('aria-label')).toBe('OpenAI')
    w.unmount()
  })
  it('renders category text safely and never confuses a custom All label with the all filter', async () => {
    const w = make([item(1, 'openai', '<img src=x onerror=alert(1)>'), item(2, 'grok', '全部')])
    expect(w.find('img').exists()).toBe(false)
    expect(w.text()).toContain('<img src=x onerror=alert(1)>')
    await w.findAll('button').find(b => b.text() === '全部 (1)')!.trigger('click')
    expect(w.findAll('section')).toHaveLength(1)
    w.unmount()
  })
})
