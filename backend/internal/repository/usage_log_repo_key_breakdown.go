package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageLogRepository) GetUserAPIKeyBreakdown(ctx context.Context, q usagestats.APIKeyBreakdownQuery) (*usagestats.APIKeyUsageBreakdown, error) {
	modelExpr := resolveModelDimensionExpressionWithAlias(usagestats.ModelSourceRequested, "ul")
	// Ownership and every aggregation share one snapshot. GROUPING distinguishes
	// a real NULL historical group from the key-wide total produced by the rollup.
	query := `WITH owned_key AS (
		SELECT id, name, deleted_at IS NOT NULL AS deleted
		FROM api_keys WHERE user_id = $1 AND id = $2
	), counts AS (
		SELECT ul.group_id, ` + modelExpr + ` AS model,
		GROUPING(ul.group_id) AS all_groups, GROUPING(` + modelExpr + `) AS all_models,
		COUNT(*) AS requests,
		COALESCE(SUM(ul.input_tokens),0) AS input_tokens,
		COALESCE(SUM(ul.output_tokens),0) AS output_tokens,
		COALESCE(SUM(ul.cache_creation_tokens),0) AS cache_creation_tokens,
		COALESCE(SUM(ul.cache_read_tokens),0) AS cache_read_tokens,
		COALESCE(SUM(ul.input_tokens::bigint + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens),0) AS total_tokens,
		COALESCE(SUM(ul.actual_cost),0) AS actual_cost,
		COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.billing_type = 1),0) AS subscription_cost
		FROM usage_logs ul JOIN owned_key k ON k.id = ul.api_key_id
		WHERE ul.user_id = $1 AND ul.created_at >= $3 AND ul.created_at < $4
		AND ($5::bigint = 0 OR ul.group_id = $5)
		GROUP BY GROUPING SETS ((ul.group_id, ` + modelExpr + `), (ul.group_id), ())
	), stats AS (
		SELECT group_id, model, all_groups, all_models, actual_cost,
		jsonb_build_object('requests', requests, 'input_tokens', input_tokens,
		'output_tokens', output_tokens, 'cache_creation_tokens', cache_creation_tokens,
		'cache_read_tokens', cache_read_tokens, 'total_tokens', total_tokens,
		'actual_cost', actual_cost, 'subscription_cost', subscription_cost) AS totals
		FROM counts
	), group_rows AS (
		SELECT s.group_id, s.actual_cost, s.totals || jsonb_build_object(
		'group_id', s.group_id, 'group_name', COALESCE(g.name,''), 'group_platform', COALESCE(g.platform,''),
		'group_deleted', s.group_id IS NOT NULL AND (g.id IS NULL OR g.deleted_at IS NOT NULL),
		'models', COALESCE((SELECT jsonb_agg(m.totals || jsonb_build_object('model', m.model)
			ORDER BY m.actual_cost DESC, m.model ASC)
			FROM stats m WHERE m.all_groups = 0 AND m.all_models = 0
			AND m.group_id IS NOT DISTINCT FROM s.group_id), '[]'::jsonb)) AS data
		FROM stats s LEFT JOIN groups g ON g.id = s.group_id
		WHERE s.all_groups = 0 AND s.all_models = 1
	)
	SELECT jsonb_build_object('api_key_id', k.id, 'name', k.name, 'deleted', k.deleted,
	'summary', (SELECT totals FROM stats WHERE all_groups = 1),
	'groups', COALESCE((SELECT jsonb_agg(data ORDER BY actual_cost DESC, group_id ASC NULLS LAST)
		FROM group_rows), '[]'::jsonb)) FROM owned_key k`
	var raw []byte
	err := scanSingleRow(ctx, r.sql, query, []any{q.UserID, q.APIKeyID, q.StartTime, q.EndTime, q.GroupID}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	var result usagestats.APIKeyUsageBreakdown
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.PreviousSummary, result.Series, err = r.keyUsageCompareAndTrend(ctx, q.UserID, []int64{q.APIKeyID}, q.GroupID, q.StartTime, q.EndTime, q.APIKeyUsageTrendOptions)
	if err != nil {
		return nil, err
	}
	result.StartTime, result.EndTime = q.StartTime, q.EndTime
	return &result, nil
}
