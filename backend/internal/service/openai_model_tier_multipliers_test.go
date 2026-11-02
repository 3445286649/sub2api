package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIModelTierMultipliers(t *testing.T) {
	tiers := map[string]string{"fast-model": OpenAIFastTierPriority, "ultra-model": OpenAIFastTierUltrafast, "standard-model": "default"}
	got, err := normalizeOpenAIModelTierMultipliers(map[string]float64{"fast-model": 3, "ultra-model": 0.01}, tiers)
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"fast-model": 3, "ultra-model": 0.01}, got)

	for _, tc := range []struct {
		name, model string
		value       float64
	}{
		{"standard assignment", "standard-model", 3},
		{"unknown assignment", "unknown-model", 3},
		{"zero", "fast-model", 0},
		{"negative", "fast-model", -1},
		{"too small", "fast-model", 0.009},
		{"too large", "fast-model", 1001},
		{"NaN", "fast-model", math.NaN()},
		{"infinite", "fast-model", math.Inf(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeOpenAIModelTierMultipliers(map[string]float64{tc.model: tc.value}, tiers)
			require.Error(t, err)
		})
	}
}
