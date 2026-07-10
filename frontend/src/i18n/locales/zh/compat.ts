// Generated from the pre-split locale. Current modules always take precedence.
export default {
  "nav": {
    "rechargeStorefront": "充值商城",
    "rechargeStorefrontPickerTitle": "选择充值渠道",
    "rechargeStorefrontPickerDescription": "如主卡网异常，可切换备用卡网继续充值。",
    "rechargeStorefrontPrimary": "主卡网",
    "rechargeStorefrontPrimaryHint": "默认推荐，优先使用主充值通道。",
    "rechargeStorefrontBackup": "备用卡网",
    "rechargeStorefrontBackupHint": "主线路异常时，可切换备用通道。",
    "pixmoStudio": "Pixmo 生图"
  },
  "monitorCommon": {
    "attempts": "尝试",
    "failureCategories": {
      "config_error": "检测配置错误",
      "auth_error": "鉴权错误",
      "rate_limited": "上游限流",
      "upstream_error": "上游错误",
      "network_error": "网络错误",
      "timeout": "请求超时",
      "protocol_error": "协议错误",
      "challenge_mismatch": "响应校验异常",
      "empty_response": "空响应"
    }
  },
  "acquisition": {
    "title": "拉新活动",
    "description": "查看当前拉新活动、邀请进度、排行榜和抽奖奖励",
    "emptyTitle": "暂无进行中的拉新活动",
    "emptyDescription": "当前没有可参与的活动，请留意后续开放。",
    "loadFailed": "加载拉新活动失败",
    "inviteCode": "我的邀请码",
    "inviteLink": "邀请链接",
    "copyCode": "复制邀请码",
    "copyLink": "复制链接",
    "codeCopied": "邀请码已复制",
    "linkCopied": "邀请链接已复制",
    "stats": {
      "validInvites": "有效拉新",
      "rank": "当前排名",
      "tickets": "抽奖券",
      "pool": "排行榜奖池"
    },
    "flags": {
      "leaderboardOn": "排行榜已开启",
      "leaderboardOff": "排行榜未开启",
      "lotteryOn": "抽奖已开启",
      "lotteryOff": "抽奖未开启"
    },
    "leaderboard": {
      "title": "活动排行榜",
      "rank": "名次",
      "user": "用户",
      "invites": "有效拉新数",
      "reward": "预计奖励",
      "empty": "暂无有效拉新记录"
    },
    "rewards": {
      "title": "我的奖励",
      "empty": "活动结算后会在这里展示奖励记录",
      "leaderboard": "排行榜第 {rank} 名",
      "lottery": "抽中奖项：{prize}"
    },
    "status": {
      "draft": "草稿",
      "active": "进行中",
      "settling": "结算中",
      "settled": "已结算"
    }
  },
  "redeem": {
    "subscriptionQuotaReset": "订阅额度已刷新",
    "quotaResetScopes": {
      "daily": "日额度",
      "weekly": "周额度",
      "monthly": "月额度",
      "all": "全部额度"
    }
  },
  "admin": {
    "acquisition": {
      "title": "拉新活动",
      "description": "创建周期拉新活动，配置排行榜奖池、抽奖奖项和结算发放。",
      "loadFailed": "加载活动列表失败",
      "detailLoadFailed": "加载活动详情失败",
      "saveSuccess": "活动已保存",
      "saveFailed": "保存活动失败",
      "settleSuccess": "结算任务已执行",
      "settleFailed": "结算活动失败",
      "campaigns": "活动列表",
      "empty": "暂无活动，请先创建一期活动。",
      "pool": "奖池",
      "prizes": "奖项",
      "actions": {
        "create": "创建活动",
        "newDraft": "新建草稿",
        "addPrize": "添加奖项",
        "settle": "结算"
      },
      "status": {
        "draft": "草稿",
        "active": "进行中",
        "settling": "结算中",
        "settled": "已结算"
      },
      "form": {
        "editTitle": "编辑活动",
        "createTitle": "活动配置",
        "defaultName": "拉新活动",
        "defaultPrize": "小额奖",
        "name": "活动名称",
        "status": "状态",
        "pool": "排行榜奖池（USD）",
        "startsAt": "开始时间",
        "endsAt": "结束时间",
        "leaderboardEnabled": "启用排行榜",
        "leaderboardHint": "按有效拉新人数计算前五名奖励。",
        "lotteryEnabled": "启用抽奖",
        "lotteryHint": "邀请人和被邀请人各获得一张抽奖券。",
        "shares": "前五名分配比例",
        "prizes": "抽奖奖项",
        "prizeName": "奖项名称",
        "prizeAmount": "金额",
        "prizeCount": "数量",
        "prizeCap": "单用户上限",
        "seed": "固定随机种子"
      },
      "detail": {
        "title": "参与与发奖审计",
        "participants": "参与用户",
        "rewards": "奖励记录",
        "paid": "已发放",
        "rewardType": "奖励类型",
        "user": "用户",
        "amount": "金额",
        "status": "状态",
        "emptyRewards": "暂无奖励记录",
        "leaderboardReward": "排行榜第 {rank} 名",
        "lotteryReward": "抽奖：{prize}"
      }
    },
    "affiliates": {
      "records": {
        "sourceRedeemCode": "订阅卡",
        "rebateBaseAmount": "返利基数"
      }
    },
    "accounts": {
      "rateMultiplierMissing": "倍率未配置",
      "rateMultiplierCostHint": "账号级上游成本倍率，仅影响账号成本统计和调度成本排序，不是用户售卖倍率",
      "rateMultiplierSaved": "上游成本倍率已保存",
      "rateMultiplierSaveFailed": "上游成本倍率保存失败",
      "rateMultiplierInvalid": "请输入不小于 0 的成本倍率",
      "healthDetailSettings": "健康详情/设置",
      "probeInterval": "探活间隔（分钟）",
      "healthyProbeIntervalOption": "{hours} 小时"
    },
    "redeem": {
      "types": {
        "subscription_quota_reset": "订阅额度刷新"
      },
      "subscriptionQuotaReset": "订阅额度刷新",
      "affiliateRebateBase": "返利基数 ($)",
      "affiliateRebateBaseShort": "返利基数",
      "affiliateRebateBasePlaceholder": "0 表示不返利",
      "affiliateRebateBaseHint": "订阅卡兑换成功后，按该金额 × 邀请返利比例给邀请人发放站内余额返利。",
      "quotaResetScope": "刷新范围",
      "quotaResetScopes": {
        "daily": "日额度",
        "weekly": "周额度",
        "monthly": "月额度",
        "all": "全部额度"
      }
    },
    "support": {
      "title": "工单管理",
      "description": "处理用户一对一咨询与问题闭环",
      "searchPlaceholder": "搜索用户邮箱、昵称、UID 或标题",
      "unreadOnly": "只看未读",
      "empty": "暂无工单",
      "disabledTitle": "工单模块已关闭",
      "disabledDescription": "可在系统设置的功能开关中启用站内工单；历史工单数据仍会保留。",
      "noSelection": "选择一个工单",
      "noSelectionHint": "左侧队列中选择工单后即可查看消息并回复。",
      "replyPlaceholder": "输入管理员回复..."
    },
    "settings": {
      "features": {
        "supportTickets": {
          "title": "站内工单",
          "description": "控制用户工单和后台工单管理入口。关闭后入口隐藏，接口拒绝访问，历史工单数据保留。",
          "configureLink": "前往 工单管理 查看工单队列",
          "enabled": "启用站内工单",
          "enabledHint": "关闭后用户和管理员都无法进入工单页面，已有消息不会被删除。"
        },
        "acquisition": {
          "title": "拉新活动",
          "description": "在现有邀请关系上叠加周期活动能力，控制用户入口、排行榜与抽奖模块。",
          "configureLink": "前往 拉新活动 配置活动和奖项",
          "enabled": "启用拉新活动",
          "enabledHint": "关闭后用户侧边栏入口隐藏，用户接口返回 403；后台配置仍可访问。",
          "leaderboardEnabled": "启用排行榜模块",
          "leaderboardHint": "关闭后不展示排行榜入口，也不生成排行榜奖励。",
          "lotteryEnabled": "启用抽奖模块",
          "lotteryHint": "关闭后不展示抽奖券信息，也不生成抽奖奖励。"
        }
      },
      "site": {
        "rechargeStorefront": {
          "title": "充值商城",
          "description": "在顶部导航展示充值商城入口，支持自定义按钮文字和跳转链接",
          "buttonText": "按钮文字",
          "buttonTextPlaceholder": "充值商城",
          "primaryUrl": "主卡网链接",
          "primaryUrlPlaceholder": "https://shop.example.com",
          "primaryUrlHint": "主充值通道链接，建议填写完整的 http(s) 地址",
          "backupUrl": "备用卡网链接",
          "backupUrlPlaceholder": "https://backup-shop.example.com",
          "backupUrlHint": "可选。填写后，用户点击充值商城会先弹出主卡网 / 备用卡网选择卡片"
        },
        "supportGroup": {
          "title": "售后群",
          "description": "在顶部导航展示售后群入口，支持跳转链接或弹出二维码卡片",
          "buttonText": "按钮文字",
          "buttonTextPlaceholder": "售后群",
          "dialogTitle": "弹窗标题",
          "dialogTitlePlaceholder": "售后服务群",
          "qrCodeUrl": "二维码图片 URL",
          "qrCodeUrlPlaceholder": "https://example.com/support-qr.png",
          "qrCodeUrlHint": "可选，填写完整的 http(s) 图片链接后，未配置直达链接时会弹出二维码卡片",
          "linkUrl": "直达链接",
          "linkUrlPlaceholder": "https://qm.qq.com/xxxxx",
          "linkUrlHint": "可选，填写后点击入口将直接跳转；未填写时才会使用二维码弹窗",
          "dialogDescription": "弹窗说明",
          "dialogDescriptionPlaceholder": "扫码加入售后群，处理订单、兑换码和使用问题"
        },
        "pixmoStudio": {
          "title": "Pixmo 生图",
          "description": "在顶部导航展示 Pixmo 生图入口，支持自定义按钮文字和跳转链接",
          "buttonText": "按钮文字",
          "buttonTextPlaceholder": "Pixmo 生图",
          "url": "跳转链接",
          "urlPlaceholder": "https://pixmo.example.com",
          "urlHint": "启用后必须填写完整的 http(s) 链接"
        },
        "usageHelp": {
          "title": "使用帮助",
          "description": "在右上角显示站内使用帮助弹窗入口"
        },
        "modelRadar": {
          "title": "模型雷达",
          "description": "在右上角显示每日模型评分和推荐入口"
        }
      },
      "payment": {
        "rechargeBonusDisplay": "前端展示充值活动",
        "rechargeBonusDisplayHint": "仅控制用户充值页是否展示活动文案；实际到账仍按上方倍率计算。",
        "rechargeBonusDisplayPreview": "当前会展示为：赠送 {percent}%",
        "rechargeBonusRule": "满额赠送活动",
        "rechargeBonusRuleHint": "开启后，余额充值将按下方阈值和赠送百分比计算到账，不再使用上方充值倍率。",
        "rechargeBonusThreshold": "赠送阈值金额",
        "rechargeBonusThresholdHint": "充值金额达到该值后触发赠送活动",
        "rechargeBonusPercent": "赠送百分比",
        "rechargeBonusPercentHint": "例如 20 表示赠送 20%",
        "rechargeBonusRulePreview": "当前活动：满 {threshold} CNY 赠送 {percent}%",
        "rechargeBonusRuleMultiplierFallback": "开启活动后，上方充值倍率仅在活动关闭时生效，不参与当前实际到账计算。",
        "providerUsdtBsc": "USDT-BSC",
        "field_receiveAddress": "收款钱包地址",
        "field_cnyPerUsdt": "手动 CNY/USDT 汇率",
        "field_rateMode": "汇率模式",
        "field_rateApiUrl": "自动汇率 API URL",
        "field_rateJSONPath": "自动汇率 JSON 路径",
        "field_rateCacheSeconds": "汇率缓存秒数",
        "field_rateFallbackToManual": "自动汇率失败时回退手动汇率",
        "field_confirmations": "确认数",
        "field_bscscanApiKey": "BscScan API Key",
        "field_bscscanApiBase": "BscScan API 地址",
        "field_rpcUrl": "BSC RPC 地址",
        "field_tokenContract": "USDT 合约地址",
        "field_usdtReceiveAddressHint": "只填写 BNB Smart Chain / BEP20 网络的 USDT 收款地址。服务端不会保存私钥。",
        "field_usdtRateModeHint": "推荐使用自动模式：创建订单时实时获取 USDT/CNY 汇率并锁定应付 USDT 数量；已创建订单不会随汇率继续变化。",
        "field_usdtCnyPerUsdtHint": "手动汇率，也是自动汇率失败时的兜底值。含义是 1 USDT ≈ 多少 CNY，例如 7.2；不要填 1，除非你明确要按 1 CNY = 1 USDT 测试。",
        "field_usdtRateApiUrlHint": "留空时默认使用 Binance P2P USDT/CNY 卖出价。仅在需要自定义价格接口时填写；接口必须返回 JSON。",
        "field_usdtRateJSONPathHint": "自定义价格接口的 JSON 点号路径。默认 tether.cny，对应 {\"tether\":{\"cny\":7.2}}；默认 Binance P2P 源不使用此项。",
        "field_usdtRateCacheSecondsHint": "同一服务进程内的自动汇率缓存时间。默认 300 秒；填 0 表示每次下单都请求接口。",
        "field_usdtRateFallbackToManualHint": "建议开启。自动汇率接口失败时使用上面的手动汇率继续创建订单；关闭后接口失败会阻止下单。",
        "field_usdtConfirmationsHint": "链上确认数达到该值后才会自动入账。建议 15-30，默认 20。",
        "field_usdtRpcUrlHint": "用于查询 BSC 链上 USDT 转账日志。默认 https://1rpc.io/bnb；公共节点可能限流，生产建议换成自有或第三方 BSC RPC。",
        "field_usdtBscscanApiKeyHint": "可选。填写后可提高 BscScan 查询稳定性；作为敏感字段保存，不会回显。",
        "field_usdtBscscanApiBaseHint": "可选。默认 https://api.bscscan.com/api，除非使用兼容代理一般不用改。",
        "field_usdtTokenContractHint": "默认 BSC USDT 合约 0x55d398326f99059ff775485246999027b3197955，一般不要修改。"
      }
    }
  },
  "support": {
    "title": "工单",
    "description": "创建工单并与管理员一对一沟通",
    "newTicket": "新建工单",
    "create": "创建工单",
    "send": "发送",
    "sending": "发送中...",
    "close": "关闭",
    "reopen": "重开",
    "empty": "暂无工单",
    "disabledTitle": "工单暂未开放",
    "disabledDescription": "当前站点未启用站内工单，请通过其他客服入口联系管理员。",
    "noSelection": "选择一个工单",
    "noSelectionHint": "选择左侧工单查看沟通记录，或新建一个工单。",
    "replyPlaceholder": "输入回复内容...",
    "closedHint": "工单已关闭，重开后才能继续回复",
    "failed": "操作失败，请稍后重试",
    "filters": {
      "all": "全部工单",
      "allStatus": "全部状态",
      "allCategory": "全部分类",
      "allPriority": "全部优先级"
    },
    "form": {
      "title": "标题",
      "category": "分类",
      "content": "问题描述"
    },
    "status": {
      "open": "打开",
      "pending_admin": "待管理员处理",
      "pending_user": "待用户回复",
      "closed": "已关闭"
    },
    "category": {
      "general": "普通问题",
      "recharge": "充值问题",
      "subscription": "订阅问题",
      "api_issue": "API 异常",
      "account": "账号问题"
    },
    "priority": {
      "low": "低",
      "normal": "普通",
      "high": "高",
      "urgent": "紧急"
    },
    "sender": {
      "user": "用户",
      "admin": "管理员",
      "system": "系统"
    }
  },
  "payment": {
    "quickAmountCredit": "到账 {amount}",
    "methods": {
      "usdt_bsc": "USDT-BSC"
    },
    "crypto": {
      "scanUsdtBsc": "USDT-BSC 转账充值",
      "scanUsdtBscHint": "请使用钱包扫码或复制地址转账，仅支持 BNB Smart Chain / BEP20 的 USDT",
      "networkLabel": "收款网络",
      "usdtBscNetwork": "BNB Smart Chain（BEP20）",
      "payAmount": "应付数量",
      "exchangeRate": "转换倍率",
      "exchangeRateValue": "1 USDT ≈ {rate} CNY",
      "walletAddress": "收款钱包地址",
      "copyAmount": "复制数量",
      "copyAddress": "复制地址",
      "copied": "已复制",
      "amountCopied": "应付数量已复制",
      "addressCopied": "钱包地址已复制",
      "feeWarning": "建议优先使用 Web3 钱包转账。BSC 网络矿工费通常用 BNB 单独支付，不会从 USDT 转账数量里扣除；请直接复制上方应付数量转账。",
      "exchangeWithdrawalWarning": "不建议从交易所直接提币到本站地址。若必须使用交易所提币，请在交易所确认扣除提币手续费后，实际到达本站地址的 USDT 数量仍与上方应付数量完全一致，否则系统不会自动到账，需要人工处理。",
      "usdtBscWarning": "请务必选择 USDT-BSC / BEP20 网络转账。转错链、转错币种或转错地址将无法自动到账，需要人工处理。"
    },
    "subscriptionPlans": "订阅套餐",
    "rechargeBonusBanner": "当前充值活动：赠送 {percent}%",
    "rechargeBonusPreview": "支付 {pay}，到账 {credit}",
    "quotaReset": {
      "sectionTitle": "额度刷新卡",
      "typeLabel": "刷新卡",
      "buyReset": "购买重置",
      "resetAction": "重置",
      "resetValue": "刷新额度",
      "daily": "日额度",
      "weekly": "周额度",
      "monthly": "月额度",
      "all": "全部额度",
      "once": "次",
      "showAll": "查看全部",
      "noAvailableResetPlans": "暂无适用于当前订阅的刷新卡"
    },
    "admin": {
      "planType": "商品类型",
      "planTypeSubscription": "订阅套餐",
      "planTypeQuotaReset": "额度刷新卡",
      "planTypeQuotaResetWithValue": "刷新卡 ${value}",
      "quotaResetScope": "刷新范围",
      "quotaResetValue": "刷新额度",
      "quotaResetDaily": "日额度",
      "quotaResetValueRequired": "刷新额度必须大于 0"
    }
  }
} as const
