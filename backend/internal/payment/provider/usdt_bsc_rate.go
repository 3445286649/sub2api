package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// USDT/CNY 自动汇率。
//
// 设计要点：
//   - 每次下单都会新建 provider 实例，所以缓存放在包级别，按「汇率源 + 金额档位」共享。
//   - 缓存过期后先返回旧值、后台刷新，下单不等外部接口；完全没有缓存时才同步拉取。
//   - Binance P2P 取前几个报价的中位数，不只看第一个商家，避免单个异常报价带偏汇率。
//   - 新汇率超出合理区间，或与最近一次可用汇率偏差过大时丢弃，继续用最近一次可用汇率。
//   - 拉取失败时先用最近一次可用汇率（有效期内），都没有才回退手动汇率并记警告日志。
const (
	usdtBSCRateFetchTimeout   = 6 * time.Second
	usdtBSCRateLastGoodMaxAge = 30 * time.Minute
	usdtBSCRateMaxDeviation   = 0.05
	usdtBSCRateBinanceRows    = 10
	usdtBSCRateMedianSamples  = 5
)

var (
	// 1 USDT 折合人民币的合理区间，超出视为异常报价。
	usdtBSCRateMin = big.NewRat(5, 1)
	usdtBSCRateMax = big.NewRat(9, 1)

	// 按订单金额分档查询 Binance P2P：大额广告的报价通常高于小额，
	// 用相近金额的报价更接近实际可成交价。档位而非精确金额，是为了让缓存能命中。
	usdtBSCRateAmountTiers = []int64{100, 1000, 10000}
)

type usdtBSCRateEntry struct {
	rate      *big.Rat
	fetchedAt time.Time
}

type usdtBSCRateStore struct {
	mu      sync.Mutex
	entries map[string]usdtBSCRateEntry
	flight  singleflight.Group
	now     func() time.Time
}

var usdtBSCRates = newUSDTBSCRateStore()

func newUSDTBSCRateStore() *usdtBSCRateStore {
	return &usdtBSCRateStore{entries: map[string]usdtBSCRateEntry{}, now: time.Now}
}

func (s *usdtBSCRateStore) get(key string) (usdtBSCRateEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	return entry, ok
}

func (s *usdtBSCRateStore) put(key string, rate *big.Rat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = usdtBSCRateEntry{rate: new(big.Rat).Set(rate), fetchedAt: s.now()}
}

// refresh 拉取一次汇率并校验，通过后写入缓存。并发调用合并为一次请求。
func (s *usdtBSCRateStore) refresh(ctx context.Context, key string, fetch func(context.Context) (*big.Rat, error)) (*big.Rat, error) {
	result, err, _ := s.flight.Do(key, func() (any, error) {
		rate, err := fetch(ctx)
		if err != nil {
			return nil, err
		}
		if err := validateUSDTBSCRate(rate); err != nil {
			return nil, err
		}
		if last, ok := s.get(key); ok && s.now().Sub(last.fetchedAt) <= usdtBSCRateLastGoodMaxAge {
			if deviation := usdtBSCRateDeviation(rate, last.rate); deviation > usdtBSCRateMaxDeviation {
				return nil, fmt.Errorf("rate %s deviates %.1f%% from last good %s",
					ratToFixed(rate, 4), deviation*100, ratToFixed(last.rate, 4))
			}
		}
		s.put(key, rate)
		return new(big.Rat).Set(rate), nil
	})
	if err != nil {
		return nil, err
	}
	return new(big.Rat).Set(result.(*big.Rat)), nil
}

func validateUSDTBSCRate(rate *big.Rat) error {
	if rate == nil || rate.Cmp(usdtBSCRateMin) < 0 || rate.Cmp(usdtBSCRateMax) > 0 {
		return fmt.Errorf("rate %s is outside the accepted range %s-%s",
			ratToFixed(rate, 4), usdtBSCRateMin.FloatString(0), usdtBSCRateMax.FloatString(0))
	}
	return nil
}

func usdtBSCRateDeviation(rate, reference *big.Rat) float64 {
	if reference == nil || reference.Sign() <= 0 {
		return 0
	}
	diff := new(big.Rat).Sub(rate, reference)
	diff.Abs(diff)
	ratio, _ := new(big.Rat).Quo(diff, reference).Float64()
	return ratio
}

func usdtBSCRateAmountTier(cnyAmount *big.Rat) int64 {
	for _, tier := range usdtBSCRateAmountTiers {
		if cnyAmount.Cmp(big.NewRat(tier, 1)) <= 0 {
			return tier
		}
	}
	return usdtBSCRateAmountTiers[len(usdtBSCRateAmountTiers)-1]
}

