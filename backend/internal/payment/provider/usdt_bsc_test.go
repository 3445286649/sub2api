package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestUSDTBSCCreatePaymentBuildsUniqueAmountAndAddressQR(t *testing.T) {
	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"rateMode":       "manual",
		"confirmations":  "20",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260621abc12345",
		Amount:  "10.00",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}

	if resp.QRCode != "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106" {
		t.Fatalf("QRCode = %q", resp.QRCode)
	}
	if resp.Currency != "USDT" {
		t.Fatalf("Currency = %q, want USDT", resp.Currency)
	}
	if resp.CryptoCurrency != "USDT" || resp.CryptoNetwork != "BSC" || resp.ReceiveAddress != resp.QRCode {
		t.Fatalf("crypto fields = currency:%q network:%q address:%q", resp.CryptoCurrency, resp.CryptoNetwork, resp.ReceiveAddress)
	}
	if resp.CryptoAmount == "" || !strings.Contains(resp.TradeNo, resp.CryptoAmount) {
		t.Fatalf("CryptoAmount %q should be embedded in TradeNo %q", resp.CryptoAmount, resp.TradeNo)
	}
	if resp.CryptoAmount == "1.388889" {
		t.Fatalf("CryptoAmount should include unique tail, got %q", resp.CryptoAmount)
	}
	intent, err := parseUSDTBSCIntent(resp.TradeNo)
	if err != nil {
		t.Fatalf("parseUSDTBSCIntent() error = %v", err)
	}
	if intent.TokenAmount != resp.CryptoAmount {
		t.Fatalf("locked token amount = %q, want %q", intent.TokenAmount, resp.CryptoAmount)
	}
	if intent.LockedRate != "7.200000" {
		t.Fatalf("locked rate = %q, want 7.200000", intent.LockedRate)
	}
	if len(resp.TradeNo) > 128 {
		t.Fatalf("TradeNo length = %d, want <= 128", len(resp.TradeNo))
	}
	if _, err := time.Parse(time.RFC3339, intent.LockedAt); err != nil {
		t.Fatalf("locked_at = %q is not RFC3339: %v", intent.LockedAt, err)
	}
}

func TestUSDTBSCRequiresExplicitCNYPerUSDTRate(t *testing.T) {
	_, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"rateMode":       "manual",
	})
	if err == nil || !strings.Contains(err.Error(), "cnyPerUsdt is required") {
		t.Fatalf("NewUSDTBSC() error = %v, want cnyPerUsdt required", err)
	}
}

func TestUSDTBSCCreatePaymentConvertsCNYAmountByRate(t *testing.T) {
	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"rateMode":       "manual",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260621_rate_guard",
		Amount:  "72.00",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}

	amount, err := strconv.ParseFloat(resp.CryptoAmount, 64)
	if err != nil {
		t.Fatalf("CryptoAmount = %q is not numeric: %v", resp.CryptoAmount, err)
	}
	if amount < 10 || amount >= 10.001 {
		t.Fatalf("CryptoAmount = %q, want about 10 USDT plus unique micro tail", resp.CryptoAmount)
	}
}

func TestUSDTBSCCreatePaymentUsesAutoCNYPerUSDTRate(t *testing.T) {
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple/price" || r.URL.Query().Get("ids") != "tether" || r.URL.Query().Get("vs_currencies") != "cny" {
			t.Fatalf("unexpected rate request: %s", r.URL.String())
		}
		fmt.Fprint(w, `{"tether":{"cny":7.25}}`)
	}))
	defer rateAPI.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"rateMode":       "auto",
		"rateApiUrl":     rateAPI.URL + "/simple/price?ids=tether&vs_currencies=cny",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260623_auto_rate",
		Amount:  "72.50",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}

	amount, err := strconv.ParseFloat(resp.CryptoAmount, 64)
	if err != nil {
		t.Fatalf("CryptoAmount = %q is not numeric: %v", resp.CryptoAmount, err)
	}
	if amount < 10 || amount >= 10.001 {
		t.Fatalf("CryptoAmount = %q, want about 10 USDT plus unique micro tail from auto rate", resp.CryptoAmount)
	}
}

