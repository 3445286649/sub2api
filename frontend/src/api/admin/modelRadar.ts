import { apiClient } from '../client'
import type { ModelRadarConfig, ModelRadarRun, ModelRadarRunDetail } from '../modelRadar'

export async function getConfig(): Promise<ModelRadarConfig> {
  const { data } = await apiClient.get<ModelRadarConfig>('/admin/model-radar/config')
  return data
}

export async function updateConfig(config: ModelRadarConfig): Promise<ModelRadarConfig> {
  const { data } = await apiClient.put<ModelRadarConfig>('/admin/model-radar/config', config)
  return data
}

export async function runNow(): Promise<ModelRadarRunDetail> {
  const { data } = await apiClient.post<ModelRadarRunDetail>('/admin/model-radar/run')
  return data
}

export async function listRuns(limit = 30): Promise<{ items: ModelRadarRun[] }> {
  const { data } = await apiClient.get<{ items: ModelRadarRun[] }>('/admin/model-radar/runs', {
    params: { limit },
  })
  return data
}

export async function getRun(id: number): Promise<ModelRadarRunDetail> {
  const { data } = await apiClient.get<ModelRadarRunDetail>(`/admin/model-radar/runs/${id}`)
  return data
}

export const adminModelRadarAPI = {
  getConfig,
  updateConfig,
  runNow,
  listRuns,
  getRun,
}

export default adminModelRadarAPI
