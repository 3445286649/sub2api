package provider

import (
	"context"
	"crypto/sha256"
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
	defaultUSDTBSCRPCURL        = "https://1rpc.io/bnb"
	defaultUSDTBSCConfirmations = 20
	usdtBSCIntentPrefix         = "usdt_bsc"
	usdtBSCTokenDecimals        = 18
	usdtBSCTransferTopic        = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	usdtBSCDefaultLookback      = int64(3600)
	usdtBSCRPCWindowBlocks      = int64(50)
)

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// USDTBSC implements fixed-address USDT-BEP20 balance recharge.
// It never holds private keys; it only creates an expected transfer intent and
// queries public chain explorer data to confirm incoming token transfers.
type USDTBSC struct {
	instanceID     string
	receiveAddress string
	tokenContract  string
	bscscanAPIBase string
	bscscanAPIKey  string
	rpcURLs        []string
	rpcDisabled    bool
	cnyPerUSDT     *big.Rat
	confirmations  int64
	httpClient     *http.Client
}

func NewUSDTBSC(instanceID string, config map[string]string) (*USDTBSC, error) {
	if config == nil {
		config = map[string]string{}
	}
	receiveAddress := strings.TrimSpace(configValue(config, "receiveAddress", "receive_address"))
	if !evmAddressPattern.MatchString(receiveAddress) {
		return nil, fmt.Errorf("usdt_bsc receiveAddress is required and must be an EVM address")
	}
	rateRaw := strings.TrimSpace(configValue(config, "cnyPerUsdt", "cny_per_usdt", "manualCnyPerUsdt"))
	if rateRaw == "" {
		return nil, fmt.Errorf("usdt_bsc cnyPerUsdt is required")
	}
	rate, ok := new(big.Rat).SetString(rateRaw)
	if !ok || rate.Sign() <= 0 {
		return nil, fmt.Errorf("usdt_bsc cnyPerUsdt must be a positive number")
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
		rpcURLs = []string{defaultUSDTBSCRPCURL}
	}
	return &USDTBSC{
		instanceID:     instanceID,
		receiveAddress: checksumPreservingAddress(receiveAddress),
		tokenContract:  strings.ToLower(tokenContract),
		bscscanAPIBase: strings.TrimRight(apiBase, "?"),
		bscscanAPIKey:  strings.TrimSpace(configValue(config, "bscscanApiKey", "apiKey")),
		rpcURLs:        rpcURLs,
		rpcDisabled:    rpcDisabled,
		cnyPerUSDT:     rate,
		confirmations:  confirmations,
		httpClient:     &http.Client{Timeout: 12 * time.Second},
	}, nil
}

func (p *USDTBSC) Name() string { return "USDT-BSC" }

func (p *USDTBSC) ProviderKey() string { return payment.TypeUSDTBSC }

func (p *USDTBSC) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeUSDTBSC}
}

func (p *USDTBSC) CreatePayment(_ context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	cnyAmount, ok := new(big.Rat).SetString(strings.TrimSpace(req.Amount))
	if !ok || cnyAmount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid usdt_bsc payment amount")
	}
	base := new(big.Rat).Quo(cnyAmount, p.cnyPerUSDT)
	baseAmount := ratToFixed(base, 6)
	amountWithTail := addUniqueMicroTail(baseAmount, req.OrderID)
	intent := encodeUSDTBSCIntent(req.OrderID, req.Amount, amountWithTail, p.receiveAddress)
	return &payment.CreatePaymentResponse{
		TradeNo:        intent,
		QRCode:         p.receiveAddress,
		Currency:       "USDT",
		CryptoAmount:   amountWithTail,
		CryptoCurrency: "USDT",
		CryptoNetwork:  "BSC",
		ReceiveAddress: p.receiveAddress,
	}, nil
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
	events, err := p.fetchTokenTransfers(ctx, intent.ReceiveAddress, expectedRaw)
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
		confirmations, _ := strconv.ParseInt(strings.TrimSpace(event.Confirmations), 10, 64)
		if confirmations < p.confirmations {
			return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
		}
		cny, _ := strconv.ParseFloat(intent.CNYAmount, 64)
		return &payment.QueryOrderResponse{
			TradeNo: event.Hash,
			Status:  payment.ProviderStatusPaid,
			Amount:  cny,
			Metadata: map[string]string{
				"tx_hash":       event.Hash,
				"block_number":  event.BlockNumber,
				"from_address":  event.From,
				"to_address":    event.To,
				"token_amount":  intent.TokenAmount,
				"confirmations": event.Confirmations,
				"network":       "BSC",
			},
		}, nil
	}
	return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
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
}

