package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/google/uuid"
)

const BepusdtWebhookPath = "/api/v1/payment/webhook/bepusdt"

type BepusdtCreateRequest struct {
	OutTradeNo   string `json:"out_trade_no"`
	FiatAmount   string `json:"fiat_amount"`
	FiatCurrency string `json:"fiat_currency"`
	Asset        string `json:"asset"`
	Network      string `json:"network"`
	ExpiresAt    string `json:"expires_at"`
}

type BepusdtPayment struct {
	TxHash        string `json:"tx_hash"`
	EventIndex    int64  `json:"event_index"`
	BlockNumber   int64  `json:"block_number"`
	BlockHash     string `json:"block_hash"`
	BlockTime     string `json:"block_time"`
	FromAddress   string `json:"from_address"`
	ToAddress     string `json:"to_address"`
	AmountAtomic  string `json:"amount_atomic"`
	Confirmations int    `json:"confirmations"`
	Confirmed     bool   `json:"confirmed"`
}

type BepusdtOrder struct {
	ProtocolVersion       string          `json:"protocol_version"`
	MerchantID            string          `json:"merchant_id"`
	OutTradeNo            string          `json:"out_trade_no"`
	GatewayOrderID        string          `json:"gateway_order_id"`
	RequestDigest         string          `json:"request_digest"`
	OrderVersion          int64           `json:"order_version"`
	Status                string          `json:"status"`
	FiatAmount            string          `json:"fiat_amount"`
	FiatCurrency          string          `json:"fiat_currency"`
	Asset                 string          `json:"asset"`
	Network               string          `json:"network"`
	TokenContract         string          `json:"token_contract"`
	TokenDecimals         int             `json:"token_decimals"`
	AmountAtomic          string          `json:"amount_atomic"`
	CryptoAmount          string          `json:"crypto_amount"`
	ReceiveAddress        string          `json:"receive_address"`
	LockedRate            string          `json:"locked_rate"`
	Rounding              string          `json:"rounding"`
	QuoteVersion          int             `json:"quote_version"`
	CreatedAt             string          `json:"created_at"`
	ExpiresAt             string          `json:"expires_at"`
	RequiredConfirmations int             `json:"required_confirmations"`
	Payment               *BepusdtPayment `json:"payment,omitempty"`
}

type BepusdtEvent struct {
	EventID string       `json:"event_id"`
	Order   BepusdtOrder `json:"order"`
}

type BepusdtAPIError struct {
	Status int
	Code   string
}

func (e *BepusdtAPIError) Error() string {
	return fmt.Sprintf("bepusdt returned HTTP %d (%s)", e.Status, e.Code)
}
func BepusdtDefinitiveError(err error) bool {
	var upstream *BepusdtAPIError
	return errors.As(err, &upstream) && upstream.Status == 400
}

type Bepusdt struct {
	baseURL                                      *url.URL
	merchantID, keyID, apiSecret, callbackSecret string
	client                                       *http.Client
}

func BepusdtNetworks(config map[string]string) []string {
	var result []string
	for _, network := range strings.Split(config["allowedNetworks"], ",") {
		network = strings.TrimSpace(network)
		if network != "bsc_mainnet" && network != "tron_mainnet" {
			continue
		}
		found := false
		for _, existing := range result {
			if existing == network {
				found = true
			}
		}
		if !found {
			result = append(result, network)
		}
	}
	return result
}

