package provider

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	defaultUSDTBSCContract      = "0x55d398326f99059ff775485246999027b3197955"
	defaultBscScanAPIBase       = "https://api.bscscan.com/api"
	defaultUSDTBSCRPCURLs       = "https://bsc-rpc.publicnode.com,https://1rpc.io/bnb"
	defaultUSDTBSCRateAPIURL    = "https://p2p.binance.com/bapi/c2c/v2/friendly/c2c/adv/search"
	defaultUSDTBSCRateJSONPath  = "tether.cny"
	defaultUSDTBSCRateCacheSecs = int64(300)
	defaultUSDTBSCConfirmations = 20
	usdtBSCRateSourceBinanceP2P = "binance_p2p_cny_sell"
	usdtBSCRateSourceCustomAPI  = "custom_api"
	usdtBSCRateSourceManual     = "manual"
	usdtBSCIntentPrefix         = "usdt_bsc"
	usdtBSCTokenDecimals        = 18
	usdtBSCTransferTopic        = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	usdtBSCMaxIntentLength      = 128
	usdtBSCMicroTailMax         = uint64(899)
	usdtBSCChainClockSkew       = 2 * time.Minute
)

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// USDTBSC implements fixed-address USDT-BEP20 balance recharge.
// It never holds private keys; it only creates an expected transfer intent and
// queries public chain explorer data to confirm incoming token transfers.
type USDTBSC struct {
	instanceID           string
	receiveAddress       string
	tokenContract        string
	bscscanAPIBase       string
	bscscanAPIKey        string
	rpcURLs              []string
	rpcDisabled          bool
	cnyPerUSDT           *big.Rat
	rateMode             string
	rateAPIURL           string
	rateJSONPath         string
	rateSource           string
	rateFallbackToManual bool
	rateCacheTTL         time.Duration
	confirmations        int64
	httpClient           *http.Client
}

