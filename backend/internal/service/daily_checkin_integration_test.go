//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestDailyCheckinRewardsIdempotencySnapshotAndBudget(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("sub2api_checkin"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.Eventually(t, func() bool { return db.PingContext(ctx) == nil }, 30*time.Second, 250*time.Millisecond)
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	require.NoError(t, timezone.Init("Asia/Shanghai"))

	svc := service.NewDailyCheckinService(db, nil)
	cfg := service.DailyCheckinConfig{Enabled: true, BaseReward: 0.13, CycleDays: 30, Milestone7: 2, Milestone15: 5, Milestone30: 8, RuleVersion: 1}
	_, err = svc.UpdateConfig(ctx, cfg)
	require.NoError(t, err)
	userID := insertDailyCheckinUser(t, db, "checkin@example.test")

	first, err := svc.Claim(ctx, userID, "127.0.0.1", "integration-test")
	require.NoError(t, err)
	require.InDelta(t, 0.13, first.TotalReward, 1e-9)
	duplicate, err := svc.Claim(ctx, userID, "127.0.0.1", "integration-test")
	require.NoError(t, err)
	require.Equal(t, first.ID, duplicate.ID)

	// Move each completed business day into the past so the next call exercises
	// the public claim path without a production clock override.
	rewards := map[int]float64{7: 2.13, 15: 5.13, 30: 8.13}
	for day := 2; day <= 30; day++ {
		moveLatestDailyCheckinToYesterday(t, db, userID)
		record, claimErr := svc.Claim(ctx, userID, "127.0.0.1", "integration-test")
		require.NoError(t, claimErr)
		if expected, ok := rewards[day]; ok {
			require.InDelta(t, expected, record.TotalReward, 1e-9, "cycle day %d", day)
		} else {
			require.InDelta(t, 0.13, record.TotalReward, 1e-9, "cycle day %d", day)
		}
	}
	var cycleTotal, balance float64
	var status string
	require.NoError(t, db.QueryRow(`SELECT status,total_reward FROM daily_checkin_cycles WHERE user_id=$1 AND cycle_number=1`, userID).Scan(&status, &cycleTotal))
	require.Equal(t, "completed", status)
	require.InDelta(t, 18.90, cycleTotal, 1e-9)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=$1`, userID).Scan(&balance))
	require.InDelta(t, 18.90, balance, 1e-9)

	moveLatestDailyCheckinToYesterday(t, db, userID)
	secondCycle, err := svc.Claim(ctx, userID, "127.0.0.1", "integration-test")
	require.NoError(t, err)
	require.Equal(t, 1, secondCycle.CycleDay)
	var cycles int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM daily_checkin_cycles WHERE user_id=$1`, userID).Scan(&cycles))
	require.Equal(t, 2, cycles)

	snapshotUser := insertDailyCheckinUser(t, db, "snapshot@example.test")
	_, err = svc.Claim(ctx, snapshotUser, "127.0.0.1", "snapshot")
	require.NoError(t, err)
	cfg.BaseReward = 1
	cfg.Milestone7 = 20
	cfg.RuleVersion = 2
	_, err = svc.UpdateConfig(ctx, cfg)
	require.NoError(t, err)
	moveLatestDailyCheckinToYesterday(t, db, snapshotUser)
	snapshotSecond, err := svc.Claim(ctx, snapshotUser, "127.0.0.1", "snapshot")
	require.NoError(t, err)
	require.InDelta(t, 0.13, snapshotSecond.TotalReward, 1e-9)
	require.Equal(t, 1, snapshotSecond.RuleVersion)
	_, err = db.Exec(`UPDATE daily_checkins SET business_date=$2,business_key='test-moved:'||id WHERE user_id=$1 AND business_date=$3`, snapshotUser, timezone.Today().AddDate(0, 0, -3), timezone.Today())
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE daily_checkin_cycles SET last_checkin_on=$2 WHERE user_id=$1 AND status='active'`, snapshotUser, timezone.Today().AddDate(0, 0, -3))
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM daily_checkin_daily_totals WHERE business_date=$1`, timezone.Today())
	require.NoError(t, err)
	_, err = svc.Claim(ctx, snapshotUser, "127.0.0.1", "snapshot")
	require.NoError(t, err)
	var progress, consecutive int
	require.NoError(t, db.QueryRow(`SELECT checkin_count,consecutive_days FROM daily_checkin_cycles WHERE user_id=$1 AND status='active'`, snapshotUser).Scan(&progress, &consecutive))
	require.Equal(t, 3, progress, "missing a day must not reset cycle progress")
	require.Equal(t, 1, consecutive, "missing a day must reset consecutive days")

	budgetCfg := cfg
	budgetCfg.DailyBudget = 0.5
	_, err = svc.UpdateConfig(ctx, budgetCfg)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM daily_checkin_daily_totals WHERE business_date=$1`, timezone.Today())
	require.NoError(t, err)
	budgetUser := insertDailyCheckinUser(t, db, "budget@example.test")
	_, err = svc.Claim(ctx, budgetUser, "127.0.0.1", "budget")
	require.Error(t, err)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=$1`, budgetUser).Scan(&balance))
	require.Zero(t, balance)
	var checkins int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM daily_checkins WHERE user_id=$1`, budgetUser).Scan(&checkins))
	require.Zero(t, checkins)

	eligibilityCfg := budgetCfg
	eligibilityCfg.DailyBudget = 0
	eligibilityCfg.MinAccountAgeHours = 24
	_, err = svc.UpdateConfig(ctx, eligibilityCfg)
	require.NoError(t, err)
	newUser := insertDailyCheckinNewUser(t, db, "new-account@example.test")
	statusView, err := svc.GetStatus(ctx, newUser)
	require.NoError(t, err)
	require.False(t, statusView.Eligible)
	require.Equal(t, "account_too_new", statusView.IneligibleReason)
	_, err = svc.Claim(ctx, newUser, "127.0.0.1", "eligibility")
	require.Error(t, err)

	eligibilityCfg.MinAccountAgeHours = 0
	eligibilityCfg.RequireVerified = true
	_, err = svc.UpdateConfig(ctx, eligibilityCfg)
	require.NoError(t, err)
	unverifiedUser := insertDailyCheckinUser(t, db, "unverified@example.test")
	statusView, err = svc.GetStatus(ctx, unverifiedUser)
	require.NoError(t, err)
	require.False(t, statusView.Eligible)
	require.Equal(t, "verification_required", statusView.IneligibleReason)
	_, err = svc.Claim(ctx, unverifiedUser, "127.0.0.1", "eligibility")
	require.Error(t, err)
}

func TestDailyCheckinConcurrentClaimsOnlyGrantOnce(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("sub2api_checkin_concurrency"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.Eventually(t, func() bool { return db.PingContext(ctx) == nil }, 30*time.Second, 250*time.Millisecond)
	require.NoError(t, repository.ApplyMigrations(ctx, db))

	svc := service.NewDailyCheckinService(db, nil)
	_, err = svc.UpdateConfig(ctx, service.DailyCheckinConfig{Enabled: true, BaseReward: 0.13, CycleDays: 30, Milestone7: 2, Milestone15: 5, Milestone30: 8, RuleVersion: 1})
	require.NoError(t, err)
	userID := insertDailyCheckinUser(t, db, "concurrent-checkin@example.test")

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, claimErr := svc.Claim(ctx, userID, "127.0.0.1", "concurrent")
			results <- claimErr
		}()
	}
	wg.Wait()
	close(results)
	for claimErr := range results {
		require.NoError(t, claimErr)
	}
	var count int
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM daily_checkins WHERE user_id=$1`, userID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=$1`, userID).Scan(&balance))
	require.InDelta(t, 0.13, balance, 1e-9)

	_, err = db.Exec(`DELETE FROM daily_checkin_daily_totals WHERE business_date=$1`, timezone.Today())
	require.NoError(t, err)
	userA := insertDailyCheckinUser(t, db, "concurrent-a@example.test")
	userB := insertDailyCheckinUser(t, db, "concurrent-b@example.test")
	firstDayResults := make(chan error, 2)
	for _, concurrentUserID := range []int64{userA, userB} {
		wg.Add(1)
		go func(claimUserID int64) {
			defer wg.Done()
			_, claimErr := svc.Claim(ctx, claimUserID, "127.0.0.1", "concurrent-first-day")
			firstDayResults <- claimErr
		}(concurrentUserID)
	}
	wg.Wait()
	close(firstDayResults)
	for claimErr := range firstDayResults {
		require.NoError(t, claimErr, "different users must safely race to initialize the daily budget row")
	}
	require.NoError(t, db.QueryRow(`SELECT claim_count,total_reward FROM daily_checkin_daily_totals WHERE business_date=$1`, timezone.Today()).Scan(&count, &balance))
	require.Equal(t, 2, count)
	require.InDelta(t, 0.26, balance, 1e-9)
}

func insertDailyCheckinUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO users(email,password_hash,role,status,balance,concurrency,signup_source,created_at) VALUES($1,'test','user','active',0,5,'email',NOW()-INTERVAL '10 days') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertDailyCheckinNewUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO users(email,password_hash,role,status,balance,concurrency,signup_source) VALUES($1,'test','user','active',0,5,'email') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

func moveLatestDailyCheckinToYesterday(t *testing.T, db *sql.DB, userID int64) {
	t.Helper()
	var cycleDay int
	require.NoError(t, db.QueryRow(`SELECT cycle_day FROM daily_checkins WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, userID).Scan(&cycleDay))
	date := timezone.Today().AddDate(0, 0, -cycleDay)
	_, err := db.Exec(`UPDATE daily_checkins SET business_date=$2,business_key='test-moved:'||id WHERE user_id=$1 AND business_date=$3`, userID, date, timezone.Today())
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE daily_checkin_cycles SET last_checkin_on=$2 WHERE user_id=$1 AND status='active'`, userID, date)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM daily_checkin_daily_totals WHERE business_date=$1`, timezone.Today())
	require.NoError(t, err)
}
