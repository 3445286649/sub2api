import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type Messages = Record<string, unknown>

const frontendSrc = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const staticTranslationCall = /(?:\$t|\bt)\(\s*(['"`])([^'"`$]+)\1/g

function walkSourceFiles(directory: string): string[] {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === '__tests__') return []
      return walkSourceFiles(fullPath)
    }
    return /\.(?:ts|vue)$/.test(entry.name) ? [fullPath] : []
  })
}

function staticTranslationKeys(): string[] {
  const keys = new Set<string>()
  for (const file of walkSourceFiles(frontendSrc)) {
    const source = fs.readFileSync(file, 'utf8')
    for (const match of source.matchAll(staticTranslationCall)) {
      if (!match[2].endsWith('.')) keys.add(match[2])
    }
  }
  return [...keys].sort()
}

function hasMessage(messages: Messages, key: string): boolean {
  let current: unknown = messages
  for (const segment of key.split('.')) {
    if (!current || typeof current !== 'object' || !(segment in current)) return false
    current = (current as Messages)[segment]
  }
  return typeof current === 'string' || typeof current === 'number'
}

const localFeatureKeys = [
  'nav.rechargeStorefront',
  'nav.usageHelp',
  'nav.modelRadar',
  'admin.settings.features.acquisition.title',
  'admin.settings.features.supportTickets.title',
  'admin.settings.features.channelMonitor.title',
  'payment.crypto.usdtBscNetwork',
  'payment.rechargeBonusBanner',
  'admin.redeem.affiliateRebateBase',
] as const

describe.each([
  ['zh', zh],
  ['en', en],
] as const)('%s locale completeness', (_locale, messages) => {
  it('contains every statically referenced production translation key', () => {
    const missing = staticTranslationKeys().filter((key) => !hasMessage(messages, key))
    expect(missing).toEqual([])
  })

  it('retains translations for local-only feature areas', () => {
    const missing = localFeatureKeys.filter((key) => !hasMessage(messages, key))
    expect(missing).toEqual([])
  })
})