func NewUSDTBSC(instanceID string, config map[string]string) (*USDTBSC, error) {
	if config == nil {
		config = map[string]string{}
	}
	receiveAddress := strings.TrimSpace(configValue(config, "receiveAddress", "receive_address"))
	if !evmAddressPattern.MatchString(receiveAddress) {
		return nil, fmt.Errorf("usdt_bsc receiveAddress is required and must be an EVM address")
	}
	rateMode := normalizeUSDTBSCRateMode(configValue(config, "rateMode", "rate_mode", "cnyPerUsdtMode"))
	rateFallbackToManual := parseBoolConfig(configValue(config, "rateFallbackToManual", "rate_fallback_to_manual"), true)
	rateRaw := strings.TrimSpace(configValue(config, "cnyPerUsdt", "cny_per_usdt", "manualCnyPerUsdt"))
	var rate *big.Rat
	if rateRaw != "" {
		parsedRate, ok := new(big.Rat).SetString(rateRaw)
		if !ok || parsedRate.Sign() <= 0 {
			return nil, fmt.Errorf("usdt_bsc cnyPerUsdt must be a positive number")
		}
		rate = parsedRate
	}
	// 自动模式下手动汇率可留空：此时自动汇率与最近一次可用汇率都拿不到才会拒绝下单
	if rate == nil && rateMode == "manual" {
		return nil, fmt.Errorf("usdt_bsc cnyPerUsdt is required")
	}
	rateAPIURL := strings.TrimSpace(configValue(config, "rateApiUrl", "rate_api_url"))
	rateSource := ""
	if rateAPIURL == "" {
		if binanceURL := strings.TrimSpace(configValue(config, "binanceP2PRateApiUrl", "binance_p2p_rate_api_url")); binanceURL != "" {
			rateAPIURL = binanceURL
			rateSource = usdtBSCRateSourceBinanceP2P
		}
	}
	if rateAPIURL == "" {
		rateAPIURL = defaultUSDTBSCRateAPIURL
		rateSource = usdtBSCRateSourceBinanceP2P
	} else if rateSource == "" {
		rateSource = usdtBSCRateSourceCustomAPI
		if strings.Contains(strings.ToLower(rateAPIURL), "p2p.binance.com/") {
			rateSource = usdtBSCRateSourceBinanceP2P
		}
	}
	if rateMode == "auto" {
		if err := validateHTTPURL(rateAPIURL, "usdt_bsc rateApiUrl"); err != nil {
			return nil, err
		}
	}
	rateJSONPath := strings.TrimSpace(configValue(config, "rateJSONPath", "rateJsonPath", "rate_json_path"))
	if rateJSONPath == "" {
		rateJSONPath = defaultUSDTBSCRateJSONPath
	}
	rateCacheSecs := parseInt64Config(configValue(config, "rateCacheSeconds", "rate_cache_seconds"), defaultUSDTBSCRateCacheSecs)
	if rateCacheSecs < 0 {
		rateCacheSecs = 0
	}
	confirmations := parseInt64Config(configValue(config, "confirmations"), defaultUSDTBSCConfirmations)
	if confirmations <= 0 {
		confirmations = defaultUSDTBSCConfirmations
	}
	tokenContract := strings.TrimSpace(configValue(config, "tokenContract", "token_contract"))
	if tokenContract == "" {
		tokenContract = defaultUSDTBSCContract
	}
	if !evmAddressPattern.MatchString(tokenContract) {
		return nil, fmt.Errorf("usdt_bsc tokenContract must be an EVM address")
	}
	apiBase := strings.TrimSpace(configValue(config, "bscscanApiBase", "apiBase"))
	if apiBase == "" {
		apiBase = defaultBscScanAPIBase
	}
	rpcRaw := configValue(config, "rpcUrl", "bscRpcUrl", "rpc_url")
	rpcDisabled := rpcURLDisabled(rpcRaw)
	rpcURLs := parseRPCURLs(rpcRaw)
	if len(rpcURLs) == 0 && !rpcDisabled {
		rpcURLs = parseRPCURLs(defaultUSDTBSCRPCURLs)
	}
	return &USDTBSC{
		instanceID:           instanceID,
		receiveAddress:       checksumPreservingAddress(receiveAddress),
		tokenContract:        strings.ToLower(tokenContract),
		bscscanAPIBase:       strings.TrimRight(apiBase, "?"),
		bscscanAPIKey:        strings.TrimSpace(configValue(config, "bscscanApiKey", "apiKey")),
		rpcURLs:              rpcURLs,
		rpcDisabled:          rpcDisabled,
		cnyPerUSDT:           rate,
		rateMode:             rateMode,
		rateAPIURL:           rateAPIURL,
		rateJSONPath:         rateJSONPath,
		rateSource:           rateSource,
		rateFallbackToManual: rateFallbackToManual,
		rateCacheTTL:         time.Duration(rateCacheSecs) * time.Second,
		confirmations:        confirmations,
		httpClient:           &http.Client{Timeout: 12 * time.Second},
	}, nil
}

func (p *USDTBSC) Name() string { return "USDT-BSC" }

func (p *USDTBSC) ProviderKey() string { return payment.TypeUSDTBSC }

func (p *USDTBSC) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeUSDTBSC}
}

func (p *USDTBSC) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	cnyAmount, ok := new(big.Rat).SetString(strings.TrimSpace(req.Amount))
	if !ok || cnyAmount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid usdt_bsc payment amount")
	}
	rate, rateSource, err := p.resolveCNYPerUSDT(ctx, cnyAmount)
	if err != nil {
		return nil, err
	}
	base := new(big.Rat).Quo(cnyAmount, rate)
	baseAmount := ratToFixed(base, 6)
	amountWithTail := addUniqueMicroTail(baseAmount, req.OrderID, req.ReservedCryptoAmounts)
	lockedAt := time.Now().UTC()
	intent := encodeUSDTBSCIntent(req.OrderID, req.Amount, amountWithTail, p.receiveAddress, rate, lockedAt)
	if len(intent) > usdtBSCMaxIntentLength {
		return nil, fmt.Errorf("usdt_bsc locked payment intent is too long")
	}
	return &payment.CreatePaymentResponse{
		TradeNo:        intent,
		QRCode:         p.receiveAddress,
		Currency:       "USDT",
		CryptoAmount:   amountWithTail,
		CryptoCurrency: "USDT",
		CryptoNetwork:  "BSC",
		ReceiveAddress: p.receiveAddress,
		Metadata:       buildUSDTBSCCreateMetadata(req.Amount, amountWithTail, intent, rate, rateSource, lockedAt.UTC().Format(time.RFC3339)),
	}, nil
}

