package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/lib/pq"
)

// keyUsageCompareAndTrend aggregates the previous period and time buckets for a
// fixed key set, so both stay consistent with the totals the caller already computed.
func (r *usageLogRepository) keyUsageCompareAndTrend(ctx context.Context, userID int64, keyIDs []int64, groupID int64, start, end time.Time, opts usagestats.APIKeyUsageTrendOptions) (*usagestats.APIKeyUsageCompare, []usagestats.APIKeyUsageBucket, error) {
	var previous *usagestats.APIKeyUsageCompare
	if opts.PrevStartTime != nil && opts.PrevEndTime != nil {
		previous = &usagestats.APIKeyUsageCompare{}
		if len(keyIDs) > 0 {
			err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*),
				COALESCE(SUM(input_tokens::bigint + output_tokens + cache_creation_tokens + cache_read_tokens),0),
				COALESCE(SUM(actual_cost),0)
				FROM usage_logs WHERE user_id = $1 AND api_key_id = ANY($2)
				AND created_at >= $3 AND created_at < $4 AND ($5::bigint = 0 OR group_id = $5)`,
				[]any{userID, pq.Array(keyIDs), *opts.PrevStartTime, *opts.PrevEndTime, groupID},
				&previous.Requests, &previous.TotalTokens, &previous.ActualCost)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	starts, ends := usagestats.KeyUsageBuckets(start, end, opts.Granularity, opts.Location)
	if starts == nil {
		return previous, nil, nil
	}
	buckets := make([]usagestats.APIKeyUsageBucket, len(starts))
	for i := range starts {
		buckets[i].Bucket = starts[i]
	}
	if len(keyIDs) == 0 {
		return previous, buckets, nil
	}
	// Bucket edges are computed in Go (DST-safe) and passed as parameters.
	startText, endText := make([]string, len(starts)), make([]string, len(ends))
	for i := range starts {
		startText[i], endText[i] = starts[i].Format(time.RFC3339Nano), ends[i].Format(time.RFC3339Nano)
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT b.i - 1, COUNT(*),
		COALESCE(SUM(ul.input_tokens::bigint + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens),0),
		COALESCE(SUM(ul.actual_cost),0)
		FROM unnest($3::timestamptz[], $4::timestamptz[]) WITH ORDINALITY AS b(s, e, i)
		JOIN usage_logs ul ON ul.created_at >= b.s AND ul.created_at < b.e
		WHERE ul.user_id = $1 AND ul.api_key_id = ANY($2) AND ($5::bigint = 0 OR ul.group_id = $5)
		GROUP BY b.i`, userID, pq.Array(keyIDs), pq.Array(startText), pq.Array(endText), groupID)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var i int
		var c usagestats.APIKeyUsageCompare
		if err := rows.Scan(&i, &c.Requests, &c.TotalTokens, &c.ActualCost); err != nil {
			return nil, nil, err
		}
		if i >= 0 && i < len(buckets) {
			buckets[i].APIKeyUsageCompare = c
		}
	}
	return previous, buckets, rows.Err()
}