func TestUSDTBSCCreatePaymentUsesDefaultBinanceP2PRate(t *testing.T) {
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var req struct {
			Asset     string `json:"asset"`
			Fiat      string `json:"fiat"`
			TradeType string `json:"tradeType"`
			Page      int    `json:"page"`
			Rows      int    `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode binance p2p request: %v", err)
		}
		if req.Asset != "USDT" || req.Fiat != "CNY" || req.TradeType != "SELL" || req.Page != 1 || req.Rows != 5 {
			t.Fatalf("unexpected binance p2p request: %+v", req)
		}
		fmt.Fprint(w, `{"code":"000000","data":[{"adv":{"price":"6.77"}},{"adv":{"price":"6.78"}}]}`)
	}))
	defer rateAPI.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress":           "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":               "7.2",
		"rateMode":                 "auto",
		"binanceP2PRateApiUrl":     rateAPI.URL,
		"rateFallbackToManual":     "true",
		"rateCacheSeconds":         "0",
		"usdtBscRateSourceForTest": "ignored",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260623_binance_rate",
		Amount:  "67.70",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}

	amount, err := strconv.ParseFloat(resp.CryptoAmount, 64)
	if err != nil {
		t.Fatalf("CryptoAmount = %q is not numeric: %v", resp.CryptoAmount, err)
	}
	if amount < 10 || amount >= 10.001 {
		t.Fatalf("CryptoAmount = %q, want about 10 USDT plus unique micro tail from Binance P2P rate", resp.CryptoAmount)
	}
	intent, err := parseUSDTBSCIntent(resp.TradeNo)
	if err != nil {
		t.Fatalf("parseUSDTBSCIntent() error = %v", err)
	}
	if intent.LockedRate != "6.770000" {
		t.Fatalf("locked rate = %q, want 6.770000", intent.LockedRate)
	}
	if resp.Metadata["locked_cny_per_usdt"] != "6.770000" || resp.Metadata["rate_source"] != "binance_p2p_cny_sell" {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
	if resp.Metadata["token_amount"] != resp.CryptoAmount || resp.Metadata["intent_trade_no"] != resp.TradeNo {
		t.Fatalf("metadata should preserve intent and token amount, got %+v", resp.Metadata)
	}
}

func TestUSDTBSCCreatePaymentFallsBackToManualRateWhenAutoRateFails(t *testing.T) {
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate unavailable", http.StatusBadGateway)
	}))
	defer rateAPI.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress":       "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":           "7.2",
		"rateMode":             "auto",
		"rateApiUrl":           rateAPI.URL,
		"rateFallbackToManual": "true",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260623_auto_rate_fallback",
		Amount:  "72.00",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	amount, err := strconv.ParseFloat(resp.CryptoAmount, 64)
	if err != nil {
		t.Fatalf("CryptoAmount = %q is not numeric: %v", resp.CryptoAmount, err)
	}
	if amount < 10 || amount >= 10.001 {
		t.Fatalf("CryptoAmount = %q, want manual fallback about 10 USDT plus unique micro tail", resp.CryptoAmount)
	}
}

func TestUSDTBSCCreatePaymentFailsWhenAutoRateFailsWithoutFallback(t *testing.T) {
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate unavailable", http.StatusBadGateway)
	}))
	defer rateAPI.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress":       "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":           "7.2",
		"rateMode":             "auto",
		"rateApiUrl":           rateAPI.URL,
		"rateFallbackToManual": "false",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	_, err = p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260623_auto_rate_no_fallback",
		Amount:  "72.00",
	})
	if err == nil || !strings.Contains(err.Error(), "fetch usdt_bsc cny/usdt rate") {
		t.Fatalf("CreatePayment() error = %v, want auto rate failure", err)
	}
}

func TestUSDTBSCCreatePaymentUsesAutoRateWithoutManualFallbackConfig(t *testing.T) {
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tether":{"cny":7.25}}`)
	}))
	defer rateAPI.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress":       "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"rateMode":             "auto",
		"rateApiUrl":           rateAPI.URL,
		"rateFallbackToManual": "false",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2_20260623_auto_rate_no_manual",
		Amount:  "72.50",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	amount, err := strconv.ParseFloat(resp.CryptoAmount, 64)
	if err != nil {
		t.Fatalf("CryptoAmount = %q is not numeric: %v", resp.CryptoAmount, err)
	}
	if amount < 10 || amount >= 10.001 {
		t.Fatalf("CryptoAmount = %q, want about 10 USDT plus unique micro tail", resp.CryptoAmount)
	}
}