func buildUSDTBSCQueryMetadata(intent usdtBSCIntent, event bscScanTokenTx) map[string]string {
	metadata := map[string]string{
		"tx_hash":       event.Hash,
		"block_number":  event.BlockNumber,
		"from_address":  event.From,
		"to_address":    event.To,
		"token_amount":  intent.TokenAmount,
		"confirmations": event.Confirmations,
		"network":       "BSC",
	}
	if event.Timestamp != "" {
		metadata["block_timestamp"] = event.Timestamp
	}
	if intent.LockedRate != "" {
		metadata["locked_cny_per_usdt"] = intent.LockedRate
	}
	if intent.LockedAt != "" {
		metadata["locked_at"] = intent.LockedAt
	}
	return metadata
}

func (p *USDTBSC) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}

func (p *USDTBSC) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("usdt_bsc refund is not supported")
}

type usdtBSCIntent struct {
	OrderID        string
	CNYAmount      string
	TokenAmount    string
	ReceiveAddress string
	LockedRate     string
	LockedAt       string
}

func encodeUSDTBSCIntent(orderID, cnyAmount, tokenAmount, receiveAddress string, lockedRate *big.Rat, lockedAt time.Time) string {
	if lockedAt.IsZero() {
		lockedAt = time.Now().UTC()
	}
	return strings.Join([]string{
		usdtBSCIntentPrefix,
		strings.TrimSpace(orderID),
		strings.TrimSpace(cnyAmount),
		strings.TrimSpace(tokenAmount),
		strings.TrimSpace(receiveAddress),
		ratToFixed(lockedRate, 6),
		strconv.FormatInt(lockedAt.UTC().Unix(), 10),
	}, "|")
}

func parseUSDTBSCLockedAt(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("invalid usdt_bsc locked time")
	}
	if sec, err := strconv.ParseInt(raw, 10, 64); err == nil && sec > 0 {
		return time.Unix(sec, 0).UTC().Format(time.RFC3339), nil
	}
	lockedAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", fmt.Errorf("invalid usdt_bsc locked time")
	}
	return lockedAt.UTC().Format(time.RFC3339), nil
}

func parseUSDTBSCIntent(raw string) (usdtBSCIntent, error) {
	parts := strings.Split(strings.TrimSpace(raw), "|")
	if (len(parts) != 5 && len(parts) != 7) || parts[0] != usdtBSCIntentPrefix {
		return usdtBSCIntent{}, fmt.Errorf("invalid usdt_bsc intent")
	}
	if parts[1] == "" || parts[2] == "" || parts[3] == "" || !evmAddressPattern.MatchString(parts[4]) {
		return usdtBSCIntent{}, fmt.Errorf("invalid usdt_bsc intent fields")
	}
	intent := usdtBSCIntent{
		OrderID:        parts[1],
		CNYAmount:      parts[2],
		TokenAmount:    parts[3],
		ReceiveAddress: parts[4],
	}
	if len(parts) == 7 {
		if rate, ok := new(big.Rat).SetString(strings.TrimSpace(parts[5])); !ok || rate.Sign() <= 0 {
			return usdtBSCIntent{}, fmt.Errorf("invalid usdt_bsc locked rate")
		}
		lockedAt, err := parseUSDTBSCLockedAt(parts[6])
		if err != nil {
			return usdtBSCIntent{}, err
		}
		intent.LockedRate = strings.TrimSpace(parts[5])
		intent.LockedAt = lockedAt
	}
	return intent, nil
}

type bscScanTokenTxResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type bscScanTokenTx struct {
	BlockNumber     string `json:"blockNumber"`
	Hash            string `json:"hash"`
	From            string `json:"from"`
	To              string `json:"to"`
	ContractAddress string `json:"contractAddress"`
	Value           string `json:"value"`
	TokenDecimal    string `json:"tokenDecimal"`
	Confirmations   string `json:"confirmations"`
	Timestamp       string `json:"timeStamp"`
}

func buildUSDTBSCCreateMetadata(cnyAmount, tokenAmount, intent string, lockedRate *big.Rat, rateSource string, lockedAt string) map[string]string {
	metadata := map[string]string{
		"intent_trade_no":     intent,
		"cny_amount":          strings.TrimSpace(cnyAmount),
		"token_amount":        strings.TrimSpace(tokenAmount),
		"locked_cny_per_usdt": ratToFixed(lockedRate, 6),
		"locked_at":           lockedAt,
		"rate_source":         strings.TrimSpace(rateSource),
		"crypto_currency":     "USDT",
		"network":             "BSC",
	}
	return metadata
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcLog struct {
	Address          string   `json:"address"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	Removed          bool     `json:"removed"`
	TransactionIndex string   `json:"transactionIndex"`
	LogIndex         string   `json:"logIndex"`
}

func (p *USDTBSC) rpcCall(ctx context.Context, rpcURL string, method string, params any) (json.RawMessage, error) {
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query bsc rpc: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bsc rpc status %d", resp.StatusCode)
	}
	var parsed rpcResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode bsc rpc response: %w", err)
	}
	if parsed.Error != nil {
		return nil, parsed.Error
	}
	return parsed.Result, nil
}

// Error 让 RPC 业务错误（区块跨度超限、限流等）与网络错误区分开。
func (e *rpcError) Error() string {
	return fmt.Sprintf("bsc rpc error %d: %s", e.Code, e.Message)
}

func rpcLogToTokenTx(log rpcLog, latestBlock int64) (bscScanTokenTx, bool) {
	if len(log.Topics) < 3 {
		return bscScanTokenTx{}, false
	}
	blockNumber, err := parseHexInt64(log.BlockNumber)
	if err != nil {
		return bscScanTokenTx{}, false
	}
	value, ok := hexQuantityToDecimal(log.Data)
	if !ok {
		return bscScanTokenTx{}, false
	}
	confirmations := latestBlock - blockNumber + 1
	if confirmations < 0 {
		confirmations = 0
	}
	return bscScanTokenTx{
		BlockNumber:     strconv.FormatInt(blockNumber, 10),
		Hash:            log.TransactionHash,
		From:            topicToAddress(log.Topics[1]),
		To:              topicToAddress(log.Topics[2]),
		ContractAddress: log.Address,
		Value:           value,
		TokenDecimal:    strconv.Itoa(usdtBSCTokenDecimals),
		Confirmations:   strconv.FormatInt(confirmations, 10),
	}, true
}

func configValue(config map[string]string, keys ...string) string {
	for _, want := range keys {
		for got, value := range config {
			if strings.EqualFold(got, want) {
				return value
			}
		}
	}
	return ""
}

func normalizeUSDTBSCRateMode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "manual" || raw == "fixed" || raw == "static" {
		return "manual"
	}
	return "auto"
}

func parseBoolConfig(raw string, fallback bool) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	case "0", "false", "no", "n", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func validateHTTPURL(raw string, field string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must be a valid http(s) URL", field)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", field)
	}
	return nil
}

func parseInt64Config(raw string, fallback int64) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseRPCURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if rpcURLDisabled(raw) {
		return []string{}
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimRight(strings.TrimSpace(part), "/")
		if part != "" {
			urls = append(urls, part)
		}
	}
	return urls
}

func rpcURLDisabled(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.EqualFold(raw, "disabled") || strings.EqualFold(raw, "none") || raw == "-"
}

func ratToFixed(r *big.Rat, decimals int) string {
	f, _ := r.Float64()
	pow := math.Pow10(decimals)
	rounded := math.Round(f*pow) / pow
	return strconv.FormatFloat(rounded, 'f', decimals, 64)
}

// addUniqueMicroTail 在应付金额后加 0.000001~0.000899 的尾数，用来区分同金额订单。
// 尾数由订单号决定；与仍可能到账的其他订单撞上时顺延到下一个空闲尾数。
func addUniqueMicroTail(baseAmount, orderID string, reserved []string) string {
	raw, err := strconv.ParseFloat(baseAmount, 64)
	if err != nil {
		return baseAmount
	}
	taken := make(map[string]struct{}, len(reserved))
	for _, amount := range reserved {
		if r, ok := new(big.Rat).SetString(strings.TrimSpace(amount)); ok {
			taken[r.FloatString(6)] = struct{}{}
		}
	}
	hash := sha256.Sum256([]byte(orderID))
	start := binary.BigEndian.Uint64(hash[:8]) % usdtBSCMicroTailMax
	format := func(offset uint64) string {
		tail := (start+offset)%usdtBSCMicroTailMax + 1
		return strconv.FormatFloat(raw+float64(tail)/1_000_000, 'f', 6, 64)
	}
	for offset := uint64(0); offset < usdtBSCMicroTailMax; offset++ {
		amount := format(offset)
		if _, used := taken[amount]; !used {
			return amount
		}
	}
	return format(0)
}

// USDTBSCIntentTokenAmount 从订单的支付意图中取出应付 USDT 数量，供下单时避开重复尾数。
func USDTBSCIntentTokenAmount(tradeNo string) (string, bool) {
	intent, err := parseUSDTBSCIntent(tradeNo)
	if err != nil {
		return "", false
	}
	return intent.TokenAmount, true
}

func usdtBSCTransferInIntentWindow(rawTimestamp, lockedAt string, now time.Time) bool {
	chainSeconds, err := strconv.ParseInt(strings.TrimSpace(rawTimestamp), 10, 64)
	if err != nil || chainSeconds <= 0 {
		return false
	}
	locked, err := time.Parse(time.RFC3339, strings.TrimSpace(lockedAt))
	if err != nil {
		return false
	}
	chainAt := time.Unix(chainSeconds, 0).UTC()
	return !chainAt.Before(locked.UTC().Add(-usdtBSCChainClockSkew)) && !chainAt.After(now.UTC().Add(usdtBSCChainClockSkew))
}

func decimalToRaw(decimal string, decimals int64) (*big.Int, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(decimal))
	if !ok || r.Sign() < 0 {
		return nil, fmt.Errorf("invalid decimal amount")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	if !scaled.IsInt() {
		return nil, fmt.Errorf("decimal amount has too many digits")
	}
	return scaled.Num(), nil
}

func parseHexInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "0x"))
	if raw == "" {
		return 0, fmt.Errorf("empty hex")
	}
	return strconv.ParseInt(raw, 16, 64)
}

func int64ToHex(v int64) string {
	if v < 0 {
		v = 0
	}
	return "0x" + strconv.FormatInt(v, 16)
}

func evmAddressTopic(address string) string {
	address = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(address)), "0x")
	if len(address) > 40 {
		address = address[len(address)-40:]
	}
	return "0x" + strings.Repeat("0", 64-len(address)) + address
}

func topicToAddress(topic string) string {
	topic = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(topic)), "0x")
	if len(topic) < 40 {
		return ""
	}
	return "0x" + topic[len(topic)-40:]
}

func hexQuantityToDecimal(raw string) (string, bool) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "0x"))
	if raw == "" {
		return "", false
	}
	value := new(big.Int)
	if _, ok := value.SetString(raw, 16); !ok {
		return "", false
	}
	return value.String(), true
}

func checksumPreservingAddress(address string) string {
	if address == "" {
		return ""
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(address, "0x"))
	if err != nil || len(decoded) != 20 {
		return address
	}
	return "0x" + strings.TrimPrefix(address, "0x")
}
