package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestZhifuFMQueryContract(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		paid, fail bool
	}{
		{"paid", `{"success":true,"code":200,"data":{"merchantNum":"123","orderNo":"sub2_test","orderId":"fm123","amount":"18.00","tradeMoney":"17.84","orderState":"4"}}`, true, false},
		{"pending", `{"success":true,"code":200,"data":{"merchantNum":"123","orderNo":"sub2_test","orderId":"fm123","amount":"18","orderState":"2"}}`, false, false},
		{"foreign merchant", `{"success":true,"code":200,"data":{"merchantNum":"456","orderNo":"sub2_test","orderId":"fm123","amount":"18","orderState":"4"}}`, false, true},
		{"foreign order", `{"success":true,"code":200,"data":{"merchantNum":"123","orderNo":"other","orderId":"fm123","amount":"18","orderState":"4"}}`, false, true},
		{"unknown state", `{"success":true,"code":200,"data":{"merchantNum":"123","orderNo":"sub2_test","orderId":"fm123","amount":"18","orderState":"99"}}`, false, true},
		{"permission", `{"success":false,"code":403,"msg":"secret must not be logged"}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				require.Equal(t, "/queryOutOrder", r.URL.Path)
				require.Equal(t, "123", r.URL.Query().Get("merchantNum"))
				require.Equal(t, "sub2_test", r.URL.Query().Get("orderNo"))
				require.Equal(t, fmQuerySign("123", "sub2_test", "test-secret"), r.URL.Query().Get("sign"))
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			// Production constructor requires HTTPS; a local server tests the transport directly.
			ep, err := NewEasyPay("1", map[string]string{"pid": "123", "pkey": "test-secret", "apiBase": server.URL, "notifyUrl": "https://site.test/api/v1/payment/webhook/zhifufm", "returnUrl": "https://site.test/payment/result"})
			require.NoError(t, err)
			p := &ZhifuFM{EasyPay: ep, queryBase: server.URL}
			result, err := p.QueryOrder(context.Background(), "sub2_test")
			if tc.fail {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "secret")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.paid, result.Status == payment.ProviderStatusPaid)
			require.Equal(t, float64(18), result.Amount)
			require.Equal(t, "123", result.Metadata["pid"])
		})
	}
}

func TestZhifuFMConstructorAndIdentity(t *testing.T) {
	cfg := map[string]string{"pid": "123", "pkey": "test-secret", "apiBase": "https://pay.test/epay", "queryBase": "https://pay.test/api/pay", "notifyUrl": "https://site.test/api/v1/payment/webhook/zhifufm", "returnUrl": "https://site.test/payment/result"}
	p, err := NewZhifuFM("1", cfg)
	require.NoError(t, err)
	require.Equal(t, payment.ProviderZhifuFM, p.ProviderKey())
	require.True(t, p.VerifyAfterCallback())
	cfg["queryBase"] = "http://pay.test"
	_, err = NewZhifuFM("1", cfg)
	require.Error(t, err)
}

func TestZhifuFMCheckoutAndCallbackSignature(t *testing.T) {
	p, err := NewZhifuFM("1", map[string]string{"pid": "123", "pkey": "fixture-secret", "apiBase": "https://fm.test/epay", "queryBase": "https://fm.test/api/pay", "notifyUrl": "https://site.test/api/v1/payment/webhook/zhifufm", "returnUrl": "https://site.test/payment/result", "paymentMode": "popup"})
	require.NoError(t, err)
	result, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2_test", PaymentType: payment.TypeAlipay, Amount: "18.00", Subject: "Recharge", ReturnURL: "https://site.test/payment/result?order_id=1"})
	require.NoError(t, err)
	u, err := url.Parse(result.PayURL)
	require.NoError(t, err)
	require.Equal(t, "https://site.test/api/v1/payment/webhook/zhifufm", u.Query().Get("notify_url"))
	require.Equal(t, "alipay", u.Query().Get("type"))
	values := map[string]string{"pid": "123", "trade_no": "fm123", "out_trade_no": "sub2_test", "type": "alipay", "money": "18.00", "trade_status": "TRADE_SUCCESS", "sign_type": "MD5"}
	values["sign"] = easyPaySign(values, "fixture-secret")
	form := url.Values{}
	for k, v := range values {
		form.Set(k, v)
	}
	notification, err := p.VerifyNotification(context.Background(), form.Encode(), nil)
	require.NoError(t, err)
	require.Equal(t, "sub2_test", notification.OrderID)
	form.Set("money", "19.00")
	_, err = p.VerifyNotification(context.Background(), form.Encode(), nil)
	require.Error(t, err)
	_, err = p.VerifyNotification(context.Background(), u.RawQuery, nil)
	require.Error(t, err, "a signed checkout request must not be accepted as a paid callback")
}
