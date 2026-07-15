package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountSchedulingHoldRepository struct {
	db             *sql.DB
	accountRepo    service.AccountRepository
	schedulerCache service.SchedulerCache
}

func NewAccountSchedulingHoldRepository(
	db *sql.DB,
	accountRepo service.AccountRepository,
	schedulerCache service.SchedulerCache,
) service.AccountSchedulingHoldRepository {
	return &accountSchedulingHoldRepository{db: db, accountRepo: accountRepo, schedulerCache: schedulerCache}
}

func (r *accountSchedulingHoldRepository) GetSchedulingState(ctx context.Context, accountID int64, now time.Time) (*service.AccountSchedulingState, error) {
	if r == nil || r.db == nil || r.accountRepo == nil {
		return nil, service.ErrSchedulingHoldUnavailable
	}
	account, err := r.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	state := buildAccountSchedulingState(account, now)
	if err := r.readHold(ctx, accountID, service.AccountSchedulingHoldOwner, now, state); err != nil {
		return nil, err
	}
	probeEnabled := account.HealthProbeEnabled || (state.ExternalHold != nil && state.ExternalHold.Active)
	if err := r.readHealth(ctx, accountID, state, probeEnabled); err != nil {
		return nil, err
	}
	finalizeAccountSchedulingState(state)
	return state, nil
}

