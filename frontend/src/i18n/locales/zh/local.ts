export default {
  common: {
    apply: '应用',
    clear: '清除',
    creating: '创建中...',
    previous: '上一步',
    required: '必填',
    sending: '发送中...',
    tryAgain: '重试',
  },
  admin: {
    accounts: {
      fromModel: '原模型',
      toModel: '目标模型',
      messages: {
        accountCreated: '账号创建成功',
      },
      oauth: {
        openai: {
          accessTokenAuth: 'Access Token 认证',
          mobileRefreshTokenAuth: '移动端 Refresh Token 认证',
        },
      },
    },
    channels: {
      emptyModelsInPricing: '{platform} 价格配置中至少需要一个模型',
      noGroupsSelected: '请至少选择一个分组',
    },
    ops: {
      runtime: {
        metricThresholds: '指标阈值',
        metricThresholdsHint: '配置运行时指标触发告警的阈值。',
        requestErrorRateMaxPercent: '请求错误率上限',
        requestErrorRateMaxPercentHint: '请求错误率超过该百分比时触发告警。',
        slaMinPercent: 'SLA 下限',
        slaMinPercentHint: 'SLA 低于该百分比时触发告警。',
        ttftP99MaxMs: 'P99 首 Token 延迟上限',
        ttftP99MaxMsHint: 'P99 首 Token 延迟超过该毫秒值时触发告警。',
        upstreamErrorRateMaxPercent: '上游错误率上限',
        upstreamErrorRateMaxPercentHint: '上游错误率超过该百分比时触发告警。',
      },
    },
    users: {
      passwordCopied: '密码已复制',
    },
  },
} as const
