import type { HeaderButton, PublicSettings } from '@/types'

export function isHeaderButtonURLSafe(value: string): boolean {
  if (!/^https?:\/\//i.test(value) || /[\s\\]/.test(value) || value.length > 2048) return false
  try {
    const url = new URL(value)
    return !!url.hostname && !url.username && !url.password
  } catch {
    return false
  }
}

export function resolveHeaderButtons(settings: Partial<PublicSettings> | null | undefined, t: (key: string) => string): HeaderButton[] {
  if (Array.isArray(settings?.header_buttons)) return settings.header_buttons.map(button => ({ ...button }))
  return [
    { action: 'recharge', name: settings?.recharge_storefront_button_text?.trim() || t('nav.rechargeStorefront'), url: '', enabled: settings?.recharge_storefront_enabled === true },
    { action: 'rebate', name: t('nav.usageRebate'), url: '', enabled: settings?.usage_rebate_enabled === true },
    { action: 'help', name: t('nav.usageHelp'), url: '', enabled: settings?.usage_help_enabled === true },
    { action: 'radar', name: t('nav.modelRadar'), url: 'https://codexradar.com/#model-ratings', enabled: settings?.model_radar_enabled === true },
    { action: 'support', name: settings?.support_group_button_text?.trim() || t('nav.supportGroup'), url: settings?.support_group_link_url?.trim() || '', enabled: settings?.support_group_enabled === true },
    { action: 'pixmo', name: settings?.pixmo_studio_button_text?.trim() || t('nav.pixmoStudio'), url: settings?.pixmo_studio_url?.trim() || 'https://pixmo.loucer.cn/', enabled: settings?.pixmo_studio_enabled === true },
  ]
}
