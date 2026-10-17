package provider

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const jianPayDefaultGateway = "https://jpay.hzjianban.com"

// JianPay uses the native JSON cashier API, not the EasyPay protocol.
type JianPay struct {
	apiBase, clientNo, merchantKey, notifyURL, returnURL string
	httpClient                                           *http.Client
}

func NewJianPay(_ string, config map[string]string) (*JianPay, error) {
	if value := config["verifyAfterCallback"]; value != "" && value != "true" && value != "false" {
		return nil, errors.New("invalid JianPay verification setting")
	}
	p := &JianPay{apiBase: strings.TrimRight(strings.TrimSpace(config["apiBase"]), "/"), clientNo: strings.TrimSpace(config["clientNo"]), merchantKey: config["merchantKey"], notifyURL: strings.TrimSpace(config["notifyUrl"]), returnURL: strings.TrimSpace(config["returnUrl"])}
	if p.apiBase == "" {
		p.apiBase = jianPayDefaultGateway
	}
	if p.clientNo == "" || len(p.clientNo) > 128 || strings.TrimSpace(p.merchantKey) == "" {
		return nil, errors.New("JianPay merchant number and key are required")
	}
	for _, value := range []string{p.apiBase, p.notifyURL, p.returnURL} {
		if !jianPayHTTPSURL(value) {
			return nil, errors.New("JianPay gateway and callback URLs must use HTTPS without credentials")
		}
	}
	u, _ := url.Parse(p.apiBase)
	if u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("JianPay gateway must be an HTTPS origin")
	}
	p.httpClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return p, nil
}

func jianPayHTTPSURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}

func (*JianPay) Name() string        { return "简付" }
func (*JianPay) ProviderKey() string { return payment.ProviderJianPay }
func (*JianPay) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay}
}
func (p *JianPay) MerchantIdentityMetadata() map[string]string {
	return map[string]string{"clientNo": p.clientNo}
}
func (*JianPay) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, errors.New("JianPay refunds are not enabled in this integration")
}

func jianPaySign(params map[string]any, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k != "sign" && k != "sign_type" && v != nil && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		var value string
		switch v := params[k].(type) {
		case string:
			value = v
		case json.Number:
			value = v.String()
		default:
			b, _ := json.Marshal(v)
			value = string(b)
		}
		parts = append(parts, k+"="+value)
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

func jianPayAmountCents(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid JianPay amount")
	}
	for _, part := range parts {
		if part == "" {
			return 0, errors.New("invalid JianPay amount")
		}
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return 0, errors.New("invalid JianPay amount")
			}
		}
	}
	frac := "00"
	if len(parts) == 2 {
		if len(parts[1]) > 2 {
			return 0, errors.New("JianPay amount must have at most two decimal places")
		}
		frac = (parts[1] + "0")[:2]
	}
	cents, err := strconv.ParseInt(parts[0]+frac, 10, 64)
	if err != nil || cents <= 0 || cents > 9007199254740991 {
		return 0, errors.New("invalid JianPay amount")
	}
	return cents, nil
}

type jianPayOrder struct {
	OrderID         string `json:"orderId"`
	ClientNo        string `json:"clientNo"`
	MerchantOrderNo string `json:"merchantOrderNo"`
	Amount          int64  `json:"amount"`
	Status          *int   `json:"status"`
	PayURL          string `json:"payUrl"`
	PayMethod       string `json:"payMethod"`
}