func (r *accountSchedulingHoldRepository) PutSchedulingHold(ctx context.Context, command service.AccountSchedulingHoldPut, now time.Time) (*service.AccountSchedulingState, error) {
	if r == nil || r.db == nil || r.accountRepo == nil {
		return nil, service.ErrSchedulingHoldUnavailable
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	defer func() { _ = tx.Rollback() }()

	replayed, err := schedulingHoldCommandReplay(ctx, tx, command.Owner, command.DecisionID, command.AccountID, command.RequestHash)
	if err != nil {
		return nil, err
	}
	if replayed {
		if err := tx.Commit(); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		state, err := r.GetSchedulingState(ctx, command.AccountID, now)
		if state != nil {
			state.IdempotentReplay = true
		}
		return state, err
	}

	account, platform, err := lockSchedulingAccount(ctx, tx, command.AccountID)
	if err != nil {
		return nil, err
	}
	if !account.UpdatedAt.Equal(command.ExpectedAccountUpdatedAt) {
		return nil, service.ErrAccountStateDrift.WithMetadata(map[string]string{
			"current_account_updated_at": account.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	if !account.IsActive() || !account.Schedulable {
		return nil, service.ErrManualSchedulingDisabled
	}

	firstHeldAt := now
	var currentStatus string
	var currentFirstHeldAt time.Time
	var currentLeaseUntil time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT status, first_held_at, lease_until
		FROM account_scheduling_holds
		WHERE account_id = $1 AND owner = $2
		FOR UPDATE
	`, command.AccountID, command.Owner).Scan(&currentStatus, &currentFirstHeldAt, &currentLeaseUntil)
	continuingActiveHold := err == nil && currentStatus == "active" && currentLeaseUntil.After(now)
	if continuingActiveHold && currentFirstHeldAt.Before(now) {
		firstHeldAt = currentFirstHeldAt
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}

	groupIDs, err := lockSchedulingCapacityScopes(ctx, tx, command.AccountID, platform)
	if err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	// Renewing an already-active hold does not remove additional capacity. New
	// holds and expired-hold replacements must still pass the capacity guard.
	if !continuingActiveHold {
		if err := requireSchedulingCapacity(ctx, tx, command.AccountID, platform, groupIDs, now); err != nil {
			return nil, err
		}
	}
	if command.LeaseUntil.Sub(firstHeldAt) > command.MaximumCumulativeLease {
		return nil, service.ErrLeaseOutOfRange
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_scheduling_holds (
			account_id, owner, decision_id, reason_code, request_hash, lease_until,
			status, expected_account_updated_at, first_held_at, created_at, updated_at, released_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7, $8, $9, $9, NULL, 1)
		ON CONFLICT (account_id, owner) DO UPDATE SET
			decision_id = EXCLUDED.decision_id,
			reason_code = EXCLUDED.reason_code,
			request_hash = EXCLUDED.request_hash,
			lease_until = EXCLUDED.lease_until,
			status = 'active',
			expected_account_updated_at = EXCLUDED.expected_account_updated_at,
			first_held_at = $8,
			updated_at = $9,
			released_at = NULL,
			version = account_scheduling_holds.version + 1
	`, command.AccountID, command.Owner, command.DecisionID, command.ReasonCode, command.RequestHash,
		command.LeaseUntil, command.ExpectedAccountUpdatedAt, firstHeldAt, now); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET external_scheduling_hold_until = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, command.AccountID, command.LeaseUntil); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_health_states (account_id, next_probe_at, created_at, updated_at)
		VALUES ($1, $2, $2, $2)
		ON CONFLICT (account_id) DO UPDATE SET
			next_probe_at = LEAST(COALESCE(account_health_states.next_probe_at, EXCLUDED.next_probe_at), EXCLUDED.next_probe_at),
			updated_at = EXCLUDED.updated_at
	`, command.AccountID, now); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if err := insertSchedulingHoldEvent(ctx, tx, command.AccountID, command.Owner, command.DecisionID, "put", command.RequestHash, command.ReasonCode, &command.LeaseUntil, "active", now); err != nil {
		return nil, err
	}
	if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &command.AccountID, nil, buildSchedulerGroupPayload(groupIDs)); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	r.syncAccountCache(ctx, command.AccountID)
	return r.GetSchedulingState(ctx, command.AccountID, now)
}

func (r *accountSchedulingHoldRepository) ReleaseSchedulingHold(ctx context.Context, command service.AccountSchedulingHoldRelease, now time.Time) (*service.AccountSchedulingState, error) {
	if r == nil || r.db == nil || r.accountRepo == nil {
		return nil, service.ErrSchedulingHoldUnavailable
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	defer func() { _ = tx.Rollback() }()

	replayed, err := schedulingHoldCommandReplay(ctx, tx, command.Owner, command.DecisionID, command.AccountID, command.RequestHash)
	if err != nil {
		return nil, err
	}
	if replayed {
		if err := tx.Commit(); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		state, err := r.GetSchedulingState(ctx, command.AccountID, now)
		if state != nil {
			state.IdempotentReplay = true
		}
		return state, err
	}

	_, _, err = lockSchedulingAccount(ctx, tx, command.AccountID)
	if err != nil {
		return nil, err
	}
	var currentDecision, status string
	var leaseUntil time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT decision_id, status, lease_until
		FROM account_scheduling_holds
		WHERE account_id = $1 AND owner = $2
		FOR UPDATE
	`, command.AccountID, command.Owner).Scan(&currentDecision, &status, &leaseUntil)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}

	result := "noop"
	changed := false
	if err == nil && status == "active" && leaseUntil.After(now) {
		if currentDecision != command.ExpectedHoldDecisionID {
			return nil, service.ErrHoldReleaseConflict.WithMetadata(map[string]string{"current_hold_decision_id": currentDecision})
		}
		result = "released"
		changed = true
		if _, err := tx.ExecContext(ctx, `
			UPDATE account_scheduling_holds
			SET status = 'released', released_at = $3, updated_at = $3, version = version + 1
			WHERE account_id = $1 AND owner = $2
		`, command.AccountID, command.Owner, now); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
	} else if err == nil && status == "active" {
		changed = true
		result = "expired"
		if _, err := tx.ExecContext(ctx, `
			UPDATE account_scheduling_holds
			SET status = 'expired', updated_at = $3, version = version + 1
			WHERE account_id = $1 AND owner = $2
		`, command.AccountID, command.Owner, now); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
	}
	if changed {
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET external_scheduling_hold_until = NULL WHERE id = $1`, command.AccountID); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
	}
	if err := insertSchedulingHoldEvent(ctx, tx, command.AccountID, command.Owner, command.DecisionID, "release", command.RequestHash, "", nil, result, now); err != nil {
		return nil, err
	}
	if changed {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &command.AccountID, nil, nil); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if changed {
		r.syncAccountCache(ctx, command.AccountID)
	}
	return r.GetSchedulingState(ctx, command.AccountID, now)
}

func (r *accountSchedulingHoldRepository) ExpireSchedulingHolds(ctx context.Context, owner string, now time.Time, limit int) ([]int64, error) {
	if r == nil || r.db == nil || limit <= 0 {
		return nil, service.ErrSchedulingHoldUnavailable
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
		SELECT account_id, decision_id, lease_until
		FROM account_scheduling_holds
		WHERE owner = $1 AND status = 'active' AND lease_until <= $2
		ORDER BY lease_until ASC, account_id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $3
	`, owner, now, limit)
	if err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	type expiredHold struct {
		accountID  int64
		decisionID string
		leaseUntil time.Time
	}
	var expired []expiredHold
	for rows.Next() {
		var item expiredHold
		if err := rows.Scan(&item.accountID, &item.decisionID, &item.leaseUntil); err != nil {
			_ = rows.Close()
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		expired = append(expired, item)
	}
	if err := rows.Close(); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if err := rows.Err(); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	ids := make([]int64, 0, len(expired))
	for _, item := range expired {
		if _, err := tx.ExecContext(ctx, `
			UPDATE account_scheduling_holds
			SET status = 'expired', updated_at = $3, version = version + 1
			WHERE account_id = $1 AND owner = $2 AND status = 'active'
		`, item.accountID, owner, now); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE accounts
			SET external_scheduling_hold_until = NULL
			WHERE id = $1 AND external_scheduling_hold_until <= $2
		`, item.accountID, now); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		expireDecisionID := schedulingHoldExpiryDecisionID(item.accountID, item.decisionID, item.leaseUntil)
		requestHash := schedulingHoldEventHash("expire", item.accountID, item.decisionID, item.leaseUntil.UTC().Format(time.RFC3339Nano))
		if err := insertSchedulingHoldEvent(ctx, tx, item.accountID, owner, expireDecisionID, "expire", requestHash, "", &item.leaseUntil, "expired", now); err != nil {
			return nil, err
		}
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &item.accountID, nil, nil); err != nil {
			return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		ids = append(ids, item.accountID)
	}
	if err := tx.Commit(); err != nil {
		return nil, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	for _, accountID := range ids {
		r.syncAccountCache(ctx, accountID)
	}
	return ids, nil
}

func schedulingHoldCommandReplay(ctx context.Context, tx *sql.Tx, owner, decisionID string, accountID int64, requestHash string) (bool, error) {
	var storedAccountID int64
	var storedHash string
	err := tx.QueryRowContext(ctx, `
		SELECT account_id, request_hash
		FROM account_scheduling_hold_events
		WHERE owner = $1 AND decision_id = $2
	`, owner, decisionID).Scan(&storedAccountID, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if storedAccountID != accountID || storedHash != requestHash {
		return false, service.ErrHoldDecisionConflict
	}
	return true, nil
}

func lockSchedulingAccount(ctx context.Context, tx *sql.Tx, accountID int64) (*service.Account, string, error) {
	var account service.Account
	var platform string
	var deletedAt sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT id, status, schedulable, platform, updated_at, deleted_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`, accountID).Scan(&account.ID, &account.Status, &account.Schedulable, &platform, &account.UpdatedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && deletedAt.Valid) {
		return nil, "", service.ErrAccountNotFound
	}
	if err != nil {
		return nil, "", service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	return &account, platform, nil
}

func lockSchedulingCapacityScopes(ctx context.Context, tx *sql.Tx, accountID int64, platform string) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT group_id FROM account_groups WHERE account_id = $1 ORDER BY group_id ASC`, accountID)
	if err != nil {
		return nil, err
	}
	var groupIDs []int64
	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	if len(groupIDs) == 0 {
		_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "account_scheduling_hold:ungrouped:"+platform)
		return groupIDs, err
	}
	for _, groupID := range groupIDs {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "account_scheduling_hold:group:"+strconv.FormatInt(groupID, 10)); err != nil {
			return nil, err
		}
	}
	return groupIDs, nil
}

func requireSchedulingCapacity(ctx context.Context, tx *sql.Tx, accountID int64, platform string, groupIDs []int64, now time.Time) error {
	if len(groupIDs) == 0 {
		remaining, err := hasQuotaEligibleSchedulingAccount(ctx, tx, `
			SELECT a.type, COALESCE(a.extra, '{}'::jsonb)::text
			FROM accounts a
			WHERE a.id <> $1
			  AND a.deleted_at IS NULL
			  AND a.platform = $2
			  AND a.status = 'active'
			  AND a.schedulable IS TRUE
			  AND NOT EXISTS (SELECT 1 FROM account_groups ag WHERE ag.account_id = a.id)
			  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= $3)
			  AND (a.external_scheduling_hold_until IS NULL OR a.external_scheduling_hold_until <= $3)
			  AND (a.expires_at IS NULL OR a.expires_at > $3 OR a.auto_pause_on_expired IS FALSE)
			  AND (a.overload_until IS NULL OR a.overload_until <= $3)
			  AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= $3)
		`, accountID, platform, now)
		if err != nil {
			return service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		if !remaining {
			return service.ErrCapacityGuardBlocked.WithMetadata(map[string]string{"scope": "ungrouped", "platform": platform})
		}
		return nil
	}
	for _, groupID := range groupIDs {
		remaining, err := hasQuotaEligibleSchedulingAccount(ctx, tx, `
			SELECT a.type, COALESCE(a.extra, '{}'::jsonb)::text
			FROM account_groups ag
			JOIN accounts a ON a.id = ag.account_id
			WHERE ag.group_id = $1
			  AND a.id <> $2
			  AND a.deleted_at IS NULL
			  AND a.status = 'active'
			  AND a.schedulable IS TRUE
			  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= $3)
			  AND (a.external_scheduling_hold_until IS NULL OR a.external_scheduling_hold_until <= $3)
			  AND (a.expires_at IS NULL OR a.expires_at > $3 OR a.auto_pause_on_expired IS FALSE)
			  AND (a.overload_until IS NULL OR a.overload_until <= $3)
			  AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= $3)
		`, groupID, accountID, now)
		if err != nil {
			return service.ErrSchedulingHoldUnavailable.WithCause(err)
		}
		if !remaining {
			return service.ErrCapacityGuardBlocked.WithMetadata(map[string]string{"group_id": strconv.FormatInt(groupID, 10)})
		}
	}
	return nil
}

func hasQuotaEligibleSchedulingAccount(ctx context.Context, tx *sql.Tx, query string, args ...any) (bool, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var account service.Account
		var extraRaw string
		if err := rows.Scan(&account.Type, &extraRaw); err != nil {
			return false, err
		}
		if extraRaw != "" && extraRaw != "null" {
			if err := json.Unmarshal([]byte(extraRaw), &account.Extra); err != nil {
				return false, err
			}
		}
		if !account.IsAPIKeyOrBedrock() || !account.IsQuotaExceeded() {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func insertSchedulingHoldEvent(ctx context.Context, tx *sql.Tx, accountID int64, owner, decisionID, command, requestHash, reason string, leaseUntil *time.Time, result string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO account_scheduling_hold_events (
			account_id, owner, decision_id, command, request_hash, reason_code, lease_until, result_status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, accountID, owner, decisionID, command, requestHash, reason, leaseUntil, result, now)
	if err != nil {
		return service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	return nil
}

func (r *accountSchedulingHoldRepository) readHold(ctx context.Context, accountID int64, owner string, now time.Time, state *service.AccountSchedulingState) error {
	var hold service.AccountSchedulingExternalHold
	err := r.db.QueryRowContext(ctx, `
		SELECT owner, decision_id, reason_code, status, lease_until, version
		FROM account_scheduling_holds
		WHERE account_id = $1 AND owner = $2
	`, accountID, owner).Scan(&hold.Owner, &hold.DecisionID, &hold.ReasonCode, &hold.Status, &hold.LeaseUntil, &hold.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	hold.Active = hold.Status == "active" && hold.LeaseUntil.After(now)
	if hold.Status == "active" && !hold.Active {
		hold.Status = "expired"
	}
	state.ExternalHold = &hold
	return nil
}

func (r *accountSchedulingHoldRepository) readHealth(ctx context.Context, accountID int64, state *service.AccountSchedulingState, probeEnabled bool) error {
	var health service.AccountSchedulingHealthEvidence
	var lastChecked, nextProbe sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT score, status, consecutive_successes, last_checked_at, next_probe_at
		FROM account_health_states
		WHERE account_id = $1
	`, accountID).Scan(&health.Score, &health.Status, &health.ConsecutiveSuccesses, &lastChecked, &nextProbe)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return service.ErrSchedulingHoldUnavailable.WithCause(err)
	}
	if lastChecked.Valid {
		value := lastChecked.Time
		health.LastCheckedAt = &value
	}
	if nextProbe.Valid {
		value := nextProbe.Time
		health.NextProbeAt = &value
	}
	health.ProbeEnabled = probeEnabled
	state.Health = &health
	return nil
}

