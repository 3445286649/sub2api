import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

function flattenKeys(obj: Record<string, any>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      keys.push(...flattenKeys(v, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

describe('ops locale key completeness', () => {
  const requiredKeys = [
    'admin.ops.result',
    'admin.ops.timeRange.custom',
    'admin.ops.customTimeRange.startTime',
    'admin.ops.customTimeRange.endTime',
  ]

  for (const key of requiredKeys) {
    it(`en locale has ${key}`, () => {
      const enKeys = flattenKeys(en)
      expect(enKeys).toContain(key)
    })
  }
})

describe('groups locale key completeness', () => {
  it('en locale has admin.groups.failedToSave', () => {
    const enKeys = flattenKeys(en)
    expect(enKeys).toContain('admin.groups.failedToSave')
  })
})


describe('local feature locale key completeness', () => {
  const requiredKeys = [
    'nav.modelRadar',
    'nav.usageHelp',
    'nav.support',
    'nav.supportManagement',
    'nav.acquisition',
    'nav.acquisitionManagement',
    'admin.accounts.columns.health',
    'admin.accounts.columns.autoStatus',
    'admin.accounts.columns.avgLatency',
    'admin.accounts.columns.nextProbe',
    'admin.accounts.healthOverview',
    'admin.accounts.overviewColumns.account',
    'admin.accounts.overviewColumns.health',
    'admin.accounts.overviewColumns.autoStatus',
    'admin.accounts.autoStatus.enabled',
    'admin.accounts.autoStatus.isolated',
    'admin.accounts.upstreamBalanceRefresh',
    'admin.accounts.upstreamBalanceValue',
    'admin.accounts.healthyProbeInterval',
    'admin.accounts.healthyProbeIntervalOptionMinutes',
    'admin.accounts.healthProbeModel',
  ]

  for (const locale of [
    { name: 'en', messages: en },
    { name: 'zh', messages: zh },
  ]) {
    const keys = flattenKeys(locale.messages)

    for (const key of requiredKeys) {
      it(`${locale.name} locale has ${key}`, () => {
        expect(keys).toContain(key)
      })
    }
  }
})