func NewBepusdt(_ string, config map[string]string) (*Bepusdt, error) {
	u, err := url.Parse(strings.TrimSpace(config["apiUrl"]))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) {
		return nil, fmt.Errorf("bepusdt apiUrl must be an HTTPS origin without credentials, query or fragment")
	}
	p := &Bepusdt{baseURL: u, merchantID: strings.TrimSpace(config["merchantId"]), keyID: strings.TrimSpace(config["keyId"]), apiSecret: config["apiSecret"], callbackSecret: config["callbackSecret"]}
	if p.merchantID == "" || p.keyID == "" || len(p.apiSecret) < 32 || len(p.callbackSecret) < 32 || p.apiSecret == p.callbackSecret {
		return nil, fmt.Errorf("bepusdt requires merchant/key IDs and distinct secrets of at least 32 bytes")
	}
	if !bepusdtIDPattern.MatchString(p.merchantID) || !bepusdtIDPattern.MatchString(p.keyID) {
		return nil, errors.New("bepusdt merchant/key IDs must contain only unreserved ASCII")
	}
	if len(BepusdtNetworks(config)) == 0 {
		return nil, fmt.Errorf("bepusdt requires an allowed mainnet")
	}
	for _, network := range strings.Split(config["allowedNetworks"], ",") {
		network = strings.TrimSpace(network)
		if network != "" && network != "bsc_mainnet" && network != "tron_mainnet" {
			return nil, fmt.Errorf("unsupported bepusdt network")
		}
	}
	p.client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("bepusdt redirects are disabled") }}
	return p, nil
}

func (*Bepusdt) Name() string        { return "BEpusdt" }
func (*Bepusdt) ProviderKey() string { return payment.ProviderBepusdt }
func (*Bepusdt) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeUSDTGateway}
}
func (p *Bepusdt) MerchantIdentityMetadata() map[string]string {
	return map[string]string{"merchant_id": p.merchantID, "currency": "CNY"}
}

func BepusdtSignature(secret, merchant, keyID, method, path, timestamp, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	canonical := strings.Join([]string{"2", merchant, keyID, strings.ToUpper(method), path, timestamp, nonce, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func BepusdtRequestDigest(merchant string, req BepusdtCreateRequest) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{merchant, req.OutTradeNo, req.FiatAmount, req.FiatCurrency, req.Asset, req.Network, req.ExpiresAt}, "\n")))
	return hex.EncodeToString(digest[:])
}

func (p *Bepusdt) callRaw(ctx context.Context, method, path string, input any) ([]byte, error) {
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			return nil, err
		}
	}
	u := *p.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	ts, nonce := strconv.FormatInt(time.Now().Unix(), 10), uuid.NewString()
	req.Header.Set("Content-Type", "application/json")
	for k, v := range map[string]string{"X-Merchant-Id": p.merchantID, "X-Key-Id": p.keyID, "X-Timestamp": ts, "X-Nonce": nonce, "X-Signature": BepusdtSignature(p.apiSecret, p.merchantID, p.keyID, method, u.EscapedPath(), ts, nonce, body)} {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bepusdt request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, errors.New("bepusdt response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var response struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &response)
		return nil, &BepusdtAPIError{Status: resp.StatusCode, Code: response.Error.Code}
	}
	return raw, nil
}

func (p *Bepusdt) call(ctx context.Context, method, path string, input any) (*BepusdtOrder, error) {
	raw, err := p.callRaw(ctx, method, path, input)
	if err != nil {
		return nil, err
	}
	var order BepusdtOrder
	if err = json.Unmarshal(raw, &order); err != nil {
		return nil, fmt.Errorf("decode bepusdt order: %w", err)
	}
	if err = ValidateBepusdtOrder(&order); err != nil {
		return nil, err
	}
	if order.MerchantID != p.merchantID {
		return nil, errors.New("bepusdt merchant mismatch")
	}
	return &order, nil
}

