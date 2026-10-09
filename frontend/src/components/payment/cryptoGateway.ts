import type { CryptoNetwork } from '@/types/payment'

export const CRYPTO_NETWORKS: { value: CryptoNetwork; label: string }[] = [
  { value: 'bsc_mainnet', label: 'BNB Smart Chain (BEP20)' },
  { value: 'tron_mainnet', label: 'TRON (TRC20)' },
]

export function supportedCryptoNetworks(values: unknown): CryptoNetwork[] {
  if (!Array.isArray(values)) return []
  return CRYPTO_NETWORKS.filter(network => values.includes(network.value)).map(network => network.value)
}

export function cryptoNetworkLabel(network?: string): string {
  return CRYPTO_NETWORKS.find(item => item.value === network)?.label || ''
}

export function gatewaySubscriptionPrice(price: number, currency?: string, usdToCnyRate?: number): number | null {
  if (!Number.isFinite(price) || price <= 0) return null
  const normalized = (currency || 'USD').trim().toUpperCase() || 'USD'
  if (normalized === 'CNY') return price
  if (normalized === 'USD' && Number.isFinite(usdToCnyRate) && (usdToCnyRate || 0) > 0) {
    const converted = price * usdToCnyRate!
    return Number.isFinite(converted) && converted > 0 ? converted : null
  }
  return null
}

export function cryptoTransactionUrl(hash: string, network = 'bsc_mainnet'): string {
  const clean = hash.trim()
  if (network === 'bsc_mainnet' && /^0x[0-9a-fA-F]{64}$/.test(clean)) {
    return `https://bscscan.com/tx/${clean}`
  }
  if (network === 'tron_mainnet' && /^[0-9a-fA-F]{64}$/.test(clean)) {
    return `https://tronscan.org/#/transaction/${clean}`
  }
  return ''
}

const intentStoragePrefix = 'payment.crypto.intent.'
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

function newCryptoRequestId(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export function cryptoPaymentRequestId(
  storage: Pick<Storage, 'getItem' | 'setItem'>,
  userId: number,
  intent: { amount: number; orderType: string; planId?: number; network: CryptoNetwork },
  newId = newCryptoRequestId,
): string {
  const key = intentStoragePrefix + userId
  const fingerprint = JSON.stringify([intent.amount, intent.orderType, intent.planId || 0, intent.network])
  try {
    const saved = JSON.parse(storage.getItem(key) || 'null')
    if (saved?.fingerprint === fingerprint && uuidPattern.test(saved.id)) return saved.id
  } catch { /* A damaged draft must not become a request identifier. */ }
  const id = newId()
  storage.setItem(key, JSON.stringify({ fingerprint, id }))
  return id
}

export function clearCryptoPaymentIntent(storage: Pick<Storage, 'removeItem'>, userId: number): void {
  storage.removeItem(intentStoragePrefix + userId)
}
