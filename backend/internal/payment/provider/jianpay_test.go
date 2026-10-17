package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func jianPayTestProvider(t *testing.T) *JianPay {
	t.Helper()
	p, err := NewJianPay("1", map[string]string{"clientNo": "JP-test", "merchantKey": "fixture-key", "notifyUrl": "https://site.test/api/v1/payment/webhook/jianpay", "returnUrl": "https://site.test/payment/result"})
	require.NoError(t, err)
	return p
}

func TestJianPayCheckoutAndQuery(t *testing.T) {
	p := jianPayTestProvider(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		d := json.NewDecoder(r.Body)
		d.UseNumber()
		require.NoError(t, d.Decode(&body))
		require.Equal(t, "JP-test", body["clientNo"])
		require.Equal(t, jianPaySign(body, "fixture-key"), body["sign"])
		switch r.URL.Path {
		case "/open/payment/pay/create":
			require.Equal(t, json.Number("1001"), body["amount"])
			require.Equal(t, "alipay", body["payMethod"])
			require.Equal(t, "local-order", body["orderNo"])
			require.Equal(t, "https://site.test/payment/result?resume_token=signed-token", body["returnUrl"])
			fmt.Fprint(w, `{"code":1000,"data":{"clientNo":"JP-test","merchantOrderNo":"local-order","orderId":"upstream-order","amount":1001,"status":1,"payUrl":"https://jpay.hzjianban.com/#/pay?orderId=upstream-order","payQrcodeUrl":"https://example.test/qr.png"}}`)
		case "/open/payment/pay/info":
			require.Equal(t, "local-order", body["merchantOrderNo"])
			fmt.Fprint(w, `{"code":1000,"data":{"clientNo":"JP-test","merchantOrderNo":"local-order","orderId":"upstream-order","amount":1001,"status":2,"payMethod":"alipay"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	p.apiBase, p.httpClient = server.URL, server.Client()
	res, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "local-order", Amount: "10.01", PaymentType: "alipay", Subject: "充值", ReturnURL: "https://site.test/payment/result?resume_token=signed-token"})
	require.NoError(t, err)
	require.Equal(t, "upstream-order", res.TradeNo)
	require.Equal(t, res.PayURL, res.QRCode)
	require.NotContains(t, res.QRCode, "qr.png")
	query, err := p.QueryOrder(context.Background(), "local-order")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusPaid, query.Status)
	require.Equal(t, 10.01, query.Amount)
	n := &payment.PaymentNotification{OrderID: "local-order", TradeNo: "upstream-order", Amount: 10.01, Metadata: map[string]string{"clientNo": "JP-test", "payMethod": "alipay"}}
	require.NoError(t, p.ConfirmNotification(context.Background(), n))
	n.Amount = 10
	require.Error(t, p.ConfirmNotification(context.Background(), n))
}

func TestJianPayNotification(t *testing.T) {
	p := jianPayTestProvider(t)
	body := map[string]any{"clientNo": "JP-test", "merchantOrderNo": "local-order", "orderId": "upstream-order", "amount": json.Number("1001"), "status": json.Number("2"), "payMethod": "wx", "sign_type": "MD5"}
	encode := func() string {
		body["sign"] = jianPaySign(body, "fixture-key")
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		return string(raw)
	}
	n, err := p.VerifyNotification(context.Background(), encode(), nil)
	require.NoError(t, err)
	require.Equal(t, 10.01, n.Amount)
	require.Equal(t, "wx", n.Metadata["payMethod"])
	body["clientNo"] = "other-merchant"
	_, err = p.VerifyNotification(context.Background(), encode(), nil)
	require.Error(t, err)
	body["clientNo"] = "JP-test"
	body["amount"] = json.Number("1001.5")
	_, err = p.VerifyNotification(context.Background(), encode(), nil)
	require.Error(t, err)
	body["amount"] = json.Number("1001")
	body["status"] = json.Number("1")
	n, err = p.VerifyNotification(context.Background(), encode(), nil)
	require.NoError(t, err)
	require.Nil(t, n)
	body["status"] = json.Number("2")
	raw := encode()
	_, err = p.VerifyNotification(context.Background(), raw+"{}", nil)
	require.Error(t, err)
	body["sign"] = "invalid"
	b, _ := json.Marshal(body)
	_, err = p.VerifyNotification(context.Background(), string(b), nil)
	require.Error(t, err)
}

func TestJianPayRejectInvalidRequests(t *testing.T) {
	p := jianPayTestProvider(t)
	for _, amount := range []string{"0", "-1", "NaN", "1.001", "1e2", "99999999999999999999"} {
		_, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "test", Amount: amount, PaymentType: "alipay"})
		require.Error(t, err, amount)
	}
	_, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "test", Amount: "1.00", PaymentType: "stripe"})
	require.Error(t, err)
	_, err = NewJianPay("1", map[string]string{"clientNo": "JP-test", "merchantKey": "key", "apiBase": "http://bad.test"})
	require.Error(t, err)
}

func TestJianPayCallbackQuerySettingValidation(t *testing.T) {
	for _, value := range []string{"", "true", "false", "yes", "0", "FALSE"} {
		_, err := NewJianPay("1", map[string]string{
			"clientNo": "JP-test", "merchantKey": "fixture-key", "verifyAfterCallback": value,
			"notifyUrl": "https://site.test/api/v1/payment/webhook/jianpay", "returnUrl": "https://site.test/payment/result",
		})
		if value == "" || value == "true" || value == "false" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestJianPaySignatureVector(t *testing.T) {
	require.Equal(t, "b23d749134c728a1da852b90d6b11bb2", jianPaySign(map[string]any{"amount": json.Number("1001"), "clientNo": "JP-test", "goodsName": "充值", "status": json.Number("0"), "sign": "excluded", "sign_type": "MD5", "empty": "", "nil": nil}, "fixture-key"))
}

func TestJianPayQueryStatuses(t *testing.T) {
	for status, expected := range map[int]string{0: payment.ProviderStatusPending, 1: payment.ProviderStatusPending, 2: payment.ProviderStatusPaid, 3: payment.ProviderStatusFailed, 4: payment.ProviderStatusFailed, 5: ""} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			p := jianPayTestProvider(t)
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, `{"code":1000,"data":{"clientNo":"JP-test","merchantOrderNo":"local","orderId":"upstream","amount":100,"status":%d,"payMethod":"alipay"}}`, status)
			}))
			defer s.Close()
			p.apiBase, p.httpClient = s.URL, s.Client()
			q, err := p.QueryOrder(context.Background(), "local")
			if expected == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, expected, q.Status)
			}
		})
	}
}

func TestJianPayRecoversAmbiguousCreationWithSameOrder(t *testing.T) {
	p := jianPayTestProvider(t)
	creates := 0
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open/payment/pay/create" {
			creates++
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		var b map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&b))
		require.Equal(t, "original-order", b["merchantOrderNo"])
		fmt.Fprint(w, `{"code":1000,"data":{"clientNo":"JP-test","merchantOrderNo":"original-order","orderId":"upstream","amount":100,"status":1,"payMethod":"wx","payUrl":"https://jpay.hzjianban.com/#/pay?orderId=upstream"}}`)
	}))
	defer s.Close()
	p.apiBase, p.httpClient = s.URL, s.Client()
	res, err := p.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "original-order", Amount: "1.00", PaymentType: "wxpay"})
	require.NoError(t, err)
	require.Equal(t, "upstream", res.TradeNo)
	require.Equal(t, 1, creates)
}
