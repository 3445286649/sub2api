/**
 * 首页「满血保障」监测数据（公开端点，可匿名访问）。
 * 后端从站长配置的公开降智监测站拉取并缓存，这里只拿精简后的结果。
 */

import { apiClient } from './client'

export type QualityStatus = 'passed' | 'failed' | 'error'

export interface QualityMonitorPoint {
  /** passed=通过，failed=答错/疑似降智，error=接口异常（不计入通过率） */
  s: QualityStatus
  /** 检测完成时间（ISO 8601） */
  t: string
}

export interface QualityMonitorGroup {
  /** 监测站分组固定标识，如 channel-1 */
  id: string
  name: string
  model: string
  /** 推理程度；为空表示接口默认 */
  reasoning_effort?: string
  /** 检测间隔；0 表示定时检测未启用 */
  interval_seconds: number
  next_run_at?: string
  /** 通过 ÷（通过 + 疑似降智）；没有有效结果时为 null */
  pass_rate: number | null
  passed: number
  failed: number
  errored: number
  latest_status?: QualityStatus
  /** 最近最多 60 次检测，从旧到新 */
  history: QualityMonitorPoint[]
}

export interface QualityMonitorSnapshot {
  detail_url: string
  updated_at: string
  /** 上游暂时拉取失败，当前为缓存的旧数据 */
  stale: boolean
  groups: QualityMonitorGroup[]
}

export interface QualityMonitorResponse {
  /** 后台是否配置了监测地址；未配置时首页整章不显示 */
  enabled: boolean
  /** 当前是否有可展示的数据；为 false 时只展示静态文案 */
  available: boolean
  data?: QualityMonitorSnapshot
  /** available=false 时后端仍会给出监测站地址，供首页放链接 */
  detail_url?: string
}

export async function getQualityMonitor(): Promise<QualityMonitorResponse> {
  const { data } = await apiClient.get<QualityMonitorResponse>('/settings/quality-monitor')
  return data
}
