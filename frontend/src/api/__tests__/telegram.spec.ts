import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getTelegramSettings, testTelegramSettings, updateTelegramSettings } from '@/api/admin/telegram'

const client = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))

describe('Telegram settings API', () => {
  beforeEach(() => Object.values(client).forEach(mock => mock.mockReset()))

  it('loads only configuration status from the dedicated admin endpoint', async () => {
    const state = { enabled: true, verify_easypay_orders: false, chat_id: '-10012', bot_token_configured: true }
    client.get.mockResolvedValue({ data: state })
    expect(await getTelegramSettings()).toEqual(state)
    expect(client.get).toHaveBeenCalledWith('/admin/settings/telegram')
  })

  it('keeps the write-only token optional and uses PUT to save', async () => {
    const request = { enabled: true, verify_easypay_orders: true, chat_id: '-10012' }
    client.put.mockResolvedValue({ data: { ...request, bot_token_configured: true } })
    await updateTelegramSettings(request)
    expect(client.put).toHaveBeenCalledWith('/admin/settings/telegram', request)
  })

  it('sends a draft configuration to the explicit test endpoint', async () => {
    const request = { enabled: false, verify_easypay_orders: true, chat_id: '-10012', bot_token: '123:test-only-token' }
    client.post.mockResolvedValue({ data: { message: 'sent' } })
    expect(await testTelegramSettings(request)).toEqual({ message: 'sent' })
    expect(client.post).toHaveBeenCalledWith('/admin/settings/telegram/test', request)
    expect(client.put).not.toHaveBeenCalled()
  })
})
