package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func (r *usageLogRepository) OwnsHistoricalAPIKey(ctx context.Context, userID, keyID int64) (bool, error) {
	var owned bool
	err := scanSingleRow(ctx, r.sql, `SELECT EXISTS(SELECT 1 FROM api_keys WHERE user_id = $1 AND id = $2)`, []any{userID, keyID}, &owned)
	return owned, err
}

func (r *usageLogRepository) GetUserAPIKeySummary(ctx context.Context, q usagestats.APIKeySummaryQuery) (*usagestats.APIKeyUsageSummary, error) {
	// Sort expressions are constants; all user input is passed as query parameters.
	sortColumn := map[string]string{"requests": "requests", "total_tokens": "total_tokens", "actual_cost": "actual_cost"}[q.SortBy]
	if sortColumn == "" {
		sortColumn = "actual_cost"
	}
	direction := "DESC"
	if q.SortOrder == "asc" {
		direction = "ASC"
	}
	// One statement gives rows and totals the same database snapshot, including empty pages.
	query := `WITH usage AS (
		SELECT api_key_id, COUNT(*) AS requests,
		COALESCE(SUM(input_tokens),0) AS input_tokens,
		COALESCE(SUM(output_tokens),0) AS output_tokens,
		COALESCE(SUM(cache_creation_tokens),0) AS cache_creation_tokens,
		COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens,
		COALESCE(SUM(input_tokens::bigint + output_tokens + cache_creation_tokens + cache_read_tokens),0) AS total_tokens,
		COALESCE(SUM(actual_cost),0) AS actual_cost,
		COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = 1),0) AS subscription_cost
		FROM usage_logs WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		AND ($4::bigint = 0 OR group_id = $4) AND ($5::bigint = 0 OR api_key_id = $5)
		GROUP BY api_key_id
	), key_rows AS (
		SELECT k.id AS api_key_id, k.name, k.deleted_at IS NOT NULL AS deleted,
		COALESCE(g.name,'') AS group_name,
		COALESCE(u.requests,0) AS requests, COALESCE(u.input_tokens,0) AS input_tokens,
		COALESCE(u.output_tokens,0) AS output_tokens, COALESCE(u.cache_creation_tokens,0) AS cache_creation_tokens,
		COALESCE(u.cache_read_tokens,0) AS cache_read_tokens, COALESCE(u.total_tokens,0) AS total_tokens,
		COALESCE(u.actual_cost,0) AS actual_cost, COALESCE(u.subscription_cost,0) AS subscription_cost
		FROM api_keys k LEFT JOIN usage u ON u.api_key_id = k.id LEFT JOIN groups g ON g.id = k.group_id
		WHERE k.user_id = $1 AND (k.deleted_at IS NULL OR u.requests > 0)
		AND ($5::bigint = 0 OR k.id = $5)
		AND ($4::bigint = 0 OR u.requests > 0 OR k.group_id = $4)
		AND ($6 = '' OR POSITION(LOWER($6) IN LOWER(k.name)) > 0)
		AND (NOT $7::boolean OR u.requests > 0)
	), totals AS (
		SELECT COUNT(*) AS total, COALESCE(SUM(requests),0) AS requests,
		COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens,
		COALESCE(SUM(cache_creation_tokens),0) AS cache_creation_tokens,
		COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens, COALESCE(SUM(total_tokens),0) AS total_tokens,
		COALESCE(SUM(actual_cost),0) AS actual_cost, COALESCE(SUM(subscription_cost),0) AS subscription_cost
		FROM key_rows
	), page AS (SELECT * FROM key_rows ORDER BY ` + sortColumn + ` ` + direction + `, api_key_id ASC LIMIT $8 OFFSET $9)
	SELECT json_build_object('items', COALESCE((SELECT json_agg(p ORDER BY p.` + sortColumn + ` ` + direction + `, p.api_key_id ASC) FROM page p),'[]'::json),
	'summary', (SELECT row_to_json(t) FROM totals t), 'total', (SELECT total FROM totals))`
	var raw []byte
	err := scanSingleRow(ctx, r.sql, query, []any{q.UserID, q.StartTime, q.EndTime, q.GroupID, q.APIKeyID, q.Search, q.OnlyUsed, q.PageSize, (q.Page - 1) * q.PageSize}, &raw)
	if err != nil {
		return nil, err
	}
	var result usagestats.APIKeyUsageSummary
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.Page, result.PageSize = q.Page, q.PageSize
	return &result, nil
}
