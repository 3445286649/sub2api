import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      invitees: [],
    })
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toEqual(expect.arrayContaining([
        'min-w-0',
        'break-all',
        'sm:flex-1',
        'sm:truncate',
      ]))
      expect(Array.from(value.element.parentElement?.classList ?? [])).toEqual(expect.arrayContaining([
        'flex-col',
        'items-stretch',
        'sm:flex-row',
        'sm:items-center',
      ]))
    }

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toEqual(expect.arrayContaining([
        'w-full',
        'sm:w-auto',
        'sm:shrink-0',
      ]))
    }

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenNthCalledWith(1, affiliateCode, 'affiliate.codeCopied')
    expect(copyToClipboard).toHaveBeenNthCalledWith(
      2,
      `${window.location.origin}/register?aff=${encodeURIComponent(affiliateCode)}`,
      'affiliate.linkCopied',
    )
  })

  it('renders all invite points statuses with server-calculated progress and timing', async () => {
    const baseInvitee = {
      email: 'm***@e***.com',
      username: 'invitee',
      created_at: '2026-08-01T00:00:00Z',
      total_rebate: 0,
      qualifying_amount: 0,
      threshold_amount: 50,
      reward_points: 1,
      qualification_window_days: 30,
      freeze_hours: 168,
      qualification_deadline: '2026-08-31T00:00:00Z',
    }
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 6,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      invitees: [
        { ...baseInvitee, user_id: 1, points_status: 'not_recharged' },
        { ...baseInvitee, user_id: 2, points_status: 'progressing', qualifying_amount: 30 },
        { ...baseInvitee, user_id: 3, points_status: 'pending', qualifying_amount: 50, release_at: '2026-08-20T00:00:00Z' },
        { ...baseInvitee, user_id: 4, points_status: 'available', qualifying_amount: 50, released_at: '2026-08-13T00:00:00Z' },
        { ...baseInvitee, user_id: 5, points_status: 'revoked', points_status_reason: 'refund_below_threshold', revoked_at: '2026-08-12T00:00:00Z' },
        { ...baseInvitee, user_id: 6, points_status: 'expired', points_status_reason: 'qualification_window_expired' },
        { user_id: 7, email: 'o***@e***.com', username: 'old-api', created_at: '2026-08-01T00:00:00Z', total_rebate: 0 },
      ],
    })

    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    for (const status of ['notRecharged', 'progressing', 'pending', 'available', 'revoked', 'expired']) {
      expect(wrapper.text()).toContain(`affiliate.invitees.status.${status}`)
    }
    expect(wrapper.text()).toContain('30 / 50')
    expect(wrapper.text()).toContain('affiliate.invitees.timeline.releaseAt')
    expect(wrapper.text()).toContain('affiliate.invitees.timeline.releasedAt')
    expect(wrapper.text()).toContain('affiliate.invitees.reason.refundBelowThreshold')
    expect(wrapper.text()).toContain('affiliate.invitees.reason.qualificationWindowExpired')
    expect(wrapper.text()).toContain('affiliate.invitees.status.syncing')
  })
})