func (p *Bepusdt) Health(ctx context.Context) (map[string]any, error) {
	raw, err := p.callRaw(ctx, http.MethodGet, "/api/v2/merchant/health", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if result["protocol_version"] != "2" || result["merchant_id"] != p.merchantID {
		return nil, errors.New("invalid gateway health identity")
	}
	return result, nil
}

var bepusdtIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var bepusdtDecimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var bepusdtAtomicPattern = regexp.MustCompile(`^[1-9][0-9]{0,77}$`)
var bepusdtEVMPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var cryptoHashPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
var bepusdtTronPattern = regexp.MustCompile(`^T[1-9A-HJ-NP-Za-km-z]{33}$`)

func BepusdtAsset(network string) (string, int, error) {
	switch network {
	case "bsc_mainnet":
		return "0x55d398326f99059ff775485246999027b3197955", 18, nil
	case "tron_mainnet":
		return "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 6, nil
	default:
		return "", 0, errors.New("unsupported bepusdt network")
	}
}

func BepusdtAmountAtomic(amount string, decimals int) (string, error) {
	if len(amount) > 100 || !bepusdtDecimalPattern.MatchString(amount) || decimals < 0 || decimals > 18 {
		return "", errors.New("invalid crypto amount")
	}
	parts := strings.SplitN(amount, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > decimals {
		return "", errors.New("crypto amount exceeds token precision")
	}
	value := new(big.Int)
	value.SetString(parts[0]+fraction+strings.Repeat("0", decimals-len(fraction)), 10)
	if value.Sign() <= 0 {
		return "", errors.New("crypto amount must be positive")
	}
	return value.String(), nil
}

func ValidateBepusdtOrder(order *BepusdtOrder) error {
	if order == nil || order.ProtocolVersion != "2" || order.MerchantID == "" || !bepusdtIDPattern.MatchString(order.OutTradeNo) || order.GatewayOrderID == "" || len(order.RequestDigest) != 64 || order.OrderVersion <= 0 || order.FiatCurrency != "CNY" || order.Asset != "USDT" || order.QuoteVersion != 1 || order.Rounding != "ceil_6dp" || order.RequiredConfirmations <= 0 {
		return errors.New("invalid bepusdt order identity or quote")
	}
	if len(order.FiatAmount) > 64 || !regexp.MustCompile(`^[0-9]+\.[0-9]{2}$`).MatchString(order.FiatAmount) {
		return errors.New("invalid bepusdt fiat amount")
	}
	contract, decimals, err := BepusdtAsset(order.Network)
	if err != nil {
		return err
	}
	minConfirmations := 20
	if order.Network == "tron_mainnet" {
		minConfirmations = 30
	}
	if order.RequiredConfirmations < minConfirmations {
		return errors.New("bepusdt required confirmations are below the safety floor")
	}
	if order.TokenContract != contract && !(order.Network == "bsc_mainnet" && strings.EqualFold(order.TokenContract, contract)) {
		return errors.New("bepusdt token contract mismatch")
	}
	if order.TokenDecimals != decimals {
		return errors.New("bepusdt token precision mismatch")
	}
	if (order.Network == "bsc_mainnet" && !bepusdtEVMPattern.MatchString(order.ReceiveAddress)) || (order.Network == "tron_mainnet" && !bepusdtTronPattern.MatchString(order.ReceiveAddress)) {
		return errors.New("invalid bepusdt receive address")
	}
	atomic, err := BepusdtAmountAtomic(order.CryptoAmount, decimals)
	if err != nil || atomic != order.AmountAtomic || !bepusdtAtomicPattern.MatchString(order.AmountAtomic) {
		return errors.New("bepusdt quote amount mismatch")
	}
	if len(order.LockedRate) > 64 || !bepusdtDecimalPattern.MatchString(order.LockedRate) {
		return errors.New("invalid bepusdt exchange rate")
	}
	if rate, ok := new(big.Rat).SetString(order.LockedRate); !ok || rate.Sign() <= 0 {
		return errors.New("invalid bepusdt exchange rate")
	}
	if err := validateBepusdtQuoteArithmetic(order); err != nil {
		return err
	}
	created, e1 := time.Parse(time.RFC3339, order.CreatedAt)
	expires, e2 := time.Parse(time.RFC3339, order.ExpiresAt)
	if e1 != nil || e2 != nil || !expires.After(created) {
		return errors.New("invalid bepusdt order times")
	}
	switch order.Status {
	case "WAITING", "CONFIRMING", "PAID", "EXPIRED", "CANCELLED", "REVIEW":
	default:
		return errors.New("invalid bepusdt status")
	}
	if order.Status == "PAID" {
		proof := order.Payment
		if proof == nil || !proof.Confirmed || proof.Confirmations < order.RequiredConfirmations || proof.EventIndex < 0 || proof.BlockNumber <= 0 || proof.AmountAtomic != order.AmountAtomic || !bepusdtSameAddress(order.Network, proof.ToAddress, order.ReceiveAddress) {
			return errors.New("bepusdt payment proof mismatch")
		}
		blockTime, err := time.Parse(time.RFC3339, proof.BlockTime)
		if err != nil || blockTime.Before(created) || !blockTime.Before(expires) {
			return errors.New("bepusdt payment is outside quote window")
		}
		if order.Network == "bsc_mainnet" {
			if !cryptoHashPattern.MatchString(proof.TxHash) || !cryptoHashPattern.MatchString(proof.BlockHash) {
				return errors.New("invalid bepusdt BSC hash")
			}
		} else {
			if !regexp.MustCompile(`^[0-9a-fA-F]{64}$`).MatchString(proof.TxHash) || !regexp.MustCompile(`^[0-9a-fA-F]{64}$`).MatchString(proof.BlockHash) {
				return errors.New("invalid bepusdt TRON hash")
			}
		}
	}
	return nil
}

func validateBepusdtQuoteArithmetic(order *BepusdtOrder) error {
	fiat, ok := new(big.Rat).SetString(order.FiatAmount)
	if !ok || fiat.Sign() <= 0 || len(order.FiatAmount) > 64 {
		return errors.New("invalid positive gateway fiat amount")
	}
	rate, ok := new(big.Rat).SetString(order.LockedRate)
	if !ok || rate.Sign() <= 0 || len(order.LockedRate) > 64 {
		return errors.New("invalid gateway rate")
	}
	baseRat := new(big.Rat).Mul(fiat, new(big.Rat).SetInt64(1_000_000))
	baseRat.Quo(baseRat, rate)
	base, remainder := new(big.Int), new(big.Int)
	base.QuoRem(baseRat.Num(), baseRat.Denom(), remainder)
	if remainder.Sign() > 0 {
		base.Add(base, big.NewInt(1))
	}
	atomic, ok := new(big.Int).SetString(order.AmountAtomic, 10)
	if !ok {
		return errors.New("invalid gateway atomic amount")
	}
	shift := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(order.TokenDecimals-6)), nil)
	micro, residue := new(big.Int), new(big.Int)
	micro.QuoRem(atomic, shift, residue)
	if residue.Sign() != 0 {
		return errors.New("gateway quote must use six decimal USDT precision")
	}
	tail := new(big.Int).Sub(micro, base)
	if tail.Sign() <= 0 || tail.Cmp(big.NewInt(899)) > 0 {
		return errors.New("gateway quote tail exceeds the fixed allocation range")
	}
	return nil
}

