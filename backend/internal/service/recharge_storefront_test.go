package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRechargeStorefrontChannelsSortsAndNormalizes(t *testing.T) {
	channels, err := NormalizeRechargeStorefrontChannels([]RechargeStorefrontChannel{
		{ID: "backup-2", Name: "备用二", URL: "https://pay.example.com/shop/2", Enabled: true, SortOrder: 20},
		{ID: "backup-1", Name: "备用一", URL: "https://shop.example.com/", Enabled: true, SortOrder: 1},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"backup-1", "backup-2"}, []string{channels[0].ID, channels[1].ID})
	require.Equal(t, []int{1, 2}, []int{channels[0].SortOrder, channels[1].SortOrder})
}

func TestParseRechargeStorefrontChannelsFallsBackToLegacyFields(t *testing.T) {
	channels := parseRechargeStorefrontChannels("", "https://shop.example.com/", "https://backup.example.com/")
	require.Len(t, channels, 2)
	require.Equal(t, "backup-1", channels[0].ID)
	require.Equal(t, "backup-2", channels[1].ID)
}

func TestNormalizeRechargeStorefrontChannelsRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"http://shop.example.com/",
		"javascript:alert(1)",
		"https://user:pass@example.com/",
		"https://localhost/shop",
		"https://intranet/shop",
		"https://store.internal/shop",
		"https://store.lan/shop",
		"https://127.0.0.1/shop",
		"https://192.168.1.10/shop",
	} {
		_, err := NormalizeRechargeStorefrontChannels([]RechargeStorefrontChannel{{ID: "backup-1", URL: raw}})
		require.Error(t, err, raw)
	}
}

func TestEnabledRechargeStorefrontChannelsHonorsGlobalAndChannelSwitches(t *testing.T) {
	channels := []RechargeStorefrontChannel{
		{ID: "backup-1", URL: "https://one.example.com", Enabled: true, SortOrder: 1},
		{ID: "backup-2", URL: "https://two.example.com", Enabled: false, SortOrder: 2},
	}
	require.Empty(t, enabledRechargeStorefrontChannels(channels, false))
	enabled := enabledRechargeStorefrontChannels(channels, true)
	require.Len(t, enabled, 1)
	require.Equal(t, "backup-1", enabled[0].ID)
}
