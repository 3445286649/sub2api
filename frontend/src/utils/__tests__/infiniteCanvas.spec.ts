import { describe, expect, it } from 'vitest'
import type { ApiKey } from '@/types'
import { canvasURL, isCanvasKeyAvailable, isCanvasMessage, CANVAS_PROTOCOL, CANVAS_PROTOCOL_VERSION } from '../infiniteCanvas'

const key = { user_id: 1, status: 'active', key: 'fixture', quota: 10, quota_used: 0,
  expires_at: null, group: { platform: 'openai', allow_image_generation: true } } as ApiKey

describe('canvas access boundaries', () => {
  it('requires an owned, active key in an image-enabled OpenAI group', () => {
    expect(isCanvasKeyAvailable(key, 1)).toBe(true)
    for (const candidate of [
      { ...key, user_id: 2 }, { ...key, status: 'inactive' }, { ...key, quota_used: 10 },
      { ...key, expires_at: '2020-01-01' }, { ...key, expires_at: 'invalid' }, { ...key, key: '' },
      { ...key, group: { ...key.group, allow_image_generation: false } },
      { ...key, group: { ...key.group, platform: 'anthropic' } }, { ...key, group: undefined },
    ]) expect(isCanvasKeyAvailable(candidate as ApiKey, 1)).toBe(false)
  })

  it('offers Gemini image keys only with image permission and valid ownership and status', () => {
    const gemini = { ...key, group: { ...key.group, platform: 'gemini' } } as ApiKey
    expect(isCanvasKeyAvailable(gemini, 1, 'image')).toBe(true)
    expect(isCanvasKeyAvailable(gemini, 1, 'text')).toBe(false)
    expect(isCanvasKeyAvailable(gemini, 1, 'video')).toBe(false)
    for (const candidate of [
      { ...gemini, user_id: 2 }, { ...gemini, status: 'inactive' }, { ...gemini, quota_used: 10 },
      { ...gemini, expires_at: '2020-01-01' }, { ...gemini, key: '' },
      { ...gemini, group: { ...gemini.group, status: 'inactive' } },
      { ...gemini, group: { ...gemini.group, allow_image_generation: false } },
      { ...gemini, group: { ...gemini.group, allow_image_generation: undefined } },
    ]) expect(isCanvasKeyAvailable(candidate as ApiKey, 1, 'image')).toBe(false)
  })

  it('supports text independently of image permission and restricts video to enabled Grok groups', () => {
    const text = { ...key, group: { platform: 'openai', allow_image_generation: false } } as ApiKey
    expect(isCanvasKeyAvailable(text, 1, 'text')).toBe(true)
    expect(isCanvasKeyAvailable(text, 1, 'image')).toBe(false)
    expect(isCanvasKeyAvailable(key, 1, 'video')).toBe(false)
    const grok = { ...key, group: { platform: 'grok', allow_image_generation: true } } as ApiKey
    expect(isCanvasKeyAvailable(grok, 1, 'video')).toBe(true)
    expect(isCanvasKeyAvailable({ ...grok, group: { ...grok.group, status: 'disabled' } } as ApiKey, 1, 'video')).toBe(false)
  })

  it('only embeds a same-origin canvas path without credentials or URL parameters', () => {
    expect(canvasURL('https://site.test', '/canvas/releases/one/')).toBe('https://site.test/canvas/releases/one/')
    for (const path of ['https://other.test/canvas/', '//other.test/canvas/', '/canvas/../admin', '/canvas/?apiKey=fixture', '/canvas/#key', 'https://name:pass@site.test/canvas/']) {
      expect(() => canvasURL('https://site.test', path)).toThrow()
    }
  })

  it('rejects wrong senders, origins, protocol versions and stale sessions', () => {
    const source = window
    const data = { protocol: CANVAS_PROTOCOL, version: CANVAS_PROTOCOL_VERSION, type: 'ACK', sessionId: 'current' }
    const event = (changes: MessageEventInit = {}) => new MessageEvent('message', { source, origin: window.location.origin, data, ...changes })
    expect(isCanvasMessage(event(), source, window.location.origin, 'ACK', 'current')).toBe(true)
    expect(isCanvasMessage(event({ origin: 'https://other.test' }), source, window.location.origin, 'ACK', 'current')).toBe(false)
    expect(isCanvasMessage(event({ source: null }), source, window.location.origin, 'ACK', 'current')).toBe(false)
    expect(isCanvasMessage(event({ data: { ...data, version: 1 } }), source, window.location.origin, 'ACK', 'current')).toBe(false)
    expect(isCanvasMessage(event(), source, window.location.origin, 'ACK', 'old')).toBe(false)
    expect(isCanvasMessage(event({ data: null }), source, window.location.origin, 'ACK', 'current')).toBe(false)
  })
})