func bepusdtSameAddress(network, a, b string) bool {
	if network == "bsc_mainnet" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
func bepusdtMetadata(order *BepusdtOrder) map[string]string {
	raw, _ := json.Marshal(order)
	return map[string]string{"bepusdt_order": string(raw), "merchant_id": order.MerchantID, "currency": "CNY"}
}

func (p *Bepusdt) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	if !bepusdtIDPattern.MatchString(req.OrderID) {
		return nil, errors.New("invalid bepusdt order reference")
	}
	input := BepusdtCreateRequest{req.OrderID, req.Amount, "CNY", "USDT", req.CryptoNetwork, req.ExpiresAt}
	order, err := p.call(ctx, http.MethodPost, "/api/v2/merchant/orders", input)
	if err != nil {
		return nil, err
	}
	if order.OutTradeNo != input.OutTradeNo || order.RequestDigest != BepusdtRequestDigest(p.merchantID, input) {
		return nil, errors.New("bepusdt create intent mismatch")
	}
	return &payment.CreatePaymentResponse{TradeNo: order.GatewayOrderID, Currency: "CNY", CryptoCurrency: "USDT", CryptoNetwork: order.Network, CryptoAmount: order.CryptoAmount, ReceiveAddress: order.ReceiveAddress, QRCode: order.ReceiveAddress, Metadata: bepusdtMetadata(order)}, nil
}

