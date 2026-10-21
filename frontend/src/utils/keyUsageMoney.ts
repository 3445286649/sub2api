const formatter = new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

// Display rounds to cents; the exact 4-digit value stays available via title.
export function formatKeyUsageMoney(value: number) {
  if (value === 0) return '$0.00'
  if (value > 0 && value < 0.01) return '<$0.01'
  return `$${formatter.format(value)}`
}

export const keyUsageMoneyTitle = (value: number) => `$${value.toFixed(4)}`

// Integer share; tiny non-zero shares show "<1%", empty totals "—".
export function formatKeyUsageShare(part: number, total: number) {
  if (!(total > 0)) return '—'
  const pct = (part / total) * 100
  if (pct > 0 && pct < 1) return '<1%'
  return `${Math.round(pct)}%`
}

export const keyUsageShareWidth = (part: number, total: number) => total > 0 ? `${Math.min(100, Math.max(0, (part / total) * 100))}%` : '0%'
