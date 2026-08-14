import { apiClient } from '../client'
import type { PointsConfig, PointsPage, PointsProduct, PointsProductInput, PointsShopOrder } from '@/types/points'

export const adminPointsAPI = {
  getConfig() {
    return apiClient.get<PointsConfig>('/admin/points-shop/config')
  },
  updateConfig(data: PointsConfig) {
    return apiClient.put<PointsConfig>('/admin/points-shop/config', data)
  },
  getProducts() {
    return apiClient.get<PointsProduct[]>('/admin/points-shop/products')
  },
  createProduct(data: PointsProductInput) {
    return apiClient.post<PointsProduct>('/admin/points-shop/products', data)
  },
  updateProduct(id: number, data: PointsProductInput) {
    return apiClient.put<PointsProduct>(`/admin/points-shop/products/${id}`, data)
  },
  deleteProduct(id: number) {
    return apiClient.delete(`/admin/points-shop/products/${id}`)
  },
  getOrders(params?: { page?: number; page_size?: number }) {
    return apiClient.get<PointsPage<PointsShopOrder>>('/admin/points-shop/orders', { params })
  },
}
