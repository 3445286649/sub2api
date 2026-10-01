package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const testUSDTReceiveAddress = "0x3b210bdc924c685fDd10Ae96b7f95D0E14536106"

// fakeBSCChain 模拟一条出块时间固定的链：区块 n 的时间为 genesis + n*blockSeconds。
type fakeBSCChain struct {
	mu           sync.Mutex
	head         int64
	genesis      int64
	blockSeconds int64
	maxRange     int64
	transfers    []fakeBSCTransfer
	logCalls     int
	ranges       [][2]int64
	failAll      bool
}

type fakeBSCTransfer struct {
	block int64
	hash  string
	value *big.Int
}

func newFakeBSCChain(head int64, now time.Time) *fakeBSCChain {
	return &fakeBSCChain{head: head, blockSeconds: 1, genesis: now.Unix() - head, maxRange: 50}
}

func (c *fakeBSCChain) blockTime(n int64) int64 { return c.genesis + n*c.blockSeconds }

func (c *fakeBSCChain) addTransfer(block int64, hash, tokenAmount string) {
	raw, err := decimalToRaw(tokenAmount, usdtBSCTokenDecimals)
	if err != nil {
		panic(err)
	}
	c.transfers = append(c.transfers, fakeBSCTransfer{block: block, hash: hash, value: raw})
}

func (c *fakeBSCChain) serve(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode rpc request: %v", err)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.failAll {
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"limit exceeded"}}`)
			return
		}
		switch req.Method {
		case "eth_getBlockByNumber":
			var tag string
			_ = json.Unmarshal(req.Params[0], &tag)
			n := c.head
			if tag != "latest" {
				n, _ = parseHexInt64(tag)
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"number":"%s","timestamp":"0x%x"}}`, int64ToHex(n), c.blockTime(n))
		case "eth_getLogs":
			c.logCalls++
			var filter struct {
				FromBlock string `json:"fromBlock"`
				ToBlock   string `json:"toBlock"`
			}
			_ = json.Unmarshal(req.Params[0], &filter)
			from, _ := parseHexInt64(filter.FromBlock)
			to, _ := parseHexInt64(filter.ToBlock)
			if to-from+1 > c.maxRange {
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"eth_getLogs is limited to 0 - %d blocks range"}}`, c.maxRange)
				return
			}
			c.ranges = append(c.ranges, [2]int64{from, to})
			logs := make([]string, 0)
			for i, tr := range c.transfers {
				if tr.block < from || tr.block > to {
					continue
				}
				logs = append(logs, fmt.Sprintf(`{"address":"%s","blockNumber":"%s","transactionHash":"%s","logIndex":"0x%x","topics":["%s","%s","%s"],"data":"0x%s","removed":false}`,
					defaultUSDTBSCContract, int64ToHex(tr.block), tr.hash, i, usdtBSCTransferTopic,
					evmAddressTopic("0x161ba15a00000000000000000076fbb64576fbb6"), evmAddressTopic(testUSDTReceiveAddress), tr.value.Text(16)))
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":[%s]}`, strings.Join(logs, ","))
		default:
			t.Errorf("unexpected rpc method: %s", req.Method)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func resetUSDTBSCScans(t *testing.T, now func() time.Time) {
	t.Helper()
	prev := usdtBSCScans
	usdtBSCScans = &usdtBSCScanRegistry{states: map[string]*usdtBSCScanState{}, now: now}
	t.Cleanup(func() { usdtBSCScans = prev })
}

func newTestUSDTBSC(t *testing.T, rpcURLs ...string) *USDTBSC {
	t.Helper()
	p, err := NewUSDTBSC("1", map[string]string{
		"receiveAddress": testUSDTReceiveAddress,
		"cnyPerUsdt":     "7.2",
		"rateMode":       "manual",
		"confirmations":  "20",
		"rpcUrl":         strings.Join(rpcURLs, ","),
	})
	if err != nil {
		t.Fatalf("NewUSDTBSC() error = %v", err)
	}
	return p
}

func usdtIntentRef(orderID, tokenAmount string, lockedAt time.Time) string {
	return fmt.Sprintf("usdt_bsc|%s|72.00|%s|%s|7.200000|%d", orderID, tokenAmount, testUSDTReceiveAddress, lockedAt.Unix())
}

func TestUSDTBSCQueryOrderScansFromLockedTimeBeyondLegacyLookback(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetUSDTBSCScans(t, func() time.Time { return now })
	chain := newFakeBSCChain(100_000, now)
	chain.maxRange = 5000
	// 用户在下单 50 分钟后付款 —— 旧实现固定只回看 3600 块，在快速出块的链上会漏掉更早的付款
	lockedAt := now.Add(-50 * time.Minute)
	paidBlock := chain.head - int64(45*60)
	chain.addTransfer(paidBlock, "0xpaid", "10.000123")
	server := chain.serve(t)

	resp, err := newTestUSDTBSC(t, server.URL).QueryOrder(context.Background(), usdtIntentRef("sub2_late", "10.000123", lockedAt))
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid || resp.TradeNo != "0xpaid" {
		t.Fatalf("response = %+v, want paid by 0xpaid", resp)
	}
	if resp.Metadata["block_timestamp"] != strconv.FormatInt(chain.blockTime(paidBlock), 10) {
		t.Fatalf("metadata = %+v, want block timestamp of paid block", resp.Metadata)
	}
}

