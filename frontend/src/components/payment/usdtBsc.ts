// USDT-BSC 收银台的纯展示工具，独立出来便于单测。

/**
 * 金额拆成「整数 + 前两位小数」和「其余小数」两段。后端给每笔订单加 6 位小数的
 * 唯一尾数来区分同金额订单，页面高亮后一段提醒用户尾数也必须一致。
 */
export function splitUsdtAmount(raw: string): { head: string; tail: string } {
  const amount = raw.trim()
  const dot = amount.indexOf('.')
  if (dot < 0) return { head: amount, tail: '' }
  return { head: amount.slice(0, dot + 3), tail: amount.slice(dot + 3) }
}

/** 地址按 4 位一组拆开；0x 前缀并入第一组。只用于展示，复制时仍用原始地址。 */
export function groupWalletAddress(raw: string): string[] {
  const address = raw.trim()
  const prefix = /^0x/i.test(address) ? address.slice(0, 2) : ''
  const groups = address.slice(prefix.length).match(/.{1,4}/g) ?? []
  if (prefix && groups.length > 0) groups[0] = prefix + groups[0]
  return groups
}

export function formatCountdown(totalSeconds: number): string {
  const seconds = Math.max(0, Math.floor(totalSeconds))
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

export function formatUsdtRate(raw: string | undefined): string {
  const rate = (raw || '').trim()
  if (!rate) return ''
  const numeric = Number(rate)
  if (!Number.isFinite(numeric) || numeric <= 0) return rate
  return numeric.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
}

const TX_HASH_PATTERN = /^0x[0-9a-fA-F]{64}$/

export function isCryptoTxHash(raw: string | undefined): boolean {
  return TX_HASH_PATTERN.test((raw || '').trim())
}

/** 0x9c3e5b1f…7d9e1f3a：保留前 10 位和后 8 位，够用户和钱包里的记录对上 */
export function shortenTxHash(raw: string): string {
  const hash = raw.trim()
  if (hash.length <= 20) return hash
  return `${hash.slice(0, 10)}…${hash.slice(-8)}`
}

/** 浏览器链接写死 BSC 主网；测试网订单的链接会打不开，但不影响到账 */
export function bscScanTxUrl(hash: string): string {
  return `https://bscscan.com/tx/${hash.trim()}`
}
