// Generated from the pre-split locale. Current modules always take precedence.
export default {
  "nav": {
    "rechargeStorefront": "Recharge Store",
    "rechargeStorefrontPickerTitle": "Choose Recharge Channel",
    "rechargeStorefrontPickerDescription": "If the primary store is unavailable, switch to the backup store.",
    "rechargeStorefrontPrimary": "Primary Store",
    "rechargeStorefrontPrimaryHint": "Recommended by default. Use the main recharge channel first.",
    "rechargeStorefrontBackup": "Backup Store",
    "rechargeStorefrontBackupHint": "Use this when the primary route is temporarily unavailable.",
    "pixmoStudio": "Pixmo Images"
  },
  "monitorCommon": {
    "attempts": "Attempts",
    "failureCategories": {
      "config_error": "Check config error",
      "auth_error": "Auth error",
      "rate_limited": "Upstream rate limited",
      "upstream_error": "Upstream error",
      "network_error": "Network error",
      "timeout": "Timeout",
      "protocol_error": "Protocol error",
      "challenge_mismatch": "Response validation issue",
      "empty_response": "Empty response"
    }
  },
  "acquisition": {
    "title": "Acquisition Campaigns",
    "description": "Track the active campaign, invite progress, leaderboard, and lottery rewards",
    "emptyTitle": "No active acquisition campaign",
    "emptyDescription": "There is no campaign available right now. Check back later.",
    "loadFailed": "Failed to load acquisition campaign",
    "inviteCode": "My Invite Code",
    "inviteLink": "Invite Link",
    "copyCode": "Copy Code",
    "copyLink": "Copy Link",
    "codeCopied": "Invite code copied",
    "linkCopied": "Invite link copied",
    "stats": {
      "validInvites": "Valid Invites",
      "rank": "Current Rank",
      "tickets": "Lottery Tickets",
      "pool": "Leaderboard Pool"
    },
    "flags": {
      "leaderboardOn": "Leaderboard on",
      "leaderboardOff": "Leaderboard off",
      "lotteryOn": "Lottery on",
      "lotteryOff": "Lottery off"
    },
    "leaderboard": {
      "title": "Campaign Leaderboard",
      "rank": "Rank",
      "user": "User",
      "invites": "Valid Invites",
      "reward": "Estimated Reward",
      "empty": "No valid invite records yet"
    },
    "rewards": {
      "title": "My Rewards",
      "empty": "Reward records will appear here after settlement",
      "leaderboard": "Leaderboard rank #{rank}",
      "lottery": "Lottery prize: {prize}"
    },
    "status": {
      "draft": "Draft",
      "active": "Active",
      "settling": "Settling",
      "settled": "Settled"
    }
  },
  "redeem": {
    "subscriptionQuotaReset": "Subscription quota reset",
    "quotaResetScopes": {
      "daily": "Daily quota",
      "weekly": "Weekly quota",
      "monthly": "Monthly quota",
      "all": "All quotas"
    }
  },
  "admin": {
    "acquisition": {
      "title": "Acquisition Campaigns",
      "description": "Create period-based campaigns, configure leaderboard pools, lottery prizes, and settlement payouts.",
      "loadFailed": "Failed to load campaigns",
      "detailLoadFailed": "Failed to load campaign details",
      "saveSuccess": "Campaign saved",
      "saveFailed": "Failed to save campaign",
      "settleSuccess": "Settlement completed",
      "settleFailed": "Failed to settle campaign",
      "campaigns": "Campaigns",
      "empty": "No campaigns yet. Create the first campaign.",
      "pool": "Pool",
      "prizes": "Prizes",
      "actions": {
        "create": "Create Campaign",
        "newDraft": "New Draft",
        "addPrize": "Add Prize",
        "settle": "Settle"
      },
      "status": {
        "draft": "Draft",
        "active": "Active",
        "settling": "Settling",
        "settled": "Settled"
      },
      "form": {
        "editTitle": "Edit Campaign",
        "createTitle": "Campaign Config",
        "defaultName": "Acquisition Campaign",
        "defaultPrize": "Small Prize",
        "name": "Campaign Name",
        "status": "Status",
        "pool": "Leaderboard Pool (USD)",
        "startsAt": "Starts At",
        "endsAt": "Ends At",
        "leaderboardEnabled": "Enable Leaderboard",
        "leaderboardHint": "Reward the top five by valid invite count.",
        "lotteryEnabled": "Enable Lottery",
        "lotteryHint": "The inviter and invitee each receive one ticket.",
        "shares": "Top Five Shares",
        "prizes": "Lottery Prizes",
        "prizeName": "Prize Name",
        "prizeAmount": "Amount",
        "prizeCount": "Count",
        "prizeCap": "Per-user Cap",
        "seed": "Fixed Random Seed"
      },
      "detail": {
        "title": "Participation and Payout Audit",
        "participants": "Participants",
        "rewards": "Rewards",
        "paid": "Paid",
        "rewardType": "Reward Type",
        "user": "User",
        "amount": "Amount",
        "status": "Status",
        "emptyRewards": "No reward records yet",
        "leaderboardReward": "Leaderboard rank #{rank}",
        "lotteryReward": "Lottery: {prize}"
      }
    },
    "accounts": {
      "rateMultiplierMissing": "Multiplier missing",
      "rateMultiplierCostHint": "Account-level upstream cost multiplier. It affects account cost stats and cost-aware scheduling, not user selling rates.",
      "rateMultiplierSaved": "Upstream cost multiplier saved",
      "rateMultiplierSaveFailed": "Failed to save upstream cost multiplier",
      "rateMultiplierInvalid": "Enter a cost multiplier greater than or equal to 0",
      "healthDetailSettings": "Health Details/Settings",
      "probeInterval": "Probe interval (minutes)",
      "healthyProbeIntervalOption": "{hours} hours"
    },
    "redeem": {
      "subscriptionQuotaReset": "Subscription Quota Reset",
      "types": {
        "subscription_quota_reset": "Subscription Quota Reset"
      },
      "quotaResetScope": "Reset Scope",
      "quotaResetScopes": {
        "daily": "Daily quota",
        "weekly": "Weekly quota",
        "monthly": "Monthly quota",
        "all": "All quotas"
      }
    },
    "support": {
      "title": "Support Tickets",
      "description": "Handle one-to-one user issues through trackable tickets",
      "searchPlaceholder": "Search email, username, UID, or title",
      "unreadOnly": "Unread only",
      "empty": "No tickets",
      "disabledTitle": "Ticket module disabled",
      "disabledDescription": "Enable Support Tickets in System Settings > Feature Switches. Existing ticket data is preserved.",
      "noSelection": "Select a ticket",
      "noSelectionHint": "Choose a ticket from the queue to view messages and reply.",
      "replyPlaceholder": "Type an admin reply..."
    },
    "settings": {
      "features": {
        "supportTickets": {
          "title": "Support Tickets",
          "description": "Control user tickets and the admin ticket queue. When off, entries are hidden, APIs return forbidden, and existing data is preserved.",
          "configureLink": "Open Support Tickets queue",
          "enabled": "Enable Support Tickets",
          "enabledHint": "When off, users and admins cannot enter ticket pages; existing messages are not deleted."
        },
        "acquisition": {
          "title": "Acquisition Campaigns",
          "description": "Layer period-based campaigns on top of existing invite relationships and control the user entry, leaderboard, and lottery modules.",
          "configureLink": "Configure campaigns and prizes",
          "enabled": "Enable Acquisition Campaigns",
          "enabledHint": "When off, the user sidebar entry is hidden and user APIs return 403. Admin configuration remains available.",
          "leaderboardEnabled": "Enable Leaderboard Module",
          "leaderboardHint": "When off, leaderboard UI is hidden and leaderboard rewards are not generated.",
          "lotteryEnabled": "Enable Lottery Module",
          "lotteryHint": "When off, lottery ticket UI is hidden and lottery rewards are not generated."
        }
      },
      "site": {
        "rechargeStorefront": {
          "title": "Recharge Store",
          "description": "Show a recharge store entry in the header with custom button text and destination URL.",
          "buttonText": "Button Text",
          "buttonTextPlaceholder": "Recharge Store",
          "primaryUrl": "Primary Store URL",
          "primaryUrlPlaceholder": "https://shop.example.com",
          "primaryUrlHint": "Main recharge channel URL. Use a full http(s) address.",
          "backupUrl": "Backup Store URL",
          "backupUrlPlaceholder": "https://backup-shop.example.com",
          "backupUrlHint": "Optional. When filled, clicking Recharge Store opens a picker card with primary and backup options."
        },
        "supportGroup": {
          "title": "Support Group",
          "description": "Show a support group entry in the header with either a direct link or a QR code modal.",
          "buttonText": "Button Text",
          "buttonTextPlaceholder": "Support Group",
          "dialogTitle": "Dialog Title",
          "dialogTitlePlaceholder": "Support Service Group",
          "qrCodeUrl": "QR Code Image URL",
          "qrCodeUrlPlaceholder": "https://example.com/support-qr.png",
          "qrCodeUrlHint": "Optional. If provided and no direct link is configured, users will see the QR code modal.",
          "linkUrl": "Direct Link URL",
          "linkUrlPlaceholder": "https://qm.qq.com/xxxxx",
          "linkUrlHint": "Optional. When provided, clicking the entry opens this link directly instead of the QR modal.",
          "dialogDescription": "Dialog Description",
          "dialogDescriptionPlaceholder": "Scan to join the support group for orders, redeem codes, and usage issues"
        },
        "pixmoStudio": {
          "title": "Pixmo Images",
          "description": "Show a Pixmo Images entry in the header with custom button text and destination URL.",
          "buttonText": "Button Text",
          "buttonTextPlaceholder": "Pixmo Images",
          "url": "Destination URL",
          "urlPlaceholder": "https://pixmo.example.com",
          "urlHint": "A full http(s) URL is required when enabled."
        },
        "usageHelp": {
          "title": "Usage Help",
          "description": "Show an in-site usage help modal entry in the top-right header."
        },
        "modelRadar": {
          "title": "Model Radar",
          "description": "Show daily model benchmark scores and recommendations in the top-right header."
        }
      },
      "payment": {
        "rechargeBonusDisplay": "Show Recharge Campaign",
        "rechargeBonusDisplayHint": "Only controls whether the user recharge page shows campaign copy; credited balance still uses the multiplier above.",
        "rechargeBonusDisplayPreview": "Currently shown as: bonus {percent}%",
        "rechargeBonusRule": "Threshold Bonus Campaign",
        "rechargeBonusRuleHint": "When enabled, balance recharge credited amount uses the threshold and bonus percent below instead of the multiplier above.",
        "rechargeBonusThreshold": "Bonus Threshold Amount",
        "rechargeBonusThresholdHint": "Recharge amount must reach this value to trigger the bonus rule",
        "rechargeBonusPercent": "Bonus Percent",
        "rechargeBonusPercentHint": "For example, 20 means bonus 20%",
        "rechargeBonusRulePreview": "Current campaign: recharge {threshold} CNY and get {percent}% bonus",
        "rechargeBonusRuleMultiplierFallback": "When this campaign is enabled, the multiplier above is only used after the campaign is turned off.",
        "providerUsdtBsc": "USDT-BSC",
        "field_receiveAddress": "Receiving wallet address",
        "field_cnyPerUsdt": "Manual CNY/USDT rate",
        "field_rateMode": "Rate mode",
        "field_rateApiUrl": "Auto rate API URL",
        "field_rateJSONPath": "Auto rate JSON path",
        "field_rateCacheSeconds": "Rate cache seconds",
        "field_rateFallbackToManual": "Fallback to manual rate when auto rate fails",
        "field_confirmations": "Confirmations",
        "field_bscscanApiKey": "BscScan API Key",
        "field_bscscanApiBase": "BscScan API URL",
        "field_rpcUrl": "BSC RPC URL",
        "field_tokenContract": "USDT contract address",
        "field_usdtReceiveAddressHint": "Use only a BNB Smart Chain / BEP20 USDT receiving address. The server never stores private keys.",
        "field_usdtRateModeHint": "Auto is recommended: the server fetches the USDT/CNY rate when creating an order and locks the exact USDT amount. Existing orders do not change when the rate moves later.",
        "field_usdtCnyPerUsdtHint": "Manual rate and the fallback value for auto mode. This means how many CNY 1 USDT is worth, e.g. 7.2; do not use 1 unless you intentionally test 1 CNY = 1 USDT.",
        "field_usdtRateApiUrlHint": "Leave empty to use Binance P2P USDT/CNY sell price by default. Fill this only for a custom price endpoint; it must return JSON.",
        "field_usdtRateJSONPathHint": "Dot path for custom price endpoint JSON. Default tether.cny matches {\"tether\":{\"cny\":7.2}}; the default Binance P2P source does not use this.",
        "field_usdtRateCacheSecondsHint": "Auto-rate cache lifetime inside this service process. Default is 300 seconds; use 0 to request the endpoint for every order.",
        "field_usdtRateFallbackToManualHint": "Recommended. If the auto-rate endpoint fails, use the manual rate above to keep order creation available. Disable this to fail order creation instead.",
        "field_usdtConfirmationsHint": "Auto-credit only after this many block confirmations. Recommended 15-30; default is 20.",
        "field_usdtRpcUrlHint": "Used to query BSC USDT transfer logs. Defaults to https://1rpc.io/bnb; public nodes may be rate-limited, so use your own or third-party BSC RPC in production.",
        "field_usdtBscscanApiKeyHint": "Optional. Improves BscScan query stability. Stored as a sensitive value and never echoed back.",
        "field_usdtBscscanApiBaseHint": "Optional. Defaults to https://api.bscscan.com/api; normally only change this for a compatible proxy.",
        "field_usdtTokenContractHint": "Defaults to BSC USDT contract 0x55d398326f99059ff775485246999027b3197955. Usually do not change it."
      }
    }
  },
  "support": {
    "title": "Tickets",
    "description": "Create support tickets and chat with administrators",
    "newTicket": "New Ticket",
    "create": "Create Ticket",
    "send": "Send",
    "sending": "Sending...",
    "close": "Close",
    "reopen": "Reopen",
    "empty": "No tickets",
    "disabledTitle": "Tickets are not available",
    "disabledDescription": "The site ticket module is currently disabled. Please use another support channel.",
    "noSelection": "Select a ticket",
    "noSelectionHint": "Choose a ticket to view the conversation or create a new one.",
    "replyPlaceholder": "Type a reply...",
    "closedHint": "This ticket is closed. Reopen it before replying.",
    "failed": "Operation failed. Please try again later.",
    "filters": {
      "all": "All tickets",
      "allStatus": "All status",
      "allCategory": "All categories",
      "allPriority": "All priorities"
    },
    "form": {
      "title": "Title",
      "category": "Category",
      "content": "Description"
    },
    "status": {
      "open": "Open",
      "pending_admin": "Pending admin",
      "pending_user": "Pending user",
      "closed": "Closed"
    },
    "category": {
      "general": "General",
      "recharge": "Recharge",
      "subscription": "Subscription",
      "api_issue": "API issue",
      "account": "Account"
    },
    "priority": {
      "low": "Low",
      "normal": "Normal",
      "high": "High",
      "urgent": "Urgent"
    },
    "sender": {
      "user": "User",
      "admin": "Admin",
      "system": "System"
    }
  },
  "payment": {
    "quickAmountCredit": "Receive {amount}",
    "methods": {
      "usdt_bsc": "USDT-BSC"
    },
    "crypto": {
      "scanUsdtBsc": "USDT-BSC Transfer",
      "scanUsdtBscHint": "Scan with your wallet or copy the address. Only USDT on BNB Smart Chain / BEP20 is supported.",
      "networkLabel": "Receiving Network",
      "usdtBscNetwork": "BNB Smart Chain (BEP20)",
      "payAmount": "Amount to Pay",
      "exchangeRate": "Exchange Rate",
      "exchangeRateValue": "1 USDT ≈ {rate} CNY",
      "walletAddress": "Receiving Wallet Address",
      "copyAmount": "Copy Amount",
      "copyAddress": "Copy Address",
      "copied": "Copied",
      "amountCopied": "Payment amount copied",
      "addressCopied": "Wallet address copied",
      "feeWarning": "We recommend paying with a Web3 wallet. BSC gas is usually paid separately in BNB and does not reduce the USDT transfer amount; copy the exact amount above when transferring.",
      "exchangeWithdrawalWarning": "Direct exchange withdrawals are not recommended. If you must withdraw from an exchange, make sure the USDT actually received by this address after withdrawal fees still exactly matches the amount above, otherwise it will not be credited automatically and requires manual review.",
      "usdtBscWarning": "Please transfer only USDT-BSC / BEP20. Wrong chain, token, or address will not be credited automatically and requires manual review."
    },
    "subscriptionPlans": "Subscription Plans",
    "rechargeBonusBanner": "Current recharge campaign: bonus {percent}%",
    "rechargeBonusPreview": "Pay {pay}, receive {credit}",
    "quotaReset": {
      "sectionTitle": "Quota Reset Cards",
      "typeLabel": "Reset",
      "buyReset": "Buy Reset",
      "resetAction": "Reset",
      "resetValue": "Reset Value",
      "daily": "Daily quota",
      "weekly": "Weekly quota",
      "monthly": "Monthly quota",
      "all": "All quota",
      "once": "time",
      "showAll": "Show all",
      "noAvailableResetPlans": "No reset card is available for this subscription"
    },
    "admin": {
      "planType": "Product Type",
      "planTypeSubscription": "Subscription",
      "planTypeQuotaReset": "Quota Reset Card",
      "planTypeQuotaResetWithValue": "Reset ${value}",
      "quotaResetScope": "Reset Scope",
      "quotaResetValue": "Reset Value",
      "quotaResetDaily": "Daily quota",
      "quotaResetValueRequired": "Reset value must be greater than 0"
    }
  }
} as const
