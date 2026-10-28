package service

import (
	"fmt"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

const (
	DeepSeekToolRateGroup       = "group"
	DeepSeekToolRateFixed       = "fixed"
	DeepSeekToolRateIndependent = "independent"
)

func NormalizeDeepSeekToolPricing(pricing map[string]domain.DeepSeekToolPricing) (map[string]domain.DeepSeekToolPricing, error) {
	if len(pricing) == 0 {
		return nil, nil
	}
	result := make(map[string]domain.DeepSeekToolPricing, len(pricing))
	for tool, entry := range pricing {
		if tool != string(DeepSeekToolSearch) && tool != string(DeepSeekToolFetch) {
			return nil, fmt.Errorf("unsupported DeepSeek tool pricing key %q", tool)
		}
		if entry.Mode != DeepSeekToolRateGroup && entry.Mode != DeepSeekToolRateFixed && entry.Mode != DeepSeekToolRateIndependent {
			return nil, fmt.Errorf("invalid DeepSeek %s pricing mode", tool)
		}
		if entry.PricePerCall == nil || math.IsNaN(*entry.PricePerCall) || math.IsInf(*entry.PricePerCall, 0) || *entry.PricePerCall < 0 || *entry.PricePerCall > 1e6 {
			return nil, fmt.Errorf("DeepSeek %s price_per_call must be between 0 and 1000000", tool)
		}
		if entry.Mode == DeepSeekToolRateIndependent && (entry.RateMultiplier == nil || math.IsNaN(*entry.RateMultiplier) || math.IsInf(*entry.RateMultiplier, 0) || *entry.RateMultiplier <= 0 || *entry.RateMultiplier > 1e6) {
			return nil, fmt.Errorf("DeepSeek %s rate_multiplier must be between 0 and 1000000", tool)
		}
		price := *entry.PricePerCall
		entry.PricePerCall = &price
		if entry.Mode == DeepSeekToolRateIndependent {
			rate := *entry.RateMultiplier
			entry.RateMultiplier = &rate
		} else {
			entry.RateMultiplier = nil
		}
		result[tool] = entry
	}
	return result, nil
}

func CloneDeepSeekToolPricing(pricing map[string]domain.DeepSeekToolPricing) map[string]domain.DeepSeekToolPricing {
	if len(pricing) == 0 {
		return nil
	}
	result := make(map[string]domain.DeepSeekToolPricing, len(pricing))
	for tool, entry := range pricing {
		if entry.PricePerCall != nil {
			price := *entry.PricePerCall
			entry.PricePerCall = &price
		}
		if entry.RateMultiplier != nil {
			rate := *entry.RateMultiplier
			entry.RateMultiplier = &rate
		}
		result[tool] = entry
	}
	return result
}

func webSearchBillingTerms(apiKey *APIKey, model string, groupRate float64) (*float64, float64) {
	price := webSearchPricePerCallFromAPIKey(apiKey)
	if apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != PlatformDeepseek {
		return price, groupRate
	}
	var tool string
	switch model {
	case "deepseek-tool-search":
		tool = string(DeepSeekToolSearch)
	case "deepseek-tool-fetch":
		tool = string(DeepSeekToolFetch)
	default:
		return price, groupRate
	}
	entry, ok := apiKey.Group.DeepSeekToolPricing[tool]
	if !ok || entry.PricePerCall == nil {
		return price, groupRate
	}
	switch entry.Mode {
	case DeepSeekToolRateGroup:
		return entry.PricePerCall, groupRate
	case DeepSeekToolRateFixed:
		return entry.PricePerCall, 1
	case DeepSeekToolRateIndependent:
		if entry.RateMultiplier != nil {
			return entry.PricePerCall, *entry.RateMultiplier
		}
	}
	return price, groupRate
}
