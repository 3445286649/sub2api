package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func telegramTestPaymentEvent() TelegramPaymentEvent {
	return TelegramPaymentEvent{EventKey: "payment:10:completed", OrderID: 10, UserID: 2, Username: "user", Email: "user@example.test", OccurredAt: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), PayAmount: 7.2, CreditedAmount: 1, Currency: "CNY", Outcome: TelegramOutcomeRechargeSuccess}
}

func TestTelegramOutboxDisabledSkipsNewEvents(t *testing.T) {
	svc, _ := newTelegramTestService(t)
	require.NoError(t, svc.EnqueueTx(context.Background(), nil, telegramTestPaymentEvent()))
}

func TestTelegramOutboxSettingsUnavailableDefersTarget(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc, repo := newTelegramTestService(t)
	svc.db = db
	repo.err = errors.New("settings temporarily unavailable")
	event := telegramTestPaymentEvent()
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	mock.ExpectExec(regexp.QuoteMeta(telegramOutboxInsertSQL)).WithArgs(event.EventKey, event.OrderID, string(payload), "").WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, svc.Enqueue(context.Background(), event))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramOutboxNewEventsUniqueAndTransactional(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc, _ := newTelegramTestService(t)
	_, err = svc.UpdateSettings(context.Background(), UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	event := telegramTestPaymentEvent()
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	mock.ExpectExec(regexp.QuoteMeta(telegramOutboxInsertSQL)).WithArgs(event.EventKey, event.OrderID, string(payload), "-1001234567890").WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, svc.EnqueueTx(context.Background(), tx, event))
	mock.ExpectExec(regexp.QuoteMeta(telegramOutboxInsertSQL)).WithArgs(event.EventKey, event.OrderID, string(payload), "-1001234567890").WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, svc.EnqueueTx(context.Background(), tx, event))
	mock.ExpectCommit()
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramOutboxDeferredTargetResolvedOnClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := NewTelegramNotificationService(nil, nil, db)
	payload, err := json.Marshal(telegramTestPaymentEvent())
	require.NoError(t, err)
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(telegramOutboxClaimSQL)).WithArgs(now, now.Add(telegramOutboxLease), sqlmock.AnyArg(), "-1009999999999").
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload", "chat_id_snapshot", "attempts"}).AddRow(1, payload, "-1009999999999", 1))
	task, err := svc.claimTaskForTarget(context.Background(), now, "-1009999999999")
	require.NoError(t, err)
	require.Equal(t, "-1009999999999", task.chatID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramOutboxClaimsExpiredLeaseAndGuardsCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := NewTelegramNotificationService(nil, nil, db)
	event := telegramTestPaymentEvent()
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(telegramOutboxClaimSQL)).WithArgs(now, now.Add(telegramOutboxLease), sqlmock.AnyArg(), "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload", "chat_id_snapshot", "attempts"}).AddRow(1, payload, "-1001234567890", 2))
	task, err := svc.claimTask(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, event, task.event)
	require.Equal(t, 2, task.attempts)
	mock.ExpectExec("UPDATE payment_telegram_outbox").WithArgs("sent", now, nil, now, task.id, task.lease).WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorContains(t, svc.finishTask(context.Background(), task, nil, now), "lease was lost")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramOutboxFailureSchedulesBoundedRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := NewTelegramNotificationService(nil, nil, db)
	now := time.Now().UTC()
	task := &telegramOutboxTask{id: 1, lease: "lease-one", attempts: 2}
	err = &telegramSendError{code: 429, retryAfter: 90 * time.Second}
	mock.ExpectExec("UPDATE payment_telegram_outbox").WithArgs("pending", now.Add(90*time.Second), "telegram delivery failed (code 429)", nil, task.id, task.lease).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.finishTask(context.Background(), task, err, now))
	err = &telegramSendError{code: 401}
	mock.ExpectExec("UPDATE payment_telegram_outbox").WithArgs("pending", now.Add(20*time.Second), "telegram delivery failed (code 401)", nil, task.id, task.lease).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.finishTask(context.Background(), task, err, now))
	require.Equal(t, time.Hour, telegramRetryDelay(100, errors.New("network")))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTelegramOutboxNoReadyTaskReturnsCleanly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := NewTelegramNotificationService(nil, nil, db)
	mock.ExpectQuery(regexp.QuoteMeta(telegramOutboxClaimSQL)).WillReturnError(sql.ErrNoRows)
	task, err := svc.claimTask(context.Background(), time.Now())
	require.NoError(t, err)
	require.Nil(t, task)
	require.NoError(t, mock.ExpectationsWereMet())
}
