export interface DailyCheckinConfig {
  enabled: boolean
  base_reward: number
  cycle_days: number
  milestone_7: number
  milestone_15: number
  milestone_30: number
  min_account_age_hours: number
  require_verified: boolean
  daily_budget: number
  rule_version: number
}

export interface DailyCheckinCycle {
  id: number
  cycle_number: number
  status: 'active' | 'completed'
  cycle_days: number
  checkin_count: number
  consecutive_days: number
  started_on: string
  last_checkin_on?: string | null
  completed_at?: string | null
  base_reward: number
  milestone_7_reward: number
  milestone_15_reward: number
  milestone_30_reward: number
  rule_version: number
  total_reward: number
}

export interface DailyCheckinRecord {
  id: number
  user_id?: number
  user_email?: string
  business_date: string
  cycle_day: number
  base_reward: number
  milestone_reward: number
  total_reward: number
  balance_before: number
  balance_after: number
  rule_version: number
  created_at: string
}

export interface DailyCheckinStatus {
  config: DailyCheckinConfig
  today: string
  today_claimed: boolean
  eligible: boolean
  ineligible_reason?: 'disabled' | 'inactive' | 'account_too_new' | 'verification_required' | string
  current_cycle?: DailyCheckinCycle | null
  history_reward: number
  completed_cycles: number
  next_milestone?: number | null
  days_to_next_milestone: number
}

export interface DailyCheckinPage {
  items: DailyCheckinRecord[]
  total: number
  page: number
  page_size: number
}

export interface DailyCheckinStats {
  today_claims: number
  today_reward: number
  month_reward: number
  completed_cycles: number
}