func buildAccountSchedulingState(account *service.Account, now time.Time) *service.AccountSchedulingState {
	state := &service.AccountSchedulingState{
		AccountID:            account.ID,
		AccountUpdatedAt:     account.UpdatedAt,
		ManualSchedulable:    account.Schedulable,
		InternalReasonCodes:  []string{},
		EffectiveReasonCodes: []string{},
	}
	if !account.IsActive() {
		state.InternalReasonCodes = append(state.InternalReasonCodes, "status_inactive")
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		state.InternalReasonCodes = append(state.InternalReasonCodes, "expired")
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		state.InternalReasonCodes = append(state.InternalReasonCodes, "overloaded")
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		state.InternalReasonCodes = append(state.InternalReasonCodes, "rate_limited")
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		state.InternalReasonCodes = append(state.InternalReasonCodes, "temp_unschedulable")
	}
	if account.IsAPIKeyOrBedrock() && account.IsQuotaExceeded() {
		state.InternalReasonCodes = append(state.InternalReasonCodes, "quota_exceeded")
	}
	state.InternalBlocked = len(state.InternalReasonCodes) > 0
	return state
}

func finalizeAccountSchedulingState(state *service.AccountSchedulingState) {
	if state == nil {
		return
	}
	reasons := make([]string, 0, len(state.InternalReasonCodes)+2)
	if !state.ManualSchedulable {
		reasons = append(reasons, "manual_unschedulable")
	}
	reasons = append(reasons, state.InternalReasonCodes...)
	if state.ExternalHold != nil && state.ExternalHold.Active {
		reasons = append(reasons, "external_hold")
	}
	state.EffectiveReasonCodes = reasons
	state.EffectiveSchedulable = len(reasons) == 0
}

