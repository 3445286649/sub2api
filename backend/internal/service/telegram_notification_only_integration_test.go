//go:build unit && integration

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestTelegramNotificationOnlyPreservesPaymentAndRetriesDelivery(t *testing.T) {
	ctx := context.Background()
	db := newTelegramTestPostgres(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	migration, err := migrations.FS.ReadFile("242_payment_telegram_outbox.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	order := createPendingEasyPayFulfillmentOrder(t, ctx, client)

	telegram, _ := newTelegramTestService(t)
	telegram.db = db
	_, err = telegram.UpdateSettings(ctx, UpdateTelegramSettingsRequest{
		Enabled: true, VerifyEasyPayOrders: false,
		BotToken: telegramTestToken, ChatID: "-1001234567890",
	})
	require.NoError(t, err)
	deliveries := 0
	telegram.httpClient = telegramTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "api.telegram.org", req.URL.Host)
		deliveries++
		return nil, errors.New("fixture Telegram network failure")
	})

	provider := &telegramPaymentQueryProvider{
		paymentFulfillmentTestProvider: paymentFulfillmentTestProvider{
			key: payment.TypeEasyPay, supportedTypes: []payment.PaymentType{payment.TypeAlipay},
		},
		err: errors.New("order queries are unavailable for this gateway"),
	}
	registry := payment.NewRegistry()
	registry.Register(provider)
	credited := 0.0
	redeemRepo := &paymentFulfillmentRedeemRepo{}
	userRepo := &mockUserRepo{getByIDUser: &User{ID: order.UserID}}
	userRepo.updateBalanceFn = func(_ context.Context, userID int64, amount float64) error {
		require.Equal(t, order.UserID, userID)
		credited += amount
		return nil
	}
	svc := &PaymentService{
		entClient: client, registry: registry, providersLoaded: true, telegramService: telegram,
		redeemService: NewRedeemService(redeemRepo, userRepo, nil, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil),
	}
	notification := telegramPaymentTestNotification(order)
	notification.Metadata = map[string]string{"pid": "pid-expected"}
	for i := 0; i < 2; i++ {
		require.NoError(t, svc.HandlePaymentNotification(ctx, notification, payment.TypeEasyPay))
	}
	require.Empty(t, provider.queries)
	require.Equal(t, order.Amount, credited)
	require.Zero(t, deliveries, "payment callbacks only enqueue notifications")
	completed, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletedAt)
	var events int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox
        WHERE order_id=$1 AND payload->>'outcome'=$2`, order.ID, TelegramOutcomeRechargeSuccess).Scan(&events))
	require.Equal(t, 1, events)

	beforeDelivery := time.Now()
	require.NoError(t, telegram.processReady(ctx))
	require.Equal(t, 1, deliveries)
	var deliveryStatus string
	var attempts int
	var nextDelivery time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status, attempts, available_at
        FROM payment_telegram_outbox WHERE order_id=$1`, order.ID).Scan(&deliveryStatus, &attempts, &nextDelivery))
	require.Equal(t, "pending", deliveryStatus)
	require.Equal(t, 1, attempts)
	require.True(t, nextDelivery.After(beforeDelivery), "network failure schedules a later retry")
	require.NoError(t, telegram.processReady(ctx))
	require.Equal(t, 1, deliveries, "failed delivery is not retried before its scheduled time")
	completed, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completed.Status)
	require.Equal(t, order.Amount, credited)
	require.Empty(t, provider.queries)
}
