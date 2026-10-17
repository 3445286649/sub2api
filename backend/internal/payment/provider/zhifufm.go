package provider

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// ZhifuFM uses FM's EasyPay-compatible checkout and signed notifications,
// but verifies payments with FM's native order-query protocol.
type ZhifuFM struct {
	*EasyPay
	queryBase string
}

type FMQueryError struct {
	Retryable bool
	Code      int
}

func (e *FMQueryError) Error() string { return fmt.Sprintf("FM query rejected (code %d)", e.Code) }

func NewZhifuFM(instanceID string, config map[string]string) (*ZhifuFM, error) {
	for _, key := range []string{"apiBase", "queryBase", "notifyUrl", "returnUrl"} {
		u, err := url.Parse(strings.TrimSpace(config[key]))
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("FM %s must be an HTTPS URL without credentials, query or fragment", key)
		}
	}
	if pid := strings.TrimSpace(config["pid"]); pid == "" || len(pid) > 100 {
		return nil, errors.New("invalid FM merchant number")
	}
	if v := config["verifyAfterCallback"]; v != "" && v != "true" && v != "false" {
		return nil, errors.New("invalid FM verification setting")
	}
	ep, err := NewEasyPay(instanceID, config)
	if err != nil {
		return nil, err
	}
	ep.httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &ZhifuFM{EasyPay: ep, queryBase: strings.TrimRight(strings.TrimSpace(config["queryBase"]), "/")}, nil
}
func (f *ZhifuFM) Name() string              { return "支付FM" }
func (f *ZhifuFM) ProviderKey() string       { return payment.ProviderZhifuFM }
func (f *ZhifuFM) VerifyAfterCallback() bool { return f.config["verifyAfterCallback"] != "false" }

func (f *ZhifuFM) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	result, err := f.EasyPay.CreatePayment(ctx, req)
	if err != nil {
		return nil, err
	}
	// Some FM channels return only a hosted checkout URL from mapi.php.
	// Keep QR mode on our checkout by encoding that URL when no QR is supplied.
	if f.config["paymentMode"] != paymentModePopup && result.QRCode == "" {
		result.QRCode = result.PayURL
	}
	return result, nil
}

func (f *ZhifuFM) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, errors.New("FM refunds are not supported")
}

func fmQuerySign(merchant, order, key string) string {
	sum := md5.Sum([]byte(merchant + order + key))
	return hex.EncodeToString(sum[:])
}

func (f *ZhifuFM) QueryOrder(ctx context.Context, orderNo string) (*payment.QueryOrderResponse, error) {
	if strings.TrimSpace(orderNo) == "" || len(orderNo) > 64 {
		return nil, errors.New("invalid FM order number")
	}
	merchant := strings.TrimSpace(f.config["pid"])
	query := url.Values{"merchantNum": {merchant}, "orderNo": {orderNo}, "sign": {fmQuerySign(merchant, orderNo, f.config["pkey"])}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.queryBase+"/queryOutOrder?"+query.Encode(), nil)
	if err != nil {
		return nil, errors.New("unable to create FM query request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, &FMQueryError{Retryable: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &FMQueryError{Retryable: resp.StatusCode == 429 || resp.StatusCode >= 500, Code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxEasypayResponseSize+1))
	if err != nil {
		return nil, &FMQueryError{Retryable: true}
	}
	if len(body) > maxEasypayResponseSize {
		return nil, errors.New("FM query response exceeds limit")
	}
	var result struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    struct {
			Merchant string          `json:"merchantNum"`
			OrderNo  string          `json:"orderNo"`
			OrderID  string          `json:"orderId"`
			Amount   string          `json:"amount"`
			State    json.RawMessage `json:"orderState"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil {
		return nil, errors.New("invalid FM query response")
	}
	if !result.Success || result.Code != 200 {
		return nil, &FMQueryError{Retryable: result.Code == 429 || result.Code >= 500, Code: result.Code}
	}
	d := result.Data
	if d.Merchant != merchant || d.OrderNo != orderNo || strings.TrimSpace(d.OrderID) == "" {
		return nil, errors.New("FM query order or merchant mismatch")
	}
	amount, err := strconv.ParseFloat(d.Amount, 64)
	if err != nil || !validEasyPayAmount(amount) {
		return nil, errors.New("invalid FM query amount")
	}
	state := strings.Trim(string(d.State), "\"")
	status := payment.ProviderStatusPending
	switch state {
	case "4":
		status = payment.ProviderStatusPaid
	case "2":
	case "7":
		status = payment.ProviderStatusFailed
	default:
		return nil, errors.New("unknown FM order state")
	}
	return &payment.QueryOrderResponse{TradeNo: d.OrderID, Status: status, Amount: amount, Metadata: map[string]string{"pid": d.Merchant, "out_trade_no": d.OrderNo}}, nil
}
