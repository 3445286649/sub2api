import { describe, expect, it } from 'vitest'
import { resolveRechargeStorefrontChannels } from '../rechargeStorefront'
import type { PublicSettings } from '@/types'

function settings(overrides: Partial<PublicSettings>): PublicSettings {
  return {
    registration_enabled: true,
    email_verify_enabled: false,
    force_email_on_third_party_signup: false,
    registration_email_suffix_whitelist: [],
    promo_code_enabled: true,
    password_reset_enabled: false,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: 'Sub2API',
    site_logo: '',
    site_subtitle: '',
    api_base_url: '',
    contact_info: '',
    doc_url: '',
    home_content: '',
    compact_home_enabled: false,
    backend_mode_enabled: false,
    payment_enabled: false,
    version: 'test',
    ...overrides,
  }
}

describe('resolveRechargeStorefrontChannels', () => {
  it('prefers sorted enabled JSON channels', () => {
    const result = resolveRechargeStorefrontChannels(settings({
      recharge_storefront_enabled: true,
      recharge_storefront_url: 'https://legacy.example.com',
      recharge_storefront_channels: [
        { id: 'two', name: 'Two', url: 'https://two.example.com', enabled: true, sort_order: 2 },
        { id: 'one', name: 'One', url: 'https://one.example.com', enabled: true, sort_order: 1 },
        { id: 'off', name: 'Off', url: 'https://off.example.com', enabled: false, sort_order: 3 },
      ],
    }))

    expect(result.map((channel) => channel.id)).toEqual(['one', 'two'])
  })

  it('falls back to legacy HTTPS fields when JSON channels are absent', () => {
    const result = resolveRechargeStorefrontChannels(settings({
      recharge_storefront_enabled: true,
      recharge_storefront_url: 'https://shop.example.com/',
      recharge_storefront_backup_url: 'https://backup.example.com/',
    }))

    expect(result.map((channel) => channel.id)).toEqual(['backup-1', 'backup-2'])
  })

  it('does not revive legacy channels when the JSON channel list is present but empty', () => {
    const result = resolveRechargeStorefrontChannels(settings({
      recharge_storefront_enabled: true,
      recharge_storefront_url: 'https://shop.example.com/',
      recharge_storefront_backup_url: 'https://backup.example.com/',
      recharge_storefront_channels: [],
    }))

    expect(result).toEqual([])
  })

  it('returns no channels when the global switch is off', () => {
    expect(resolveRechargeStorefrontChannels(settings({
      recharge_storefront_enabled: false,
      recharge_storefront_url: 'https://shop.example.com/',
    }))).toEqual([])
  })
})
