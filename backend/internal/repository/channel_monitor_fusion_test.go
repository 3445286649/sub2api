package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Uses an explicitly supplied local, disposable PostgreSQL database only.
func TestChannelMonitorFusionPostgres(t *testing.T) {
	dsn := os.Getenv("SUBAPI_FUSION_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL fixture not supplied")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(4)
	ctx := context.Background()
	_, err = db.Exec(`CREATE TABLE groups(id bigint PRIMARY KEY);
 CREATE TABLE channel_monitors(id bigint PRIMARY KEY,enabled boolean,interval_seconds int,jitter_seconds int);
 CREATE TABLE channel_monitor_histories(monitor_id bigint,model text,status text,checked_at timestamptz);
 INSERT INTO groups VALUES(1); INSERT INTO channel_monitors VALUES(1,true,120,0);`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/241_channel_monitor_v3_fusion.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	repo := &channelMonitorRepository{db: db}
	release, ok, err := repo.TryBeginProbe(ctx, 1, true)
	require.NoError(t, err)
	require.True(t, ok)
	_, ok, err = repo.TryBeginProbe(ctx, 1, true)
	require.NoError(t, err)
	require.False(t, ok, "concurrent replica cannot enter")
	release()
	_, ok, err = repo.TryBeginProbe(ctx, 1, true)
	require.NoError(t, err)
	require.False(t, ok, "staggered replica must wait for due time")
	_, err = db.Exec("UPDATE channel_monitors SET probe_next_check_at=now()-interval '1 second'")
	require.NoError(t, err)
	release, ok, err = repo.TryBeginProbe(ctx, 1, true)
	require.NoError(t, err)
	require.True(t, ok)
	release()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = repo.TryBeginProbe(canceled, 1, true)
	require.Error(t, err)
	m := &service.ChannelMonitor{ID: 1, PrimaryModel: "primary", Provider: "openai", Endpoint: "https://fixture.invalid"}
	stamp := time.Now().UTC().Truncate(5 * time.Minute).Add(-5 * time.Minute)
	hash := service.ChannelMonitorProbeTargetHash(m)
	for _, row := range []struct {
		model, status, target string
		probe                 bool
		at                    time.Time
	}{
		{"primary", "operational", hash, true, stamp.Add(time.Minute)},
		{"primary", "degraded", hash, true, stamp.Add(2 * time.Minute)},
		{"primary", "failed", hash, true, stamp.Add(3 * time.Minute)},
		{"extra", "operational", hash, true, stamp},
		{"primary", "operational", "old-target", true, stamp},
		{"primary", "operational", hash, false, stamp},
		{"primary", "error", hash, true, stamp.Add(5 * time.Minute)},
	} {
		_, err = db.Exec(`INSERT INTO channel_monitor_histories VALUES(1,$1,$2,$3,$4,$5)`, row.model, row.status, row.at, row.probe, row.target)
		require.NoError(t, err)
	}
	filter := service.ChannelMonitorV2Filter{Start: stamp, End: stamp.Add(5 * time.Minute), Bucket: 5 * time.Minute}
	result, err := repo.ProbeBuckets(ctx, []*service.ChannelMonitor{m}, filter)
	require.NoError(t, err)
	require.Len(t, result[1], 1)
	require.Equal(t, int64(2), result[1][0].SuccessCount)
	require.Equal(t, int64(1), result[1][0].FailureCount)
	require.True(t, stamp.Equal(result[1][0].BucketStart))
}