func TestUSDTBSCAutoRateRequiresHTTPURL(t *testing.T) {
	_, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress":       "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"rateMode":             "auto",
		"rateApiUrl":           "file:///etc/passwd",
		"rateFallbackToManual": "false",
	})
	if err == nil || !strings.Contains(err.Error(), "http(s) URL") {
		t.Fatalf("NewUSDTBSC() error = %v, want http(s) URL validation", err)
	}
}

func TestUSDTBSCQueryOrderMatchesConfirmedTokenTransfer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("module") != "account" || q.Get("action") != "tokentx" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"status":"1","message":"OK","result":[{"blockNumber":"123","timeStamp":"1760000000","hash":"0xpaid","nonce":"1","blockHash":"0xblock","from":"0xfrom","contractAddress":"0x55d398326f99059ff775485246999027b3197955","to":"0x3b210bdc924c685fdd10ae96b7f95d0e14536106","value":"1388891000000000000","tokenName":"Tether USD","tokenSymbol":"USDT","tokenDecimal":"18","transactionIndex":"1","gas":"1","gasPrice":"1","gasUsed":"1","cumulativeGasUsed":"1","input":"deprecated","confirmations":"21"}]}`)
	}))
	defer server.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"confirmations":  "20",
		"bscscanApiBase": server.URL + "/api",
		"rpcUrl":         "disabled",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.QueryOrder(context.Background(), "usdt_bsc|sub2_order|10.00|1.388891|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106")
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid {
		t.Fatalf("Status = %q, want paid", resp.Status)
	}
	if resp.TradeNo != "0xpaid" || resp.Amount != 10.00 {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Metadata["tx_hash"] != "0xpaid" || resp.Metadata["confirmations"] != "21" {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
}

func TestUSDTBSCQueryOrderKeepsLockedRateMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"1","message":"OK","result":[{"blockNumber":"123","hash":"0xpaid","from":"0xfrom","contractAddress":"0x55d398326f99059ff775485246999027b3197955","to":"0x3b210bdc924c685fdd10ae96b7f95d0e14536106","value":"10000123000000000000","tokenDecimal":"18","confirmations":"21"}]}`)
	}))
	defer server.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"rateMode":       "manual",
		"confirmations":  "20",
		"bscscanApiBase": server.URL + "/api",
		"rpcUrl":         "disabled",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	queryRef := "usdt_bsc|sub2_order|72.00|10.000123|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106|7.200000|1782201600"
	resp, err := p.QueryOrder(context.Background(), queryRef)
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid {
		t.Fatalf("Status = %q, want paid", resp.Status)
	}
	if resp.Metadata["token_amount"] != "10.000123" || resp.Metadata["locked_cny_per_usdt"] != "7.200000" || resp.Metadata["locked_at"] != "2026-06-23T08:00:00Z" {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
}

