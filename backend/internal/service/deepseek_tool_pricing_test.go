//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekToolPricingModesMatchInflightReservation(t *testing.T) {
	groupID := int64(42)
	legacyPrice := 0.024
	fixedPrice := 0.03
	independentPrice := 0.02
	independentRate := 0.5
	group := &Group{
		ID: groupID, Platform: PlatformDeepseek, RateMultiplier: 3,
		WebSearchPricePerCall: &legacyPrice,
	}
	apiKey := &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: group}
	svc := newInflightEstimateGateway(t, nil)
	usage := &OpenAIGatewayService{billingService: &BillingService{}}
	for _, model := range []string{"deepseek-tool-search", "deepseek-tool-fetch"} {
		price, rate := webSearchBillingTerms(apiKey, model, 3)
		require.InDelta(t, legacyPrice, *price, 1e-12)
		require.Equal(t, float64(3), rate)
	}

	group.DeepSeekToolPricing = map[string]domain.DeepSeekToolPricing{
		"search": {Mode: DeepSeekToolRateFixed, PricePerCall: &fixedPrice},
		"fetch":  {Mode: DeepSeekToolRateIndependent, PricePerCall: &independentPrice, RateMultiplier: &independentRate},
	}
	for _, tc := range []struct {
		model string
		want  float64
		rate  float64
	}{
		{"deepseek-tool-search", 0.03, 1},
		{"deepseek-tool-fetch", 0.01, 0.5},
	} {
		price, rate := webSearchBillingTerms(apiKey, tc.model, 3)
		require.Equal(t, tc.rate, rate)
		require.InDelta(t, tc.want, (&BillingService{}).CalculateWebSearchCost(1, price, rate).ActualCost, 1e-12)
		est, priced := svc.EstimateInflightReservation(context.Background(), apiKey, InflightEstimateRequest{
			Model: tc.model, Kind: InflightEstimatePerRequest, WebSearchCalls: 1,
		})
		require.True(t, priced)
		require.InDelta(t, tc.want, est, 1e-12)
		result := &OpenAIForwardResult{Model: tc.model, UpstreamModel: tc.model, WebSearchCalls: 1}
		actual, err := usage.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{tc.model}, 3, 1, 1, 3, UsageTokens{}, "", boolPtr(false), false, time.Time{})
		require.NoError(t, err)
		require.InDelta(t, est, actual.ActualCost, 1e-12)
	}
	group.DeepSeekToolPricing["search"] = domain.DeepSeekToolPricing{Mode: DeepSeekToolRateGroup, PricePerCall: &fixedPrice}
	price, rate := webSearchBillingTerms(apiKey, "deepseek-tool-search", 2.5)
	require.InDelta(t, 0.075, (&BillingService{}).CalculateWebSearchCost(1, price, rate).ActualCost, 1e-12)
}

func TestDeepSeekToolPricingSurvivesAuthSnapshot(t *testing.T) {
	groupID := int64(9)
	price := 0.024
	apiKey := &APIKey{
		ID: 1, UserID: 2, GroupID: &groupID, Key: "snapshot-test", Status: StatusActive,
		User: &User{ID: 2, Status: StatusActive, Role: RoleUser},
		Group: &Group{
			ID: groupID, Name: "deepseek", Platform: PlatformDeepseek, Status: StatusActive,
			SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 3,
			DeepSeekToolPricing: map[string]domain.DeepSeekToolPricing{
				"search": {Mode: DeepSeekToolRateFixed, PricePerCall: &price},
			},
		},
	}
	svc := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	copy := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	require.Equal(t, apiKey.Group.DeepSeekToolPricing, copy.Group.DeepSeekToolPricing)
	*copy.Group.DeepSeekToolPricing["search"].PricePerCall = 1
	require.Equal(t, 0.024, price)
}

func TestNormalizeDeepSeekToolPricingRejectsInvalidAdminValues(t *testing.T) {
	price := 0.024
	rate := 0.5
	valid := map[string]domain.DeepSeekToolPricing{
		"search": {Mode: DeepSeekToolRateIndependent, PricePerCall: &price, RateMultiplier: &rate},
	}
	got, err := NormalizeDeepSeekToolPricing(valid)
	require.NoError(t, err)
	require.Equal(t, valid, got)
	*got["search"].PricePerCall = 1
	require.Equal(t, 0.024, price)

	for _, tc := range []map[string]domain.DeepSeekToolPricing{
		{"unknown": {Mode: DeepSeekToolRateFixed, PricePerCall: &price}},
		{"search": {Mode: "other", PricePerCall: &price}},
		{"search": {Mode: DeepSeekToolRateFixed}},
		{"search": {Mode: DeepSeekToolRateFixed, PricePerCall: float64Ptr(math.NaN())}},
		{"search": {Mode: DeepSeekToolRateFixed, PricePerCall: float64Ptr(-1)}},
		{"fetch": {Mode: DeepSeekToolRateIndependent, PricePerCall: &price}},
		{"fetch": {Mode: DeepSeekToolRateIndependent, PricePerCall: &price, RateMultiplier: float64Ptr(0)}},
	} {
		_, err := NormalizeDeepSeekToolPricing(tc)
		require.Error(t, err)
	}
}
