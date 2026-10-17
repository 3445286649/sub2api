//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestJianPayCallbackQueryPolicySnapshot(t *testing.T) {
	for _, value := range []string{"", "true", "false"} {
		snapshot := buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{
			ProviderKey: payment.ProviderJianPay, InstanceID: "1",
			Config: map[string]string{"clientNo": "JP-test", "verifyAfterCallback": value},
		}, CreateOrderRequest{PaymentType: payment.TypeAlipay})
		require.Equal(t, value != "false", snapshot["jianpay_verify_after_callback"])
		require.NotContains(t, snapshot, "fm_verify_after_callback")
	}
}

func TestJianPayCallbackQuerySwitchPreservesOrderPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, current string
		frozen        any
		queries       int32
	}{
		{"off", "false", false, 0},
		{"on", "true", true, 2},
		{"frozen off ignores current on", "true", false, 0},
		{"frozen on ignores current off", "false", true, 2},
		{"legacy order still queries", "false", nil, 2},
		{"malformed snapshot still queries", "false", "false", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, order, inst, credited, calls := newJianPayFixture(t)
			ctx := context.Background()
			encryptionKey := []byte(strings.Repeat("k", 32))
			svc.configService = NewPaymentConfigService(svc.entClient, nil, encryptionKey)
			svc.loadBalancer = payment.NewDefaultLoadBalancer(svc.entClient, encryptionKey)
			snapshot := cloneProviderSnapshot(order.ProviderSnapshot)
			delete(snapshot, "jianpay_verify_after_callback")
			if tc.frozen != nil {
				snapshot["jianpay_verify_after_callback"] = tc.frozen
			}
			_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(snapshot).Save(ctx)
			require.NoError(t, err)
			_, err = svc.configService.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{Config: map[string]string{"verifyAfterCallback": tc.current}})
			require.NoError(t, err)
			prov, err := svc.GetWebhookProvider(ctx, payment.ProviderJianPay, order.OutTradeNo)
			require.NoError(t, err)
			for i := 0; i < 2; i++ {
				n := jianPayFixtureNotification(order)
				require.NoError(t, svc.ConfirmNotificationUpstream(ctx, prov, n))
				require.NoError(t, svc.HandlePaymentNotification(ctx, n, prov.ProviderKey()))
			}
			require.Equal(t, tc.queries, calls.Load())
			require.Equal(t, order.Amount, *credited)
		})
	}
}

func TestJianPayEnabledQueryFailureHoldsCredit(t *testing.T) {
	svc, order, _, credited, _ := newJianPayFixture(t)
	calls := 0
	prov := paymentConfirmTestProvider{
		paymentFulfillmentTestProvider: paymentFulfillmentTestProvider{key: payment.ProviderJianPay},
		err:                            errors.New("query unavailable"), calls: &calls,
	}
	require.ErrorContains(t, svc.ConfirmNotificationUpstream(context.Background(), prov, jianPayFixtureNotification(order)), "query unavailable")
	require.Equal(t, 1, calls)
	require.Zero(t, *credited)
}

func TestJianPayDisabledQueryStillRejectsMismatchedNotification(t *testing.T) {
	for _, mismatch := range []string{"merchant", "amount", "method", "trade", "order"} {
		t.Run(mismatch, func(t *testing.T) {
			svc, order, _, credited, calls := newJianPayFixture(t)
			ctx := context.Background()
			snapshot := cloneProviderSnapshot(order.ProviderSnapshot)
			snapshot["jianpay_verify_after_callback"] = false
			_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(snapshot).Save(ctx)
			require.NoError(t, err)
			prov, err := svc.GetWebhookProvider(ctx, payment.ProviderJianPay, order.OutTradeNo)
			require.NoError(t, err)
			n := jianPayFixtureNotification(order)
			switch mismatch {
			case "merchant":
				n.Metadata["clientNo"] = "other"
			case "amount":
				n.Metadata["amount_cents"], n.Amount = "7999", 79.99
			case "method":
				n.Metadata["payMethod"] = "wx"
			case "trade":
				n.TradeNo = ""
			case "order":
				n.Metadata["merchantOrderNo"] = "other"
			}
			require.NoError(t, svc.ConfirmNotificationUpstream(ctx, prov, n))
			require.Error(t, svc.HandlePaymentNotification(ctx, n, prov.ProviderKey()))
			require.Zero(t, *credited)
			require.Zero(t, calls.Load())
		})
	}
}
