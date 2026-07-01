package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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

func (r *accountHealthRepository) InsertEvent(ctx context.Context, event *service.AccountHealthEvent) error {
	if event == nil {
		return nil
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_health_events (
			account_id, source, event_type, score_before, score_after,
			status_before, status_after, delta, error_category, error_message,
			latency_ms, affected_group_ids, actor_user_id, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14
		)
	`, event.AccountID, event.Source, event.EventType, event.ScoreBefore, event.ScoreAfter,
		event.StatusBefore, event.StatusAfter, event.Delta, accountHealthNullString(event.ErrorCategory), accountHealthNullString(event.ErrorMessage),
		accountHealthNullInt64(event.LatencyMs), pq.Array(event.AffectedGroupIDs), accountHealthNullInt64(event.ActorUserID), event.CreatedAt)
	return err
}

func (r *accountHealthRepository) ListEvents(ctx context.Context, accountID int64, eventType string, params pagination.PaginationParams) (*service.AccountHealthEventList, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	where := "WHERE account_id = $1"
	args := []any{accountID}
	if strings.TrimSpace(eventType) != "" {
		where += " AND event_type = $2"
		args = append(args, strings.TrimSpace(eventType))
	}
	var total int64
	countQuery := "SELECT COUNT(*) FROM account_health_events " + where
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, err
	}
	offset := (params.Page - 1) * params.PageSize
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.PageSize, offset)
	limitPos := len(queryArgs) - 1
	offsetPos := len(queryArgs)
	rows, err := r.sql.QueryContext(ctx, accountHealthEventSelectSQL+" "+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+itoa(limitPos)+` OFFSET $`+itoa(offsetPos), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.AccountHealthEvent, 0)
	for rows.Next() {
		event, err := scanAccountHealthEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	totalPages := 0
	if params.PageSize > 0 {
		totalPages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}
	return &service.AccountHealthEventList{Items: items, Total: total, Page: params.Page, PageSize: params.PageSize, TotalPages: totalPages}, nil
}

func (r *accountHealthRepository) DeleteEventsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.sql.ExecContext(ctx, `DELETE FROM account_health_events WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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

const accountHealthEventSelectSQL = `
	SELECT id, account_id, source, event_type, score_before, score_after,
		status_before, status_after, delta, error_category, error_message,
		latency_ms, affected_group_ids, actor_user_id, created_at
	FROM account_health_events`

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

func scanAccountHealthEvent(scanner accountHealthScanner) (*service.AccountHealthEvent, error) {
	var event service.AccountHealthEvent
	var errorCategory, errorMessage sql.NullString
	var latency, actor sql.NullInt64
	var affected []int64
	if err := scanner.Scan(
		&event.ID, &event.AccountID, &event.Source, &event.EventType, &event.ScoreBefore, &event.ScoreAfter,
		&event.StatusBefore, &event.StatusAfter, &event.Delta, &errorCategory, &errorMessage,
		&latency, pq.Array(&affected), &actor, &event.CreatedAt,
	); err != nil {
		return nil, err
	}
	event.ErrorCategory = errorCategory.String
	event.ErrorMessage = errorMessage.String
	if latency.Valid {
		v := latency.Int64
		event.LatencyMs = &v
	}
	event.AffectedGroupIDs = affected
	if actor.Valid {
		v := actor.Int64
		event.ActorUserID = &v
	}
	return &event, nil
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

func accountHealthNullInt64(value *int64) any {
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
