package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestValidatePointsConfig(t *testing.T) {
	valid := PointsConfig{Enabled: true, InviteThresholdAmount: 50, InviteRewardPoints: 1, QualificationWindowDays: 30, FreezeHours: 168}
	require.NoError(t, validatePointsConfig(valid))

	invalid := valid
	invalid.InviteThresholdAmount = 0
	require.Error(t, validatePointsConfig(invalid))
	invalid = valid
	invalid.InviteRewardPoints = 0
	require.Error(t, validatePointsConfig(invalid))
	invalid = valid
	invalid.FreezeHours = 9000
	require.Error(t, validatePointsConfig(invalid))
}

func TestValidatePointsProductInput(t *testing.T) {
	valid := PointsProductInput{ProductType: "balance", Name: "Balance 5", PointsPrice: 10, BalanceAmount: 5, ForSale: true}
	require.NoError(t, validatePointsProductInput(valid))

	invalid := valid
	invalid.PointsPrice = 0
	require.Error(t, validatePointsProductInput(invalid))
	invalid = valid
	invalid.ProductType = "subscription"
	require.Error(t, validatePointsProductInput(invalid))
}

func TestBuildPaymentOrderProviderSnapshotStoresBaseRechargeAmount(t *testing.T) {
	snapshot := buildPaymentOrderProviderSnapshot(nil, CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: 50})
	require.Equal(t, 3, snapshot["schema_version"])
	require.Equal(t, float64(50), snapshot["base_recharge_amount"])

	subscription := buildPaymentOrderProviderSnapshot(nil, CreateOrderRequest{OrderType: payment.OrderTypeSubscription, Amount: 50})
	require.NotContains(t, subscription, "base_recharge_amount")
}
