package provider

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayNotificationRejectsReusedCheckoutSignature(t *testing.T) {
	t.Parallel()

	provider := newTestEasyPay(t, "https://pay.example.com")
	checkout := map[string]string{
		"pid":          "pid-1",
		"type":         "alipay",
		"out_trade_no": "sub2-unpaid",
		"name":         "Recharge",
		"money":        "1.00",
		"notify_url":   "https://example.com/notify",
		"return_url":   "https://example.com/payment/result&trade_no=forged-trade&trade_status=TRADE_SUCCESS",
	}
	forged := make(map[string]string, len(checkout)+3)
	for key, value := range checkout {
		forged[key] = value
	}
	forged["return_url"] = "https://example.com/payment/result"
	forged["trade_no"] = "forged-trade"
	forged["trade_status"] = tradeStatusSuccess
	forged["sign"] = easyPaySign(checkout, provider.config["pkey"])
	forged["sign_type"] = signTypeMD5

	if easyPaySign(forged, provider.config["pkey"]) != forged["sign"] {
		t.Fatal("fixture must reproduce the original protocol's ambiguous signature input")
	}
	if _, err := provider.VerifyNotification(context.Background(), encodeEasyPayNotification(forged), nil); err == nil {
		t.Fatal("a checkout signature reused as a payment notification must be rejected")
	}
}

func TestEasyPayVerifyNotificationAcceptsDocumentedFields(t *testing.T) {
	t.Parallel()

	provider := newTestEasyPay(t, "https://pay.example.com")
	params := validEasyPayNotificationParams()
	params["channel_order_no"] = "alipay-channel-123"
	params["param"] = "merchant-extra"
	params["sign"] = easyPaySign(params, provider.config["pkey"])
	notification, err := provider.VerifyNotification(context.Background(), encodeEasyPayNotification(params), nil)
	if err != nil {
		t.Fatalf("VerifyNotification: %v", err)
	}
	if notification.Status != payment.ProviderStatusSuccess || notification.Amount != 1.23 || notification.TradeNo != "gateway-123" || notification.OrderID != "sub2-order-123" {
		t.Fatalf("notification = %+v", notification)
	}
	if notification.Metadata["pid"] != "pid-1" || notification.Metadata["channel_order_no"] != "alipay-channel-123" {
		t.Fatalf("metadata = %+v", notification.Metadata)
	}
}

func TestEasyPayVerifyNotificationRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(map[string]string)
		extra  string
	}{
		{name: "missing pid", mutate: func(p map[string]string) { delete(p, "pid") }},
		{name: "wrong pid", mutate: func(p map[string]string) { p["pid"] = "other-merchant" }},
		{name: "pid whitespace is not normalized", mutate: func(p map[string]string) { p["pid"] = " pid-1 " }},
		{name: "missing trade no", mutate: func(p map[string]string) { delete(p, "trade_no") }},
		{name: "blank trade no", mutate: func(p map[string]string) { p["trade_no"] = " " }},
		{name: "missing merchant order no", mutate: func(p map[string]string) { delete(p, "out_trade_no") }},
		{name: "wrong status", mutate: func(p map[string]string) { p["trade_status"] = "WAITING" }},
		{name: "missing status", mutate: func(p map[string]string) { delete(p, "trade_status") }},
		{name: "invalid money", mutate: func(p map[string]string) { p["money"] = "invalid" }},
		{name: "zero money", mutate: func(p map[string]string) { p["money"] = "0.00" }},
		{name: "negative money", mutate: func(p map[string]string) { p["money"] = "-1.00" }},
		{name: "NaN money", mutate: func(p map[string]string) { p["money"] = "NaN" }},
		{name: "infinite money", mutate: func(p map[string]string) { p["money"] = "+Inf" }},
		{name: "overflow money", mutate: func(p map[string]string) { p["money"] = "1e9999" }},
		{name: "unsupported sign type", mutate: func(p map[string]string) { p["sign_type"] = "RSA" }},
		{name: "return url", mutate: func(p map[string]string) { p["return_url"] = "https://example.com/payment/result" }},
		{name: "notify url", mutate: func(p map[string]string) { p["notify_url"] = "https://example.com/notify" }},
		{name: "device", mutate: func(p map[string]string) { p["device"] = "mobile" }},
		{name: "unknown field", mutate: func(p map[string]string) { p["unexpected"] = "1" }},
		{name: "empty unknown field", mutate: func(p map[string]string) { p["unexpected"] = "" }},
		{name: "duplicate money", extra: "&money=1.23"},
		{name: "duplicate pid", extra: "&pid=pid-1"},
		{name: "duplicate sign", extra: "&sign=another-sign"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := newTestEasyPay(t, "https://pay.example.com")
			params := validEasyPayNotificationParams()
			if tt.mutate != nil {
				tt.mutate(params)
			}
			params["sign"] = easyPaySign(params, provider.config["pkey"])
			if _, err := provider.VerifyNotification(context.Background(), encodeEasyPayNotification(params)+tt.extra, nil); err == nil {
				t.Fatal("invalid notification was accepted")
			}
		})
	}
}

func TestEasyPayVerifyNotificationRejectsTamperedSignature(t *testing.T) {
	t.Parallel()
	provider := newTestEasyPay(t, "https://pay.example.com")
	params := validEasyPayNotificationParams()
	params["sign"] = easyPaySign(params, provider.config["pkey"])
	params["money"] = "9.99"
	if _, err := provider.VerifyNotification(context.Background(), encodeEasyPayNotification(params), nil); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("tampered callback error = %v", err)
	}
}

func TestEasyPayCreatePaymentRequiresServerReturnURL(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{paymentModePopup, "qrcode"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			provider := newTestEasyPay(t, "https://pay.example.com")
			provider.config["paymentMode"] = mode
			if _, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
				OrderID: "sub2-unpaid", Amount: "1.00", PaymentType: payment.TypeAlipay,
			}); err == nil {
				t.Fatal("missing server return URL must not fall back to instance returnUrl")
			}
		})
	}
}

func validEasyPayNotificationParams() map[string]string {
	return map[string]string{
		"pid": "pid-1", "type": "alipay", "out_trade_no": "sub2-order-123",
		"trade_no": "gateway-123", "trade_status": tradeStatusSuccess,
		"name": "Recharge", "money": "1.23", "sign_type": signTypeMD5,
	}
}

func encodeEasyPayNotification(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}
