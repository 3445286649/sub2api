package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIModelTiers(t *testing.T) {
	tiers, err := normalizeOpenAIModelTiers(map[string]string{
		" gpt-6-astra ":  OpenAIFastTierUltrafast,
		"gpt-6-sol":      OpenAIFastTierPriority,
		"standard-model": "default",
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"gpt-6-astra":    OpenAIFastTierUltrafast,
		"gpt-6-sol":      OpenAIFastTierPriority,
		"standard-model": "default",
	}, tiers)
	for _, invalid := range []map[string]string{
		{"": OpenAIFastTierPriority},
		{"gpt-*": OpenAIFastTierPriority},
		{"gpt-6-astra": "standard"},
		{"gpt-6-astra": "fast"},
		{"__proto__": OpenAIFastTierPriority},
		{"gpt-6-astra": OpenAIFastTierPriority, " gpt-6-astra ": OpenAIFastTierUltrafast},
	} {
		_, err := normalizeOpenAIModelTiers(invalid)
		require.Error(t, err)
	}
}
