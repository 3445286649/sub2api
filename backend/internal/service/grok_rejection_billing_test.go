package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGrokRejectionReceipts(t *testing.T) {
	cfg := GrokRejectionConfig{Enabled: true, SourceID: "supe", AllowedOrigins: []string{"https://supe.example"}, AmountMode: "upstream"}
	body := []byte(`{"normal_billing":"replaced","grok_rejection_billing":{"charged_amount":99,"charges":[{"billing_request_id":"a","billing_status":"charged","charged_amount":0.10},{"billing_request_id":"b","billing_status":"charged","charged_amount":"0.10"},{"billing_request_id":"c","billing_status":"unconfirmed","charged_amount":0.10}]}}`)
	receipts, replaced, err := ParseGrokRejectionReceipts(body, cfg)
	require.NoError(t, err)
	require.True(t, replaced)
	require.Len(t, receipts, 3)
	require.Equal(t, "0.1", receipts[0].UserAmount)
	require.Equal(t, "0.1", receipts[1].UserAmount)
	require.False(t, receipts[2].Chargeable())
	cfg.AmountMode = "fixed"
	cfg.FixedAmountUSD = "0.25"
	receipts, _, err = ParseGrokRejectionReceipts(body, cfg)
	require.NoError(t, err)
	require.Equal(t, "0.25", receipts[0].UserAmount)
	require.Equal(t, "0.1", receipts[0].UpstreamAmount)
}

func TestGrokRejectionInvalidReceiptDoesNotGuess(t *testing.T) {
	cfg := GrokRejectionConfig{AmountMode: "fixed", FixedAmountUSD: "0.25"}
	for _, charge := range []string{
		`{"billing_request_id":"a","billing_status":"charged"}`,
		`{"billing_status":"charged","charged_amount":0.1}`,
		`{"billing_request_id":"a","billing_status":"charged","charged_amount":-1}`,
		`{"billing_request_id":"a","billing_status":"charged","charged_amount":"NaN"}`,
		`{"billing_request_id":"a","billing_status":"charged","charged_amount":0.1,"currency":"CNY"}`,
	} {
		rows, _, err := ParseGrokRejectionReceipts([]byte(`{"grok_rejection_billing":{"charges":[`+charge+`]}}`), cfg)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.False(t, rows[0].Chargeable(), charge)
	}
}

func TestGrokRejectionConfig(t *testing.T) {
	valid := map[string]any{"grok_rejection_billing": map[string]any{"enabled": true, "source_id": "supe", "allowed_origins": []string{"https://supe.example"}, "amount_mode": "fixed", "fixed_amount_usd": "0.25"}}
	cfg, err := ParseGrokRejectionConfig(valid)
	require.NoError(t, err)
	require.True(t, cfg.Trusts("https://supe.example/v1/responses"))
	require.False(t, cfg.Trusts("https://supe.example.evil/v1/responses"))
	require.False(t, cfg.Trusts("http://supe.example/v1/responses"))
	valid["grok_rejection_billing"].(map[string]any)["fixed_amount_usd"] = "-1"
	_, err = ParseGrokRejectionConfig(valid)
	require.Error(t, err)
}

func TestGrokRejectionReplacedSurvivesInvalidCharges(t *testing.T) {
	rows, replaced, err := ParseGrokRejectionReceipts([]byte(`{"normal_billing":"replaced","grok_rejection_billing":{"charges":"invalid"}}`), GrokRejectionConfig{})
	require.Error(t, err)
	require.Empty(t, rows)
	require.True(t, replaced)
}
