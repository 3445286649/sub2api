//go:build integration

package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func newTelegramTestPostgres(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("telegram_outbox"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(8)
	require.NoError(t, db.PingContext(ctx))
	return db
}

func TestTelegramOutboxPostgresLeasePersistenceAndIdempotency(t *testing.T) {
	ctx := context.Background()
	db := newTelegramTestPostgres(t)
	var err error
	_, err = db.ExecContext(ctx, `CREATE TABLE payment_orders (id BIGINT PRIMARY KEY); INSERT INTO payment_orders(id) VALUES (10),(11),(12)`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("242_payment_telegram_outbox.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)

	svc, repo := newTelegramTestService(t)
	svc.db = db
	_, err = svc.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	event := telegramTestPaymentEvent()

	// An uncommitted completion must not leave a notification behind.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, svc.EnqueueTx(ctx, tx, event))
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox`).Scan(&count))
	require.Equal(t, 0, count)

	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, svc.EnqueueTx(ctx, tx, event))
	require.NoError(t, svc.EnqueueTx(ctx, tx, event))
	require.NoError(t, tx.Commit())
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM payment_telegram_outbox`).Scan(&count))
	require.Equal(t, 1, count)
	var payload string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT payload::text FROM payment_telegram_outbox`).Scan(&payload))
	require.NotContains(t, payload, telegramTestToken)

	// Recreating the service sees the persisted task. Two competing workers can
	// only receive one unexpired lease, then a later worker can recover it.
	other := NewTelegramNotificationService(repo, telegramTestCipher{}, db)
	start := make(chan struct{})
	results := make(chan *telegramOutboxTask, 2)
	claimErrors := make(chan error, 2)
	var workers sync.WaitGroup
	now := time.Now().UTC()
	for _, worker := range []*TelegramNotificationService{svc, other} {
		workers.Add(1)
		go func(worker *TelegramNotificationService) {
			defer workers.Done()
			<-start
			task, err := worker.claimTask(ctx, now)
			results <- task
			claimErrors <- err
		}(worker)
	}
	close(start)
	workers.Wait()
	close(results)
	close(claimErrors)
	for err := range claimErrors {
		require.NoError(t, err)
	}
	var claimed *telegramOutboxTask
	claims := 0
	for task := range results {
		if task != nil {
			claimed, claims = task, claims+1
		}
	}
	require.Equal(t, 1, claims)
	recovered, err := other.claimTask(ctx, now.Add(telegramOutboxLease+time.Second))
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.Equal(t, claimed.id, recovered.id)
	require.NotEqual(t, claimed.lease, recovered.lease)
	require.Equal(t, 2, recovered.attempts)
	require.ErrorContains(t, svc.finishTask(ctx, claimed, nil, now), "lease was lost")
	require.NoError(t, other.finishTask(ctx, recovered, nil, now))
	var status string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM payment_telegram_outbox`).Scan(&status))
	require.Equal(t, "sent", status)
	noTask, err := svc.claimTask(ctx, now.Add(time.Hour))
	require.NoError(t, err)
	require.Nil(t, noTask)

	// Target groups are snapshotted with the event, so configuration changes
	// cannot unexpectedly redirect an already queued payment notification.
	event.OrderID, event.EventKey = 11, "payment:11:completed"
	require.NoError(t, svc.Enqueue(ctx, event))
	_, err = svc.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, ChatID: "-1009999999999"})
	require.NoError(t, err)
	next, err := other.claimTask(ctx, time.Now().UTC().Add(time.Second))
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, "-1001234567890", next.chatID)
	require.NoError(t, other.finishTask(ctx, next, &telegramSendError{code: 429, retryAfter: 45 * time.Second}, now))
	tooEarly, err := svc.claimTask(ctx, now.Add(44*time.Second))
	require.NoError(t, err)
	require.Nil(t, tooEarly)
	ready, err := svc.claimTask(ctx, now.Add(46*time.Second))
	require.NoError(t, err)
	require.NotNil(t, ready)
	require.NoError(t, other.finishTask(ctx, ready, nil, now))

	// A fresh event survives a settings outage and resolves its destination
	// once configuration is healthy; afterwards that resolved group is fixed.
	repo.err = errors.New("temporary settings database failure")
	event.OrderID, event.EventKey = 12, "payment:12:completed"
	require.NoError(t, svc.Enqueue(ctx, event))
	var target string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT chat_id_snapshot FROM payment_telegram_outbox WHERE order_id=12`).Scan(&target))
	require.Empty(t, target)
	repo.err = nil
	deferred, err := other.claimTaskForTarget(ctx, time.Now().UTC().Add(time.Second), "-1009999999999")
	require.NoError(t, err)
	require.NotNil(t, deferred)
	require.Equal(t, "-1009999999999", deferred.chatID)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT chat_id_snapshot FROM payment_telegram_outbox WHERE order_id=12`).Scan(&target))
	require.Equal(t, "-1009999999999", target)
}
