import { apiClient } from './client'
import type { PointsLedgerEntry, PointsPage, PointsProduct, PointsShopOrder, PointsSummary } from '@/types/points'

export const pointsAPI = {
  getSummary() {
    return apiClient.get<PointsSummary>('/points-shop/summary')
  },
  getProducts() {
    return apiClient.get<PointsProduct[]>('/points-shop/products')
  },
  redeem(productId: number, idempotencyKey: string) {
    return apiClient.post<PointsShopOrder>(`/points-shop/products/${productId}/redeem`, { idempotency_key: idempotencyKey })
  },
  getLedger(params?: { page?: number; page_size?: number }) {
    return apiClient.get<PointsPage<PointsLedgerEntry>>('/points-shop/ledger', { params })
  },
  getOrders(params?: { page?: number; page_size?: number }) {
    return apiClient.get<PointsPage<PointsShopOrder>>('/points-shop/orders', { params })
  },
}
