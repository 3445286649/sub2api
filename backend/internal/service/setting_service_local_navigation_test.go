//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_LocalNavigationSettingsRoundTrip(t *testing.T) {
	repo := &bmUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	want := &SystemSettings{
		RechargeStorefrontEnabled:     true,
		RechargeStorefrontButtonText:  "快速充值",
		RechargeStorefrontURL:         "https://shop.example.com/",
		RechargeStorefrontBackupURL:   "https://backup.example.com/",
		SupportGroupEnabled:           true,
		SupportGroupButtonText:        "售后群",
		SupportGroupTitle:             "售后服务群",
		SupportGroupDescription:       "扫码或点击链接加入",
		SupportGroupQRCodeURL:         "https://cdn.example.com/group.png",
		SupportGroupLinkURL:           "https://chat.example.com/group",
		SupportTicketsEnabled:         false,
		PixmoStudioEnabled:            true,
		PixmoStudioButtonText:         "Pixmo 生图",
		PixmoStudioURL:                "https://pixmo.example.com/",
		UsageHelpEnabled:              true,
		ModelRadarEnabled:             true,
		AcquisitionEnabled:            true,
		AcquisitionLeaderboardEnabled: true,
		AcquisitionLotteryEnabled:     true,
		DocURL:                        "https://docs.example.com/",
	}

	require.NoError(t, svc.UpdateSettings(context.Background(), want))

	got := svc.parseSettings(repo.updates)
	require.Equal(t, want.RechargeStorefrontEnabled, got.RechargeStorefrontEnabled)
	require.Equal(t, want.RechargeStorefrontButtonText, got.RechargeStorefrontButtonText)
	require.Equal(t, want.RechargeStorefrontURL, got.RechargeStorefrontURL)
	require.Equal(t, want.RechargeStorefrontBackupURL, got.RechargeStorefrontBackupURL)
	require.Equal(t, want.SupportGroupEnabled, got.SupportGroupEnabled)
	require.Equal(t, want.SupportGroupButtonText, got.SupportGroupButtonText)
	require.Equal(t, want.SupportGroupTitle, got.SupportGroupTitle)
	require.Equal(t, want.SupportGroupDescription, got.SupportGroupDescription)
	require.Equal(t, want.SupportGroupQRCodeURL, got.SupportGroupQRCodeURL)
	require.Equal(t, want.SupportGroupLinkURL, got.SupportGroupLinkURL)
	require.Equal(t, want.SupportTicketsEnabled, got.SupportTicketsEnabled)
	require.Equal(t, want.PixmoStudioEnabled, got.PixmoStudioEnabled)
	require.Equal(t, want.PixmoStudioButtonText, got.PixmoStudioButtonText)
	require.Equal(t, want.PixmoStudioURL, got.PixmoStudioURL)
	require.Equal(t, want.UsageHelpEnabled, got.UsageHelpEnabled)
	require.Equal(t, want.ModelRadarEnabled, got.ModelRadarEnabled)
	require.Equal(t, want.AcquisitionEnabled, got.AcquisitionEnabled)
	require.Equal(t, want.AcquisitionLeaderboardEnabled, got.AcquisitionLeaderboardEnabled)
	require.Equal(t, want.AcquisitionLotteryEnabled, got.AcquisitionLotteryEnabled)
	require.Equal(t, want.DocURL, got.DocURL)
}
