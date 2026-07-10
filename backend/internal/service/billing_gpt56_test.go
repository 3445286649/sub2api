//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type gpt56ExpectedPricing struct {
	model         string
	inputPrice    float64
	cacheRead     float64
	cacheCreation float64
	outputPrice   float64
}

var gpt56ExpectedPricings = []gpt56ExpectedPricing{
	{model: "gpt-5.6-sol", inputPrice: 5e-6, cacheRead: 0.5e-6, cacheCreation: 6.25e-6, outputPrice: 30e-6},
	{model: "gpt-5.6-terra", inputPrice: 2.5e-6, cacheRead: 0.25e-6, cacheCreation: 3.125e-6, outputPrice: 15e-6},
	{model: "gpt-5.6-luna", inputPrice: 1e-6, cacheRead: 0.1e-6, cacheCreation: 1.25e-6, outputPrice: 6e-6},
}

func TestGetModelPricingOpenAIGPT56BusinessDefaults(t *testing.T) {
	svc := newTestBillingService()

	for _, tt := range gpt56ExpectedPricings {
		t.Run(tt.model, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tt.model)
			require.NoError(t, err)
			require.NotNil(t, pricing)
			require.InDelta(t, tt.inputPrice, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, tt.inputPrice, pricing.InputPricePerTokenPriority, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadPricePerToken, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadPricePerTokenPriority, 1e-12)
			require.InDelta(t, tt.cacheCreation, pricing.CacheCreationPricePerToken, 1e-12)
			require.InDelta(t, tt.outputPrice, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, tt.outputPrice, pricing.OutputPricePerTokenPriority, 1e-12)
			require.Equal(t, 272000, pricing.LongContextInputThreshold)
			require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
			require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
		})
	}
}

func TestCalculateCostOpenAIGPT56LongContextBoundary(t *testing.T) {
	svc := newTestBillingService()

	for _, tt := range gpt56ExpectedPricings {
		t.Run(tt.model, func(t *testing.T) {
			shortTokens := UsageTokens{InputTokens: 200000, CacheReadTokens: 72000, CacheCreationTokens: 1000, OutputTokens: 100}
			shortCost, err := svc.CalculateCost(tt.model, shortTokens, 1.0)
			require.NoError(t, err)
			require.InDelta(t, float64(shortTokens.InputTokens)*tt.inputPrice, shortCost.InputCost, 1e-10)
			require.InDelta(t, float64(shortTokens.CacheReadTokens)*tt.cacheRead, shortCost.CacheReadCost, 1e-10)
			require.InDelta(t, float64(shortTokens.CacheCreationTokens)*tt.cacheCreation, shortCost.CacheCreationCost, 1e-10)
			require.InDelta(t, float64(shortTokens.OutputTokens)*tt.outputPrice, shortCost.OutputCost, 1e-10)

			longTokens := shortTokens
			longTokens.CacheReadTokens++
			longCost, err := svc.CalculateCost(tt.model, longTokens, 1.0)
			require.NoError(t, err)
			require.InDelta(t, float64(longTokens.InputTokens)*tt.inputPrice*2, longCost.InputCost, 1e-10)
			require.InDelta(t, float64(longTokens.CacheReadTokens)*tt.cacheRead*2, longCost.CacheReadCost, 1e-10)
			require.InDelta(t, float64(longTokens.CacheCreationTokens)*tt.cacheCreation*2, longCost.CacheCreationCost, 1e-10)
			require.InDelta(t, float64(longTokens.OutputTokens)*tt.outputPrice*1.5, longCost.OutputCost, 1e-10)
		})
	}
}

func TestGetModelPricingOpenAIGPT56BusinessDefaultBeatsDynamicData(t *testing.T) {
	svc := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol": {
			InputCostPerToken:  99e-6,
			OutputCostPerToken: 99e-6,
		},
	}})

	pricing, err := svc.GetModelPricing("gpt-5.6-sol")
	require.NoError(t, err)
	require.InDelta(t, 5e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.OutputPricePerToken, 1e-12)
}

func TestPricingServiceOpenAIGPT56BusinessDefaultsBeatDynamicData(t *testing.T) {
	dynamicPricing := make(map[string]*LiteLLMModelPricing, len(gpt56ExpectedPricings))
	for _, tt := range gpt56ExpectedPricings {
		dynamicPricing[tt.model] = &LiteLLMModelPricing{
			InputCostPerToken:  99e-6,
			OutputCostPerToken: 99e-6,
		}
	}
	svc := &PricingService{pricingData: dynamicPricing}

	for _, tt := range gpt56ExpectedPricings {
		t.Run(tt.model, func(t *testing.T) {
			pricing := svc.GetModelPricing("openai/" + tt.model + "-20260710")
			require.NotNil(t, pricing)
			require.InDelta(t, tt.inputPrice, pricing.InputCostPerToken, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadInputTokenCost, 1e-12)
			require.InDelta(t, tt.cacheCreation, pricing.CacheCreationInputTokenCost, 1e-12)
			require.InDelta(t, tt.outputPrice, pricing.OutputCostPerToken, 1e-12)
			require.Equal(t, 272000, pricing.LongContextInputTokenThreshold)
		})
	}

	exact := svc.GetModelPricing("gpt-5.6-sol")
	require.NotNil(t, exact)
	require.InDelta(t, 5e-6, exact.InputCostPerToken, 1e-12)
	require.InDelta(t, 30e-6, exact.OutputCostPerToken, 1e-12)
}

func TestGetModelPricingWithChannelGPT56OverridesBusinessDefault(t *testing.T) {
	svc := newTestBillingService()
	channelPricing := &ChannelModelPricing{
		InputPrice:      testPtrFloat64(8e-6),
		OutputPrice:     testPtrFloat64(40e-6),
		CacheWritePrice: testPtrFloat64(10e-6),
		CacheReadPrice:  testPtrFloat64(0.8e-6),
	}

	pricing, err := svc.GetModelPricingWithChannel("gpt-5.6-sol", channelPricing)
	require.NoError(t, err)
	require.InDelta(t, 8e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 40e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 10e-6, pricing.CacheCreationPricePerToken, 1e-12)
	require.InDelta(t, 0.8e-6, pricing.CacheReadPricePerToken, 1e-12)
}
