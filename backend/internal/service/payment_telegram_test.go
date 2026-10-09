//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type telegramPaymentQueryProvider struct {
	paymentFulfillmentTestProvider
	result  *payment.QueryOrderResponse
	err     error
	queries []string
}

func (p *telegramPaymentQueryProvider) QueryOrder(_ context.Context, reference string) (*payment.QueryOrderResponse, error) {
	p.queries = append(p.queries, reference)
	return p.result, p.err
}

func newTelegramPaymentGateTest(t *testing.T, verify bool) (*PaymentService, *telegramPaymentQueryProvider, *dbent.PaymentOrder, *float64) {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingEasyPayFulfillmentOrder(t, ctx, client)
	var err error
	order, err = client.PaymentOrder.UpdateOneID(order.ID).ClearProviderKey().ClearProviderSnapshot().Save(ctx)
	require.NoError(t, err)
	provider := &telegramPaymentQueryProvider{
		paymentFulfillmentTestProvider: paymentFulfillmentTestProvider{key: payment.TypeEasyPay, supportedTypes: []payment.PaymentType{payment.TypeAlipay}},
		result:                         &payment.QueryOrderResponse{Status: payment.ProviderStatusPaid, TradeNo: "platform-trade", Amount: order.PayAmount},
	}
	registry := payment.NewRegistry()
	registry.Register(provider)
	telegram, _ := newTelegramTestService(t)
	_, err = telegram.UpdateSettings(ctx, UpdateTelegramSettingsRequest{VerifyEasyPayOrders: verify})
	require.NoError(t, err)
	credited := new(float64)
	userRepo := &mockUserRepo{getByIDUser: &User{ID: order.UserID}}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, order.UserID, id)
		*credited += amount
		return nil
	}
	svc := &PaymentService{
		entClient: client, registry: registry, providersLoaded: true, telegramService: telegram,
		redeemService: NewRedeemService(&paymentFulfillmentRedeemRepo{}, userRepo, nil, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil),
	}
	return svc, provider, order, credited
}

func telegramPaymentTestNotification(order *dbent.PaymentOrder) *payment.PaymentNotification {
	return &payment.PaymentNotification{OrderID: order.OutTradeNo, TradeNo: "platform-trade", Amount: order.PayAmount, Status: payment.NotificationStatusSuccess}
}

func TestTelegramPaymentGateRejectsUnconfirmedPaymentWithoutBanning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *payment.QueryOrderResponse
		err    error
		audit  string
	}{
		{name: "unpaid", result: &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, audit: "PAYMENT_UPSTREAM_NOT_PAID"},
		{name: "query outage", err: errors.New("network failure"), audit: "PAYMENT_UPSTREAM_QUERY_FAILED"},
		{name: "wrong amount", result: &payment.QueryOrderResponse{Status: payment.ProviderStatusPaid, Amount: 1, TradeNo: "platform-trade"}, audit: "PAYMENT_UPSTREAM_AMOUNT_MISMATCH"},
		{name: "wrong transaction", result: &payment.QueryOrderResponse{Status: payment.ProviderStatusPaid, Amount: 80, TradeNo: "other-trade"}, audit: "PAYMENT_UPSTREAM_TRADE_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, provider, order, credited := newTelegramPaymentGateTest(t, true)
			provider.result, provider.err = tc.result, tc.err
			err := svc.HandlePaymentNotification(context.Background(), telegramPaymentTestNotification(order), payment.TypeEasyPay)
			require.Error(t, err)
			require.Zero(t, *credited)
			stored, err := svc.entClient.PaymentOrder.Get(context.Background(), order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusPending, stored.Status)
			require.Nil(t, stored.PaidAt)
			require.True(t, svc.hasAuditLog(context.Background(), order.ID, tc.audit))
			user, err := svc.entClient.User.Get(context.Background(), order.UserID)
			require.NoError(t, err)
			require.Equal(t, StatusActive, user.Status)
			require.Equal(t, []string{order.OutTradeNo}, provider.queries, "only the incoming order is queried")
		})
	}
}

func TestTelegramPaymentGateCreditsOnceAndDoesNotRequeryCompletedOrder(t *testing.T) {
	svc, provider, order, credited := newTelegramPaymentGateTest(t, true)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		require.NoError(t, svc.HandlePaymentNotification(ctx, telegramPaymentTestNotification(order), payment.TypeEasyPay))
	}
	require.Equal(t, order.Amount, *credited)
	stored, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, stored.Status)
	require.Equal(t, []string{order.OutTradeNo}, provider.queries)
}

func TestTelegramPaymentGateComparesUpstreamAmountDirectlyWithOrder(t *testing.T) {
	svc, provider, order, credited := newTelegramPaymentGateTest(t, true)
	notification := telegramPaymentTestNotification(order)
	notification.Amount = order.PayAmount - 0.009
	provider.result.Amount = order.PayAmount - 0.018
	require.Error(t, svc.HandlePaymentNotification(context.Background(), notification, payment.TypeEasyPay))
	require.Zero(t, *credited)
	require.True(t, svc.hasAuditLog(context.Background(), order.ID, "PAYMENT_UPSTREAM_AMOUNT_MISMATCH"))
}

