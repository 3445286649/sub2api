package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

var _ service.GrokRejectionBillingRepository = (*usageBillingRepository)(nil)

func (r *usageBillingRepository) EnqueueGrokRejection(ctx context.Context, c *service.GrokRejectionCommand) error {
	if c == nil || c.SourceID == "" || c.Usage.UserID <= 0 || c.Usage.APIKeyID <= 0 {
		return errors.New("invalid rejection billing identity")
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return err
	}
	state := "review"
	if c.Receipt.Chargeable() {
		state = "pending"
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO grok_rejection_inbox(event_key,payload,state) VALUES($1,$2,$3) ON CONFLICT(event_key) DO NOTHING`, c.EventKey(), payload, state)
	return err
}

func (r *usageBillingRepository) PendingGrokRejections(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT event_key FROM grok_rejection_inbox WHERE state='pending' AND next_attempt_at<=NOW() ORDER BY next_attempt_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *usageBillingRepository) SettleGrokRejection(ctx context.Context, key string) (_ *service.UsageBillingApplyResult, err error) {
	// Backoff is outside the settlement transaction so repeated failures cannot
	// starve the rest of the durable inbox. The row lock below serializes workers.
	_, err = r.db.ExecContext(ctx, `UPDATE grok_rejection_inbox SET attempts=attempts+1,next_attempt_at=NOW()+INTERVAL '1 minute' WHERE event_key=$1 AND state='pending'`, key)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var payload []byte
	var state string
	err = tx.QueryRowContext(ctx, `SELECT payload,state FROM grok_rejection_inbox WHERE event_key=$1 FOR UPDATE`, key).Scan(&payload, &state)
	if err != nil {
		return nil, err
	}
	if state != "pending" {
		return &service.UsageBillingApplyResult{}, nil
	}
	var c service.GrokRejectionCommand
	if err = json.Unmarshal(payload, &c); err != nil {
		return nil, err
	}
	if !c.Receipt.Chargeable() {
		return nil, errors.New("pending rejection receipt is not chargeable")
	}
	upstream, e := decimal.NewFromString(c.Receipt.UpstreamAmount)
	if e != nil || !upstream.IsPositive() {
		return nil, errors.New("invalid upstream rejection amount")
	}
	amount, e := decimal.NewFromString(c.Receipt.UserAmount)
	if e != nil || !amount.IsPositive() {
		return nil, errors.New("invalid user rejection amount")
	}
	requestID := service.GrokRejectionUsageRequestID(c.SourceID, c.Receipt.BillingRequestID)
	var claimed string
	err = tx.QueryRowContext(ctx, `INSERT INTO grok_rejection_receipts(source_id,billing_request_id,user_id,api_key_id,upstream_amount,user_amount,usage_request_id,evidence)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(source_id,billing_request_id) DO NOTHING RETURNING billing_request_id`, c.SourceID, c.Receipt.BillingRequestID, c.Usage.UserID, c.Usage.APIKeyID, upstream.String(), amount.String(), requestID, payload).Scan(&claimed)
	result := &service.UsageBillingApplyResult{}
	if errors.Is(err, sql.ErrNoRows) {
		var owner int64
		var originalAmount string
		if err = tx.QueryRowContext(ctx, `SELECT user_id,upstream_amount FROM grok_rejection_receipts WHERE source_id=$1 AND billing_request_id=$2`, c.SourceID, c.Receipt.BillingRequestID).Scan(&owner, &originalAmount); err != nil {
			return nil, err
		}
		stored, _ := decimal.NewFromString(originalAmount)
		state = "duplicate"
		if owner != c.Usage.UserID || !stored.Equal(upstream) {
			state = "conflict"
		}
	} else if err != nil {
		return nil, err
	} else {
		cost, _ := amount.Float64()
		upstreamCost, _ := upstream.Float64()
		effects := &service.UsageBillingCommand{UserID: c.Usage.UserID, APIKeyID: c.Usage.APIKeyID, AccountID: c.Usage.AccountID, AccountType: c.AccountType, BalanceCost: cost, APIKeyQuotaCost: cost, APIKeyRateLimitCost: cost, AccountQuotaCost: upstreamCost}
		if err := r.applyUsageBillingEffects(ctx, tx, effects, result); err != nil {
			return nil, err
		}
		result.Applied = true
		result.RejectionUserID = c.Usage.UserID
		result.RejectionAPIKeyID = c.Usage.APIKeyID
		log := c.Usage
		mode, label := service.GrokRejectionBillingMode, service.GrokRejectionBillingLabel
		one := 1.0
		log.RequestID = requestID
		log.BillingMode = &mode
		log.BillingTier = &label
		log.UpstreamRequestID = &c.Receipt.BillingRequestID
		log.BillingType = service.BillingTypeBalance
		log.SubscriptionID = nil
		log.TotalCost = upstreamCost
		log.ActualCost = cost
		log.AccountStatsCost = &upstreamCost
		log.RateMultiplier = 1
		log.AccountRateMultiplier = &one
		// Reuse the complete usage insert shape within the SAME sql.Tx.
		usageRepo := &usageLogRepository{sql: tx}
		inserted, e := usageRepo.createSingle(ctx, tx, &log)
		if e != nil {
			return nil, e
		}
		if !inserted {
			return nil, fmt.Errorf("rejection usage log collision: %s", requestID)
		}
		state = "settled"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE grok_rejection_inbox SET state=$2,updated_at=NOW() WHERE event_key=$1`, key, state); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *usageBillingRepository) BindGrokRejectionTask(ctx context.Context, source, task string, user, key int64) error {
	if task == "" || len(task) > 255 || user <= 0 || key <= 0 {
		return errors.New("invalid rejection task binding")
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO grok_rejection_tasks(source_id,task_id,user_id,api_key_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, source, task, user, key)
	if err != nil {
		return err
	}
	owns, err := r.OwnsGrokRejectionTask(ctx, source, task, user)
	if err != nil {
		return err
	}
	if !owns {
		return errors.New("rejection task ownership conflict")
	}
	return nil
}
func (r *usageBillingRepository) OwnsGrokRejectionTask(ctx context.Context, source, task string, user int64) (bool, error) {
	var owner int64
	err := r.db.QueryRowContext(ctx, `SELECT user_id FROM grok_rejection_tasks WHERE source_id=$1 AND task_id=$2`, source, task).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return owner == user, err
}

func (r *usageBillingRepository) GrokTaskNormalBillingReplaced(ctx context.Context, source, task string, user int64) (bool, error) {
	var replaced bool
	err := r.db.QueryRowContext(ctx, `SELECT normal_billing_replaced FROM grok_rejection_tasks WHERE source_id=$1 AND task_id=$2 AND user_id=$3`, source, task, user).Scan(&replaced)
	return replaced, err
}
func (r *usageBillingRepository) ReplaceGrokTaskNormalBilling(ctx context.Context, source, task string, user int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE grok_rejection_tasks SET normal_billing_replaced=TRUE WHERE source_id=$1 AND task_id=$2 AND user_id=$3`, source, task, user)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("task billing replacement ownership not found")
	}
	return nil
}
