export default {
  common: {
    apply: 'Apply',
    clear: 'Clear',
    creating: 'Creating...',
    previous: 'Previous',
    required: 'Required',
    sending: 'Sending...',
    tryAgain: 'Try Again',
  },
  admin: {
    accounts: {
      fromModel: 'Source model',
      toModel: 'Target model',
      messages: {
        accountCreated: 'Account created successfully',
      },
      oauth: {
        openai: {
          accessTokenAuth: 'Access token authentication',
          mobileRefreshTokenAuth: 'Mobile refresh token authentication',
        },
      },
    },
    affiliates: {
      records: {
        rebateBaseAmount: 'Rebate base amount',
        sourceRedeemCode: 'Source redeem code',
      },
    },
    channels: {
      emptyModelsInPricing: 'Add at least one model to the {platform} pricing configuration',
      noGroupsSelected: 'Select at least one group',
    },
    ops: {
      runtime: {
        metricThresholds: 'Metric thresholds',
        metricThresholdsHint: 'Configure thresholds that trigger runtime metric alerts.',
        requestErrorRateMaxPercent: 'Maximum request error rate',
        requestErrorRateMaxPercentHint: 'Trigger an alert when the request error rate exceeds this percentage.',
        slaMinPercent: 'Minimum SLA',
        slaMinPercentHint: 'Trigger an alert when SLA falls below this percentage.',
        ttftP99MaxMs: 'Maximum P99 TTFT',
        ttftP99MaxMsHint: 'Trigger an alert when P99 time to first token exceeds this value in milliseconds.',
        upstreamErrorRateMaxPercent: 'Maximum upstream error rate',
        upstreamErrorRateMaxPercentHint: 'Trigger an alert when the upstream error rate exceeds this percentage.',
      },
    },
    redeem: {
      affiliateRebateBase: 'Affiliate rebate base amount',
      affiliateRebateBaseHint: 'The amount used to calculate the affiliate rebate for this redeem code.',
      affiliateRebateBasePlaceholder: 'Enter the rebate base amount',
      affiliateRebateBaseShort: 'Rebate base',
    },
    users: {
      passwordCopied: 'Password copied',
    },
  },
} as const
