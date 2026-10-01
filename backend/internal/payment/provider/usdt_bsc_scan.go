package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// 链上到账检测。
//
// 所有 USDT 订单共用同一个收款地址，所以按「收款地址 + 合约 + RPC 列表」共享一份已扫描区块的缓存：
//   - 每次查询只补扫上次之后新出的区块，以及订单锁价时间之前尚未覆盖的区块；
//     多个待支付订单在同一轮对账里基本只触发一次链上请求，一个不付款的订单不会拖慢其他订单。
//   - 起始区块按订单锁价时间和实测出块间隔推算，不写死回看块数，BSC 出块速度变化也不会漏查。
//   - 每段扫描完成即记录进度，即使单次请求超时，下次也能从断点继续。
//   - 末尾若干区块每次重扫，覆盖链重组。
//   - 单个 RPC 的 eth_getLogs 区块跨度上限各不相同，按 2000 / 500 / 100 / 50 / 25 自动降级并记住可用跨度；
//     一个 RPC 不可用时换下一个。
const (
	usdtBSCLegacyLookback     = 2 * time.Hour
	usdtBSCMaxLookback        = 6 * time.Hour
	usdtBSCLookbackSafety     = 1.25
	usdtBSCLookbackExtra      = int64(200)
	usdtBSCReorgDepth         = int64(15)
	usdtBSCHeadReuseWindow    = 3 * time.Second
	usdtBSCBlockTimeSample    = int64(2000)
	usdtBSCBlockTimeTTL       = 30 * time.Minute
	usdtBSCDefaultBlockTimeMs = 3000
)

var usdtBSCLogWindowSizes = []int64{2000, 500, 100, 50, 25}

type usdtBSCTransfer struct {
	bscScanTokenTx
	block    int64
	logIndex string
}

type usdtBSCScanState struct {
	// lock 是容量为 1 的信号量：用户查单与后台对账串行扫描，等待时仍能响应 ctx 取消。
	lock chan struct{}

	// [lo, hi] 是已连续扫描过的区块区间；lo > hi 表示尚未扫描。
	lo, hi    int64
	transfers []usdtBSCTransfer

	head       int64
	headTime   int64
	headSeenAt time.Time

	blockTimeMs       float64
	blockTimeSampleAt time.Time

	// 每个 RPC 当前可用的 eth_getLogs 区块跨度（usdtBSCLogWindowSizes 的下标）。
	windowIndex map[string]int
	blockTimes  map[int64]string
	// 每个订单首次推算出的起始区块。固定下来，避免随时间推移重复推算导致起点不断前移、反复扫旧区块。
	orderStarts map[string]int64
}

type usdtBSCScanRegistry struct {
	mu     sync.Mutex
	states map[string]*usdtBSCScanState
	now    func() time.Time
}

var usdtBSCScans = &usdtBSCScanRegistry{states: map[string]*usdtBSCScanState{}, now: time.Now}

func (r *usdtBSCScanRegistry) state(key string) *usdtBSCScanState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.states[key]
	if !ok {
		st = &usdtBSCScanState{lock: make(chan struct{}, 1), lo: 1, hi: 0, windowIndex: map[string]int{}, blockTimes: map[int64]string{}, orderStarts: map[string]int64{}}
		r.states[key] = st
	}
	return st
}

func (p *USDTBSC) QueryOrder(ctx context.Context, queryRef string) (*payment.QueryOrderResponse, error) {
	intent, err := parseUSDTBSCIntent(queryRef)
	if err != nil {
		return nil, err
	}
	expectedRaw, err := decimalToRaw(intent.TokenAmount, usdtBSCTokenDecimals)
	if err != nil {
		return nil, err
	}

	var (
		events []bscScanTokenTx
		now    = usdtBSCScans.now().UTC()
	)
	if len(p.rpcURLs) > 0 && !p.rpcDisabled {
		events, err = p.findTransfersByRPC(ctx, intent, expectedRaw, now)
	} else {
		events, err = p.fetchTokenTransfersByBscScan(ctx, intent.ReceiveAddress)
	}
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if !strings.EqualFold(event.To, intent.ReceiveAddress) {
			continue
		}
		if !strings.EqualFold(event.ContractAddress, p.tokenContract) {
			continue
		}
		if strings.TrimSpace(event.Value) != expectedRaw.String() {
			continue
		}
		if intent.LockedAt != "" && !usdtBSCTransferInIntentWindow(event.Timestamp, intent.LockedAt, now) {
			continue
		}
		confirmations, _ := strconv.ParseInt(strings.TrimSpace(event.Confirmations), 10, 64)
		if confirmations < p.confirmations {
			return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
		}
		cny, _ := strconv.ParseFloat(intent.CNYAmount, 64)
		return &payment.QueryOrderResponse{
			TradeNo:  event.Hash,
			Status:   payment.ProviderStatusPaid,
			Amount:   cny,
			Metadata: buildUSDTBSCQueryMetadata(intent, event),
		}, nil
	}
	return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
}

