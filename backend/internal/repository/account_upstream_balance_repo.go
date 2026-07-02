package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountUpstreamBalanceRepository struct {
	sql sqlExecutor
}

func NewAccountUpstreamBalanceRepository(sqlDB *sql.DB) service.AccountUpstreamBalanceRepository {
	return &accountUpstreamBalanceRepository{sql: sqlDB}
}

func (r *accountUpstreamBalanceRepository) Get(ctx context.Context, baseURL string) (*service.AccountUpstreamBalanceSnapshot, error) {
	rows, err := r.sql.QueryContext(ctx, accountUpstreamBalanceSelectSQL+" WHERE base_url = $1", baseURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	item, err := scanAccountUpstreamBalance(rows)
	if err != nil {
		return nil, err
	}
	return item, rows.Err()
}

func (r *accountUpstreamBalanceRepository) ListByBaseURLs(ctx context.Context, baseURLs []string) (map[string]*service.AccountUpstreamBalanceSnapshot, error) {
	out := make(map[string]*service.AccountUpstreamBalanceSnapshot, len(baseURLs))
	if len(baseURLs) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, accountUpstreamBalanceSelectSQL+" WHERE base_url = ANY($1)", pq.Array(baseURLs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanAccountUpstreamBalance(rows)
		if err != nil {
			return nil, err
		}
		out[item.BaseURL] = item
	}
	return out, rows.Err()
}

func (r *accountUpstreamBalanceRepository) Ensure(ctx context.Context, baseURL string, nextCheckAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_upstream_balance_snapshots (
			base_url, status, next_check_at, updated_at
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (base_url) DO NOTHING
	`, baseURL, service.UpstreamBalanceStatusUnsupported, nextCheckAt, nextCheckAt)
	return err
}

func (r *accountUpstreamBalanceRepository) Upsert(ctx context.Context, snapshot *service.AccountUpstreamBalanceSnapshot) error {
	if snapshot == nil {
		return nil
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_upstream_balance_snapshots (
			base_url, representative_account_id, status, balance, remaining,
			unit, source_endpoint, http_status, error_message,
			checked_at, next_check_at, claim_until, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, NULL, $12
		)
		ON CONFLICT (base_url) DO UPDATE SET
			representative_account_id = EXCLUDED.representative_account_id,
			status = EXCLUDED.status,
			balance = EXCLUDED.balance,
			remaining = EXCLUDED.remaining,
			unit = EXCLUDED.unit,
			source_endpoint = EXCLUDED.source_endpoint,
			http_status = EXCLUDED.http_status,
			error_message = EXCLUDED.error_message,
			checked_at = EXCLUDED.checked_at,
			next_check_at = EXCLUDED.next_check_at,
			claim_until = NULL,
			updated_at = EXCLUDED.updated_at
	`, snapshot.BaseURL, upstreamBalanceNullInt64(snapshot.RepresentativeAccountID), snapshot.Status, upstreamBalanceNullFloat64(snapshot.Balance), upstreamBalanceNullFloat64(snapshot.Remaining),
		snapshot.Unit, snapshot.SourceEndpoint, upstreamBalanceNullInt(snapshot.HTTPStatus), upstreamBalanceNullString(snapshot.ErrorMessage),
		upstreamBalanceNullTime(snapshot.CheckedAt), upstreamBalanceNullTime(snapshot.NextCheckAt), snapshot.UpdatedAt)
	return err
}

func (r *accountUpstreamBalanceRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]string, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT base_url
		FROM account_upstream_balance_snapshots
		WHERE next_check_at IS NOT NULL
		  AND next_check_at <= $1
		  AND (claim_until IS NULL OR claim_until <= $1)
		ORDER BY next_check_at ASC, base_url ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var baseURL string
		if err := rows.Scan(&baseURL); err != nil {
			return nil, err
		}
		out = append(out, baseURL)
	}
	return out, rows.Err()
}

func (r *accountUpstreamBalanceRepository) Claim(ctx context.Context, baseURL string, now, leaseUntil time.Time) (bool, error) {
	res, err := r.sql.ExecContext(ctx, `
		UPDATE account_upstream_balance_snapshots
		SET claim_until = $3, status = $4, updated_at = $2
		WHERE base_url = $1
		  AND next_check_at IS NOT NULL
		  AND next_check_at <= $2
		  AND (claim_until IS NULL OR claim_until <= $2)
	`, baseURL, now, leaseUntil, service.UpstreamBalanceStatusChecking)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *accountUpstreamBalanceRepository) ClaimRefresh(ctx context.Context, baseURL string, now, leaseUntil time.Time) (bool, error) {
	res, err := r.sql.ExecContext(ctx, `
		UPDATE account_upstream_balance_snapshots
		SET claim_until = $3, status = $4, updated_at = $2
		WHERE base_url = $1
		  AND (claim_until IS NULL OR claim_until <= $2)
	`, baseURL, now, leaseUntil, service.UpstreamBalanceStatusChecking)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

const accountUpstreamBalanceSelectSQL = `
	SELECT base_url, representative_account_id, status, balance, remaining,
		unit, source_endpoint, http_status, error_message,
		checked_at, next_check_at, updated_at
	FROM account_upstream_balance_snapshots`

type accountUpstreamBalanceScanner interface {
	Scan(dest ...any) error
}

func scanAccountUpstreamBalance(scanner accountUpstreamBalanceScanner) (*service.AccountUpstreamBalanceSnapshot, error) {
	var item service.AccountUpstreamBalanceSnapshot
	var accountID sql.NullInt64
	var balance, remaining sql.NullFloat64
	var unit, source, message sql.NullString
	var httpStatus sql.NullInt64
	var checkedAt, nextCheckAt sql.NullTime
	if err := scanner.Scan(
		&item.BaseURL, &accountID, &item.Status, &balance, &remaining,
		&unit, &source, &httpStatus, &message,
		&checkedAt, &nextCheckAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if accountID.Valid {
		v := accountID.Int64
		item.RepresentativeAccountID = &v
	}
	if balance.Valid {
		v := balance.Float64
		item.Balance = &v
	}
	if remaining.Valid {
		v := remaining.Float64
		item.Remaining = &v
	}
	item.Unit = unit.String
	item.SourceEndpoint = source.String
	if httpStatus.Valid {
		v := int(httpStatus.Int64)
		item.HTTPStatus = &v
	}
	item.ErrorMessage = message.String
	if checkedAt.Valid {
		v := checkedAt.Time
		item.CheckedAt = &v
	}
	if nextCheckAt.Valid {
		v := nextCheckAt.Time
		item.NextCheckAt = &v
	}
	return &item, nil
}

func upstreamBalanceNullInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func upstreamBalanceNullInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func upstreamBalanceNullFloat64(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func upstreamBalanceNullTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}

func upstreamBalanceNullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
