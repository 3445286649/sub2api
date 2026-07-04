package service

import (
	"math"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const accountHealthDefaultTierDegradedMin = 60

const (
	accountScheduleDefaultWeightHealth       = 30
	accountScheduleDefaultWeightLatency      = 45
	accountScheduleDefaultWeightCost         = 15
	accountScheduleDefaultWeightLoad         = 10
	accountScheduleDefaultLatencyPenaltyMS   = 15000
	accountScheduleDefaultLatencyDowngradeMS = 30000
	accountScheduleDefaultHighLatencyPenalty = 20
	accountScheduleScoreTieEpsilon           = 0.000001
	accountScheduleShuffleScoreWindow        = 2.0
	accountScheduleUnknownLatencyScore       = 50.0
)

func sortAccountWithLoadByHealthCostAndLoad(items []accountWithLoad, health map[int64]*AccountHealthSummary, preferOAuth bool, cfg config.GatewaySchedulingConfig) {
	if !cfg.HealthSortEnabled {
		sortAccountWithLoadByLegacyLoad(items, preferOAuth)
		return
	}
	costMedian := medianAccountWithLoadBillingRateMultiplier(items)
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, aLatency := accountHealthSortValueForScheduling(health, a.account.ID, cfg)
		bScore, bLatency := accountHealthSortValueForScheduling(health, b.account.ID, cfg)
		if at, bt := accountScheduleTier(aScore, aLatency, cfg), accountScheduleTier(bScore, bLatency, cfg); at != bt {
			return at < bt
		}
		aWeighted := accountWithLoadScheduleWeightedScore(a, health, cfg, costMedian)
		bWeighted := accountWithLoadScheduleWeightedScore(b, health, cfg, costMedian)
		if math.Abs(aWeighted-bWeighted) > accountScheduleScoreTieEpsilon {
			return aWeighted > bWeighted
		}
		if aScore != bScore {
			return aScore > bScore
		}
		if aLatency != bLatency {
			return aLatency < bLatency
		}
		if ar, br := a.account.BillingRateMultiplier(), b.account.BillingRateMultiplier(); ar != br {
			return ar < br
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
	costMedian := medianAccountPointerBillingRateMultiplier(items)
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, aLatency := accountHealthSortValueForScheduling(health, a.ID, cfg)
		bScore, bLatency := accountHealthSortValueForScheduling(health, b.ID, cfg)
		if at, bt := accountScheduleTier(aScore, aLatency, cfg), accountScheduleTier(bScore, bLatency, cfg); at != bt {
			return at < bt
		}
		aWeighted := accountPointerScheduleWeightedScore(a, health, cfg, costMedian)
		bWeighted := accountPointerScheduleWeightedScore(b, health, cfg, costMedian)
		if math.Abs(aWeighted-bWeighted) > accountScheduleScoreTieEpsilon {
			return aWeighted > bWeighted
		}
		if aScore != bScore {
			return aScore > bScore
		}
		if aLatency != bLatency {
			return aLatency < bLatency
		}
		if ar, br := a.BillingRateMultiplier(), b.BillingRateMultiplier(); ar != br {
			return ar < br
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
	return accountBaseHealthTier(score, cfg)
}

func accountBaseHealthTier(score int, cfg config.GatewaySchedulingConfig) int {
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

func accountScheduleTier(score int, latency int, cfg config.GatewaySchedulingConfig) int {
	tier := accountBaseHealthTier(score, cfg)
	_, downgradeMS, _ := accountScheduleLatencyGuards(cfg)
	if latency != math.MaxInt && latency >= downgradeMS && tier < 2 {
		tier++
	}
	return tier
}

func accountWithLoadScheduleWeightedScore(item accountWithLoad, health map[int64]*AccountHealthSummary, cfg config.GatewaySchedulingConfig, costMedian float64) float64 {
	if item.account == nil {
		return 0
	}
	score, latency := accountHealthSortValueForScheduling(health, item.account.ID, cfg)
	loadRate := 0
	if item.loadInfo != nil {
		loadRate = item.loadInfo.LoadRate
	}
	return accountScheduleWeightedScore(score, latency, item.account.BillingRateMultiplier(), costMedian, loadRate, cfg)
}

func accountPointerScheduleWeightedScore(account *Account, health map[int64]*AccountHealthSummary, cfg config.GatewaySchedulingConfig, costMedian float64) float64 {
	if account == nil {
		return 0
	}
	score, latency := accountHealthSortValueForScheduling(health, account.ID, cfg)
	return accountScheduleWeightedScore(score, latency, account.BillingRateMultiplier(), costMedian, 0, cfg)
}

func accountScheduleWeightedScore(healthScore int, latency int, rateMultiplier float64, costMedian float64, loadRate int, cfg config.GatewaySchedulingConfig) float64 {
	healthWeight, latencyWeight, costWeight, loadWeight := accountScheduleScoreWeights(cfg)
	totalWeight := healthWeight + latencyWeight + costWeight + loadWeight
	if totalWeight <= 0 {
		return 0
	}
	score := normalizeScheduleHealthScore(healthScore)*healthWeight +
		normalizeScheduleLatencyScore(latency)*latencyWeight +
		normalizeScheduleCostScore(rateMultiplier, costMedian)*costWeight +
		normalizeScheduleLoadScore(loadRate)*loadWeight
	weighted := score / totalWeight
	penaltyMS, downgradeMS, penaltyScore := accountScheduleLatencyGuards(cfg)
	if latency != math.MaxInt && latency >= penaltyMS && latency < downgradeMS && penaltyScore > 0 {
		weighted -= float64(penaltyScore)
		if weighted < 0 {
			return 0
		}
	}
	return weighted
}

func accountScheduleScoreWeights(cfg config.GatewaySchedulingConfig) (health, latency, cost, load float64) {
	h := cfg.ScoreWeightHealth
	l := cfg.ScoreWeightLatency
	c := cfg.ScoreWeightCost
	ld := cfg.ScoreWeightLoad
	if h < 0 || l < 0 || c < 0 || ld < 0 || h+l+c+ld <= 0 {
		h = accountScheduleDefaultWeightHealth
		l = accountScheduleDefaultWeightLatency
		c = accountScheduleDefaultWeightCost
		ld = accountScheduleDefaultWeightLoad
	}
	return float64(h), float64(l), float64(c), float64(ld)
}

func accountScheduleLatencyGuards(cfg config.GatewaySchedulingConfig) (penaltyMS int, downgradeMS int, penaltyScore int) {
	penaltyMS = cfg.LatencyPenaltyMS
	downgradeMS = cfg.LatencyTierDowngradeMS
	penaltyScore = cfg.HighLatencyPenaltyScore
	if penaltyMS <= 0 || downgradeMS <= penaltyMS || penaltyScore < 0 {
		return accountScheduleDefaultLatencyPenaltyMS, accountScheduleDefaultLatencyDowngradeMS, accountScheduleDefaultHighLatencyPenalty
	}
	return penaltyMS, downgradeMS, penaltyScore
}

func normalizeScheduleHealthScore(score int) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return float64(score)
}

func normalizeScheduleLatencyScore(latency int) float64 {
	if latency < 0 || latency == math.MaxInt {
		return accountScheduleUnknownLatencyScore
	}
	points := []struct {
		latency int
		score   float64
	}{
		{latency: 1500, score: 100},
		{latency: 3000, score: 80},
		{latency: 6000, score: 50},
		{latency: 10000, score: 20},
		{latency: 15000, score: 0},
	}
	if latency <= points[0].latency {
		return points[0].score
	}
	for i := 1; i < len(points); i++ {
		prev := points[i-1]
		next := points[i]
		if latency <= next.latency {
			ratio := float64(latency-prev.latency) / float64(next.latency-prev.latency)
			return prev.score + (next.score-prev.score)*ratio
		}
	}
	return 0
}

func normalizeScheduleCostScore(rateMultiplier float64, median float64) float64 {
	if median <= 0 || math.IsNaN(median) || math.IsInf(median, 0) {
		median = 1
	}
	if rateMultiplier < 0 || math.IsNaN(rateMultiplier) || math.IsInf(rateMultiplier, 0) {
		rateMultiplier = 1
	}
	ratio := rateMultiplier / median
	if ratio <= 0.5 {
		return 100
	}
	if ratio >= 3.0 {
		return 0
	}
	return (3.0 - ratio) / (3.0 - 0.5) * 100
}

func normalizeScheduleLoadScore(loadRate int) float64 {
	if loadRate < 0 {
		loadRate = 0
	}
	if loadRate > 100 {
		loadRate = 100
	}
	return (1 - float64(loadRate)/100) * 100
}

func medianAccountWithLoadBillingRateMultiplier(items []accountWithLoad) float64 {
	rates := make([]float64, 0, len(items))
	for _, item := range items {
		if item.account == nil {
			continue
		}
		rates = append(rates, item.account.BillingRateMultiplier())
	}
	return medianFloat64(rates)
}

func medianAccountPointerBillingRateMultiplier(items []*Account) float64 {
	rates := make([]float64, 0, len(items))
	for _, account := range items {
		if account == nil {
			continue
		}
		rates = append(rates, account.BillingRateMultiplier())
	}
	return medianFloat64(rates)
}

func medianFloat64(values []float64) float64 {
	cleaned := make([]float64, 0, len(values))
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		cleaned = append(cleaned, value)
	}
	if len(cleaned) == 0 {
		return 1
	}
	sort.Float64s(cleaned)
	mid := len(cleaned) / 2
	if len(cleaned)%2 == 1 {
		return cleaned[mid]
	}
	return (cleaned[mid-1] + cleaned[mid]) / 2
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
