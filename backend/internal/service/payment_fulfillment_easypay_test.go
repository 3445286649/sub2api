//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestPaymentNotificationRejectsInvalidEasyPayBeforeFulfillment(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tradeNo   string
		provider  string
		pid       string
		wantError string
		audit     string
	}{
		{name: "empty trade no", provider: payment.TypeEasyPay, pid: "pid-expected", wantError: "trade no is empty", audit: "PAYMENT_MISSING_TRADE_NO"},
		{name: "blank trade no through legacy entry", tradeNo: " \t", pid: "pid-expected", wantError: "trade no is empty", audit: "PAYMENT_MISSING_TRADE_NO"},
		{name: "wrong merchant", tradeNo: "easypay-trade", provider: payment.TypeEasyPay, pid: "pid-other", wantError: "easypay pid mismatch", audit: "PAYMENT_PROVIDER_METADATA_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPendingEasyPayFulfillmentOrder(t, ctx, client)
			svc := &PaymentService{entClient: client}
			err := svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
				TradeNo:  tc.tradeNo,
				OrderID:  order.OutTradeNo,
				Amount:   order.PayAmount,
				Status:   payment.NotificationStatusSuccess,
				Metadata: map[string]string{"pid": tc.pid},
			}, tc.provider)
			require.ErrorContains(t, err, tc.wantError)
			assertRejectedEasyPayFulfillment(t, ctx, client, svc, order, tc.audit)
		})
	}
}

func createPendingEasyPayFulfillmentOrder(t *testing.T, ctx context.Context, client *dbent.Client) *dbent.PaymentOrder {
	t.Helper()
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPending, time.Now())
	order, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetOrderType(payment.OrderTypeBalance).
		SetProviderKey(payment.TypeEasyPay).
		SetProviderSnapshot(map[string]any{"schema_version": 2, "provider_key": payment.TypeEasyPay, "merchant_id": "pid-expected"}).
		ClearPlanID().
		ClearSubscriptionGroupID().
		ClearSubscriptionDays().
		SetPaymentTradeNo("").
		ClearPaidAt().
		Save(ctx)
	require.NoError(t, err)
	return order
}

func assertRejectedEasyPayFulfillment(t *testing.T, ctx context.Context, client *dbent.Client, svc *PaymentService, order *dbent.PaymentOrder, audit string) {
	t.Helper()
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
	require.Empty(t, reloaded.PaymentTradeNo)
	require.Nil(t, reloaded.PaidAt)
	require.Nil(t, reloaded.CompletedAt)
	user, err := client.User.Get(ctx, order.UserID)
	require.NoError(t, err)
	require.Zero(t, user.Balance)
	require.True(t, svc.hasAuditLog(ctx, order.ID, audit))
}
