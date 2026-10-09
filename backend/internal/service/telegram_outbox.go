package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const telegramOutboxLease = 2 * time.Minute

const telegramOutboxInsertSQL = `INSERT INTO payment_telegram_outbox
    (event_key, order_id, payload, chat_id_snapshot)
    VALUES ($1, $2, $3::jsonb, $4)
    ON CONFLICT (event_key) DO NOTHING`

const telegramOutboxClaimSQL = `WITH next_task AS (
    SELECT id FROM payment_telegram_outbox
    WHERE (status = 'pending' AND available_at <= $1)
       OR (status = 'sending' AND leased_until <= $1)
    ORDER BY available_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE payment_telegram_outbox AS task
SET status = 'sending', attempts = task.attempts + 1,
    leased_until = $2, lease_token = $3,
    chat_id_snapshot = CASE WHEN task.chat_id_snapshot = '' THEN $4 ELSE task.chat_id_snapshot END
FROM next_task
WHERE task.id = next_task.id
RETURNING task.id, task.payload, task.chat_id_snapshot, task.attempts`

type telegramOutboxTask struct {
	id       int64
	event    TelegramPaymentEvent
	chatID   string
	attempts int
	lease    string
}

// EnqueueTx is called in the same transaction that records payment completion.
// Notifications enabled later intentionally do not backfill old payment orders.
func (s *TelegramNotificationService) EnqueueTx(ctx context.Context, tx TelegramOutboxExecutor, event TelegramPaymentEvent) error {
	if s == nil {
		return nil
	}
	if err := validateTelegramPaymentEvent(event); err != nil {
		return err
	}
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		// Keep the new event for configuration recovery instead of losing it.
		return s.enqueueForTargetTx(ctx, tx, event, "")
	}
	if !cfg.Enabled {
		return nil
	}
	if tx == nil {
		return errors.New("telegram outbox transaction is unavailable")
	}
	return s.enqueueForTargetTx(ctx, tx, event, cfg.ChatID)
}

func (s *TelegramNotificationService) enqueueForTargetTx(ctx context.Context, tx TelegramOutboxExecutor, event TelegramPaymentEvent, chatID string) error {
	if err := validateTelegramPaymentEvent(event); err != nil {
		return err
	}
	if tx == nil {
		return errors.New("telegram outbox transaction is unavailable")
	}
	if chatID != "" {
		if err := validateTelegramGroupID(chatID); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return errors.New("unable to encode telegram payment event")
	}
	if _, err := tx.ExecContext(ctx, telegramOutboxInsertSQL, event.EventKey, event.OrderID, string(payload), chatID); err != nil {
		return fmt.Errorf("enqueue telegram payment event: %w", err)
	}
	return nil
}

func (s *TelegramNotificationService) Enqueue(ctx context.Context, event TelegramPaymentEvent) error {
	if s == nil {
		return nil
	}
	if s.db == nil {
		return errors.New("telegram outbox database is unavailable")
	}
	return s.EnqueueTx(ctx, s.db, event)
}

func (s *TelegramNotificationService) Start() {
	if s == nil || s.db == nil {
		return
	}
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if s.workerCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.workerCancel, s.workerDone = cancel, make(chan struct{})
	go s.runWorker(ctx, s.workerDone)
}

func (s *TelegramNotificationService) Stop() {
	if s == nil {
		return
	}
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if s.workerCancel == nil {
		return
	}
	s.workerCancel()
	<-s.workerDone
	s.workerCancel, s.workerDone = nil, nil
}

func (s *TelegramNotificationService) runWorker(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.processReady(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("telegram payment notification worker paused", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *TelegramNotificationService) processReady(ctx context.Context) error {
	for i := 0; i < 1; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		cfg, err := s.loadSettings(ctx)
		if err != nil {
			return err
		}
		if !cfg.Enabled {
			return nil
		}
		if err := validateTelegramGroupID(cfg.ChatID); err != nil {
			return err
		}
		token, err := s.botToken(cfg)
		if err != nil {
			return err
		}
		task, err := s.claimTaskForTarget(ctx, time.Now().UTC(), cfg.ChatID)
		if err != nil || task == nil {
			return err
		}
		var deliveryErr error
		if err := validateTelegramPaymentEvent(task.event); err != nil {
			deliveryErr = &telegramSendError{permanent: true}
		} else if validateTelegramGroupID(task.chatID) != nil {
			deliveryErr = &telegramSendError{permanent: true}
		} else {
			deliveryErr = s.sendMessage(ctx, token, task.chatID, formatTelegramPaymentEvent(task.event))
		}
		// A cancelled request has uncertain delivery; its lease expires for the next worker.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.finishTask(ctx, task, deliveryErr, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (s *TelegramNotificationService) claimTask(ctx context.Context, now time.Time) (*telegramOutboxTask, error) {
	return s.claimTaskForTarget(ctx, now, "")
}

func (s *TelegramNotificationService) claimTaskForTarget(ctx context.Context, now time.Time, chatID string) (*telegramOutboxTask, error) {
	lease := uuid.NewString()
	var task telegramOutboxTask
	var payload []byte
	err := s.db.QueryRowContext(ctx, telegramOutboxClaimSQL, now, now.Add(telegramOutboxLease), lease, chatID).
		Scan(&task.id, &payload, &task.chatID, &task.attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("unable to claim telegram notification task")
	}
	task.lease = lease
	if json.Unmarshal(payload, &task.event) != nil {
		// Preserve the claimed row so its invalid payload can be marked terminal.
		task.event = TelegramPaymentEvent{}
	}
	return &task, nil
}

func telegramRetryDelay(attempts int, err error) time.Duration {
	var failed *telegramSendError
	if errors.As(err, &failed) && failed.retryAfter > 0 {
		return failed.retryAfter
	}
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 10 {
		attempts = 10
	}
	delay := time.Duration(1<<uint(attempts-1)) * 10 * time.Second
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func (s *TelegramNotificationService) finishTask(ctx context.Context, task *telegramOutboxTask, deliveryErr error, now time.Time) error {
	status, availableAt := "sent", now
	var lastError any
	var sentAt any = now
	if deliveryErr != nil {
		status, sentAt = "pending", nil
		availableAt = now.Add(telegramRetryDelay(task.attempts, deliveryErr))
		var failed *telegramSendError
		if errors.As(deliveryErr, &failed) {
			if failed.permanent {
				status = "permanent_error"
			}
			lastError = failed.Error()
		} else {
			lastError = "telegram delivery failed"
		}
	}
	result, err := s.db.ExecContext(ctx, `UPDATE payment_telegram_outbox
        SET status = $1, available_at = $2, last_error = $3, sent_at = $4,
            leased_until = NULL, lease_token = NULL
        WHERE id = $5 AND status = 'sending' AND lease_token = $6`,
		status, availableAt, lastError, sentAt, task.id, task.lease)
	if err != nil {
		return errors.New("unable to persist telegram notification delivery result")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("telegram notification task lease was lost")
	}
	return nil
}
