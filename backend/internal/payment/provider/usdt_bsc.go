package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	defaultUSDTBSCConfirmations = 20
	usdtBSCIntentPrefix         = "usdt_bsc"
	usdtBSCTokenDecimals        = 18
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
	return &USDTBSC{
		instanceID:     instanceID,
		receiveAddress: checksumPreservingAddress(receiveAddress),
		tokenContract:  strings.ToLower(tokenContract),
		bscscanAPIBase: strings.TrimRight(apiBase, "?"),
		bscscanAPIKey:  strings.TrimSpace(configValue(config, "bscscanApiKey", "apiKey")),
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
	events, err := p.fetchTokenTransfers(ctx, intent.ReceiveAddress)
	if err != nil {
		return nil, err
	}
	expectedRaw, err := decimalToRaw(intent.TokenAmount, usdtBSCTokenDecimals)
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
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  []bscScanTokenTx `json:"result"`
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

func (p *USDTBSC) fetchTokenTransfers(ctx context.Context, address string) ([]bscScanTokenTx, error) {
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
	return parsed.Result, nil
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
