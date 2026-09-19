import type { ApiKey } from '@/types'

export const CANVAS_PROTOCOL = 'sub2api.canvas'
export const CANVAS_PROTOCOL_VERSION = 2
// Open workspaces recheck the switch as well as on focus and before initialization.
export const CANVAS_SETTINGS_REFRESH_MS = 30_000
export const CANVAS_CONNECT_TIMEOUT_MS = 20_000

export const CANVAS_CAPABILITIES = ['image', 'text', 'video'] as const
export type CanvasCapability = typeof CANVAS_CAPABILITIES[number]
export type CanvasSelection = Record<CanvasCapability, number[]>
export const emptyCanvasSelection = (): CanvasSelection => ({ image: [], text: [], video: [] })

export function isCanvasKeyAvailable(key: ApiKey, userId: number, capability: CanvasCapability = 'image'): boolean {
  const expires = key.expires_at ? Date.parse(key.expires_at) : null
  const platform = key.group?.platform
  const supported = capability === 'text'
    ? ['openai', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'].includes(platform || '')
    : capability === 'video' ? platform === 'grok' : platform === 'openai' || platform === 'grok'
  return key.user_id === userId && key.status === 'active'
    && Boolean(key.key) && supported && (!key.group?.status || key.group.status === 'active')
    && (capability === 'text' || key.group?.allow_image_generation === true)
    && (expires === null || (Number.isFinite(expires) && expires > Date.now()))
    && !(key.quota > 0 && key.quota_used >= key.quota)
}

// The first connection in each capability is its default. Never persist secrets.
export function parseCanvasSelection(value: unknown): CanvasSelection | null {
  if (!value || typeof value !== 'object') return null
  const selection = value as CanvasSelection
  if (!CANVAS_CAPABILITIES.every(role => Array.isArray(selection[role]) && selection[role].every(id => Number.isSafeInteger(id) && id > 0))) return null
  return Object.fromEntries(CANVAS_CAPABILITIES.map(role => [role, [...new Set(selection[role])]])) as CanvasSelection
}

export function canvasURL(origin: string, configuredPath = import.meta.env.VITE_INFINITE_CANVAS_PATH || '/canvas/'): string {
  const url = new URL(configuredPath, origin)
  if (url.origin !== origin || !url.pathname.startsWith('/canvas/') || url.search || url.hash || url.username || url.password) {
    throw new Error('Invalid canvas path')
  }
  return url.href
}

export function isCanvasMessage(
  event: MessageEvent,
  source: Window | null | undefined,
  origin: string,
  type: 'READY' | 'ACK' | 'ERROR',
  sessionId?: string,
): boolean {
  const data: unknown = event.data
  if (!source || event.source !== source || event.origin !== origin || !data || typeof data !== 'object') return false
  const msg = data as Record<string, unknown>
  return msg.protocol === CANVAS_PROTOCOL && msg.version === CANVAS_PROTOCOL_VERSION && msg.type === type
    && (sessionId === undefined || msg.sessionId === sessionId)
}

export function canvasMessage(type: 'INIT' | 'UPDATE' | 'DISPOSE', sessionId: string) {
  return { protocol: CANVAS_PROTOCOL, version: CANVAS_PROTOCOL_VERSION, type, sessionId }
}
