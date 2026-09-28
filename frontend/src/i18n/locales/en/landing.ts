export default {
  batchImageGuide: {
    title: 'Batch Image Generation',
    description: 'Submit multiple prompts in one job and download the generated images when complete'
  },
  // Home Page
  home: {
    viewOnGithub: 'View on GitHub',
    // Hero trust row (previously hardcoded Chinese in the template)
    proof: {
      sdk: 'OpenAI SDK compatible',
      setup: 'Set up in 5 minutes',
      billing: 'Pay as you go',
      quality: 'Full-strength models · publicly monitored'
    },
    // Compact home
    compact: {
      trust: 'Works with Claude Code · Codex · Gemini CLI'
    },
    viewDocs: 'View Documentation',
    docs: 'Docs',
    switchToLight: 'Switch to Light Mode',
    switchToDark: 'Switch to Dark Mode',
    dashboard: 'Dashboard',
    login: 'Login',
    getStarted: 'Get Started',
    goToDashboard: 'Go to Dashboard',
    // User-focused value proposition
    heroSubtitle: 'One Key, All AI Models',
    heroDescription: 'No need to manage multiple subscriptions. Access Claude, GPT, Gemini and more with a single API key',
    tags: {
      subscriptionToApi: 'Subscription to API',
      stickySession: 'Session Persistence',
      realtimeBilling: 'Pay As You Go'
    },
    // Pain points section
    painPoints: {
      title: 'Sound Familiar?',
      items: {
        expensive: {
          title: 'High Subscription Costs',
          desc: 'Paying for multiple AI subscriptions that add up every month'
        },
        complex: {
          title: 'Account Chaos',
          desc: 'Managing scattered accounts and API keys across different platforms'
        },
        unstable: {
          title: 'Service Interruptions',
          desc: 'Single accounts hitting rate limits and disrupting your workflow'
        },
        noControl: {
          title: 'No Usage Control',
          desc: "Can't track where your money goes or limit team member usage"
        }
      }
    },
    // Solutions section
    solutions: {
      title: 'We Solve These Problems',
      subtitle: 'Three simple steps to stress-free AI access'
    },
    features: {
      unifiedGateway: 'One-Click Access',
      unifiedGatewayDesc: 'Get a single API key to call all connected AI models. No separate applications needed.',
      multiAccount: 'Always Reliable',
      multiAccountDesc: 'Smart routing across multiple upstream accounts with automatic failover. Say goodbye to errors.',
      balanceQuota: 'Pay What You Use',
      balanceQuotaDesc: 'Usage-based billing with quota limits. Full visibility into team consumption.',
      auth: 'Unified Auth',
      authDesc: 'Issue an independent API key for every client while keeping upstream credentials private.',
      rate: 'Fine-grained Limits',
      rateDesc: 'Set request limits by key, model, and time window to prevent abuse.',
      cost: 'Accurate Billing',
      costDesc: 'Track token usage and cost for every request with a clear audit trail.',
      route: 'A Pool That Never Drops',
      routeDesc: 'Automatic load balancing across accounts. On a 429 or timeout, requests fail over to a backup — clients never notice.'
    },
    // Comparison section
    comparison: {
      title: 'Why Choose Us?',
      headers: {
        feature: 'Comparison',
        official: 'Official Subscriptions',
        us: 'Our Platform'
      },
      items: {
        pricing: {
          feature: 'Pricing',
          official: 'Fixed monthly fee, pay even if unused',
          us: 'Pay only for what you use'
        },
        models: {
          feature: 'Model Selection',
          official: 'Single provider only',
          us: 'Switch between models freely'
        },
        management: {
          feature: 'Account Management',
          official: 'Manage each service separately',
          us: 'Unified key, one dashboard'
        },
        stability: {
          feature: 'Stability',
          official: 'Single account rate limits',
          us: 'Multi-account pool, auto-failover'
        },
        control: {
          feature: 'Usage Control',
          official: 'Not available',
          us: 'Quotas & detailed analytics'
        },
        quality: {
          feature: 'Model Quality',
          official: 'Many relays may swap or downgrade models, with no way to check',
          us: 'Full-strength models, publicly tested on a schedule'
        }
      }
    },
    providers: {
      title: 'Supported AI Models',
      description: 'One API, Multiple Choices',
      supported: 'Supported',
      soon: 'Soon',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: 'More',
      viewAll: 'View all models'
    },
    // Node labels on the How-it-works stage (HowItWorks)
    architecture: {
      client: {
        title: 'Your Client',
        desc: 'Claude Code / Codex / Gemini CLI / OpenAI SDK'
      },
      gateway: {
        title: 'Gateway',
        auth: 'Auth',
        limit: 'Rate limit',
        route: 'Smart routing',
        bill: 'Metering'
      },
      pool: {
        title: 'Account Pool'
      },
      upstream: {
        title: 'Upstream Models',
        desc: 'Anthropic · OpenAI · Gemini · Antigravity'
      }
    },
    // Request journey animation (Hero right / login aside / compact home)
    journey: {
      title: 'Request Journey',
      live: 'Live',
      labels: {
        auth: 'Auth',
        quota: 'Quota',
        route: 'Route',
        session: 'Session',
        upstream: 'Upstream',
        retry: 'Retry'
      },
      scenes: {
        normal: {
          auth: 'API Key verified',
          quota: 'Concurrency 3/10 · quota available',
          route: 'Pool of 128 → picked #047 (lowest latency)',
          upstream: 'claude-opus-4 → 200 OK',
          total: '1,842 tokens',
          cost: '$0.0271',
          bound: 'session pinned'
        },
        sticky: {
          session: 'Sticky session hit → reusing account #047',
          upstream: '200 OK (context reused)',
          cacheLabel: 'Cache read',
          cacheValue: '12,480 tokens',
          savedLabel: 'Cost reduced',
          savedValue: '78%'
        },
        failover: {
          route: 'Pool → picked #012',
          upstreamFail: '429 Too Many Requests',
          retry: 'Auto-switched to backup #089',
          upstreamOk: '200 OK',
          transparent: 'Client unaware',
          switches: '1 switch total',
          overhead: '+140ms elapsed'
        }
      },
      rail: {
        pool: 'Accounts available',
        success: 'Success rate',
        latency: 'P95 latency'
      }
    },
    // CTA section
    // Top bar section anchors
    nav: {
      how: 'How it works',
      compare: 'Compare',
      models: 'Models',
      quality: 'Quality',
      start: 'Get started'
    },
    scrollHint: 'Scroll',
    // How it works (pinned walkthrough on desktop / timeline on small screens)
    how: {
      title: 'One request, five steps',
      subtitle: 'You send the request. The gateway handles the rest.',
      steps: {
        auth: {
          title: 'Unified auth',
          desc: 'Every API key is issued and verified independently. Your subscription credentials never leak.',
          detail: 'sk-••••a3f9 → verified · 12ms'
        },
        limit: {
          title: 'Fine-grained limits',
          desc: 'Cap usage per key, model and time window. Overages are blocked automatically.',
          detail: 'concurrency 3/10 · daily quota OK · 8ms'
        },
        route: {
          title: 'Smart routing',
          desc: 'Picks the lowest-latency account from the pool and keeps each session on the same account for cheaper cache hits.',
          detail: '128 accounts available → #047 selected · session bound'
        },
        failover: {
          title: 'Automatic failover',
          desc: 'On upstream 429s or timeouts it switches to a standby account and retries. Your client never notices.',
          detail: '#012 → 429 · switched to #089 → 200 OK · +140ms'
        },
        billing: {
          title: 'Per-request billing',
          desc: 'Tokens and cost for every request are recorded in real time and can be audited one by one.',
          detail: '1,842 tokens · $0.0271 · recorded'
        }
      }
    },
    // Quality guarantee: public model-quality monitor
    quality: {
      live: 'Live monitoring',
      title: 'Full-strength models. No silent downgrades.',
      subtitle: 'The full-strength group is tested automatically on a schedule. Every result is public, and you can run the tests yourself.',
      interval: 'Tested every {minutes} min',
      passRate: 'valid pass rate',
      tests: {
        logic: 'Reasoning test',
        creative: 'Generation test'
      },
      legend: {
        passed: 'Passed',
        failed: 'Possible downgrade',
        error: 'API error (not counted)'
      },
      barLabel: '{name}: {passed} passed, {failed} possible downgrades, {errored} API errors',
      updatedAt: 'Last test: {time}',
      staleAt: 'Sync failed, showing data from {time}',
      justNow: 'just now',
      minutesAgo: '{n} min ago',
      hoursAgo: '{n} h ago',
      viewAll: 'View full monitor',
      unavailable: 'Monitor data is temporarily unavailable. Check the monitor site for the latest results.',
      methods: {
        logic: {
          title: 'Reasoning test',
          desc: 'A fixed reasoning puzzle that downgraded models tend to get wrong. A wrong answer fails the run.'
        },
        creative: {
          title: 'Generation test',
          desc: 'The model draws a pelican riding a bicycle, and an AI grader reviews the result.'
        },
        self: {
          title: 'Test it yourself',
          desc: 'Plug in your own endpoint and key to run the same tests. Only you can see the result.',
          action: 'Run a test'
        }
      }
    },
    // Final page: get started
    start: {
      title: 'Get started in 3 steps',
      subtitle: 'No code changes. Just point to a new URL.',
      guide: 'Setup guide',
      usage: 'Check key usage',
      copy: 'Copy',
      copied: 'Copied',
      keyPlaceholder: 'YOUR_API_KEY',
      steps: {
        register: {
          title: 'Create an account',
          desc: 'Sign up with email in seconds'
        },
        key: {
          title: 'Create an API key',
          desc: 'One click in the dashboard'
        },
        baseUrl: {
          title: 'Swap the Base URL',
          desc: 'Copy the config below'
        }
      }
    },
    cta: {
      title: 'Ready to Get Started?',
      description: 'Sign up now and get free trial credits to experience seamless AI access',
      button: 'Sign Up Free'
    },
    footer: {
      allRightsReserved: 'All rights reserved.',
      product: 'Product',
      resources: 'Resources',
      legal: 'Legal',
      keyUsage: 'Key usage',
      contact: 'Contact'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key Usage',
    subtitle: 'Enter your API Key to view real-time spending and usage status',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: 'Query',
    querying: 'Querying...',
    privacyNote: 'Your Key is processed locally in the browser and will not be stored',
    dateRange: 'Date Range:',
    dateRangeToday: 'Today',
    dateRange7d: '7 Days',
    dateRange30d: '30 Days',
    dateRange90d: '90 Days',
    dateRangeCustom: 'Custom',
    apply: 'Apply',
    used: 'Used',
    detailInfo: 'Detail Information',
    tokenStats: 'Token Statistics',
    dailyDetail: 'Daily Detail',
    modelStats: 'Model Usage Statistics',
    // Table headers
    date: 'Date',
    model: 'Model',
    requests: 'Requests',
    inputTokens: 'Input Tokens',
    outputTokens: 'Output Tokens',
    cacheCreationTokens: 'Cache Creation',
    cacheReadTokens: 'Cache Read',
    cacheWriteTokens: 'Cache Write',
    totalTokens: 'Total Tokens',
    cost: 'Cost',
    // Status
    quotaMode: 'Key Quota Mode',
    walletBalance: 'Wallet Balance',
    // Ring card titles
    totalQuota: 'Total Quota',
    limit5h: '5-Hour Limit',
    limitDaily: 'Daily Limit',
    limit7d: '7-Day Limit',
    limitWeekly: 'Weekly Limit',
    limitMonthly: 'Monthly Limit',
    // Detail rows
    remainingQuota: 'Remaining Quota',
    expiresAt: 'Expires At',
    todayExpires: '(expires today)',
    daysLeft: '({days} days)',
    usedQuota: 'Used Quota',
    resetNow: 'Resetting soon',
    subscriptionType: 'Subscription Type',
    billingType: 'Billing Type',
    subscriptionExpires: 'Subscription Expires',
    // Usage stat cells
    todayRequests: 'Today Requests',
    todayInputTokens: 'Today Input',
    todayOutputTokens: 'Today Output',
    todayTokens: 'Today Tokens',
    todayCacheCreation: 'Today Cache Creation',
    todayCacheRead: 'Today Cache Read',
    todayCost: 'Today Cost',
    rpmTpm: 'RPM / TPM',
    totalRequests: 'Total Requests',
    totalInputTokens: 'Total Input',
    totalOutputTokens: 'Total Output',
    totalTokensLabel: 'Total Tokens',
    totalCacheCreation: 'Total Cache Creation',
    totalCacheRead: 'Total Cache Read',
    totalCost: 'Total Cost',
    avgDuration: 'Avg Duration',
    // Messages
    enterApiKey: 'Please enter an API Key',
    querySuccess: 'Query successful',
    queryFailed: 'Query failed',
    queryFailedRetry: 'Query failed, please try again later',
    noDailyUsage: 'No daily usage data',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API Setup',
    description: 'Configure your Sub2API instance',
    database: {
      title: 'Database Configuration',
      description: 'Connect to your PostgreSQL database',
      host: 'Host',
      port: 'Port',
      username: 'Username',
      password: 'Password',
      databaseName: 'Database Name',
      sslMode: 'SSL Mode',
      passwordPlaceholder: 'Password',
      ssl: {
        disable: 'Disable',
        require: 'Require',
        verifyCa: 'Verify CA',
        verifyFull: 'Verify Full'
      }
    },
    redis: {
      title: 'Redis Configuration',
      description: 'Connect to your Redis server',
      host: 'Host',
      port: 'Port',
      username: 'Username (optional)',
      password: 'Password (optional)',
      database: 'Database',
      usernamePlaceholder: 'Leave empty for default user',
      passwordPlaceholder: 'Password',
      enableTls: 'Enable TLS',
      enableTlsHint: 'Use TLS when connecting to Redis (public CA certs)'
    },
    admin: {
      title: 'Admin Account',
      description: 'Create your administrator account',
      email: 'Email',
      password: 'Password',
      confirmPassword: 'Confirm Password',
      passwordPlaceholder: 'Min 8 characters',
      confirmPasswordPlaceholder: 'Confirm password',
      passwordMismatch: 'Passwords do not match'
    },
    ready: {
      title: 'Ready to Install',
      description: 'Review your configuration and complete setup',
      database: 'Database',
      redis: 'Redis',
      adminEmail: 'Admin Email'
    },
    status: {
      testing: 'Testing...',
      success: 'Connection Successful',
      testConnection: 'Test Connection',
      installing: 'Installing...',
      completeInstallation: 'Complete Installation',
      completed: 'Installation completed!',
      redirecting: 'Redirecting to login page...',
      restarting: 'Service is restarting, please wait...',
      timeout: 'Service restart is taking longer than expected. Please refresh the page manually.'
    }
  },

  // Common
}
