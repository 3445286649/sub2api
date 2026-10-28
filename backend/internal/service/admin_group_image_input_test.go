package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeImageInputOverrides(t *testing.T) {
	normalized, err := normalizeImageInputOverrides(map[string]bool{
		" deepseek-flash ": true,
		"deepseek-v4-pro": false,
	})
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"deepseek-flash": true, "deepseek-v4-pro": false}, normalized)

	for _, input := range []map[string]bool{
		{"deepseek-*": true},
		{"": true},
		{"constructor": true},
		{" DeepSeek-Flash ": true, "deepseek-flash": false},
	} {
		_, err := normalizeImageInputOverrides(input)
		require.Error(t, err)
	}
}
