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
