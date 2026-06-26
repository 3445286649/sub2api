import { apiClient } from './client'

export interface AcquisitionCampaign {
  id: number
  name: string
  status: 'draft' | 'active' | 'settling' | 'settled' | string
  starts_at: string
  ends_at: string
  leaderboard_enabled: boolean
  lottery_enabled: boolean
  leaderboard_pool_usd: number
  leaderboard_shares: number[]
  lottery_prize_configs: AcquisitionLotteryPrizeConfig[]
  lottery_seed: string
  settled_at?: string | null
  created_at: string
  updated_at: string
}

export interface AcquisitionLotteryPrizeConfig {
  name: string
  amount_usd: number
  count: number
  per_user_cap: number
}

export interface AcquisitionLeaderboardRow {
  user_id: number
  email: string
  username: string
  invite_count: number
  last_completed_at?: string | null
  rank: number
  reward_amount: number
}

export interface AcquisitionReward {
  id: number
  campaign_id: number
  user_id: number
  reward_type: 'leaderboard' | 'lottery' | string
  reward_key: string
  amount: number
  rank?: number
  prize_name?: string
  ticket_id?: number | null
  status: 'pending' | 'paid' | 'failed' | string
  error_message?: string
  paid_at?: string | null
  created_at: string
}

export interface AcquisitionUserSummary {
  campaign?: AcquisitionCampaign | null
  aff_code?: string
  invite_link?: string
  valid_invites: number
  rank: number
  ticket_count: number
  leaderboard: AcquisitionLeaderboardRow[]
  rewards: AcquisitionReward[]
}

export function normalizeAcquisitionCampaign(campaign: AcquisitionCampaign): AcquisitionCampaign {
  return {
    ...campaign,
    leaderboard_shares: Array.isArray(campaign.leaderboard_shares) ? campaign.leaderboard_shares : [],
    lottery_prize_configs: Array.isArray(campaign.lottery_prize_configs) ? campaign.lottery_prize_configs : [],
  }
}

export function normalizeAcquisitionSummary(summary: AcquisitionUserSummary): AcquisitionUserSummary {
  return {
    ...summary,
    campaign: summary.campaign ? normalizeAcquisitionCampaign(summary.campaign) : summary.campaign,
    leaderboard: Array.isArray(summary.leaderboard) ? summary.leaderboard : [],
    rewards: Array.isArray(summary.rewards) ? summary.rewards : [],
  }
}

export async function getCurrent(): Promise<AcquisitionUserSummary> {
  const { data } = await apiClient.get<AcquisitionUserSummary>('/acquisition/current')
  return normalizeAcquisitionSummary(data)
}

export const acquisitionAPI = {
  getCurrent,
}

export default acquisitionAPI
