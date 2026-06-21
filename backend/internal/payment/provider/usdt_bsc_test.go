package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestUSDTBSCCreatePaymentBuildsUniqueAmountAndAddressQR(t *testing.T) {
	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
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
}

func TestUSDTBSCRequiresExplicitCNYPerUSDTRate(t *testing.T) {
	_, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
	})
	if err == nil || !strings.Contains(err.Error(), "cnyPerUsdt is required") {
		t.Fatalf("NewUSDTBSC() error = %v, want cnyPerUsdt required", err)
	}
}

func TestUSDTBSCCreatePaymentConvertsCNYAmountByRate(t *testing.T) {
	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106",
		"cnyPerUsdt":     "7.2",
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
