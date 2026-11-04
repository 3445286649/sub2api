import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(componentSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(componentSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar backup recharge navigation', () => {
  it('folds backup recharge into the purchase page instead of a separate group', () => {
    expect(componentSource).toContain("{ path: '/subscriptions', label: t('nav.mySubscriptions')")
    expect(componentSource).toContain("{ path: '/purchase', label: purchaseNavLabel.value")
    expect(componentSource).not.toContain("path: '/backup-recharge'")
    expect(componentSource).not.toContain("t('nav.backupRecharge')")
    expect(componentSource).toContain('const flagPurchase = () => flagPayment() || flagRechargeStorefront()')
  })
})

describe('AppSidebar subscription feature flag', () => {
  it('gates the My Subscriptions entry behind the subscription public-settings flag', () => {
    expect(componentSource).toContain('const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)')
    expect(componentSource).toMatch(/path: '\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('also hides the admin Subscription Management entry on recharge-only sites', () => {
    expect(componentSource).toMatch(/path: '\/admin\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('derives the purchase entry label from the site billing mode', () => {
    expect(componentSource).toContain("import { resolveSiteBillingMode } from '@/utils/siteBillingMode'")
    expect(componentSource).toMatch(/case 'recharge_only':\s*return t\('nav\.rechargeInvoice'\)/)
    expect(componentSource).toMatch(/case 'subscription_only':\s*return t\('nav\.subscribeInvoice'\)/)
    expect(componentSource).toMatch(/default:\s*return t\('nav\.purchaseInvoice'\)/)
    expect(componentSource).toMatch(/path: '\/purchase'[^\n]*label: purchaseNavLabel\.value/)
  })
})

describe('AppSidebar invoice pending badge', () => {
  it('shows the pending invoice count on the order management group and the invoices entry', () => {
    expect(componentSource).toMatch(/featureFlag: flagAdminPayment,\s*\/\/[^\n]*\n\s*badge: invoicePendingCount\.value/)
    expect(componentSource).toMatch(/path: '\/admin\/orders\/invoices'[^\n]*badge: invoicePendingCount\.value/)
  })

  it('polls pending invoices with the other badges and syncs from the invoice page event', () => {
    expect(componentSource).toContain('invoicePendingCount.value = (await adminInvoiceAPI.getStats()).pending')
    expect(componentSource).toContain('supportBadgeTimer = window.setInterval(refreshSidebarBadges, 30000)')
    expect(componentSource).toContain('window.addEventListener(INVOICE_PENDING_EVENT, onInvoicePendingChanged)')
    expect(componentSource).toContain('window.removeEventListener(INVOICE_PENDING_EVENT, onInvoicePendingChanged)')
  })

  it('renders group badges only while the group is collapsed so the count is not shown twice', () => {
    expect(componentSource).toContain('v-if="item.badge && !isGroupExpanded(item)"')
    expect(componentSource).toContain('v-if="child.badge"')
  })
})