func (p *USDTBSC) scanKey(address string) string {
	return strings.ToLower(address) + "|" + p.tokenContract + "|" + strings.Join(p.rpcURLs, ",")
}

// findTransfersByRPC 返回金额匹配的转账，按区块从新到旧排列，并补齐区块时间与确认数。
func (p *USDTBSC) findTransfersByRPC(ctx context.Context, intent usdtBSCIntent, expectedRaw *big.Int, now time.Time) ([]bscScanTokenTx, error) {
	st := usdtBSCScans.state(p.scanKey(intent.ReceiveAddress))
	select {
	case st.lock <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-st.lock }()

	var lastErr error
	for _, rpcURL := range p.rpcURLs {
		events, err := p.findTransfersWithRPC(ctx, st, rpcURL, intent, expectedRaw, now)
		if err == nil {
			return events, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

func (p *USDTBSC) findTransfersWithRPC(ctx context.Context, st *usdtBSCScanState, rpcURL string, intent usdtBSCIntent, expectedRaw *big.Int, now time.Time) ([]bscScanTokenTx, error) {
	if err := p.refreshHead(ctx, st, rpcURL, now); err != nil {
		return nil, err
	}
	from, floor, err := p.scanStartBlock(ctx, st, rpcURL, intent)
	if err != nil {
		return nil, err
	}
	if err := p.extendScan(ctx, st, rpcURL, intent.ReceiveAddress, from, floor); err != nil {
		return nil, err
	}

	want := expectedRaw.String()
	matches := make([]bscScanTokenTx, 0, 2)
	for i := len(st.transfers) - 1; i >= 0; i-- {
		transfer := st.transfers[i]
		if transfer.block < from || strings.TrimSpace(transfer.Value) != want {
			continue
		}
		event := transfer.bscScanTokenTx
		event.Confirmations = strconv.FormatInt(maxInt64(st.head-transfer.block+1, 0), 10)
		if intent.LockedAt != "" {
			timestamp, err := p.blockTimestamp(ctx, st, rpcURL, transfer.block)
			if err != nil {
				return nil, fmt.Errorf("fetch bsc block timestamp: %w", err)
			}
			event.Timestamp = timestamp
		}
		matches = append(matches, event)
	}
	return matches, nil
}

// refreshHead 更新最新区块号与时间；同一轮对账内短时间重复调用直接复用。
func (p *USDTBSC) refreshHead(ctx context.Context, st *usdtBSCScanState, rpcURL string, now time.Time) error {
	if st.head > 0 && now.Sub(st.headSeenAt) >= 0 && now.Sub(st.headSeenAt) < usdtBSCHeadReuseWindow {
		return nil
	}
	number, timestamp, err := p.blockHeader(ctx, rpcURL, "latest")
	if err != nil {
		return err
	}
	if number < st.head {
		// 不同 RPC 同步进度略有差异，不让最新区块倒退
		number, timestamp = st.head, st.headTime
	}
	st.head, st.headTime, st.headSeenAt = number, timestamp, now
	return nil
}

// scanStartBlock 按订单锁价时间推算需要覆盖的最早区块，同时返回缓存保留的下限（最长回看范围）。
func (p *USDTBSC) scanStartBlock(ctx context.Context, st *usdtBSCScanState, rpcURL string, intent usdtBSCIntent) (int64, int64, error) {
	lookback := usdtBSCLegacyLookback
	if intent.LockedAt != "" {
		lockedAt, err := time.Parse(time.RFC3339, intent.LockedAt)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid usdt_bsc locked time")
		}
		lookback = time.Unix(st.headTime, 0).Sub(lockedAt) + usdtBSCChainClockSkew
	}
	if lookback < 0 {
		lookback = 0
	}
	if lookback > usdtBSCMaxLookback {
		lookback = usdtBSCMaxLookback
	}
	blockTimeMs, err := p.averageBlockTimeMs(ctx, st, rpcURL)
	if err != nil {
		return 0, 0, err
	}
	blocksFor := func(d time.Duration) int64 {
		return int64(float64(d.Milliseconds())/blockTimeMs*usdtBSCLookbackSafety) + usdtBSCLookbackExtra
	}
	floor := maxInt64(st.head-blocksFor(usdtBSCMaxLookback), 0)
	for key, start := range st.orderStarts {
		if start < floor {
			delete(st.orderStarts, key)
		}
	}
	key := intent.OrderID + "|" + intent.TokenAmount + "|" + intent.LockedAt
	if start, ok := st.orderStarts[key]; ok {
		return start, floor, nil
	}
	start := maxInt64(st.head-blocksFor(lookback), floor)
	st.orderStarts[key] = start
	return start, floor, nil
}

func (p *USDTBSC) averageBlockTimeMs(ctx context.Context, st *usdtBSCScanState, rpcURL string) (float64, error) {
	if st.blockTimeMs > 0 && usdtBSCScans.now().Sub(st.blockTimeSampleAt) < usdtBSCBlockTimeTTL {
		return st.blockTimeMs, nil
	}
	sampleBlock := st.head - usdtBSCBlockTimeSample
	if sampleBlock <= 0 {
		return usdtBSCDefaultBlockTimeMs, nil
	}
	_, sampleTime, err := p.blockHeader(ctx, rpcURL, int64ToHex(sampleBlock))
	if err != nil {
		return 0, err
	}
	elapsedMs := float64(st.headTime-sampleTime) * 1000
	if elapsedMs <= 0 {
		return usdtBSCDefaultBlockTimeMs, nil
	}
	st.blockTimeMs = elapsedMs / float64(st.head-sampleBlock)
	st.blockTimeSampleAt = usdtBSCScans.now()
	return st.blockTimeMs, nil
}

// extendScan 让已扫描区间覆盖 [from, head]：有新区块时先重扫末尾防重组并补新区块，再向前补旧区块。
// floor 之前的数据超出最长回看范围，直接丢弃。
func (p *USDTBSC) extendScan(ctx context.Context, st *usdtBSCScanState, rpcURL, address string, from, floor int64) error {
	if st.lo > st.hi {
		st.lo, st.hi = st.head+1, st.head
	} else if st.head > st.hi {
		rescanFrom := maxInt64(st.lo, st.hi-usdtBSCReorgDepth+1)
		st.dropTransfersFrom(rescanFrom)
		st.hi = rescanFrom - 1
	}
	if st.lo < floor && floor <= st.hi {
		st.dropTransfersBefore(floor)
		st.lo = floor
	}

	for st.hi < st.head {
		logs, end, err := p.fetchLogsWindow(ctx, st, rpcURL, address, st.hi+1, st.head, true)
		if err != nil {
			return err
		}
		st.appendTransfers(logs)
		st.hi = end
	}
	for st.lo > from {
		logs, start, err := p.fetchLogsWindow(ctx, st, rpcURL, address, from, st.lo-1, false)
		if err != nil {
			return err
		}
		st.appendTransfers(logs)
		st.lo = start
	}
	return nil
}

// fetchLogsWindow 拉取一段区块的日志。forward=true 时从 lo 起向后取一段，否则从 hi 起向前取一段；
// 返回实际覆盖到的另一端区块号。区块跨度超限时自动缩小并记住。
func (p *USDTBSC) fetchLogsWindow(ctx context.Context, st *usdtBSCScanState, rpcURL, address string, lo, hi int64, forward bool) ([]rpcLog, int64, error) {
	var lastErr error
	for idx := st.windowIndex[rpcURL]; idx < len(usdtBSCLogWindowSizes); idx++ {
		size := usdtBSCLogWindowSizes[idx]
		from, to := lo, minInt64(lo+size-1, hi)
		if !forward {
			from, to = maxInt64(hi-size+1, lo), hi
		}
		raw, err := p.rpcCall(ctx, rpcURL, "eth_getLogs", []any{map[string]any{
			"address":   p.tokenContract,
			"fromBlock": int64ToHex(from),
			"toBlock":   int64ToHex(to),
			"topics":    []any{usdtBSCTransferTopic, nil, evmAddressTopic(address)},
		}})
		if err != nil {
			// 只有节点明确返回错误时才缩小跨度重试；网络错误直接换下一个 RPC，避免偶发超时把跨度永久降到最小
			var rpcErr *rpcError
			if !errors.As(err, &rpcErr) {
				return nil, 0, err
			}
			lastErr = err
			continue
		}
		var logs []rpcLog
		if err := json.Unmarshal(raw, &logs); err != nil {
			return nil, 0, fmt.Errorf("decode rpc logs: %w", err)
		}
		st.windowIndex[rpcURL] = idx
		if forward {
			return logs, to, nil
		}
		return logs, from, nil
	}
	return nil, 0, lastErr
}

func (st *usdtBSCScanState) appendTransfers(logs []rpcLog) {
	seen := make(map[string]struct{}, len(st.transfers))
	for _, t := range st.transfers {
		seen[t.Hash+"#"+t.logIndex] = struct{}{}
	}
	for _, log := range logs {
		if log.Removed {
			continue
		}
		event, ok := rpcLogToTokenTx(log, 0)
		if !ok {
			continue
		}
		key := event.Hash + "#" + log.LogIndex
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		block, _ := strconv.ParseInt(event.BlockNumber, 10, 64)
		st.transfers = append(st.transfers, usdtBSCTransfer{bscScanTokenTx: event, block: block, logIndex: log.LogIndex})
	}
	sort.SliceStable(st.transfers, func(i, j int) bool { return st.transfers[i].block < st.transfers[j].block })
}

func (st *usdtBSCScanState) dropTransfersFrom(block int64) {
	kept := st.transfers[:0]
	for _, t := range st.transfers {
		if t.block < block {
			kept = append(kept, t)
		}
	}
	st.transfers = kept
	for b := range st.blockTimes {
		if b >= block {
			delete(st.blockTimes, b)
		}
	}
}

func (st *usdtBSCScanState) dropTransfersBefore(block int64) {
	kept := st.transfers[:0]
	for _, t := range st.transfers {
		if t.block >= block {
			kept = append(kept, t)
		}
	}
	st.transfers = kept
	for b := range st.blockTimes {
		if b < block {
			delete(st.blockTimes, b)
		}
	}
}

func (p *USDTBSC) blockTimestamp(ctx context.Context, st *usdtBSCScanState, rpcURL string, block int64) (string, error) {
	if ts, ok := st.blockTimes[block]; ok {
		return ts, nil
	}
	_, timestamp, err := p.blockHeader(ctx, rpcURL, int64ToHex(block))
	if err != nil {
		return "", err
	}
	ts := strconv.FormatInt(timestamp, 10)
	st.blockTimes[block] = ts
	return ts, nil
}

func (p *USDTBSC) blockHeader(ctx context.Context, rpcURL, block string) (int64, int64, error) {
	raw, err := p.rpcCall(ctx, rpcURL, "eth_getBlockByNumber", []any{block, false})
	if err != nil {
		return 0, 0, err
	}
	var header struct {
		Number    string `json:"number"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return 0, 0, fmt.Errorf("decode bsc block: %w", err)
	}
	number, err := parseHexInt64(header.Number)
	if err != nil {
		return 0, 0, fmt.Errorf("parse bsc block number: %w", err)
	}
	timestamp, err := parseHexInt64(header.Timestamp)
	if err != nil {
		return 0, 0, fmt.Errorf("parse bsc block timestamp: %w", err)
	}
	return number, timestamp, nil
}

// fetchTokenTransfersByBscScan 仅在 RPC 显式设为 disabled 时使用。
func (p *USDTBSC) fetchTokenTransfersByBscScan(ctx context.Context, address string) ([]bscScanTokenTx, error) {
	u, err := url.Parse(p.bscscanAPIBase)
	if err != nil {
		return nil, fmt.Errorf("parse bscscan api base: %w", err)
	}
	q := u.Query()
	q.Set("module", "account")
	q.Set("action", "tokentx")
	q.Set("contractaddress", p.tokenContract)
	q.Set("address", address)
	q.Set("startblock", "0")
	q.Set("endblock", "99999999")
	q.Set("sort", "desc")
	if p.bscscanAPIKey != "" {
		q.Set("apikey", p.bscscanAPIKey)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query bscscan token transfers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bscscan status %d", resp.StatusCode)
	}
	var parsed bscScanTokenTxResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode bscscan response: %w", err)
	}
	if parsed.Status == "0" && strings.EqualFold(parsed.Message, "No transactions found") {
		return nil, nil
	}
	resultText := strings.TrimSpace(string(parsed.Result))
	if strings.HasPrefix(resultText, `"`) {
		var message string
		if err := json.Unmarshal(parsed.Result, &message); err == nil {
			return nil, fmt.Errorf("bscscan response error: %s", message)
		}
	}
	var events []bscScanTokenTx
	if err := json.Unmarshal(parsed.Result, &events); err != nil {
		return nil, fmt.Errorf("decode bscscan result: %w", err)
	}
	return events, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
