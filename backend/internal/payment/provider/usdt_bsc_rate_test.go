package provider

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func resetUSDTBSCRates(t *testing.T, now func() time.Time) {
	t.Helper()
	prev := usdtBSCRates
	usdtBSCRates = newUSDTBSCRateStore()
	usdtBSCRates.now = now
	t.Cleanup(func() { usdtBSCRates = prev })
}

func newAutoRateUSDTBSC(t *testing.T, rateURL string, extra map[string]string) *USDTBSC {
	t.Helper()
	config := map[string]string{
		"receiveAddress": testUSDTReceiveAddress,
		"cnyPerUsdt":     "7.2",
		"rateMode":       "auto",
		"rateApiUrl":     rateURL,
	}
	for k, v := range extra {
		config[k] = v
	}
	p, err := NewUSDTBSC("1", config)
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}
	return p
}

func lockedRate(t *testing.T, resp *payment.CreatePaymentResponse) string {
	t.Helper()
	intent, err := parseUSDTBSCIntent(resp.TradeNo)
	if err != nil {
		t.Fatalf("parseUSDTBSCIntent() error = %v", err)
	}
	return intent.LockedRate
}

func TestUSDTBSCRateIsSharedAcrossProviderInstances(t *testing.T) {
	now := time.Now()
	resetUSDTBSCRates(t, func() time.Time { return now })
	var calls atomic.Int32
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"tether":{"cny":6.65}}`)
	}))
	defer rateAPI.Close()

	// 每次下单都新建 provider 实例，汇率缓存仍应生效
	for i := 0; i < 3; i++ {
		p := newAutoRateUSDTBSC(t, rateAPI.URL, nil)
		if _, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: fmt.Sprintf("sub2_shared_%d", i), Amount: "66.50"}); err != nil {
			t.Fatalf("CreatePayment() error = %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("rate api calls = %d, want 1", got)
	}
}

func TestUSDTBSCRateUsesLastGoodWhenRefreshFails(t *testing.T) {
	clock := time.Now()
	resetUSDTBSCRates(t, func() time.Time { return clock })
	var fail atomic.Bool
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{"tether":{"cny":6.65}}`)
	}))
	defer rateAPI.Close()
	p := newAutoRateUSDTBSC(t, rateAPI.URL, map[string]string{"rateCacheSeconds": "0"})

	if _, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_good", Amount: "66.50"}); err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	fail.Store(true)
	clock = clock.Add(10 * time.Minute)
	resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_after_fail", Amount: "66.50"})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if got := lockedRate(t, resp); got != "6.650000" {
		t.Fatalf("locked rate = %s, want last good 6.65 instead of manual 7.2", got)
	}

	// 超过最长有效期后才回退到手动汇率
	clock = clock.Add(usdtBSCRateLastGoodMaxAge)
	resp, err = p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_manual", Amount: "72.00"})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if got := lockedRate(t, resp); got != "7.200000" || resp.Metadata["rate_source"] != usdtBSCRateSourceManual {
		t.Fatalf("locked rate = %s source = %s, want manual fallback", got, resp.Metadata["rate_source"])
	}
}

func TestUSDTBSCRateRejectsOutOfRangeAndJumps(t *testing.T) {
	clock := time.Now()
	resetUSDTBSCRates(t, func() time.Time { return clock })
	var price atomic.Value
	price.Store("6.65")
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tether":{"cny":%s}}`, price.Load().(string))
	}))
	defer rateAPI.Close()
	p := newAutoRateUSDTBSC(t, rateAPI.URL, map[string]string{"rateCacheSeconds": "0"})

	if _, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_base", Amount: "66.50"}); err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	for _, bad := range []string{"1", "12", "7.30"} {
		price.Store(bad)
		clock = clock.Add(time.Minute)
		resp, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_bad_" + bad, Amount: "66.50"})
		if err != nil {
			t.Fatalf("CreatePayment(%s) error = %v", bad, err)
		}
		if got := lockedRate(t, resp); got != "6.650000" {
			t.Fatalf("price %s: locked rate = %s, want rejected and keep 6.65", bad, got)
		}
	}
}

func TestUSDTBSCRateRejectsOutOfRangeWithoutHistory(t *testing.T) {
	resetUSDTBSCRates(t, time.Now)
	rateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tether":{"cny":1}}`)
	}))
	defer rateAPI.Close()
	p := newAutoRateUSDTBSC(t, rateAPI.URL, map[string]string{"rateFallbackToManual": "false"})

	_, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_one", Amount: "10.00"})
	if err == nil {
		t.Fatal("CreatePayment() succeeded with 1 CNY/USDT, want rejection")
	}
}

func TestExtractBinanceP2PCNYRateUsesMedian(t *testing.T) {
	payload := map[string]any{"data": []any{
		map[string]any{"adv": map[string]any{"price": "7.50"}},
		map[string]any{"adv": map[string]any{"price": "6.66"}},
		map[string]any{"adv": map[string]any{"price": "bad"}},
		map[string]any{"adv": map[string]any{"price": "6.64"}},
		map[string]any{"adv": map[string]any{"price": "6.65"}},
	}}
	rate, err := extractBinanceP2PCNYRate(payload)
	if err != nil {
		t.Fatalf("extractBinanceP2PCNYRate() error = %v", err)
	}
	if rate.Cmp(big.NewRat(1331, 200)) != 0 {
		t.Fatalf("rate = %s, want median 6.655", rate.FloatString(4))
	}
}

func TestUSDTBSCRateAmountTier(t *testing.T) {
	cases := map[string]int64{"50": 100, "100": 100, "100.01": 1000, "5000": 10000, "99999": 10000}
	for amount, want := range cases {
		r, _ := new(big.Rat).SetString(amount)
		if got := usdtBSCRateAmountTier(r); got != want {
			t.Fatalf("tier(%s) = %d, want %d", amount, got, want)
		}
	}
}

func TestUSDTBSCRateStoreMergesConcurrentFetches(t *testing.T) {
	store := newUSDTBSCRateStore()
	release := make(chan struct{})
	var calls atomic.Int32
	fetch := func(context.Context) (*big.Rat, error) {
		calls.Add(1)
		<-release
		return big.NewRat(665, 100), nil
	}
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := store.refresh(context.Background(), "k", fetch)
			errs <- err
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	for i := 0; i < 5; i++ {
		if err := <-errs; err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("refresh() error = %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1", got)
	}
}