func TestTelegramPersistenceFailureDoesNotMarkCreditedPaymentFailed(t *testing.T) {
	svc, _, order, credited := newTelegramPaymentGateTest(t, false)
	ctx := context.Background()
	_, err := svc.telegramService.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	// This SQLite fixture has no outbox table. The local enqueue therefore fails
	// after redeeming, while the payment lease must remain safe to retry.
	err = svc.HandlePaymentNotification(ctx, telegramPaymentTestNotification(order), payment.TypeEasyPay)
	require.ErrorIs(t, err, errPaymentCompletionPending)
	require.Equal(t, order.Amount, *credited)
	stored, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, stored.Status)
	require.Nil(t, stored.FailedAt)
	require.Nil(t, stored.CompletedAt)
	_, err = svc.telegramService.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: false})
	require.NoError(t, err)
	require.NoError(t, svc.HandlePaymentNotification(ctx, telegramPaymentTestNotification(order), payment.TypeEasyPay))
	require.Equal(t, order.Amount, *credited, "recovering completion must not credit twice")
	stored, err = svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, stored.Status)
}

func TestTelegramPaymentGateDefaultsOffAndDoesNotRepeatTrustedQuery(t *testing.T) {
	for _, fromQuery := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "queried upstream"}[fromQuery], func(t *testing.T) {
			svc, provider, order, credited := newTelegramPaymentGateTest(t, fromQuery)
			provider.err = errors.New("query must not be called")
			require.NoError(t, svc.handlePaymentNotification(context.Background(), telegramPaymentTestNotification(order), payment.TypeEasyPay, fromQuery))
			require.Empty(t, provider.queries)
			require.Equal(t, order.Amount, *credited)
		})
	}
}

func TestTelegramPaymentGateRejectsUnreadableSettingsBeforeCredit(t *testing.T) {
	svc, provider, order, credited := newTelegramPaymentGateTest(t, true)
	svc.telegramService.settingRepo.(*telegramTestSettingsRepo).err = errors.New("database failure")
	require.Error(t, svc.HandlePaymentNotification(context.Background(), telegramPaymentTestNotification(order), payment.TypeEasyPay))
	require.Empty(t, provider.queries)
	require.Zero(t, *credited)
}

func TestTelegramVerifyOrderPublicMethodUsesOneQueryAndEnforcesOwnership(t *testing.T) {
	for _, correctAmount := range []bool{true, false} {
		t.Run(map[bool]string{true: "paid", false: "mismatched amount"}[correctAmount], func(t *testing.T) {
			svc, provider, order, credited := newTelegramPaymentGateTest(t, true)
			ctx := context.Background()
			_, err := svc.VerifyOrderByOutTradeNo(ctx, order.OutTradeNo, order.UserID+1)
			require.Error(t, err)
			require.Empty(t, provider.queries)
			if !correctAmount {
				provider.result.Amount = order.PayAmount - 1
			}
			result, err := svc.VerifyOrderByOutTradeNo(ctx, order.OutTradeNo, order.UserID)
			require.NoError(t, err)
			require.Equal(t, []string{order.OutTradeNo}, provider.queries)
			if correctAmount {
				require.Equal(t, OrderStatusCompleted, result.Status)
				require.Equal(t, order.Amount, *credited)
			} else {
				require.Equal(t, OrderStatusPending, result.Status)
				require.Zero(t, *credited)
			}
		})
	}
}

func TestTelegramPaymentEventSeparatesPaymentFromCreditAmounts(t *testing.T) {
	when := time.Now()
	o := &dbent.PaymentOrder{ID: 42, UserID: 7, Amount: 12, BonusAmount: 2, PayAmount: 71, UserName: "user", OutTradeNo: "sub2_one"}
	success := telegramPaymentEventForOrder(o, TelegramOutcomeRechargeSuccess, when, "passed")
	require.Equal(t, "order:42:recharge_success", success.EventKey)
	require.Equal(t, 71.0, success.PayAmount)
	require.Equal(t, 12.0, success.CreditedAmount)
	require.Equal(t, 2.0, success.BonusAmount)
	require.Equal(t, when, success.OccurredAt)
	review := telegramPaymentEventForOrder(o, TelegramOutcomePaymentReview, when, "review")
	require.Zero(t, review.CreditedAmount)
	require.Zero(t, review.BonusAmount)
}

func TestTelegramRejectedCallbackOnlyAlertsCurrentUnpaidOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingEasyPayFulfillmentOrder(t, ctx, client)
	telegram, _ := newTelegramTestService(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	telegram.db = db
	_, err = telegram.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	svc := &PaymentService{entClient: client, telegramService: telegram}
	mock.ExpectExec(regexp.QuoteMeta(telegramOutboxInsertSQL)).
		WithArgs("order:"+fmt.Sprint(order.ID)+":review:PAYMENT_CALLBACK_REJECTED", order.ID, sqlmock.AnyArg(), "-1001234567890").
		WillReturnResult(sqlmock.NewResult(1, 1))
	svc.RecordRejectedPaymentCallback(ctx, payment.TypeEasyPay, order.OutTradeNo)
	require.NoError(t, mock.ExpectationsWereMet())

	// Unknown, wrong-provider, old and already-paid orders never create alerts.
	svc.RecordRejectedPaymentCallback(ctx, payment.TypeEasyPay, "sub2_unknown")
	svc.RecordRejectedPaymentCallback(ctx, payment.TypeStripe, order.OutTradeNo)
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetExpiresAt(time.Now().Add(-time.Hour)).Save(ctx)
	require.NoError(t, err)
	svc.RecordRejectedPaymentCallback(ctx, payment.TypeEasyPay, order.OutTradeNo)
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetExpiresAt(time.Now().Add(time.Hour)).SetStatus(OrderStatusCompleted).SetPaidAt(time.Now()).Save(ctx)
	require.NoError(t, err)
	svc.RecordRejectedPaymentCallback(ctx, payment.TypeEasyPay, order.OutTradeNo)
	require.NoError(t, mock.ExpectationsWereMet())
	user, err := client.User.Get(ctx, order.UserID)
	require.NoError(t, err)
	require.Equal(t, StatusActive, user.Status)
}
