import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn()
}))

vi.mock('../client', () => ({
  apiClient: {
    post
  }
}))

import { generateCampaign } from '@/api/admin/redeem'

describe('admin redeem campaign API', () => {
  beforeEach(() => {
    post.mockReset()
  })

  it('posts campaign-only fields to the dedicated endpoint', async () => {
    post.mockResolvedValue({ data: [{ id: 1, code: 'campaign-code' }] })

    const result = await generateCampaign(' August benefit ', 10, 5, 7)

    expect(post).toHaveBeenCalledWith('/admin/redeem-campaigns/generate', {
      name: 'August benefit',
      count: 10,
      value: 5,
      expires_in_days: 7
    })
    expect(result).toEqual([{ id: 1, code: 'campaign-code' }])
  })
})
