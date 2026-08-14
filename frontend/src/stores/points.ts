import { defineStore } from 'pinia'
import { ref } from 'vue'
import { pointsAPI } from '@/api/points'
import type { PointsProduct, PointsShopOrder, PointsSummary } from '@/types/points'

function redemptionKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `points-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export const usePointsStore = defineStore('points', () => {
  const summary = ref<PointsSummary | null>(null)
  const products = ref<PointsProduct[]>([])
  const loading = ref(false)
  const redeemingProductId = ref<number | null>(null)

  async function fetchSummary(): Promise<PointsSummary | null> {
    try {
      const response = await pointsAPI.getSummary()
      summary.value = response.data
      return summary.value
    } catch {
      summary.value = null
      return null
    }
  }

  async function fetchProducts(): Promise<PointsProduct[]> {
    try {
      const response = await pointsAPI.getProducts()
      products.value = response.data || []
      return products.value
    } catch {
      products.value = []
      return []
    }
  }

  async function load(): Promise<void> {
    if (loading.value) return
    loading.value = true
    try {
      await Promise.all([fetchSummary(), fetchProducts()])
    } finally {
      loading.value = false
    }
  }

  async function redeem(productId: number): Promise<PointsShopOrder> {
    redeemingProductId.value = productId
    try {
      const response = await pointsAPI.redeem(productId, redemptionKey())
      await Promise.all([fetchSummary(), fetchProducts()])
      return response.data
    } finally {
      redeemingProductId.value = null
    }
  }

  return { summary, products, loading, redeemingProductId, fetchSummary, fetchProducts, load, redeem }
})
