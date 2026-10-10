//go:build unit

package provider

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func bepusdtFixture(network string) BepusdtOrder {
	contract, decimals, _ := BepusdtAsset(network)
	address := "0x1111111111111111111111111111111111111111"
	if network == "tron_mainnet" {
		address = "TYrRGMhFiCVJ3D9s1VsFxbbxFtq5eER3qJ"
	}
	amount, _ := BepusdtAmountAtomic("1.000123", decimals)
	confirmations := 20
	if network == "tron_mainnet" {
		confirmations = 30
	}
	return BepusdtOrder{ProtocolVersion: "2", MerchantID: "subapi", OutTradeNo: "sub2_fixture", GatewayOrderID: "gw_fixture", RequestDigest: strings.Repeat("a", 64), OrderVersion: 1, Status: "WAITING", FiatAmount: "7.00", FiatCurrency: "CNY", Asset: "USDT", Network: network, TokenContract: contract, TokenDecimals: decimals, AmountAtomic: amount, CryptoAmount: "1.000123", ReceiveAddress: address, LockedRate: "7", Rounding: "ceil_6dp", QuoteVersion: 1, CreatedAt: time.Now().Add(-time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339), ExpiresAt: time.Now().Add(time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339), RequiredConfirmations: confirmations}
}
func TestBepusdtExactAmountsAndProof(t *testing.T) {
	for _, network := range []string{"bsc_mainnet", "tron_mainnet"} {
		quote := bepusdtFixture(network)
		require.NoError(t, ValidateBepusdtOrder(&quote))
		quote.Status = "PAID"
		hash := "0x" + strings.Repeat("b", 64)
		if network == "tron_mainnet" {
			hash = strings.Repeat("b", 64)
		}
		quote.Payment = &BepusdtPayment{TxHash: hash, EventIndex: 0, BlockNumber: 100, BlockHash: hash, BlockTime: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), ToAddress: quote.ReceiveAddress, AmountAtomic: quote.AmountAtomic, Confirmations: quote.RequiredConfirmations, Confirmed: true}
		require.NoError(t, ValidateBepusdtOrder(&quote))
		quote.Payment.Confirmations = 19
		require.Error(t, ValidateBepusdtOrder(&quote))
	}
	amount, err := BepusdtAmountAtomic("9007199254740993.000001", 18)
	require.NoError(t, err)
	require.Equal(t, "9007199254740993000001000000000000", amount)
	for _, bad := range []string{"NaN", "Inf", "-1", "1e9", "0", "1.0000001"} {
		_, err = BepusdtAmountAtomic(bad, 6)
		require.Error(t, err)
	}
}

func TestBepusdtBSCIsolationQuoteVersion(t *testing.T) {
	quote := bepusdtFixture("bsc_mainnet")
	quote.QuoteVersion = 2
	quote.Rounding = "ceil_6dp_bsc_7"
	quote.CryptoAmount = "1.0001231"
	quote.AmountAtomic, _ = BepusdtAmountAtomic(quote.CryptoAmount, quote.TokenDecimals)
	require.NoError(t, ValidateBepusdtOrder(&quote))
	for _, tc := range []struct {
		name   string
		mutate func(*BepusdtOrder)
	}{
		{"v1_with_seventh_digit", func(q *BepusdtOrder) { q.QuoteVersion = 1; q.Rounding = "ceil_6dp" }},
		{"v2_with_old_rounding", func(q *BepusdtOrder) { q.Rounding = "ceil_6dp" }},
		{"unknown_version", func(q *BepusdtOrder) { q.QuoteVersion = 3 }},
		{"tron_v2", func(q *BepusdtOrder) {
			q.Network = "tron_mainnet"
			q.TokenContract, q.TokenDecimals, _ = BepusdtAsset(q.Network)
			q.ReceiveAddress = "TYrRGMhFiCVJ3D9s1VsFxbbxFtq5eER3qJ"
			q.RequiredConfirmations = 30
			q.CryptoAmount = "1.000123"
			q.AmountAtomic, _ = BepusdtAmountAtomic(q.CryptoAmount, q.TokenDecimals)
		}},
		{"wrong_isolation_digit", func(q *BepusdtOrder) {
			q.CryptoAmount = "1.0001232"
			q.AmountAtomic, _ = BepusdtAmountAtomic(q.CryptoAmount, q.TokenDecimals)
		}},
		{"extra_precision", func(q *BepusdtOrder) {
			q.CryptoAmount = "1.00012311"
			q.AmountAtomic, _ = BepusdtAmountAtomic(q.CryptoAmount, q.TokenDecimals)
		}},
		{"zero_tail", func(q *BepusdtOrder) {
			q.CryptoAmount = "1.0000001"
			q.AmountAtomic, _ = BepusdtAmountAtomic(q.CryptoAmount, q.TokenDecimals)
		}},
		{"excessive_tail", func(q *BepusdtOrder) {
			q.CryptoAmount = "1.0009001"
			q.AmountAtomic, _ = BepusdtAmountAtomic(q.CryptoAmount, q.TokenDecimals)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := quote
			tc.mutate(&changed)
			require.Error(t, ValidateBepusdtOrder(&changed))
		})
	}
	for _, amount := range []string{"1.0000011", "1.0008991"} {
		quote.CryptoAmount = amount
		quote.AmountAtomic, _ = BepusdtAmountAtomic(amount, quote.TokenDecimals)
		require.NoError(t, ValidateBepusdtOrder(&quote))
	}
}

