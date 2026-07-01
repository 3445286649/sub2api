package service

import "sort"

func sortAccountWithLoadByHealthCostAndLoad(items []accountWithLoad, health map[int64]*AccountHealthSummary, preferOAuth bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, aLatency := AccountHealthSortValue(health, a.account.ID)
		bScore, bLatency := AccountHealthSortValue(health, b.account.ID)
		if aScore != bScore {
			return aScore > bScore
		}
		if ar, br := a.account.BillingRateMultiplier(), b.account.BillingRateMultiplier(); ar != br {
			return ar < br
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

func sortAccountPointersByHealthCostAndLRU(items []*Account, health map[int64]*AccountHealthSummary, preferOAuth bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, aLatency := AccountHealthSortValue(health, a.ID)
		bScore, bLatency := AccountHealthSortValue(health, b.ID)
		if aScore != bScore {
			return aScore > bScore
		}
		if ar, br := a.BillingRateMultiplier(), b.BillingRateMultiplier(); ar != br {
			return ar < br
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
