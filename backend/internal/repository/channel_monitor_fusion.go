package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Session locks prevent overlap; persisted due times also prevent staggered replicas
// from each probing after the previous replica finishes. No long transaction is held.
func (r *channelMonitorRepository) TryBeginProbe(ctx context.Context, id int64, scheduled bool) (func(), bool, error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	key := fmt.Sprintf("channel-monitor-probe:%d", id)
	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, key).Scan(&acquired); err != nil {
		conn.Close()
		return nil, false, err
	}
	if !acquired {
		conn.Close()
		return nil, false, nil
	}
	release := func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var unlocked bool
		err := conn.QueryRowContext(cleanup, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, key).Scan(&unlocked)
		if err != nil || !unlocked {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}
	result, err := conn.ExecContext(ctx, `UPDATE channel_monitors
 SET probe_next_check_at = clock_timestamp() + make_interval(secs => GREATEST(15, interval_seconds - jitter_seconds))
 WHERE id=$1 AND (NOT $2 OR (enabled AND (probe_next_check_at IS NULL OR probe_next_check_at <= clock_timestamp())))`, id, scheduled)
	if err != nil {
		release()
		return nil, false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count == 0 {
		release()
		return nil, false, err
	}
	return release, true, nil
}

func (r *channelMonitorRepository) ProbeBuckets(ctx context.Context, monitors []*service.ChannelMonitor, filter service.ChannelMonitorV2Filter) (map[int64][]service.ChannelMonitorProbeBucket, error) {
	result := make(map[int64][]service.ChannelMonitorProbeBucket)
	if len(monitors) == 0 {
		return result, nil
	}
	ids, models, hashes := make([]int64, 0, len(monitors)), make([]string, 0, len(monitors)), make([]string, 0, len(monitors))
	for _, m := range monitors {
		ids = append(ids, m.ID)
		models = append(models, m.PrimaryModel)
		hashes = append(hashes, service.ChannelMonitorProbeTargetHash(m))
	}
	rows, err := r.db.QueryContext(ctx, `SELECT h.monitor_id,
 to_timestamp(floor(extract(epoch FROM h.checked_at)/$4)*$4) AS bucket_start,
 count(*) FILTER (WHERE h.status IN ('operational','degraded')),
 count(*) FILTER (WHERE h.status NOT IN ('operational','degraded')), max(h.checked_at)
 FROM unnest($1::bigint[], $2::text[], $3::text[]) AS m(id,model,target_hash)
 JOIN channel_monitor_histories h ON h.monitor_id=m.id AND h.model=m.model AND h.probe_target_hash=m.target_hash
 WHERE h.is_probe AND h.checked_at >= $5 AND h.checked_at < $6 AND h.checked_at <= CURRENT_TIMESTAMP
 GROUP BY h.monitor_id,bucket_start ORDER BY h.monitor_id,bucket_start`, pq.Array(ids), pq.Array(models), pq.Array(hashes), int64(filter.Bucket.Seconds()), filter.Start, filter.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var bucket service.ChannelMonitorProbeBucket
		if err := rows.Scan(&id, &bucket.BucketStart, &bucket.SuccessCount, &bucket.FailureCount, &bucket.LastCheckedAt); err != nil {
			return nil, err
		}
		result[id] = append(result[id], bucket)
	}
	return result, rows.Err()
}