func TestBepusdtBSCIsolationRejectsSelfTransferAndPreservesV1(t *testing.T) {
	quote := bepusdtFixture("bsc_mainnet")
	quote.Status = "PAID"
	quote.Payment = &BepusdtPayment{TxHash: "0x" + strings.Repeat("b", 64), EventIndex: 0, BlockNumber: 100, BlockHash: "0x" + strings.Repeat("c", 64), BlockTime: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), FromAddress: quote.ReceiveAddress, ToAddress: quote.ReceiveAddress, AmountAtomic: quote.AmountAtomic, Confirmations: 20, Confirmed: true}
	require.NoError(t, ValidateBepusdtOrder(&quote), "historical v1 receipt remains readable")
	quote.QuoteVersion = 2
	quote.Rounding = "ceil_6dp_bsc_7"
	quote.CryptoAmount = "1.0001231"
	quote.AmountAtomic, _ = BepusdtAmountAtomic(quote.CryptoAmount, quote.TokenDecimals)
	quote.Payment.AmountAtomic = quote.AmountAtomic
	require.ErrorContains(t, ValidateBepusdtOrder(&quote), "self-transfer")
	quote.Payment.FromAddress = "0x2222222222222222222222222222222222222222"
	require.NoError(t, ValidateBepusdtOrder(&quote))
	quote.Payment.FromAddress = ""
	require.ErrorContains(t, ValidateBepusdtOrder(&quote), "sender")
}

