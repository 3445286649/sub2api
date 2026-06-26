import { apiClient } from '../client'
import type {
  AcquisitionCampaign,
  AcquisitionLotteryPrizeConfig,
  AcquisitionUserSummary,
} from '../acquisition'
import {
  normalizeAcquisitionCampaign,
  normalizeAcquisitionSummary,
} from '../acquisition'

export interface AcquisitionCampaignInput {
  name: string
  status: 'draft' | 'active' | string
  starts_at: string
  ends_at: string
  leaderboard_enabled: boolean
  lottery_enabled: boolean
  leaderboard_pool_usd: number
  leaderboard_shares: number[]
  lottery_prize_configs: AcquisitionLotteryPrizeConfig[]
  lottery_seed?: string
}

export async function listCampaigns(params: { status?: string } = {}): Promise<{ items: AcquisitionCampaign[] }> {
  const { data } = await apiClient.get<{ items: AcquisitionCampaign[] }>('/admin/acquisition/campaigns', {
    params,
  })
  return {
    ...data,
    items: Array.isArray(data.items) ? data.items.map(normalizeAcquisitionCampaign) : [],
  }
}

export async function getCampaign(id: number): Promise<AcquisitionUserSummary> {
  const { data } = await apiClient.get<AcquisitionUserSummary>(`/admin/acquisition/campaigns/${id}`)
  return normalizeAcquisitionSummary(data)
}

export async function createCampaign(payload: AcquisitionCampaignInput): Promise<AcquisitionCampaign> {
  const { data } = await apiClient.post<AcquisitionCampaign>('/admin/acquisition/campaigns', payload)
  return normalizeAcquisitionCampaign(data)
}

export async function updateCampaign(id: number, payload: AcquisitionCampaignInput): Promise<AcquisitionCampaign> {
  const { data } = await apiClient.put<AcquisitionCampaign>(`/admin/acquisition/campaigns/${id}`, payload)
  return normalizeAcquisitionCampaign(data)
}

export async function settleCampaign(id: number): Promise<{ id: number }> {
  const { data } = await apiClient.post<{ id: number }>(`/admin/acquisition/campaigns/${id}/settle`)
  return data
}

export const acquisitionAdminAPI = {
  listCampaigns,
  getCampaign,
  createCampaign,
  updateCampaign,
  settleCampaign,
}

export default acquisitionAdminAPI
