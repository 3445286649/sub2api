import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')

describe('AppHeader desktop navigation order', () => {
  it('keeps usage rebate directly after the recharge storefront and before usage help', () => {
    const recharge = headerSource.indexOf('<!-- Recharge Storefront -->')
    const usageRebate = headerSource.indexOf('<!-- Usage Rebate -->')
    const usageHelp = headerSource.indexOf('<!-- Usage Help -->')
    const modelRadar = headerSource.indexOf('<!-- Model Radar -->')

    expect(recharge).toBeGreaterThan(-1)
    expect(usageRebate).toBeGreaterThan(recharge)
    expect(usageHelp).toBeGreaterThan(usageRebate)
    expect(modelRadar).toBeGreaterThan(usageHelp)
  })

  it('shows usage rebate in the user dropdown directly after the recharge storefront', () => {
    const menuStart = headerSource.indexOf('<!-- Dropdown Menu -->')
    const menuSource = headerSource.slice(menuStart)
    const recharge = menuSource.indexOf('@click="openRechargeStorefrontFromMenu"')
    const usageRebate = menuSource.indexOf('to="/usage-rebate"')
    const usageHelp = menuSource.indexOf('@click="openUsageHelpFromMenu"')

    expect(menuStart).toBeGreaterThan(-1)
    expect(recharge).toBeGreaterThan(-1)
    expect(usageRebate).toBeGreaterThan(recharge)
    expect(usageHelp).toBeGreaterThan(usageRebate)
    expect(menuSource.slice(usageRebate, usageHelp)).not.toContain('sm:hidden')
  })
})