func TestUSDTBSCQueryOrderMatchesConfirmedTokenTransferByRPC(t *testing.T) {
	const address = "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		switch req.Method {
		case "eth_blockNumber":
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x100"}`)
		case "eth_getLogs":
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":[{"address":"0x55d398326f99059ff775485246999027b3197955","blockNumber":"0xf0","transactionHash":"0xrpcpaid","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x000000000000000000000000161ba15a00000000000000000076fbb64576fbb645","0x0000000000000000000000003b210bdc924c685fdd10ae96b7f95d0e14536106"],"data":"0x134655017ebcb000","removed":false}]}`)
		default:
			t.Fatalf("unexpected rpc method: %s", req.Method)
		}
	}))
	defer server.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"confirmations":  "16",
		"rpcUrl":         server.URL,
		"bscscanApiBase": "",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.QueryOrder(context.Background(), "usdt_bsc|sub2_order|10.00|1.388891|"+address)
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid {
		t.Fatalf("Status = %q, want paid", resp.Status)
	}
	if resp.TradeNo != "0xrpcpaid" || resp.Metadata["network"] != "BSC" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestUSDTBSCQueryOrderTriesNextRPCURL(t *testing.T) {
	badRPC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"limit exceeded"}}`)
	}))
	defer badRPC.Close()
	goodRPC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		if req.Method == "eth_blockNumber" {
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":"0x100"}`)
			return
		}
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":[{"address":"0x55d398326f99059ff775485246999027b3197955","blockNumber":"0xf0","transactionHash":"0xrpcpaid2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x000000000000000000000000161ba15a00000000000000000076fbb64576fbb645","0x0000000000000000000000003b210bdc924c685fdd10ae96b7f95d0e14536106"],"data":"0x134655017ebcb000","removed":false}]}`)
	}))
	defer goodRPC.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"confirmations":  "16",
		"rpcUrl":         badRPC.URL + "," + goodRPC.URL,
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.QueryOrder(context.Background(), "usdt_bsc|sub2_order|10.00|1.388891|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106")
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid || resp.TradeNo != "0xrpcpaid2" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestUSDTBSCQueryOrderReturnsReadableBscScanStringError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"0","message":"NOTOK","result":"You are using a deprecated V1 endpoint"}`)
	}))
	defer server.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"rpcUrl":         "disabled",
		"bscscanApiBase": server.URL + "/api",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	_, err = p.QueryOrder(context.Background(), "usdt_bsc|sub2_order|10.00|1.388891|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106")
	if err == nil || !strings.Contains(err.Error(), "deprecated V1 endpoint") {
		t.Fatalf("QueryOrder() error = %v, want readable bscscan error", err)
	}
}

func TestUSDTBSCQueryOrderIgnoresNetAmountBelowExpected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"1","message":"OK","result":[{"hash":"0xshort","to":"0x3b210bdc924c685fdd10ae96b7f95d0e14536106","contractAddress":"0x55d398326f99059ff775485246999027b3197955","value":"10000000000000000000","tokenDecimal":"18","confirmations":"21"}]}`)
	}))
	defer server.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"confirmations":  "20",
		"bscscanApiBase": server.URL + "/api",
		"rpcUrl":         "disabled",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.QueryOrder(context.Background(), "usdt_bsc|sub2_order|72.00|10.000123|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106")
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPending {
		t.Fatalf("Status = %q, want pending", resp.Status)
	}
}

func TestUSDTBSCQueryOrderIgnoresInsufficientConfirmations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"1","message":"OK","result":[{"hash":"0xpending","to":"0x3b210bdc924c685fdd10ae96b7f95d0e14536106","contractAddress":"0x55d398326f99059ff775485246999027b3197955","value":"1388891000000000000","tokenDecimal":"18","confirmations":"3"}]}`)
	}))
	defer server.Close()

	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
		"confirmations":  "20",
		"bscscanApiBase": server.URL + "/api",
		"rpcUrl":         "disabled",
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}

	resp, err := p.QueryOrder(context.Background(), "usdt_bsc|sub2_order|10.00|1.388891|0x3b210bdc924c685fDd10Ae96b7f95D0E14536106")
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPending {
		t.Fatalf("Status = %q, want pending", resp.Status)
	}
}
