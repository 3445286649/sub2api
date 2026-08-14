export interface PointsConfig {
  enabled: boolean
  invite_threshold_amount: number
  invite_reward_points: number
  qualification_window_days: number
  freeze_hours: number
}

export interface PointsAccount {
  available_points: number
  frozen_points: number
  debt_points: number
  lifetime_earned: number
  lifetime_spent: number
}

export interface PointsSummary {
  account: PointsAccount
  config: PointsConfig
}

export interface PointsProduct {
  id: number
  product_type: 'balance'
  name: string
  description: string
  points_price: number
  original_points_price?: number | null
  balance_amount: number
  stock_total?: number | null
  stock_redeemed: number
  per_user_limit?: number | null
  features: string
  sort_order: number
  for_sale: boolean
  created_at: string
  updated_at: string
}

export type PointsProductInput = Omit<PointsProduct, 'id' | 'stock_redeemed' | 'created_at' | 'updated_at'>

export interface PointsShopOrder {
  id: number
  order_no: string
  user_id: number
  user_email?: string
  product_id?: number | null
  product_name: string
  product_type: string
  points_price: number
  balance_amount: number
  status: string
  balance_after: number
  created_at: string
  completed_at: string
}

export interface PointsLedgerEntry {
  id: number
  action: string
  delta_available: number
  delta_frozen: number
  delta_debt: number
  available_after: number
  frozen_after: number
  debt_after: number
  description: string
  created_at: string
}

export interface PointsPage<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}