func (r *accountSchedulingHoldRepository) syncAccountCache(ctx context.Context, accountID int64) {
	if r.schedulerCache == nil || r.accountRepo == nil {
		return
	}
	account, err := r.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("repository.account_scheduling_hold", "[SchedulingHold] refresh account cache read failed account_id=%d err=%v", accountID, err)
		return
	}
	if err := r.schedulerCache.SetAccount(ctx, account); err != nil {
		logger.LegacyPrintf("repository.account_scheduling_hold", "[SchedulingHold] refresh account cache write failed account_id=%d err=%v", accountID, err)
	}
}

func schedulingHoldExpiryDecisionID(accountID int64, decisionID string, leaseUntil time.Time) string {
	hash := schedulingHoldEventHash(accountID, decisionID, leaseUntil.UTC().Format(time.RFC3339Nano))
	return "expire-" + hash[:32]
}

func schedulingHoldEventHash(parts ...any) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, fmt.Sprint(part))
	}
	return schedulingHoldHash(strings.Join(values, "\x00"))
}

func schedulingHoldHash(value string) string {
	// Keep this local so repository-generated expiry decisions do not depend on
	// the service request hash implementation.
	return fmt.Sprintf("%x", sha256Sum([]byte(value)))
}

func sha256Sum(value []byte) [32]byte {
	return sha256.Sum256(value)
}
