import type { UserMonitorView } from '@/api/channelMonitor'

export const MONITOR_CATEGORY_NAMES: Record<string, string> = {
  openai: 'OpenAI', anthropic: 'Claude', gemini: 'Gemini', grok: 'Grok',
  zhipu: '智谱 GLM', deepseek: 'DeepSeek', kimi: 'Kimi', minimax: 'MiniMax',
  antigravity: 'Antigravity',
}

export function monitorDisplayCategory(item: Pick<UserMonitorView, 'provider' | 'display_category'>): string {
  return item.display_category?.trim() || MONITOR_CATEGORY_NAMES[item.provider] || item.provider
}

export function groupMonitorCategories(items: UserMonitorView[]) {
  const groups = new Map<string, UserMonitorView[]>()
  for (const item of items) {
    const name = monitorDisplayCategory(item)
    const group = groups.get(name) ?? []
    group.push(item)
    groups.set(name, group)
  }
  const defaults = Object.values(MONITOR_CATEGORY_NAMES)
  const rank = (name: string) => defaults.includes(name) ? defaults.indexOf(name) : defaults.length
  return [...groups].sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b, 'zh-CN'))
    .map(([name, monitors]) => ({ name, items: monitors.sort((a, b) => a.id - b.id) }))
}
