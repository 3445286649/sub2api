//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestTelegramPaymentCompletionAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db := newTelegramTestPostgres(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	migration, err := migrations.FS.ReadFile("242_payment_telegram_outbox.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	telegram, settingsRepo := newTelegramTestService(t)
	telegram.db = db
	_, err = telegram.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	payment := &PaymentService{entClient: client, telegramService: telegram}
	payer, err := client.User.Create().SetEmail("payment@example.test").SetPasswordHash("disabled-test-fixture").SetStatus(StatusDisabled).Save(ctx)
	require.NoError(t, err)
	createOrder := func(orderNo string) *dbent.PaymentOrder {
		o, err := client.PaymentOrder.Create().SetUserID(payer.ID).SetUserEmail("payment@example.test").SetUserName("test payer").
			SetAmount(1.2).SetPayAmount(7.2).SetBonusAmount(0.2).SetPaymentType("alipay").SetPaymentTradeNo("upstream-" + orderNo).
			SetRechargeCode("recharge-" + orderNo).SetOutTradeNo(orderNo).SetExpiresAt(time.Now().Add(time.Hour)).
			SetStatus(OrderStatusRecharging).SetPaidAt(time.Now()).SetClientIP("127.0.0.1").SetSrcHost("payment.example.test").Save(ctx)
		require.NoError(t, err)
		return o
	}
	order := createOrder("completed-one")
	lease := &paymentFulfillmentLease{version: order.UpdatedAt}
	require.NoError(t, payment.markCompleted(ctx, order, lease, "RECHARGE_SUCCESS"))
	completed, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletedAt)
	var events int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox WHERE order_id=$1`, order.ID).Scan(&events))
	require.Equal(t, 1, events)

	// Duplicate completion and notification callbacks preserve the single event.
	require.NoError(t, payment.markCompleted(ctx, order, lease, "RECHARGE_SUCCESS"))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox WHERE order_id=$1`, order.ID).Scan(&events))
	require.Equal(t, 1, events)

	// A local persistence error rolls the order completion back together with
	// its notification. The existing redeem idempotency path can safely retry.
	_, err = db.ExecContext(ctx, `CREATE FUNCTION reject_telegram_fixture() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN RAISE EXCEPTION 'fixture outbox failure'; END $$;
        CREATE TRIGGER reject_telegram_fixture BEFORE INSERT ON payment_telegram_outbox
        FOR EACH ROW EXECUTE FUNCTION reject_telegram_fixture()`)
	require.NoError(t, err)
	retrying := createOrder("retry-one")
	err = payment.markCompleted(ctx, retrying, &paymentFulfillmentLease{version: retrying.UpdatedAt}, "RECHARGE_SUCCESS")
	require.ErrorContains(t, err, "persist payment notification")
	failed, err := client.PaymentOrder.Get(ctx, retrying.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRecharging, failed.Status)
	require.Nil(t, failed.CompletedAt)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox WHERE order_id=$1`, retrying.ID).Scan(&events))
	require.Equal(t, 0, events)
	_, err = db.ExecContext(ctx, `DROP TRIGGER reject_telegram_fixture ON payment_telegram_outbox; DROP FUNCTION reject_telegram_fixture()`)
	require.NoError(t, err)
	require.NoError(t, payment.markCompleted(ctx, retrying, &paymentFulfillmentLease{version: retrying.UpdatedAt}, "RECHARGE_SUCCESS"))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox WHERE order_id=$1`, retrying.ID).Scan(&events))
	require.Equal(t, 1, events)

	// The completion event stays durable even when reading the settings fails.
	// A recovered worker resolves its empty target without scanning old orders.
	deferred := createOrder("deferred-one")
	settingsRepo.err = errors.New("transient settings read failure")
	require.NoError(t, payment.markCompleted(ctx, deferred, &paymentFulfillmentLease{version: deferred.UpdatedAt}, "RECHARGE_SUCCESS"))
	settingsRepo.err = nil
	var deferredTarget string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT chat_id_snapshot FROM payment_telegram_outbox WHERE order_id=$1`, deferred.ID).Scan(&deferredTarget))
	require.Empty(t, deferredTarget)
	completed, err = client.PaymentOrder.Get(ctx, deferred.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completed.Status)
}
