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

func (r *accountHealthRepository) ClaimDueProbe(ctx context.Context, accountID int64, now, leaseUntil time.Time) (bool, error) {
	res, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_health_states (account_id, next_probe_at, created_at, updated_at)
		SELECT a.id, $3, $2, $2
		FROM accounts a
		WHERE a.id = $1
		  AND a.deleted_at IS NULL
		  AND a.status = 'active'
		  AND (a.schedulable IS TRUE OR COALESCE(a.extra ->> 'health_probe_when_unschedulable', '') = 'true')
		  AND a.health_probe_enabled IS TRUE
		  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= $2)
		ON CONFLICT (account_id) DO UPDATE SET
			next_probe_at = $3,
			updated_at = $2
		WHERE (account_health_states.next_probe_at IS NULL OR account_health_states.next_probe_at <= $2)
		  AND EXISTS (
			SELECT 1
			FROM accounts a
			WHERE a.id = account_health_states.account_id
			  AND a.deleted_at IS NULL
			  AND a.status = 'active'
			  AND (a.schedulable IS TRUE OR COALESCE(a.extra ->> 'health_probe_when_unschedulable', '') = 'true')
			  AND a.health_probe_enabled IS TRUE
			  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= $2)
		  )
	`, accountID, now, leaseUntil)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *accountHealthRepository) ScheduleNextProbe(ctx context.Context, accountID int64, nextProbeAt *time.Time, now time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_health_states (account_id, next_probe_at, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (account_id) DO UPDATE SET
			next_probe_at = EXCLUDED.next_probe_at,
			updated_at = EXCLUDED.updated_at
	`, accountID, accountHealthNullTime(nextProbeAt), now)
	return err
}

func (r *accountHealthRepository) GetNextProbeAt(ctx context.Context, accountID int64) (*time.Time, error) {
	var next sql.NullTime
	err := scanSingleRow(ctx, r.sql, `SELECT next_probe_at FROM account_health_states WHERE account_id = $1`, []any{accountID}, &next)
	if err != nil {
		return nil, err
	}
	if !next.Valid {
		return nil, nil
	}
	return &next.Time, nil
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
			$1, $2, $3, 0, 0,
			'', '', 0, $4, $5,
			$6, NULL, $7, $8
		)
	`, event.AccountID, event.Source, event.EventType,
		accountHealthNullString(event.ErrorCategory), accountHealthNullString(event.ErrorMessage),
		accountHealthNullInt64(event.LatencyMs), accountHealthNullInt64(event.ActorUserID), event.CreatedAt)
	return err
}

func (r *accountHealthRepository) ListProbeEvents(ctx context.Context, accountIDs []int64, since time.Time) ([]service.AccountHealthEvent, error) {
	if len(accountIDs) == 0 {
		return []service.AccountHealthEvent{}, nil
	}
	rows, err := r.sql.QueryContext(ctx, accountHealthProbeEventSelectSQL+`
		WHERE account_id = ANY($1)
		  AND source IN ('background_probe', 'manual_probe')
		  AND event_type IN ('success', 'failure')
		  AND created_at >= $2
		ORDER BY account_id ASC, created_at ASC, id ASC`, pq.Array(accountIDs), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]service.AccountHealthEvent, 0)
	for rows.Next() {
		event, err := scanAccountHealthProbeEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}
	return events, rows.Err()
}

func (r *accountHealthRepository) ListEvents(ctx context.Context, accountID int64, eventType string, since time.Time, params pagination.PaginationParams) (*service.AccountHealthEventList, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	where := `WHERE account_id = $1
		AND source IN ('background_probe', 'manual_probe')
		AND event_type IN ('success', 'failure')
		AND created_at >= $2`
	args := []any{accountID, since}
	if strings.TrimSpace(eventType) != "" {
		where += " AND event_type = $3"
		args = append(args, strings.TrimSpace(eventType))
	}
	var total int64
	if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*) FROM account_health_events "+where, args, &total); err != nil {
		return nil, err
	}
	offset := (params.Page - 1) * params.PageSize
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.PageSize, offset)
	limitPos := len(queryArgs) - 1
	offsetPos := len(queryArgs)
	rows, err := r.sql.QueryContext(ctx, accountHealthProbeEventSelectSQL+" "+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+itoa(limitPos)+` OFFSET $`+itoa(offsetPos), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.AccountHealthEvent, 0)
	for rows.Next() {
		event, err := scanAccountHealthProbeEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	totalPages := int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	return &service.AccountHealthEventList{Items: items, Total: total, Page: params.Page, PageSize: params.PageSize, TotalPages: totalPages}, nil
}

func (r *accountHealthRepository) GetRecentCacheUsage(ctx context.Context, accountID int64, from, to time.Time) (*service.AccountProbeCacheUsage, error) {
	var usage service.AccountProbeCacheUsage
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			COUNT(DISTINCT COALESCE(NULLIF(request_id, ''), 'usage:' || id::text)) FILTER (WHERE actual_cost > 0),
			COALESCE(SUM(input_tokens) FILTER (WHERE actual_cost > 0), 0),
			COALESCE(SUM(cache_creation_tokens) FILTER (WHERE actual_cost > 0), 0),
			COALESCE(SUM(cache_read_tokens) FILTER (WHERE actual_cost > 0), 0)
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2 AND created_at <= $3
	`, []any{accountID, from, to}, &usage.RequestCount, &usage.InputTokens, &usage.CacheCreationTokens, &usage.CacheReadTokens)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (r *accountHealthRepository) DeleteEventsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.sql.ExecContext(ctx, `DELETE FROM account_health_events WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

const accountHealthProbeEventSelectSQL = `
	SELECT id, account_id, source, event_type, error_category, error_message, latency_ms, actor_user_id, created_at
	FROM account_health_events`

type accountHealthScanner interface {
	Scan(dest ...any) error
}

func scanAccountHealthProbeEvent(scanner accountHealthScanner) (*service.AccountHealthEvent, error) {
	var event service.AccountHealthEvent
	var errorCategory, errorMessage sql.NullString
	var latency, actor sql.NullInt64
	if err := scanner.Scan(
		&event.ID, &event.AccountID, &event.Source, &event.EventType,
		&errorCategory, &errorMessage, &latency, &actor, &event.CreatedAt,
	); err != nil {
		return nil, err
	}
	event.ErrorCategory = errorCategory.String
	event.ErrorMessage = errorMessage.String
	if latency.Valid {
		value := latency.Int64
		event.LatencyMs = &value
	}
	if actor.Valid {
		value := actor.Int64
		event.ActorUserID = &value
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

func accountHealthNullInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
