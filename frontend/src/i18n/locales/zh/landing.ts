export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    viewOnGithub: '在 GitHub 上查看',
    // Hero 下方信任条（原为模板内硬编码中文，英文界面会露出中文）
    proof: {
      sdk: 'OpenAI SDK 兼容',
      setup: '5 分钟接入',
      billing: '按用量付费',
      quality: '满血不降智 · 公开监测'
    },
    // 紧凑首页
    compact: {
      trust: '支持 Claude Code · Codex · Gemini CLI'
    },
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    signUp: '注册',
    goToDashboard: '进入控制台',
    // 新增：面向用户的价值主张
    eyebrow: 'AI API 网关平台',
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。',
      auth: '统一鉴权',
      authDesc: '每个 API Key 独立签发，严格校验身份，原始订阅凭据永不外泄。',
      rate: '精细限额',
      rateDesc: '按 Key、模型和时段设置调用上限，超额自动拦截，杜绝滥用。',
      cost: '精准计费',
      costDesc: '记录每次请求的 Token 消耗与费用，账单清晰可追溯。',
      route: '永不掉线的账号池',
      routeDesc: '多账号自动负载均衡，上游 429 或超时自动切换备用账号，客户端完全无感。'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        },
        quality: {
          feature: '模型质量',
          official: '常见中转可能偷换、降智，无从验证',
          us: '满血直连，定时公开检测'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多',
      viewAll: '查看全部模型'
    },
    // 工作原理舞台里的节点名（HowItWorks）
    architecture: {
      client: {
        title: '你的客户端',
        desc: 'Claude Code / Codex / Gemini CLI / OpenAI SDK'
      },
      gateway: {
        title: '网关层',
        auth: '鉴权',
        limit: '限流',
        route: '智能调度',
        bill: '计量计费'
      },
      pool: {
        title: '账号池'
      },
      upstream: {
        title: '上游模型',
        desc: 'Anthropic · OpenAI · Gemini · Antigravity'
      }
    },
    // 「请求旅程」动画（Hero 右侧 / 登录页左栏 / 紧凑首页）
    journey: {
      title: '请求旅程',
      live: '实时',
      labels: {
        auth: '鉴权',
        quota: '限额',
        route: '调度',
        session: '会话',
        upstream: '上游',
        retry: '重试'
      },
      scenes: {
        normal: {
          auth: 'API Key 校验通过',
          quota: '并发 3/10 · 额度充足',
          route: '账号池 128 可用 → 选中 #047（延迟最低）',
          upstream: 'claude-opus-4 → 200 OK',
          total: '1,842 tokens',
          cost: '$0.0271',
          bound: '会话已绑定'
        },
        sticky: {
          session: '命中粘性会话 → 沿用账号 #047',
          upstream: '200 OK（上下文已复用）',
          cacheLabel: '缓存读取',
          cacheValue: '12,480 tokens',
          savedLabel: '费用降低',
          savedValue: '78%'
        },
        failover: {
          route: '账号池 → 选中 #012',
          upstreamFail: '429 Too Many Requests',
          retry: '自动切换备用账号 #089',
          upstreamOk: '200 OK',
          transparent: '客户端无感',
          switches: '累计切换 1 次',
          overhead: '耗时 +140ms'
        }
      },
      rail: {
        pool: '账号池可用',
        success: '请求成功率',
        latency: 'P95 延迟'
      }
    },
    // 顶栏章节锚点
    nav: {
      how: '工作原理',
      compare: '对比',
      models: '模型',
      quality: '满血保障',
      start: '开始使用'
    },
    scrollHint: '向下滚动',
    // 工作原理（桌面钉屏演示 / 窄屏时间线）
    how: {
      title: '一次请求，经过五道处理',
      subtitle: '你只管发请求，剩下的交给网关',
      steps: {
        auth: {
          title: '统一鉴权',
          desc: '每个 API Key 独立签发、严格校验，原始订阅凭据永不外泄。',
          detail: 'sk-••••a3f9 → 校验通过 · 12ms'
        },
        limit: {
          title: '精细限额',
          desc: '按 Key、模型和时段设置上限，超额自动拦截，杜绝滥用。',
          detail: '并发 3/10 · 今日额度充足 · 8ms'
        },
        route: {
          title: '智能调度',
          desc: '从账号池挑选延迟最低的账号；同一会话自动回到同一账号，缓存命中更省钱。',
          detail: '账号池 128 可用 → 选中 #047 · 会话已绑定'
        },
        failover: {
          title: '故障转移',
          desc: '上游 429 或超时时自动切换备用账号并重试，客户端完全无感。',
          detail: '#012 → 429 · 自动切换 #089 → 200 OK · +140ms'
        },
        billing: {
          title: '逐笔计费',
          desc: '每次请求的 Token 与费用实时入账，随时可查、逐笔可核验。',
          detail: '1,842 tokens · $0.0271 · 已入账'
        }
      }
    },
    // 满血保障：公开降智监测
    quality: {
      live: '实时监测中',
      title: '满血模型，不降智',
      subtitle: '满血分组定时自动检测，每一次结果都公开可查，也欢迎你亲自来测',
      lead: '定时自动检测推理题答对率，每一次结果都公开可查',
      interval: '每 {minutes} 分钟检测一次',
      passRate: '通过率',
      total: '总次数',
      passed: '通过',
      failed: '答错',
      errored: '失败',
      latest: '最新',
      next: '下次',
      reasoning: '推理',
      status: {
        passed: '通过',
        failed: '答错',
        error: '失败'
      },
      empty: '监测数据暂不可用',
      error: '加载失败',
      stale: '数据同步暂时失败，显示的可能不是最新结果',
      visit: '访问监测站',
      detail: '查看完整监测记录',
      tests: {
        logic: '逻辑测试',
        creative: '生成测试'
      },
      legend: {
        passed: '通过',
        failed: '疑似降智',
        error: '接口异常（不计入）',
        empty: '暂无记录'
      },
      barLabel: '{name}最近检测：通过 {passed} 次，疑似降智 {failed} 次，接口异常 {errored} 次',
      historyWindow: '最近 {n} 次检测 · 已有 {count} 条 · 左旧右新',
      updatedAt: '最近一次检测：{time}',
      staleAt: '同步暂时失败，显示的是 {time} 的数据',
      justNow: '刚刚',
      minutesAgo: '{n} 分钟前',
      hoursAgo: '{n} 小时前',
      viewAll: '查看完整监测',
      unavailable: '监测数据暂时无法同步，可前往监测站查看最新结果。',
      methods: {
        logic: {
          title: '逻辑测试',
          desc: '固定的推理题，降智模型很容易答错，答错即判不通过。'
        },
        creative: {
          title: '生成测试',
          desc: '让模型画一只骑自行车的鹈鹕，再由 AI 评估作品质量。'
        },
        self: {
          title: '自己来测',
          desc: '填入你自己的接口和 Key，用同一套题跑一遍，结果只有你能看到。',
          action: '去检测'
        }
      }
    },
    // 最后一页：开始使用
    start: {
      title: '现在开始，3 步接入',
      subtitle: '不用改代码，换个地址就能用',
      guide: '接入指南',
      usage: '查询 Key 用量',
      copy: '复制',
      copied: '已复制',
      keyPlaceholder: 'YOUR_API_KEY',
      steps: {
        register: {
          title: '注册账号',
          desc: '邮箱注册，即刻开通'
        },
        key: {
          title: '创建 API Key',
          desc: '在控制台一键生成'
        },
        baseUrl: {
          title: '替换 Base URL',
          desc: '复制下方配置即可'
        }
      }
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      allRightsReserved: '保留所有权利。',
      product: '产品',
      resources: '资源',
      legal: '条款',
      keyUsage: '用量查询',
      contact: '联系方式'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    billingType: '计费方式',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API 安装向导',
    description: '配置您的 Sub2API 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