func TestUSDTBSCQueryOrderSharesScanAcrossOrders(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetUSDTBSCScans(t, func() time.Time { return now })
	chain := newFakeBSCChain(50_000, now)
	chain.maxRange = 2000
	chain.addTransfer(chain.head-100, "0xsecond", "20.000456")
	server := chain.serve(t)
	p := newTestUSDTBSC(t, server.URL)
	lockedAt := now.Add(-20 * time.Minute)

	// 第一个订单一直没付款，第二个订单已付款：第二次查询不应再重复扫描整段区块
	first, err := p.QueryOrder(context.Background(), usdtIntentRef("sub2_unpaid", "10.000123", lockedAt))
	if err != nil || first.Status != payment.ProviderStatusPending {
		t.Fatalf("first QueryOrder() = %+v, %v; want pending", first, err)
	}
	callsAfterFirst := chain.logCalls
	second, err := p.QueryOrder(context.Background(), usdtIntentRef("sub2_paid", "20.000456", lockedAt))
	if err != nil {
		t.Fatalf("second QueryOrder() error = %v", err)
	}
	if second.Status != payment.ProviderStatusPaid || second.TradeNo != "0xsecond" {
		t.Fatalf("second response = %+v, want paid", second)
	}
	if extra := chain.logCalls - callsAfterFirst; extra != 0 {
		t.Fatalf("second order triggered %d extra eth_getLogs calls, want 0 (shared scan)", extra)
	}
}

func TestUSDTBSCQueryOrderOnlyScansNewBlocksOnNextRound(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	clock := now
	resetUSDTBSCScans(t, func() time.Time { return clock })
	chain := newFakeBSCChain(50_000, now)
	chain.maxRange = 2000
	server := chain.serve(t)
	p := newTestUSDTBSC(t, server.URL)
	ref := usdtIntentRef("sub2_wait", "10.000123", now.Add(-10*time.Minute))

	if _, err := p.QueryOrder(context.Background(), ref); err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	chain.mu.Lock()
	chain.head += 60
	chain.genesis = now.Add(time.Minute).Unix() - chain.head
	chain.ranges = nil
	chain.addTransfer(chain.head-30, "0xnew", "10.000123")
	chain.mu.Unlock()
	clock = now.Add(time.Minute)

	resp, err := p.QueryOrder(context.Background(), ref)
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid || resp.TradeNo != "0xnew" {
		t.Fatalf("response = %+v, want paid by new transfer", resp)
	}
	for _, r := range chain.ranges {
		if r[0] < chain.head-60-usdtBSCReorgDepth {
			t.Fatalf("second round rescanned old range %v, want only new blocks plus reorg depth", r)
		}
	}
}

func TestUSDTBSCQueryOrderShrinksLogWindowAndSwitchesRPC(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetUSDTBSCScans(t, func() time.Time { return now })
	broken := newFakeBSCChain(20_000, now)
	broken.failAll = true
	chain := newFakeBSCChain(20_000, now)
	chain.maxRange = 50
	chain.addTransfer(chain.head-300, "0xpaid", "10.000123")
	brokenServer := broken.serve(t)
	server := chain.serve(t)

	resp, err := newTestUSDTBSC(t, brokenServer.URL, server.URL).QueryOrder(context.Background(), usdtIntentRef("sub2_rpc", "10.000123", now.Add(-10*time.Minute)))
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPaid || resp.TradeNo != "0xpaid" {
		t.Fatalf("response = %+v, want paid via second RPC", resp)
	}
	for _, r := range chain.ranges {
		if r[1]-r[0]+1 > 50 {
			t.Fatalf("range %v exceeds RPC limit", r)
		}
	}
}

func TestUSDTBSCQueryOrderPendingUntilConfirmations(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetUSDTBSCScans(t, func() time.Time { return now })
	chain := newFakeBSCChain(20_000, now)
	chain.maxRange = 2000
	chain.addTransfer(chain.head-5, "0xfresh", "10.000123")
	server := chain.serve(t)

	resp, err := newTestUSDTBSC(t, server.URL).QueryOrder(context.Background(), usdtIntentRef("sub2_conf", "10.000123", now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPending {
		t.Fatalf("Status = %q, want pending below confirmations", resp.Status)
	}
}

func TestUSDTBSCQueryOrderIgnoresTransferBeforeLockedAtViaRPC(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetUSDTBSCScans(t, func() time.Time { return now })
	chain := newFakeBSCChain(20_000, now)
	chain.maxRange = 2000
	// 同金额的转账发生在锁价之前 5 分钟，不属于本订单
	chain.addTransfer(chain.head-15*60, "0xstale", "10.000123")
	server := chain.serve(t)

	resp, err := newTestUSDTBSC(t, server.URL).QueryOrder(context.Background(), usdtIntentRef("sub2_stale_rpc", "10.000123", now.Add(-10*time.Minute)))
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if resp.Status != payment.ProviderStatusPending {
		t.Fatalf("Status = %q, want pending for pre-intent transfer", resp.Status)
	}
}

func TestAddUniqueMicroTailSkipsReservedAmounts(t *testing.T) {
	first := addUniqueMicroTail("10.000000", "sub2_tail_collision", nil)
	second := addUniqueMicroTail("10.000000", "sub2_tail_collision", []string{first})
	if second == first {
		t.Fatalf("tail %q reused although reserved", second)
	}
	third := addUniqueMicroTail("10.000000", "sub2_tail_collision", []string{first, second})
	if third == first || third == second {
		t.Fatalf("tail %q reused although reserved", third)
	}
}

func TestUSDTBSCIntentTokenAmount(t *testing.T) {
	amount, ok := USDTBSCIntentTokenAmount(usdtIntentRef("sub2_x", "10.000123", time.Now()))
	if !ok || amount != "10.000123" {
		t.Fatalf("USDTBSCIntentTokenAmount() = %q, %v", amount, ok)
	}
	if _, ok := USDTBSCIntentTokenAmount("0xhash"); ok {
		t.Fatal("tx hash must not parse as intent")
	}
}
