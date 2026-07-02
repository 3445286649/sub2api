package service

import (
	"math"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const accountHealthDefaultTierDegradedMin = 60

func sortAccountWithLoadByHealthCostAndLoad(items []accountWithLoad, health map[int64]*AccountHealthSummary, preferOAuth bool, cfg config.GatewaySchedulingConfig) {
	if !cfg.HealthSortEnabled {
		sortAccountWithLoadByLegacyLoad(items, preferOAuth)
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, aLatency := accountHealthSortValueForScheduling(health, a.account.ID, cfg)
		bScore, bLatency := accountHealthSortValueForScheduling(health, b.account.ID, cfg)
		if at, bt := accountHealthTier(aScore, cfg), accountHealthTier(bScore, cfg); at != bt {
			return at < bt
		}
		if ar, br := a.account.BillingRateMultiplier(), b.account.BillingRateMultiplier(); ar != br {
			return ar < br
		}
		if aScore != bScore {
			return aScore > bScore
		}
		if aLatency != bLatency {
			return aLatency < bLatency
		}
		if a.loadInfo.LoadRate != b.loadInfo.LoadRate {
			return a.loadInfo.LoadRate < b.loadInfo.LoadRate
		}
		if preferOAuth && a.account.Type != b.account.Type {
			return a.account.Type == AccountTypeOAuth
		}
		if a.account.Priority != b.account.Priority {
			return a.account.Priority < b.account.Priority
		}
		return accountLRULess(a.account, b.account)
	})
}

func sortAccountPointersByHealthCostAndLRU(items []*Account, health map[int64]*AccountHealthSummary, preferOAuth bool, cfg config.GatewaySchedulingConfig) {
	if !cfg.HealthSortEnabled {
		sortAccountPointersByLegacyLRU(items, preferOAuth)
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, aLatency := accountHealthSortValueForScheduling(health, a.ID, cfg)
		bScore, bLatency := accountHealthSortValueForScheduling(health, b.ID, cfg)
		if at, bt := accountHealthTier(aScore, cfg), accountHealthTier(bScore, cfg); at != bt {
			return at < bt
		}
		if ar, br := a.BillingRateMultiplier(), b.BillingRateMultiplier(); ar != br {
			return ar < br
		}
		if aScore != bScore {
			return aScore > bScore
		}
		if aLatency != bLatency {
			return aLatency < bLatency
		}
		if preferOAuth && a.Type != b.Type {
			return a.Type == AccountTypeOAuth
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return accountLRULess(a, b)
	})
}

func accountHealthSortValueForScheduling(states map[int64]*AccountHealthSummary, accountID int64, cfg config.GatewaySchedulingConfig) (score int, latency int) {
	if state := states[accountID]; state != nil {
		return AccountHealthSortValue(states, accountID)
	}
	baseline := defaultAccountHealthScore
	healthyMin, _ := accountHealthTierThresholds(cfg)
	if healthyMin > baseline {
		baseline = healthyMin
	}
	return baseline, math.MaxInt
}

func accountHealthTier(score int, cfg config.GatewaySchedulingConfig) int {
	healthyMin, degradedMin := accountHealthTierThresholds(cfg)
	switch {
	case score >= healthyMin:
		return 0
	case score >= degradedMin:
		return 1
	default:
		return 2
	}
}

func accountHealthTierThresholds(cfg config.GatewaySchedulingConfig) (healthyMin int, degradedMin int) {
	healthyMin = cfg.HealthTierHealthyMin
	degradedMin = cfg.HealthTierDegradedMin
	if healthyMin < 0 || healthyMin > 100 || degradedMin < 0 || degradedMin > 100 || healthyMin <= degradedMin {
		return defaultAccountHealthScore, accountHealthDefaultTierDegradedMin
	}
	return healthyMin, degradedMin
}

func sortAccountWithLoadByLegacyLoad(items []accountWithLoad, preferOAuth bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.loadInfo.LoadRate != b.loadInfo.LoadRate {
			return a.loadInfo.LoadRate < b.loadInfo.LoadRate
		}
		if preferOAuth && a.account.Type != b.account.Type {
			return a.account.Type == AccountTypeOAuth
		}
		if a.account.Priority != b.account.Priority {
			return a.account.Priority < b.account.Priority
		}
		return accountLRULess(a.account, b.account)
	})
}

func sortAccountPointersByLegacyLRU(items []*Account, preferOAuth bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if preferOAuth && a.Type != b.Type {
			return a.Type == AccountTypeOAuth
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return accountLRULess(a, b)
	})
}

func accountLRULess(a, b *Account) bool {
	switch {
	case a.LastUsedAt == nil && b.LastUsedAt != nil:
		return true
	case a.LastUsedAt != nil && b.LastUsedAt == nil:
		return false
	case a.LastUsedAt == nil && b.LastUsedAt == nil:
		return a.ID < b.ID
	default:
		if a.LastUsedAt.Equal(*b.LastUsedAt) {
			return a.ID < b.ID
		}
		return a.LastUsedAt.Before(*b.LastUsedAt)
	}
}
