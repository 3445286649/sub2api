//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsRequest_DecodesLocalNavigationSettings(t *testing.T) {
	var req UpdateSettingsRequest
	err := json.Unmarshal([]byte(`{
		"recharge_storefront_enabled":true,
		"recharge_storefront_button_text":"快速充值",
		"recharge_storefront_url":"https://shop.example.com/",
		"recharge_storefront_backup_url":"https://backup.example.com/",
		"support_group_enabled":true,
		"support_group_button_text":"售后群",
		"support_group_title":"售后服务群",
		"support_group_description":"扫码或点击链接加入",
		"support_group_qr_code_url":"https://cdn.example.com/group.png",
		"support_group_link_url":"https://chat.example.com/group",
		"pixmo_studio_enabled":true,
		"pixmo_studio_button_text":"Pixmo 生图",
		"pixmo_studio_url":"https://pixmo.example.com/",
		"usage_help_enabled":true,
		"model_radar_enabled":true,
		"support_tickets_enabled":true,
		"acquisition_enabled":true,
		"acquisition_leaderboard_enabled":true,
		"acquisition_lottery_enabled":true,
		"doc_url":"https://docs.example.com/"
	}`), &req)
	require.NoError(t, err)
	require.True(t, req.RechargeStorefrontEnabled)
	require.Equal(t, "快速充值", req.RechargeStorefrontButtonText)
	require.Equal(t, "https://shop.example.com/", req.RechargeStorefrontURL)
	require.Equal(t, "https://backup.example.com/", req.RechargeStorefrontBackupURL)
	require.True(t, req.SupportGroupEnabled)
	require.Equal(t, "售后群", req.SupportGroupButtonText)
	require.Equal(t, "售后服务群", req.SupportGroupTitle)
	require.Equal(t, "扫码或点击链接加入", req.SupportGroupDescription)
	require.Equal(t, "https://cdn.example.com/group.png", req.SupportGroupQRCodeURL)
	require.Equal(t, "https://chat.example.com/group", req.SupportGroupLinkURL)
	require.True(t, req.PixmoStudioEnabled)
	require.Equal(t, "Pixmo 生图", req.PixmoStudioButtonText)
	require.Equal(t, "https://pixmo.example.com/", req.PixmoStudioURL)
	require.True(t, req.UsageHelpEnabled)
	require.True(t, req.ModelRadarEnabled)
	require.True(t, req.SupportTicketsEnabled)
	require.True(t, req.AcquisitionEnabled)
	require.True(t, req.AcquisitionLeaderboardEnabled)
	require.True(t, req.AcquisitionLotteryEnabled)
	require.Equal(t, "https://docs.example.com/", req.DocURL)
}
