import apiClient from './client'

export interface UsageRebateRate {
  rank: number
  percent: string | number
}

export interface UsageRebateCandidate {
  username: string
  rank: number
  requests: number
  tokens: number
  spend_amount: string | number
  rebate_percent: string | number
  estimated_reward: string | number
  is_me: boolean
}

export interface UsageRebateReward {
  business_date: string
  rank: number
  spend_amount: string | number
  rebate_percent: string | number
  reward_amount: string | number
  status: 'pending' | 'credited' | 'failed' | 'unknown'
  credited_at?: string | null
}

export interface UsageRebateOverview {
  enabled: boolean
  business_date: string
  timezone: string
  settlement_time: string
  rates: UsageRebateRate[]
  leaderboard: UsageRebateCandidate[]
  my_rewards: UsageRebateReward[]
}

export async function getUsageRebateOverview(): Promise<UsageRebateOverview> {
  const { data } = await apiClient.get<UsageRebateOverview>('/usage-rebate')
  return {
    ...data,
    rates: Array.isArray(data.rates) ? data.rates : [],
    leaderboard: Array.isArray(data.leaderboard) ? data.leaderboard : [],
    my_rewards: Array.isArray(data.my_rewards) ? data.my_rewards : [],
  }
}

export default { getOverview: getUsageRebateOverview }