func encodeUSDTBSCIntent(orderID, cnyAmount, tokenAmount, receiveAddress string) string {
	return strings.Join([]string{
		usdtBSCIntentPrefix,
		strings.TrimSpace(orderID),
		strings.TrimSpace(cnyAmount),
		strings.TrimSpace(tokenAmount),
		strings.TrimSpace(receiveAddress),
	}, "|")
}

func parseUSDTBSCIntent(raw string) (usdtBSCIntent, error) {
	parts := strings.Split(strings.TrimSpace(raw), "|")
	if len(parts) != 5 || parts[0] != usdtBSCIntentPrefix {
		return usdtBSCIntent{}, fmt.Errorf("invalid usdt_bsc intent")
	}
	if parts[1] == "" || parts[2] == "" || parts[3] == "" || !evmAddressPattern.MatchString(parts[4]) {
		return usdtBSCIntent{}, fmt.Errorf("invalid usdt_bsc intent fields")
	}
	return usdtBSCIntent{
		OrderID:        parts[1],
		CNYAmount:      parts[2],
		TokenAmount:    parts[3],
		ReceiveAddress: parts[4],
	}, nil
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
}

func (p *USDTBSC) fetchTokenTransfers(ctx context.Context, address string, expectedRaw *big.Int) ([]bscScanTokenTx, error) {
	if len(p.rpcURLs) > 0 && !p.rpcDisabled {
		var lastErr error
		for _, rpcURL := range p.rpcURLs {
			events, err := p.fetchTokenTransfersByRPC(ctx, rpcURL, address, expectedRaw)
			if err == nil {
				return events, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
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
	defer resp.Body.Close()
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

func (p *USDTBSC) fetchTokenTransfersByRPC(ctx context.Context, rpcURL string, address string, expectedRaw *big.Int) ([]bscScanTokenTx, error) {
	latestHex, err := p.rpcCall(ctx, rpcURL, "eth_blockNumber", []any{})
	if err != nil {
		return nil, err
	}
	var latestNumber string
	if err := json.Unmarshal(latestHex, &latestNumber); err != nil {
		return nil, fmt.Errorf("decode rpc block number: %w", err)
	}
	latest, err := parseHexInt64(latestNumber)
	if err != nil {
		return nil, fmt.Errorf("parse rpc block number: %w", err)
	}

	toTopic := evmAddressTopic(address)
	events := make([]bscScanTokenTx, 0)
	start := latest - usdtBSCDefaultLookback
	if start < 0 {
		start = 0
	}
	for end := latest; end >= start; end -= usdtBSCRPCWindowBlocks {
		from := end - usdtBSCRPCWindowBlocks + 1
		if from < start {
			from = start
		}
		rawLogs, err := p.rpcCall(ctx, rpcURL, "eth_getLogs", []any{map[string]any{
			"address":   p.tokenContract,
			"fromBlock": int64ToHex(from),
			"toBlock":   int64ToHex(end),
			"topics":    []any{usdtBSCTransferTopic, nil, toTopic},
		}})
		if err != nil {
			return nil, err
		}
		var logs []rpcLog
		if err := json.Unmarshal(rawLogs, &logs); err != nil {
			return nil, fmt.Errorf("decode rpc logs: %w", err)
		}
		for _, log := range logs {
			if log.Removed {
				continue
			}
			event, ok := rpcLogToTokenTx(log, latest)
			if !ok {
				continue
			}
			events = append(events, event)
			if expectedRaw != nil && strings.TrimSpace(event.Value) == expectedRaw.String() {
				return events, nil
			}
		}
		if from == 0 {
			break
		}
	}
	return events, nil
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
		return nil, fmt.Errorf("bsc rpc error %d: %s", parsed.Error.Code, parsed.Error.Message)
	}
	return parsed.Result, nil
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

func addUniqueMicroTail(baseAmount, orderID string) string {
	raw, err := strconv.ParseFloat(baseAmount, 64)
	if err != nil {
		return baseAmount
	}
	hash := sha256.Sum256([]byte(orderID))
	tail := int(hash[0])%899 + 1
	amount := raw + float64(tail)/1_000_000
	return strconv.FormatFloat(amount, 'f', 6, 64)
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