func TestBepusdtQuoteArithmeticKeepsLegacyBounds(t *testing.T) {
	for _, network := range []string{"bsc_mainnet", "tron_mainnet"} {
		for _, amount := range []string{"1.000000", "1.000900", "1.0001231"} {
			t.Run(network+"/"+amount, func(t *testing.T) {
				quote := bepusdtFixture(network)
				quote.CryptoAmount = amount
				quote.AmountAtomic, _ = BepusdtAmountAtomic(amount, quote.TokenDecimals)
				require.Error(t, ValidateBepusdtOrder(&quote))
			})
		}
	}
	quote := bepusdtFixture("bsc_mainnet")
	quote.FiatAmount = "7.01"
	quote.QuoteVersion = 2
	quote.Rounding = "ceil_6dp_bsc_7"
	quote.CryptoAmount = "1.0015521"
	quote.AmountAtomic, _ = BepusdtAmountAtomic(quote.CryptoAmount, quote.TokenDecimals)
	require.NoError(t, ValidateBepusdtOrder(&quote), "ceiling remains at six decimals before tail and isolation digit")
	quote.CryptoAmount = "1.0014281"
	quote.AmountAtomic, _ = BepusdtAmountAtomic(quote.CryptoAmount, quote.TokenDecimals)
	require.Error(t, ValidateBepusdtOrder(&quote), "amount below ceiling is not a valid quote")
}
func TestBepusdtTransportSignsQueryAndCreate(t *testing.T) {
	secret := strings.Repeat("a", 32)
	callback := strings.Repeat("b", 32)
	quote := bepusdtFixture("bsc_mainnet")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		want := BepusdtSignature(secret, "subapi", "key", r.Method, r.URL.EscapedPath(), r.Header.Get("X-Timestamp"), r.Header.Get("X-Nonce"), body)
		require.Equal(t, want, r.Header.Get("X-Signature"))
		if r.Method == http.MethodPost {
			var input BepusdtCreateRequest
			require.NoError(t, json.Unmarshal(body, &input))
			quote.RequestDigest = BepusdtRequestDigest("subapi", input)
		}
		_ = json.NewEncoder(w).Encode(quote)
	}))
	defer server.Close()
	prov, err := NewBepusdt("1", map[string]string{"apiUrl": server.URL, "merchantId": "subapi", "keyId": "key", "apiSecret": secret, "callbackSecret": callback, "allowedNetworks": "bsc_mainnet"})
	require.NoError(t, err)
	created, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: quote.OutTradeNo, Amount: "7.00", CryptoNetwork: quote.Network, ExpiresAt: quote.ExpiresAt})
	require.NoError(t, err)
	require.Equal(t, quote.CryptoAmount, created.CryptoAmount)
	queried, err := prov.Query(context.Background(), quote.OutTradeNo)
	require.NoError(t, err)
	require.Equal(t, quote.GatewayOrderID, queried.GatewayOrderID)
}
func TestBepusdtCallbackSignatureAndBinding(t *testing.T) {
	prov, err := NewBepusdt("1", map[string]string{"apiUrl": "https://gateway.example", "merchantId": "subapi", "keyId": "key", "apiSecret": strings.Repeat("a", 32), "callbackSecret": strings.Repeat("b", 32), "allowedNetworks": "bsc_mainnet"})
	require.NoError(t, err)
	quote := bepusdtFixture("bsc_mainnet")
	quote.Status = "PAID"
	quote.Payment = &BepusdtPayment{TxHash: "0x" + strings.Repeat("b", 64), EventIndex: 0, BlockNumber: 100, BlockHash: "0x" + strings.Repeat("c", 64), BlockTime: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), ToAddress: quote.ReceiveAddress, AmountAtomic: quote.AmountAtomic, Confirmations: 20, Confirmed: true}
	raw, _ := json.Marshal(BepusdtEvent{EventID: "event_1", Order: quote})
	headers := map[string]string{"x-merchant-id": "subapi", "x-key-id": "key", "x-timestamp": strconv.FormatInt(time.Now().Unix(), 10), "x-nonce": "0123456789012345", "request-path": BepusdtWebhookPath}
	headers["x-signature"] = BepusdtSignature(prov.callbackSecret, "subapi", "key", http.MethodPost, BepusdtWebhookPath, headers["x-timestamp"], headers["x-nonce"], raw)
	event, err := prov.VerifyEvent(string(raw), headers)
	require.NoError(t, err)
	require.Equal(t, "event_1", event.EventID)
	headers["request-path"] = "/different"
	_, err = prov.VerifyEvent(string(raw), headers)
	require.Error(t, err)
}

func TestBepusdtSharedSigningFixture(t *testing.T) {
	body := []byte(`{"out_trade_no":"sub2_fixture_1","fiat_amount":"12.34","fiat_currency":"CNY","asset":"USDT","network":"bsc_mainnet","expires_at":"2026-10-09T12:00:00Z"}`)
	require.Equal(t, "417d6f9ac17eac39e26453e080fe20ee442a6b5c594da204e95e9a6c2d2c9870", BepusdtSignature(strings.Repeat("a", 32), "subapi", "primary", "POST", "/api/v2/merchant/orders", "1791547200", "fixture_nonce_0001", body))
}