func (p *USDTBSC) resolveCNYPerUSDT(ctx context.Context, cnyAmount *big.Rat) (*big.Rat, string, error) {
	if p.rateMode != "auto" {
		if p.cnyPerUSDT == nil {
			return nil, "", fmt.Errorf("usdt_bsc cnyPerUsdt is required")
		}
		return new(big.Rat).Set(p.cnyPerUSDT), usdtBSCRateSourceManual, nil
	}

	var tier int64
	if p.rateSource == usdtBSCRateSourceBinanceP2P {
		tier = usdtBSCRateAmountTier(cnyAmount)
	}
	key := strings.Join([]string{p.rateSource, p.rateAPIURL, p.rateJSONPath, strconv.FormatInt(tier, 10)}, "|")
	fetch := func(fetchCtx context.Context) (*big.Rat, error) {
		return p.fetchAutoCNYPerUSDT(fetchCtx, tier)
	}

	store := usdtBSCRates
	cached, hasCached := store.get(key)
	age := store.now().Sub(cached.fetchedAt)
	if hasCached && p.rateCacheTTL > 0 && age < p.rateCacheTTL {
		return new(big.Rat).Set(cached.rate), p.rateSource, nil
	}
	if hasCached && p.rateCacheTTL > 0 && age <= usdtBSCRateLastGoodMaxAge {
		// 缓存过期但仍可用：先用旧值下单，后台刷新，不让用户等外部接口
		go func() {
			refreshCtx, cancel := context.WithTimeout(context.Background(), usdtBSCRateFetchTimeout)
			defer cancel()
			if _, err := store.refresh(refreshCtx, key, fetch); err != nil {
				slog.Warn("usdt_bsc rate refresh failed, keep last good rate", "source", p.rateSource, "error", err)
			}
		}()
		return new(big.Rat).Set(cached.rate), p.rateSource, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, usdtBSCRateFetchTimeout)
	defer cancel()
	rate, err := store.refresh(fetchCtx, key, fetch)
	if err == nil {
		return rate, p.rateSource, nil
	}
	if hasCached && age <= usdtBSCRateLastGoodMaxAge {
		slog.Warn("usdt_bsc rate fetch failed, use last good rate", "source", p.rateSource, "error", err)
		return new(big.Rat).Set(cached.rate), p.rateSource, nil
	}
	if p.rateFallbackToManual && p.cnyPerUSDT != nil {
		slog.Warn("usdt_bsc rate fetch failed, fall back to manual rate",
			"source", p.rateSource, "manual_rate", ratToFixed(p.cnyPerUSDT, 4), "error", err)
		return new(big.Rat).Set(p.cnyPerUSDT), usdtBSCRateSourceManual, nil
	}
	return nil, "", fmt.Errorf("fetch usdt_bsc cny/usdt rate: %w", err)
}

func (p *USDTBSC) fetchAutoCNYPerUSDT(ctx context.Context, tier int64) (*big.Rat, error) {
	var (
		req *http.Request
		err error
	)
	if p.rateSource == usdtBSCRateSourceBinanceP2P {
		req, err = newBinanceP2PRateRequest(ctx, p.rateAPIURL, tier)
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, p.rateAPIURL, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rate api status %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode rate api response: %w", err)
	}
	if p.rateSource == usdtBSCRateSourceBinanceP2P {
		return extractBinanceP2PCNYRate(payload)
	}
	return extractPositiveRate(payload, p.rateJSONPath)
}

func newBinanceP2PRateRequest(ctx context.Context, rawURL string, transAmount int64) (*http.Request, error) {
	body := map[string]any{
		"asset":         "USDT",
		"fiat":          "CNY",
		"tradeType":     "SELL",
		"page":          1,
		"rows":          usdtBSCRateBinanceRows,
		"payTypes":      []string{},
		"publisherType": nil,
	}
	if transAmount > 0 {
		body["transAmount"] = strconv.FormatInt(transAmount, 10)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "sub2api-usdt-bsc-rate/1.0")
	return req, nil
}

// extractBinanceP2PCNYRate 取前几条有效报价的中位数。
func extractBinanceP2PCNYRate(payload any) (*big.Rat, error) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("binance p2p response is not an object")
	}
	data, ok := root["data"].([]any)
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("binance p2p response has no data")
	}
	prices := make([]*big.Rat, 0, usdtBSCRateMedianSamples)
	for _, item := range data {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		adv, ok := obj["adv"].(map[string]any)
		if !ok {
			continue
		}
		rate, err := rateFromJSONValue(adv["price"])
		if err != nil {
			continue
		}
		prices = append(prices, rate)
		if len(prices) == usdtBSCRateMedianSamples {
			break
		}
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("binance p2p response has no valid price")
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].Cmp(prices[j]) < 0 })
	mid := len(prices) / 2
	if len(prices)%2 == 1 {
		return prices[mid], nil
	}
	sum := new(big.Rat).Add(prices[mid-1], prices[mid])
	return sum.Quo(sum, big.NewRat(2, 1)), nil
}

func extractPositiveRate(payload any, path string) (*big.Rat, error) {
	current := payload
	for _, key := range strings.Split(strings.TrimSpace(path), ".") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("rate api path %q is not an object", path)
		}
		current, ok = obj[key]
		if !ok {
			return nil, fmt.Errorf("rate api path %q not found", path)
		}
	}
	return rateFromJSONValue(current)
}

func rateFromJSONValue(value any) (*big.Rat, error) {
	var raw string
	switch v := value.(type) {
	case json.Number:
		raw = v.String()
	case float64:
		raw = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		raw = strings.TrimSpace(v)
	default:
		return nil, fmt.Errorf("rate api value is not numeric")
	}
	rate, ok := new(big.Rat).SetString(raw)
	if !ok || rate.Sign() <= 0 {
		return nil, fmt.Errorf("rate api value must be positive")
	}
	return rate, nil
}