func (p *Bepusdt) Query(ctx context.Context, outTradeNo string) (*BepusdtOrder, error) {
	if !bepusdtIDPattern.MatchString(outTradeNo) {
		return nil, errors.New("invalid bepusdt order reference")
	}
	order, err := p.call(ctx, http.MethodGet, "/api/v2/merchant/orders/"+outTradeNo, nil)
	if err == nil && order.OutTradeNo != outTradeNo {
		return nil, errors.New("bepusdt query order mismatch")
	}
	return order, err
}
func (p *Bepusdt) QueryOrder(ctx context.Context, outTradeNo string) (*payment.QueryOrderResponse, error) {
	order, err := p.Query(ctx, outTradeNo)
	if err != nil {
		return nil, err
	}
	amount, _ := strconv.ParseFloat(order.FiatAmount, 64)
	status := payment.ProviderStatusPending
	hash := ""
	paidAt := ""
	if order.Status == "PAID" {
		status = payment.ProviderStatusPaid
		hash = order.Payment.TxHash
		paidAt = order.Payment.BlockTime
	}
	return &payment.QueryOrderResponse{TradeNo: hash, Status: status, Amount: amount, PaidAt: paidAt, Metadata: bepusdtMetadata(order)}, nil
}
func (p *Bepusdt) CancelPayment(ctx context.Context, outTradeNo string) error {
	if !bepusdtIDPattern.MatchString(outTradeNo) {
		return errors.New("invalid bepusdt order reference")
	}
	_, err := p.call(ctx, http.MethodPost, "/api/v2/merchant/orders/"+outTradeNo+"/cancel", nil)
	return err
}
func (*Bepusdt) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, errors.New("bepusdt refunds are not supported")
}

func (p *Bepusdt) VerifyEvent(rawBody string, headers map[string]string) (*BepusdtEvent, error) {
	ts, err := strconv.ParseInt(headers["x-timestamp"], 10, 64)
	if err != nil || ts < time.Now().Unix()-300 || ts > time.Now().Unix()+300 {
		return nil, errors.New("invalid bepusdt callback timestamp")
	}
	if headers["x-merchant-id"] != p.merchantID || headers["x-key-id"] != p.keyID || len(headers["x-nonce"]) < 16 || len(headers["x-nonce"]) > 128 {
		return nil, errors.New("invalid bepusdt callback identity")
	}
	path := headers["request-path"]
	if path == "" {
		path = BepusdtWebhookPath
	}
	signature := BepusdtSignature(p.callbackSecret, p.merchantID, p.keyID, http.MethodPost, path, headers["x-timestamp"], headers["x-nonce"], []byte(rawBody))
	if !hmac.Equal([]byte(signature), []byte(headers["x-signature"])) {
		return nil, errors.New("invalid bepusdt callback signature")
	}
	var event BepusdtEvent
	if err = json.Unmarshal([]byte(rawBody), &event); err != nil {
		return nil, err
	}
	if event.EventID == "" || len(event.EventID) > 128 || event.Order.MerchantID != p.merchantID || event.Order.Status != "PAID" {
		return nil, errors.New("invalid bepusdt paid event")
	}
	if err = ValidateBepusdtOrder(&event.Order); err != nil {
		return nil, err
	}
	return &event, nil
}
func (p *Bepusdt) VerifyNotification(_ context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	event, err := p.VerifyEvent(rawBody, headers)
	if err != nil {
		return nil, err
	}
	amount, _ := strconv.ParseFloat(event.Order.FiatAmount, 64)
	return &payment.PaymentNotification{OrderID: event.Order.OutTradeNo, TradeNo: event.Order.Payment.TxHash, Amount: amount, Status: payment.NotificationStatusSuccess, Metadata: bepusdtMetadata(&event.Order)}, nil
}
