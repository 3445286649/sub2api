import { apiClient } from './client'

export type ModelRadarStatus = 'pending' | 'running' | 'succeeded' | 'failed'
export type ModelRadarTrigger = 'scheduled' | 'manual'

export interface ModelRadarCombination {
  model: string
  reasoning_effort: string
  enabled: boolean
}

export interface ModelRadarConfig {
  enabled: boolean
  api_base_url: string
  api_key_source: 'custom' | 'existing'
  api_key_id?: number | null
  api_key_name?: string
  api_key_group_name?: string
  api_key_status?: string
  api_key?: string
  api_key_configured: boolean
  api_key_masked: string
  run_hour: number
  run_minute: number
  timeout_seconds: number
  concurrency: number
  daily_budget_usd_cents: number
  matrix: ModelRadarCombination[]
}

export interface ModelRadarRun {
  id: number
  run_date: string
  trigger_type: ModelRadarTrigger
  status: ModelRadarStatus
  published: boolean
  started_at: string | null
  finished_at: string | null
  total_combinations: number
  success_combinations: number
  error_message: string
  created_at: string
  updated_at: string
}

export interface ModelRadarResult {
  id: number
  run_id: number
  model: string
  reasoning_effort: string
  score: number
  pass_count: number
  total_count: number
  avg_latency_ms: number | null
  error_count: number
  status: ModelRadarStatus
  rank: number
  error_message: string
  created_at: string
  updated_at: string
}

export interface ModelRadarTaskResult {
  id: number
  result_id: number
  task_id: string
  task_version: number
  passed: boolean
  expected_answer: string
  actual_answer: string
  latency_ms: number | null
  error_message: string
  created_at: string
}

export interface ModelRadarCurrentResponse {
  run: ModelRadarRun | null
  recommendation: ModelRadarResult | null
  results: ModelRadarResult[]
  history: ModelRadarResult[]
  updated_at: string | null
}

export interface ModelRadarRunDetail {
  run: ModelRadarRun
  results: ModelRadarResult[]
  task_results: Record<string, ModelRadarTaskResult[]>
}

export async function getCurrent(): Promise<ModelRadarCurrentResponse> {
  const { data } = await apiClient.get<ModelRadarCurrentResponse>('/model-radar/current')
  return data
}

export const modelRadarAPI = {
  getCurrent,
}

export default modelRadarAPI