func (p *JianPay) call(ctx context.Context, path string, params map[string]any) (*jianPayOrder, error) {
	params["clientNo"], params["timestamp"], params["sign_type"] = p.clientNo, strconv.FormatInt(time.Now().Unix(), 10), "MD5"
	params["sign"] = jianPaySign(params, p.merchantKey)
	body, err := json.Marshal(params)
	if err != nil {
		return nil, errors.New("invalid JianPay request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBase+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("invalid JianPay request URL")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("JianPay request failed; query the existing order before retrying")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JianPay HTTP status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxEasypayResponseSize+1))
	if err != nil || len(raw) > maxEasypayResponseSize {
		return nil, errors.New("invalid JianPay response size")
	}
	var result struct {
		Code int          `json:"code"`
		Data jianPayOrder `json:"data"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, errors.New("invalid JianPay response")
	}
	if result.Code != 1000 {
		return nil, fmt.Errorf("JianPay rejected request (code %d)", result.Code)
	}
	d := &result.Data
	if d.ClientNo != p.clientNo || d.OrderID == "" || d.Amount <= 0 || d.Amount > 9007199254740991 || d.Status == nil || *d.Status < 0 || *d.Status > 4 {
		return nil, errors.New("JianPay response has invalid merchant, order, amount or status")
	}
	return d, nil
}

func (p *JianPay) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	if req.OrderID == "" || len(req.OrderID) > 128 {
		return nil, errors.New("invalid JianPay order number")
	}
	method := ""
	switch req.PaymentType {
	case payment.TypeAlipay:
		method = "alipay"
	case payment.TypeWxpay:
		method = "wx"
	default:
		return nil, errors.New("JianPay only supports alipay and wxpay")
	}
	cents, err := jianPayAmountCents(req.Amount)
	if err != nil {
		return nil, err
	}
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = p.returnURL
	}
	if !jianPayHTTPSURL(returnURL) {
		return nil, errors.New("invalid JianPay return URL")
	}
	params := map[string]any{"orderNo": req.OrderID, "amount": cents, "goodsName": req.Subject, "payMethod": method, "notifyUrl": p.notifyURL, "returnUrl": returnURL}
	d, err := p.call(ctx, "/open/payment/pay/create", params)
	if err != nil {
		// A timeout may have happened after the upstream accepted the order.
		// Recover by merchant order number rather than creating another charge.
		if recovered, queryErr := p.query(ctx, req.OrderID); queryErr == nil && (*recovered.Status == 0 || *recovered.Status == 1 || *recovered.Status == 2) && jianPayHTTPSURL(recovered.PayURL) {
			d = recovered
		} else {
			return nil, err
		}
	}
	if d.MerchantOrderNo != req.OrderID || d.Amount != cents || *d.Status > 2 || !jianPayHTTPSURL(d.PayURL) || (d.PayMethod != "" && d.PayMethod != method) {
		return nil, errors.New("JianPay checkout does not match the requested order")
	}
	return &payment.CreatePaymentResponse{TradeNo: d.OrderID, PayURL: d.PayURL, QRCode: d.PayURL}, nil
}

func (p *JianPay) query(ctx context.Context, merchantOrderNo string) (*jianPayOrder, error) {
	if merchantOrderNo == "" || len(merchantOrderNo) > 128 {
		return nil, errors.New("invalid JianPay order number")
	}
	d, err := p.call(ctx, "/open/payment/pay/info", map[string]any{"merchantOrderNo": merchantOrderNo})
	if err != nil {
		return nil, err
	}
	if d.MerchantOrderNo != merchantOrderNo || (d.PayMethod != "alipay" && d.PayMethod != "wx") {
		return nil, errors.New("JianPay query order or payment method mismatch")
	}
	return d, nil
}

func (p *JianPay) QueryOrder(ctx context.Context, merchantOrderNo string) (*payment.QueryOrderResponse, error) {
	d, err := p.query(ctx, merchantOrderNo)
	if err != nil {
		return nil, err
	}
	status := payment.ProviderStatusPending
	switch *d.Status {
	case 2:
		status = payment.ProviderStatusPaid
	case 3, 4:
		status = payment.ProviderStatusFailed
	}
	return &payment.QueryOrderResponse{TradeNo: d.OrderID, Status: status, Amount: float64(d.Amount) / 100, Metadata: map[string]string{"clientNo": d.ClientNo, "merchantOrderNo": d.MerchantOrderNo, "payMethod": d.PayMethod, "amount_cents": strconv.FormatInt(d.Amount, 10)}}, nil
}

func (p *JianPay) VerifyNotification(_ context.Context, raw string, _ map[string]string) (*payment.PaymentNotification, error) {
	var params map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&params) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid JianPay notification JSON")
	}
	sign, _ := params["sign"].(string)
	if params["sign_type"] != "MD5" || len(sign) != 32 || subtle.ConstantTimeCompare([]byte(strings.ToLower(sign)), []byte(jianPaySign(params, p.merchantKey))) != 1 {
		return nil, errors.New("invalid JianPay notification signature")
	}
	var d jianPayOrder
	if json.Unmarshal([]byte(raw), &d) != nil || d.ClientNo != p.clientNo || d.OrderID == "" || d.MerchantOrderNo == "" || len(d.MerchantOrderNo) > 128 || d.Amount <= 0 || d.Amount > 9007199254740991 || d.Status == nil || *d.Status < 0 || *d.Status > 4 || (d.PayMethod != "alipay" && d.PayMethod != "wx") {
		return nil, errors.New("invalid JianPay notification identity, amount or status")
	}
	if _, refund := params["eventType"]; refund {
		return nil, errors.New("JianPay refund notifications are not supported")
	}
	if *d.Status != 2 {
		return nil, nil
	}
	return &payment.PaymentNotification{OrderID: d.MerchantOrderNo, TradeNo: d.OrderID, Amount: float64(d.Amount) / 100, Status: payment.NotificationStatusSuccess, Metadata: map[string]string{"clientNo": d.ClientNo, "merchantOrderNo": d.MerchantOrderNo, "payMethod": d.PayMethod, "amount_cents": strconv.FormatInt(d.Amount, 10)}}, nil
}

func (p *JianPay) ConfirmNotification(ctx context.Context, n *payment.PaymentNotification) error {
	if n == nil {
		return errors.New("missing JianPay notification")
	}
	q, err := p.QueryOrder(ctx, n.OrderID)
	if err != nil {
		return err
	}
	if q.Status != payment.ProviderStatusPaid || q.TradeNo != n.TradeNo || q.Amount != n.Amount || q.Metadata["payMethod"] != n.Metadata["payMethod"] {
		return errors.New("JianPay query does not confirm the notification")
	}
	return nil
}
