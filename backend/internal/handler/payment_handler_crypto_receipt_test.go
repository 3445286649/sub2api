//go:build unit

package handler

import (
	"encoding/json"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func marshalToMap(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestPaymentOrderResponsesIncludeCryptoReceiptOnlyWhenPaid(t *testing.T) {
	hash := "0x" + "ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12"
	intent := "sub2_x|72.00|10.056873|7.16|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106|1759300000"
	paidAt := time.Now()
	paid := &dbent.PaymentOrder{
		PaymentType:    payment.TypeUSDTBSC,
		Status:         service.OrderStatusCompleted,
		PaidAt:         &paidAt,
		PaymentTradeNo: hash,
		ProviderSnapshot: map[string]any{
			"usdt_bsc": map[string]any{"tx_hash": hash, "token_amount": "10.056873", "intent_trade_no": intent},
		},
	}
	pending := &dbent.PaymentOrder{
		PaymentType:      payment.TypeUSDTBSC,
		Status:           service.OrderStatusPending,
		PaymentTradeNo:   intent,
		ProviderSnapshot: map[string]any{"usdt_bsc": map[string]any{"intent_trade_no": intent}},
	}

	for name, build := range map[string]func(*dbent.PaymentOrder) any{
		"user":   func(o *dbent.PaymentOrder) any { return sanitizePaymentOrderForResponse(o) },
		"public": func(o *dbent.PaymentOrder) any { return buildPublicOrderResult(o) },
	} {
		got := marshalToMap(t, build(paid))
		require.Equal(t, hash, got["crypto_tx_hash"], name)
		require.Equal(t, "10.056873", got["crypto_amount"], name)
		require.Equal(t, "USDT", got["crypto_currency"], name)

		got = marshalToMap(t, build(pending))
		for _, field := range []string{"crypto_tx_hash", "crypto_amount", "crypto_currency"} {
			require.NotContains(t, got, field, name)
		}
		raw, err := json.Marshal(build(pending))
		require.NoError(t, err)
		require.NotContains(t, string(raw), "0x3b210bdc", name)
	}
}
