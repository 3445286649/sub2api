import type { PublicSettings, RechargeStorefrontChannel } from '@/types'

export function resolveRechargeStorefrontChannels(settings?: PublicSettings | null): RechargeStorefrontChannel[] {
  if (!settings?.recharge_storefront_enabled) return []

  if (Array.isArray(settings.recharge_storefront_channels)) {
    return settings.recharge_storefront_channels
      .filter((channel) => channel.enabled && channel.url.trim().startsWith('https://'))
      .map((channel) => ({ ...channel, url: channel.url.trim() }))
      .sort((left, right) => left.sort_order - right.sort_order)
  }

  const legacy: RechargeStorefrontChannel[] = []
  const primary = settings.recharge_storefront_url?.trim()
  const backup = settings.recharge_storefront_backup_url?.trim()
  if (primary?.startsWith('https://')) {
    legacy.push({ id: 'backup-1', name: '备用一', url: primary, enabled: true, sort_order: 1 })
  }
  if (backup?.startsWith('https://')) {
    legacy.push({ id: 'backup-2', name: '备用二', url: backup, enabled: true, sort_order: 2 })
  }
  return legacy
}
