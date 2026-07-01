package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountHealthRepository struct {
	sql sqlExecutor
}

func NewAccountHealthRepository(sqlDB *sql.DB) service.AccountHealthRepository {
	return &accountHealthRepository{sql: sqlDB}
}

func (r *accountHealthRepository) Get(ctx context.Context, accountID int64) (*service.AccountHealthState, error) {
	rows, err := r.sql.QueryContext(ctx, accountHealthSelectSQL+" WHERE account_id = $1", accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	state, err := scanAccountHealthState(rows)
	if err != nil {
		return nil, err
	}
	return state, rows.Err()
}

func (r *accountHealthRepository) ListByAccountIDs(ctx context.Context, ids []int64) (map[int64]*service.AccountHealthState, error) {
	out := make(map[int64]*service.AccountHealthState, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, accountHealthSelectSQL+" WHERE account_id = ANY($1)", pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		state, err := scanAccountHealthState(rows)
		if err != nil {
			return nil, err
		}
		out[state.AccountID] = state
	}
	return out, rows.Err()
}

func (r *accountHealthRepository) Upsert(ctx context.Context, state *service.AccountHealthState) error {
	if state == nil {
		return nil
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_health_states (
			account_id, score, consecutive_successes, consecutive_failures, status,
			last_success_at, last_failure_at, last_checked_at, last_error_category, last_error_message,
			latency_ewma_ms, backoff_level, next_probe_at, isolated_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16
		)
		ON CONFLICT (account_id) DO UPDATE SET
			score = EXCLUDED.score,
			consecutive_successes = EXCLUDED.consecutive_successes,
			consecutive_failures = EXCLUDED.consecutive_failures,
			status = EXCLUDED.status,
			last_success_at = EXCLUDED.last_success_at,
			last_failure_at = EXCLUDED.last_failure_at,
			last_checked_at = EXCLUDED.last_checked_at,
			last_error_category = EXCLUDED.last_error_category,
			last_error_message = EXCLUDED.last_error_message,
			latency_ewma_ms = EXCLUDED.latency_ewma_ms,
			backoff_level = EXCLUDED.backoff_level,
			next_probe_at = EXCLUDED.next_probe_at,
			isolated_at = EXCLUDED.isolated_at,
			updated_at = EXCLUDED.updated_at
	`, state.AccountID, state.Score, state.ConsecutiveSuccesses, state.ConsecutiveFailures, state.Status,
		accountHealthNullTime(state.LastSuccessAt), accountHealthNullTime(state.LastFailureAt), accountHealthNullTime(state.LastCheckedAt), accountHealthNullString(state.LastErrorCategory), accountHealthNullString(state.LastErrorMessage),
		accountHealthNullInt(state.LatencyEWMAMs), state.BackoffLevel, accountHealthNullTime(state.NextProbeAt), accountHealthNullTime(state.IsolatedAt), state.CreatedAt, state.UpdatedAt)
	return err
}

func (r *accountHealthRepository) Delete(ctx context.Context, accountID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM account_health_states WHERE account_id = $1`, accountID)
	return err
}

func (r *accountHealthRepository) ListDueForProbe(ctx context.Context, now time.Time, limit int) ([]*service.AccountHealthState, error) {
	rows, err := r.sql.QueryContext(ctx, accountHealthSelectSQL+`
		JOIN accounts a ON a.id = account_health_states.account_id
		WHERE account_health_states.status IN ('isolated', 'recovering', 'degraded')
		  AND account_health_states.next_probe_at IS NOT NULL
		  AND account_health_states.next_probe_at <= $1
		  AND a.deleted_at IS NULL
		  AND a.health_probe_enabled IS TRUE
		ORDER BY account_health_states.next_probe_at ASC
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*service.AccountHealthState
	for rows.Next() {
		state, err := scanAccountHealthState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

const accountHealthSelectSQL = `
	SELECT account_health_states.account_id,
		account_health_states.score,
		account_health_states.consecutive_successes,
		account_health_states.consecutive_failures,
		account_health_states.status,
		account_health_states.last_success_at,
		account_health_states.last_failure_at,
		account_health_states.last_checked_at,
		account_health_states.last_error_category,
		account_health_states.last_error_message,
		account_health_states.latency_ewma_ms,
		account_health_states.backoff_level,
		account_health_states.next_probe_at,
		account_health_states.isolated_at,
		account_health_states.created_at,
		account_health_states.updated_at
	FROM account_health_states`

type accountHealthScanner interface {
	Scan(dest ...any) error
}

func scanAccountHealthState(scanner accountHealthScanner) (*service.AccountHealthState, error) {
	var state service.AccountHealthState
	var lastSuccess, lastFailure, lastChecked, nextProbe, isolated sql.NullTime
	var lastCategory, lastMessage sql.NullString
	var latency sql.NullInt64
	if err := scanner.Scan(
		&state.AccountID, &state.Score, &state.ConsecutiveSuccesses, &state.ConsecutiveFailures, &state.Status,
		&lastSuccess, &lastFailure, &lastChecked, &lastCategory, &lastMessage,
		&latency, &state.BackoffLevel, &nextProbe, &isolated, &state.CreatedAt, &state.UpdatedAt,
	); err != nil {
		return nil, err
	}
	state.LastSuccessAt = ptrNullTime(lastSuccess)
	state.LastFailureAt = ptrNullTime(lastFailure)
	state.LastCheckedAt = ptrNullTime(lastChecked)
	state.LastErrorCategory = lastCategory.String
	state.LastErrorMessage = lastMessage.String
	if latency.Valid {
		v := int(latency.Int64)
		state.LatencyEWMAMs = &v
	}
	state.NextProbeAt = ptrNullTime(nextProbe)
	state.IsolatedAt = ptrNullTime(isolated)
	return &state, nil
}

func accountHealthNullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func accountHealthNullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func accountHealthNullInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func ptrNullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
