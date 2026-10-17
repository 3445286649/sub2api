//go:build unit

package service

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/stretchr/testify/require"
)

func fmSignedCallback(orderNo, merchant, amount string) string {
	params := url.Values{
		"money": {amount}, "out_trade_no": {orderNo}, "pid": {merchant},
		"trade_no": {"platform-trade"}, "trade_status": {"TRADE_SUCCESS"}, "type": {"alipay"},
	}
	base := fmt.Sprintf("money=%s&out_trade_no=%s&pid=%s&trade_no=platform-trade&trade_status=TRADE_SUCCESS&type=alipayfixture-secret", amount, orderNo, merchant)
	params.Set("sign", fmt.Sprintf("%x", md5.Sum([]byte(base))))
	params.Set("sign_type", "MD5")
	return params.Encode()
}

func TestFMWebhookConfirmationPipeline(t *testing.T) {
	for _, verify := range []bool{false, true} {
		t.Run(strconv.FormatBool(verify), func(t *testing.T) {
			svc, order, credited, calls, _ := newFMReviewFixture(t)
			ctx := context.Background()
			snapshot := cloneProviderSnapshot(order.ProviderSnapshot)
			snapshot["fm_verify_after_callback"] = verify
			_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(snapshot).Save(ctx)
			require.NoError(t, err)
			prov, err := svc.GetWebhookProvider(ctx, payment.ProviderZhifuFM, order.OutTradeNo)
			require.NoError(t, err)
			callback := fmSignedCallback(order.OutTradeNo, "123", "80.00")
			deliver := func() {
				t.Helper()
				n, err := prov.VerifyNotification(ctx, callback, nil)
				require.NoError(t, err)
				require.NotNil(t, n)
				require.NoError(t, svc.ConfirmNotificationUpstream(ctx, prov, n))
				require.NoError(t, svc.HandlePaymentNotification(ctx, n, prov.ProviderKey()))
			}
			// Exercise the same confirmation step as the webhook, including duplicates.
			deliver()
			deliver()
			require.Zero(t, calls.Load(), "FM must never use the inherited EasyPay query")
			if verify {
				require.Zero(t, *credited)
				require.Equal(t, "pending", fmFixtureState(t, svc, order.ID))
			} else {
				require.Equal(t, order.Amount, *credited)
				require.Empty(t, fmFixtureState(t, svc, order.ID))
			}
			require.NoError(t, svc.ProcessFMCallbackReview(ctx))
			require.Equal(t, order.Amount, *credited)
			deliver()
			require.NoError(t, svc.ProcessFMCallbackReview(ctx))
			require.Equal(t, order.Amount, *credited, "duplicate callbacks must not increase balance again")
			expectedQueries := int32(0)
			if verify {
				expectedQueries = 1
			}
			require.Equal(t, expectedQueries, calls.Load())
			fresh, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, fresh.Status)
		})
	}
}

func TestFMWebhookConfirmationRejectsInvalidCallback(t *testing.T) {
	for _, verify := range []bool{false, true} {
		for _, mismatch := range []string{"signature", "merchant", "amount"} {
			t.Run(fmt.Sprintf("%t/%s", verify, mismatch), func(t *testing.T) {
				svc, order, credited, calls, _ := newFMReviewFixture(t)
				ctx := context.Background()
				snapshot := cloneProviderSnapshot(order.ProviderSnapshot)
				snapshot["fm_verify_after_callback"] = verify
				_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(snapshot).Save(ctx)
				require.NoError(t, err)
				prov, err := svc.GetWebhookProvider(ctx, payment.ProviderZhifuFM, order.OutTradeNo)
				require.NoError(t, err)
				merchant, amount := "123", "80.00"
				if mismatch == "merchant" {
					merchant = "456"
				}
				if mismatch == "amount" {
					amount = "79.99"
				}
				callback := fmSignedCallback(order.OutTradeNo, merchant, amount)
				if mismatch == "signature" {
					params, err := url.ParseQuery(callback)
					require.NoError(t, err)
					params.Set("sign", "00000000000000000000000000000000")
					callback = params.Encode()
				}
				n, err := prov.VerifyNotification(ctx, callback, nil)
				if mismatch == "amount" {
					require.NoError(t, err)
					require.NoError(t, svc.ConfirmNotificationUpstream(ctx, prov, n))
					require.Error(t, svc.HandlePaymentNotification(ctx, n, prov.ProviderKey()))
				} else {
					require.Error(t, err)
				}
				require.Zero(t, calls.Load())
				require.Zero(t, *credited)
				require.Empty(t, fmFixtureState(t, svc, order.ID))
				fresh, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
				require.NoError(t, err)
				require.Equal(t, OrderStatusPending, fresh.Status)
			})
		}
	}
}

func TestEasyPayWebhookConfirmationStillQueriesUpstream(t *testing.T) {
	for _, paid := range []bool{false, true} {
		t.Run(strconv.FormatBool(paid), func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPendingEasyPayFulfillmentOrder(t, ctx, client)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, "/api.php", r.URL.Path)
				require.Equal(t, "order", r.URL.Query().Get("act"))
				require.Equal(t, order.OutTradeNo, r.URL.Query().Get("out_trade_no"))
				status := 0
				if paid {
					status = 1
				}
				fmt.Fprintf(w, `{"code":1,"pid":"123","out_trade_no":%q,"trade_no":"platform-trade","money":"80.00","status":%d}`, order.OutTradeNo, status)
			}))
			t.Cleanup(server.Close)
			prov, err := provider.NewEasyPay("fixture", map[string]string{
				"pid": "123", "pkey": "fixture-secret", "apiBase": server.URL,
				"notifyUrl": "https://site.test/api/v1/payment/webhook/easypay", "returnUrl": "https://site.test/payment/result",
			})
			require.NoError(t, err)
			n, err := prov.VerifyNotification(ctx, fmSignedCallback(order.OutTradeNo, "123", "80.00"), nil)
			require.NoError(t, err)
			svc := &PaymentService{entClient: client}
			err = svc.ConfirmNotificationUpstream(ctx, prov, n)
			if paid {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "not paid")
			}
			require.Equal(t, int32(1), calls.Load(), "ordinary EasyPay must still confirm the callback upstream")
		})
	}
}
