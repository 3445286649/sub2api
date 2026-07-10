import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')

describe('AppHeader desktop navigation order', () => {
  it('keeps usage help directly after the recharge storefront and before model radar', () => {
    const recharge = headerSource.indexOf('<!-- Recharge Storefront -->')
    const usageHelp = headerSource.indexOf('<!-- Usage Help -->')
    const modelRadar = headerSource.indexOf('<!-- Model Radar -->')

    expect(recharge).toBeGreaterThan(-1)
    expect(usageHelp).toBeGreaterThan(recharge)
    expect(modelRadar).toBeGreaterThan(usageHelp)
  })
})
