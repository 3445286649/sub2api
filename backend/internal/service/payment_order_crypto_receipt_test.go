//go:build unit

package service

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

const testCryptoTxHash = "0x9C3E5B1F2A7D4E6C8B0A1F3D5E7C9B2A4D6F8E0C1B3A5D7E9F1C3B5A7D9E1F3A"

func paidUSDTOrder(snapshot map[string]any, tradeNo string) *dbent.PaymentOrder {
	paidAt := time.Now()
	return &dbent.PaymentOrder{
		PaymentType:      payment.TypeUSDTBSC,
		Status:           OrderStatusCompleted,
		PaidAt:           &paidAt,
		PaymentTradeNo:   tradeNo,
		ProviderSnapshot: snapshot,
	}
}

func TestPaymentOrderCryptoReceiptOf_PrefersSnapshot(t *testing.T) {
	order := paidUSDTOrder(map[string]any{
		"usdt_bsc": map[string]any{"tx_hash": testCryptoTxHash, "token_amount": "10.056873"},
	}, "0x"+"1111111111111111111111111111111111111111111111111111111111111111")

	receipt := PaymentOrderCryptoReceiptOf(order)
	require.NotNil(t, receipt)
	require.Equal(t, "0x9c3e5b1f2a7d4e6c8b0a1f3d5e7c9b2a4d6f8e0c1b3a5d7e9f1c3b5a7d9e1f3a", receipt.TxHash)
	require.Equal(t, "10.056873", receipt.Amount)
	require.Equal(t, "USDT", receipt.Currency)
}

func TestPaymentOrderCryptoReceiptOf_FallsBackToTradeNoForLegacyOrders(t *testing.T) {
	receipt := PaymentOrderCryptoReceiptOf(paidUSDTOrder(nil, testCryptoTxHash))
	require.NotNil(t, receipt)
	require.Equal(t, "0x9c3e5b1f2a7d4e6c8b0a1f3d5e7c9b2a4d6f8e0c1b3a5d7e9f1c3b5a7d9e1f3a", receipt.TxHash)
	require.Empty(t, receipt.Amount)
}

func TestPaymentOrderCryptoReceiptOf_NeverExposesPaymentIntent(t *testing.T) {
	intent := "sub2_x|72.00|10.056873|7.16|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106|1759300000"
	order := paidUSDTOrder(map[string]any{"usdt_bsc": map[string]any{"intent_trade_no": intent}}, intent)
	require.Nil(t, PaymentOrderCryptoReceiptOf(order))
}

func TestPaymentOrderCryptoReceiptOf_RequiresPaidUSDTOrder(t *testing.T) {
	snapshot := map[string]any{"usdt_bsc": map[string]any{"tx_hash": testCryptoTxHash}}

	for _, status := range []string{OrderStatusPending, OrderStatusExpired, OrderStatusCancelled} {
		order := paidUSDTOrder(snapshot, testCryptoTxHash)
		order.Status = status
		require.Nil(t, PaymentOrderCryptoReceiptOf(order), status)
	}

	unpaid := paidUSDTOrder(snapshot, testCryptoTxHash)
	unpaid.PaidAt = nil
	require.Nil(t, PaymentOrderCryptoReceiptOf(unpaid))

	alipay := paidUSDTOrder(snapshot, testCryptoTxHash)
	alipay.PaymentType = payment.TypeAlipay
	require.Nil(t, PaymentOrderCryptoReceiptOf(alipay))

	byProviderKey := paidUSDTOrder(snapshot, testCryptoTxHash)
	byProviderKey.PaymentType = "usdt"
	providerKey := payment.TypeUSDTBSC
	byProviderKey.ProviderKey = &providerKey
	require.NotNil(t, PaymentOrderCryptoReceiptOf(byProviderKey))

	require.Nil(t, PaymentOrderCryptoReceiptOf(nil))
}
