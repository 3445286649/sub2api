/**
 * 首页「满血保障」监测数据（公开端点，可匿名访问）。
 * 后端从站长配置的公开降智监测站拉取并缓存，这里只拿精简后的结果。
 */

import { apiClient } from './client'

export type QualityStatus = 'passed' | 'failed' | 'error'

export interface QualityMonitorPoint {
  /** passed=通过，failed=未通过/疑似降智，error=接口异常（不计入通过率） */
  s: QualityStatus
  /** 检测完成时间（ISO 8601） */
  t: string
}

export interface QualityMonitorTest {
  /** logic=逻辑测试，creative=生成测试 */
  key: 'logic' | 'creative' | string
  /** 通过 ÷（通过 + 未通过）；没有有效结果时为 null */
  pass_rate: number | null
  passed: number
  failed: number
  errored: number
  history: QualityMonitorPoint[]
}

export interface QualityMonitorSnapshot {
  group_name: string
  model: string
  interval_seconds: number
  detail_url: string
  updated_at: string
  /** 上游暂时拉取失败，当前为缓存的旧数据 */
  stale: boolean
  tests: QualityMonitorTest[]
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
