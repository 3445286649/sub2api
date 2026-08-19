import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { enableAutoUnmount, mount } from '@vue/test-utils'

enableAutoUnmount(afterEach)

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value,
  formatRelativeTime: () => 'just now'
}))

import AccountHealthOverviewDialog from '../AccountHealthOverviewDialog.vue'

const BaseDialogStub = defineComponent({
  props: {
    show: Boolean,
    title: String
  },
  emits: ['close'],
  template: '<section v-if="show"><slot /></section>'
})

const overview = {
  generated_at: '2026-08-19T12:00:00Z',
  urls: [
    {
      base_url: 'https://bad.example.com',
      accounts: [
        {
          account_id: 1,
          account_name: 'Bad Account',
          key_fingerprint: 'bad-key',
          score: 0,
          status: 'isolated',
          schedulable: true,
          rate_multiplier: 1,
          rate_multiplier_configured: true,
          health_probe_enabled: true,
          healthy_probe_enabled: false,
          backoff_level: 2,
          consecutive_successes: 0,
          consecutive_failures: 3,
          group_ids: [10],
          group_names: ['Critical Group']
        }
      ],
      risks: [
        { level: 'critical', type: 'url_all_isolated', base_url: 'https://bad.example.com', count: 1 },
        { level: 'critical', type: 'group_no_available_accounts', base_url: 'https://bad.example.com', group_id: 10, group_name: 'Critical Group' }
      ],
      insufficient_group_ids: [10],
      insufficient_group_names: ['Critical Group']
    },
    {
      base_url: 'https://warn.example.com',
      accounts: [
        {
          account_id: 2,
          account_name: 'Healthy Account',
          key_fingerprint: 'healthy-key',
          score: 100,
          status: 'healthy',
          schedulable: true,
          rate_multiplier: 1,
          rate_multiplier_configured: true,
          health_probe_enabled: true,
          healthy_probe_enabled: true,
          backoff_level: 0,
          consecutive_successes: 8,
          consecutive_failures: 0,
          group_ids: [20],
          group_names: ['Thin Group']
        },
        {
          account_id: 3,
          account_name: 'Slow Account',
          key_fingerprint: 'slow-key',
          score: 72,
          status: 'degraded',
          schedulable: true,
          rate_multiplier: 0.8,
          rate_multiplier_configured: true,
          health_probe_enabled: true,
          healthy_probe_enabled: true,
          backoff_level: 1,
          consecutive_successes: 0,
          consecutive_failures: 1,
          group_ids: [20],
          group_names: ['Thin Group']
        }
      ],
      risks: [
        { level: 'warning', type: 'group_single_available_account', base_url: 'https://warn.example.com', group_id: 20, group_name: 'Thin Group' },
        { level: 'warning', type: 'consecutive_failures', base_url: 'https://warn.example.com', account_id: 3, count: 1 }
      ]
    },
    {
      base_url: 'https://ok.example.com',
      accounts: [
        {
          account_id: 4,
          account_name: 'Official DeepSeek',
          key_fingerprint: 'ok-key',
          score: 100,
          status: 'healthy',
          schedulable: true,
          rate_multiplier: 1,
          rate_multiplier_configured: true,
          health_probe_enabled: true,
          healthy_probe_enabled: true,
          backoff_level: 0,
          consecutive_successes: 12,
          consecutive_failures: 0,
          group_ids: [30],
          group_names: ['Stable Group']
        }
      ],
      risks: []
    }
  ],
  risks: [
    { level: 'critical', type: 'url_all_isolated', base_url: 'https://bad.example.com', count: 1 },
    { level: 'critical', type: 'group_no_available_accounts', base_url: 'https://bad.example.com', group_id: 10, group_name: 'Critical Group' },
    { level: 'critical', type: 'group_no_available_accounts', base_url: 'https://warn.example.com', group_id: 10, group_name: 'Critical Group' },
    { level: 'warning', type: 'group_single_available_account', base_url: 'https://warn.example.com', group_id: 20, group_name: 'Thin Group' },
    { level: 'warning', type: 'consecutive_failures', base_url: 'https://warn.example.com', account_id: 3, count: 1 }
  ]
} as any

function mountDialog() {
  return mount(AccountHealthOverviewDialog, {
    props: {
      show: true,
      overview,
      loading: false,
      error: '',
      refreshingBalanceUrls: new Set<string>()
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Icon: true
      }
    }
  })
}

describe('AccountHealthOverviewDialog', () => {
  it('summarizes all accounts while defaulting the list to abnormal upstreams', () => {
    const wrapper = mountDialog()

    expect(wrapper.get('[data-testid="health-total-accounts"]').text()).toBe('4')
    expect(wrapper.get('[data-testid="health-schedulable-accounts"]').text()).toBe('3')
    expect(wrapper.get('[data-testid="health-affected-groups"]').text()).toBe('2')
    expect(wrapper.findAll('[data-testid="health-upstream"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="health-risk-row"]')).toHaveLength(4)
  })

  it('shows all upstreams and filters account rows by health status', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-testid="health-show-all"]').trigger('click')
    expect(wrapper.findAll('[data-testid="health-upstream"]')).toHaveLength(3)

    await wrapper.get('[data-testid="health-status-degraded"]').trigger('click')
    expect(wrapper.findAll('[data-testid="health-upstream"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('Slow Account')
    expect(wrapper.text()).not.toContain('Healthy Account')
  })

  it('emits refresh actions without mutating overview data', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-testid="health-refresh"]').trigger('click')
    await wrapper.get('[data-testid="health-balance-refresh"]').trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('refresh-balance')?.[0]).toEqual(['https://bad.example.com'])
    expect(overview.urls).toHaveLength(3)
  })
})
