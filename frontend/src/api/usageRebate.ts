import apiClient from './client'

export interface UsageRebateRate {
  rank: number
  percent: string | number
}

export interface UsageRebatePosition {
  rank: number | null
  participant_count: number
  requests: number
  tokens: number
  spend_amount: string | number
  rebate_percent: string | number
  estimated_reward: string | number
  eligible: boolean
  previous_rank: number | null
  gap_to_previous: string | number | null
  gap_to_top20: string | number | null
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
  my_position: UsageRebatePosition
  my_rewards: UsageRebateReward[]
}

const emptyPosition: UsageRebatePosition = {
  rank: null,
  participant_count: 0,
  requests: 0,
  tokens: 0,
  spend_amount: 0,
  rebate_percent: 0,
  estimated_reward: 0,
  eligible: false,
  previous_rank: null,
  gap_to_previous: null,
  gap_to_top20: null,
}

export async function getUsageRebateOverview(): Promise<UsageRebateOverview> {
  const { data } = await apiClient.get<UsageRebateOverview>('/usage-rebate')
  return {
    ...data,
    rates: Array.isArray(data.rates) ? data.rates : [],
    my_position: data.my_position ?? { ...emptyPosition },
    my_rewards: Array.isArray(data.my_rewards) ? data.my_rewards : [],
  }
}

export default { getOverview: getUsageRebateOverview }
