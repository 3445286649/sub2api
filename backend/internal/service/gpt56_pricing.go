package service

type gpt56PricingSpec struct {
	inputPrice    float64
	cacheRead     float64
	cacheCreation float64
	outputPrice   float64
}

var gpt56BusinessPricing = map[string]gpt56PricingSpec{
	"gpt-5.6-sol": {
		inputPrice:    5e-6,
		cacheRead:     0.5e-6,
		cacheCreation: 6.25e-6,
		outputPrice:   30e-6,
	},
	"gpt-5.6-terra": {
		inputPrice:    2.5e-6,
		cacheRead:     0.25e-6,
		cacheCreation: 3.125e-6,
		outputPrice:   15e-6,
	},
	"gpt-5.6-luna": {
		inputPrice:    1e-6,
		cacheRead:     0.1e-6,
		cacheCreation: 1.25e-6,
		outputPrice:   6e-6,
	},
}

func getGPT56PricingSpec(model string) (gpt56PricingSpec, bool) {
	normalized := normalizeKnownOpenAICodexModel(model)
	spec, ok := gpt56BusinessPricing[normalized]
	return spec, ok
}

func (p gpt56PricingSpec) modelPricing() *ModelPricing {
	return &ModelPricing{
		InputPricePerToken:             p.inputPrice,
		InputPricePerTokenPriority:     p.inputPrice,
		OutputPricePerToken:            p.outputPrice,
		OutputPricePerTokenPriority:    p.outputPrice,
		CacheCreationPricePerToken:     p.cacheCreation,
		CacheReadPricePerToken:         p.cacheRead,
		CacheReadPricePerTokenPriority: p.cacheRead,
		SupportsCacheBreakdown:         false,
		LongContextInputThreshold:      openAIGPT54LongContextInputThreshold,
		LongContextInputMultiplier:     openAIGPT54LongContextInputMultiplier,
		LongContextOutputMultiplier:    openAIGPT54LongContextOutputMultiplier,
	}
}

func (p gpt56PricingSpec) liteLLMPricing() *LiteLLMModelPricing {
	return &LiteLLMModelPricing{
		InputCostPerToken:               p.inputPrice,
		InputCostPerTokenPriority:       p.inputPrice,
		OutputCostPerToken:              p.outputPrice,
		OutputCostPerTokenPriority:      p.outputPrice,
		CacheCreationInputTokenCost:     p.cacheCreation,
		CacheReadInputTokenCost:         p.cacheRead,
		CacheReadInputTokenCostPriority: p.cacheRead,
		LongContextInputTokenThreshold:  openAIGPT54LongContextInputThreshold,
		LongContextInputCostMultiplier:  openAIGPT54LongContextInputMultiplier,
		LongContextOutputCostMultiplier: openAIGPT54LongContextOutputMultiplier,
		LiteLLMProvider:                 "openai",
		Mode:                            "chat",
		SupportsPromptCaching:           true,
	}
}
