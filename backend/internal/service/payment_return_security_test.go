//go:build unit

package service

import (
	"context"
	"net/url"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderBuildsTrustedReturnURLAndKeepsOrderPending(t *testing.T) {
	for _, host := range []string{"codeplan.codes", "subapi.loucer.cn", "us.codeplan.codes"} {
		for _, clientReturnURL := range []struct {
			name string
			url  string
		}{
			{name: "injected callback query", url: "https://" + host + "/payment/result?trade_status=TRADE_SUCCESS&pid=forged&trade_no=forged&money=0.01&order_id=999&resume_token=forged"},
			{name: "external return address", url: "https://evil.example/payment/result?trade_status=TRADE_SUCCESS"},
			{name: "empty client return address"},
		} {
			t.Run(host+"/"+clientReturnURL.name, func(t *testing.T) {
				ctx := context.Background()
				client := newPaymentConfigServiceTestClient(t)
				user, err := client.User.Create().
					SetEmail("payment-return@example.com").
					SetPasswordHash("test-hash").
					SetUsername("payment-return-user").
					SetStatus(payment.EntityStatusActive).
					SetBalance(7).
					Save(ctx)
				require.NoError(t, err)

				key := []byte("0123456789abcdef0123456789abcdef")
				configSvc := NewPaymentConfigService(client, &paymentConfigSettingRepoStub{values: map[string]string{
					SettingPaymentEnabled:                    "true",
					SettingPaymentVisibleMethodAlipayEnabled: "true",
					SettingPaymentVisibleMethodAlipaySource:  VisibleMethodSourceEasyPayAlipay,
				}}, key)
				configSvc.returnOrigins = []string{"https://codeplan.codes", "https://subapi.loucer.cn", "https://us.codeplan.codes"}
				instance, err := configSvc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
					ProviderKey:    payment.TypeEasyPay,
					Name:           "Local popup test",
					Enabled:        true,
					PaymentMode:    "popup",
					SupportedTypes: []string{payment.TypeAlipay},
					Config: map[string]string{
						"pid":       "2001",
						"pkey":      "test-provider-key",
						"apiBase":   "https://pay.example.com",
						"notifyUrl": "https://" + host + "/api/v1/payment/notify/easypay",
						"returnUrl": "https://evil.example/provider-fallback?trade_status=TRADE_SUCCESS",
					},
				})
				require.NoError(t, err)

				svc := NewPaymentService(client, payment.NewRegistry(), payment.NewDefaultLoadBalancer(client, key), nil, nil, configSvc,
					&visibilityUserRepo{user: &User{ID: user.ID, Email: user.Email, Username: user.Username, Status: user.Status}}, nil, nil)
				response, err := svc.CreateOrder(ctx, CreateOrderRequest{
					UserID:      user.ID,
					Amount:      1,
					PaymentType: payment.TypeAlipay,
					ClientIP:    "127.0.0.1",
					SrcHost:     host,
					SrcURL:      "https://evil.example/?trade_status=TRADE_SUCCESS&return_url=https://evil.example",
					ReturnURL:   clientReturnURL.url,
				})
				require.NoError(t, err)
				require.Equal(t, OrderStatusPending, response.Status)

				payURL, err := url.Parse(response.PayURL)
				require.NoError(t, err)
				require.Equal(t, "pay.example.com", payURL.Host)
				require.Equal(t, "/submit.php", payURL.Path)
				require.NotEmpty(t, payURL.Query().Get("sign"))
				returnURL, err := url.Parse(payURL.Query().Get("return_url"))
				require.NoError(t, err)
				require.Equal(t, "https", returnURL.Scheme)
				require.Equal(t, host, returnURL.Host)
				require.Equal(t, "/payment/result", returnURL.Path)
				require.Empty(t, returnURL.Fragment)
				require.Nil(t, returnURL.User)
				params := returnURL.Query()
				require.Len(t, params, 4)
				require.Equal(t, strconv.FormatInt(response.OrderID, 10), params.Get("order_id"))
				require.Equal(t, response.OutTradeNo, params.Get("out_trade_no"))
				require.NotEmpty(t, response.ResumeToken)
				require.Equal(t, response.ResumeToken, params.Get("resume_token"))
				require.Equal(t, "success", params.Get("status"))

				order, err := client.PaymentOrder.Get(ctx, response.OrderID)
				require.NoError(t, err)
				require.Equal(t, OrderStatusPending, order.Status)
				require.Equal(t, user.ID, order.UserID)
				require.Equal(t, strconv.FormatInt(instance.ID, 10), *order.ProviderInstanceID)
				require.Equal(t, response.OutTradeNo, order.OutTradeNo)
				require.Empty(t, order.PaymentTradeNo)
				require.Equal(t, response.PayURL, *order.PayURL)
				unchangedUser, err := client.User.Get(ctx, user.ID)
				require.NoError(t, err)
				require.Equal(t, float64(7), unchangedUser.Balance)
			})
		}
	}
}
