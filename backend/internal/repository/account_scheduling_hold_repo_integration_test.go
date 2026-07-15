//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountSchedulingHoldLifecycleIntegration(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	platform := fmt.Sprintf("hold-integration-%d", now.UnixNano())
	cache := &schedulerCacheRecorder{}
	accountRepo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, cache)
	holdRepo := NewAccountSchedulingHoldRepository(integrationDB, accountRepo, cache)
	target := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-target", Platform: platform, Schedulable: true})
	fallback := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-fallback", Platform: platform, Schedulable: true})
	cleanupSchedulingHoldAccounts(t, target.ID, fallback.ID)
	target, err := accountRepo.GetByID(ctx, target.ID)
	require.NoError(t, err)

	put := holdPutCommand(target, "hold-lifecycle-1", now.Add(15*time.Minute))
	state, err := holdRepo.PutSchedulingHold(ctx, put, now)
	require.NoError(t, err)
	require.NotNil(t, state.ExternalHold)
	require.True(t, state.ExternalHold.Active)
	require.False(t, state.EffectiveSchedulable)
	require.NotEmpty(t, cache.setAccounts)
	require.True(t, cache.setAccounts[len(cache.setAccounts)-1].HasActiveExternalSchedulingHold(now))

	replayed, err := holdRepo.PutSchedulingHold(ctx, put, now)
	require.NoError(t, err)
	require.True(t, replayed.IdempotentReplay)
	var eventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM account_scheduling_hold_events WHERE owner = $1 AND decision_id = $2
	`, service.AccountSchedulingHoldOwner, put.DecisionID).Scan(&eventCount))
	require.Equal(t, 1, eventCount)

	_, err = integrationEntClient.Account.UpdateOneID(fallback.ID).SetSchedulable(false).Save(ctx)
	require.NoError(t, err)
	renewNow := now.Add(time.Minute)
	renew := holdPutCommand(target, "hold-lifecycle-2", renewNow.Add(20*time.Minute))
	_, err = holdRepo.PutSchedulingHold(ctx, renew, renewNow)
	require.NoError(t, err, "renewing an active hold must not consume additional capacity")

	healthRepo := NewAccountHealthRepository(integrationDB)
	due, err := healthRepo.ListDueForProbe(ctx, renewNow, 200)
	require.NoError(t, err)
	require.Contains(t, accountHealthStateIDs(due), target.ID)

	afterExpiry := renew.LeaseUntil.Add(time.Minute)
	replacement := holdPutCommand(target, "hold-lifecycle-3", afterExpiry.Add(15*time.Minute))
	_, err = holdRepo.PutSchedulingHold(ctx, replacement, afterExpiry)
	require.Equal(t, "CAPACITY_GUARD_BLOCKED", errors.Reason(err))

	_, err = integrationEntClient.Account.UpdateOneID(fallback.ID).SetSchedulable(true).Save(ctx)
	require.NoError(t, err)
	_, err = holdRepo.PutSchedulingHold(ctx, replacement, afterExpiry)
	require.NoError(t, err)
	var firstHeldAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT first_held_at FROM account_scheduling_holds WHERE account_id = $1 AND owner = $2
	`, target.ID, service.AccountSchedulingHoldOwner).Scan(&firstHeldAt))
	require.WithinDuration(t, afterExpiry, firstHeldAt, time.Millisecond)

	expiredIDs, err := holdRepo.ExpireSchedulingHolds(ctx, service.AccountSchedulingHoldOwner, replacement.LeaseUntil.Add(time.Second), 200)
	require.NoError(t, err)
	require.Contains(t, expiredIDs, target.ID)
	current, err := accountRepo.GetByID(ctx, target.ID)
	require.NoError(t, err)
	require.Nil(t, current.ExternalSchedulingHoldUntil)
	require.True(t, current.Schedulable)
	require.NotEmpty(t, cache.setAccounts)
	require.Nil(t, cache.setAccounts[len(cache.setAccounts)-1].ExternalSchedulingHoldUntil)
}

func TestAccountSchedulingHoldReleasePreservesIndependentBlocksIntegration(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	platform := fmt.Sprintf("hold-release-%d", now.UnixNano())
	accountRepo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	holdRepo := NewAccountSchedulingHoldRepository(integrationDB, accountRepo, nil)
	target := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-target", Platform: platform, Schedulable: true})
	fallback := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-fallback", Platform: platform, Schedulable: true})
	cleanupSchedulingHoldAccounts(t, target.ID, fallback.ID)
	target, err := accountRepo.GetByID(ctx, target.ID)
	require.NoError(t, err)

	put := holdPutCommand(target, "hold-release-put", now.Add(15*time.Minute))
	_, err = holdRepo.PutSchedulingHold(ctx, put, now)
	require.NoError(t, err)
	tempUntil := now.Add(30 * time.Minute)
	_, err = integrationEntClient.Account.UpdateOneID(target.ID).
		SetSchedulable(false).
		SetTempUnschedulableUntil(tempUntil).
		SetTempUnschedulableReason("integration-test").
		Save(ctx)
	require.NoError(t, err)

	state, err := holdRepo.ReleaseSchedulingHold(ctx, service.AccountSchedulingHoldRelease{
		AccountID: target.ID, Owner: service.AccountSchedulingHoldOwner,
		DecisionID: "hold-release-command", ExpectedHoldDecisionID: put.DecisionID, RequestHash: "release-hash",
	}, now.Add(time.Minute))
	require.NoError(t, err)
	require.NotNil(t, state.ExternalHold)
	require.Equal(t, "released", state.ExternalHold.Status)
	require.False(t, state.ExternalHold.Active)
	require.False(t, state.ManualSchedulable)
	require.True(t, state.InternalBlocked)
	require.False(t, state.EffectiveSchedulable)
	current, err := accountRepo.GetByID(ctx, target.ID)
	require.NoError(t, err)
	require.False(t, current.Schedulable)
	require.NotNil(t, current.TempUnschedulableUntil)
	require.WithinDuration(t, tempUntil, *current.TempUnschedulableUntil, time.Millisecond)
}

func TestAccountSchedulingHoldBlocksWhenAnyGroupLosesLastAccountIntegration(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	platform := fmt.Sprintf("hold-groups-%d", now.UnixNano())
	accountRepo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	holdRepo := NewAccountSchedulingHoldRepository(integrationDB, accountRepo, nil)
	target := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-target", Platform: platform, Schedulable: true})
	fallback := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-fallback", Platform: platform, Schedulable: true})
	groupWithFallback := mustCreateGroup(t, integrationEntClient, &service.Group{Name: platform + "-a", Platform: platform, RateMultiplier: 1})
	groupWithoutFallback := mustCreateGroup(t, integrationEntClient, &service.Group{Name: platform + "-b", Platform: platform, RateMultiplier: 1})
	cleanupSchedulingHoldGroups(t, []int64{target.ID, fallback.ID}, []int64{groupWithFallback.ID, groupWithoutFallback.ID})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO account_groups (account_id, group_id, priority, created_at) VALUES
		($1, $3, 1, NOW()), ($2, $3, 2, NOW()), ($1, $4, 1, NOW())
	`, target.ID, fallback.ID, groupWithFallback.ID, groupWithoutFallback.ID)
	require.NoError(t, err)
	target, err = accountRepo.GetByID(ctx, target.ID)
	require.NoError(t, err)

	_, err = holdRepo.PutSchedulingHold(ctx, holdPutCommand(target, "hold-group-capacity", now.Add(15*time.Minute)), now)
	require.Equal(t, "CAPACITY_GUARD_BLOCKED", errors.Reason(err))
}

func TestAccountSchedulingHoldCapacityIgnoresQuotaExhaustedFallbackIntegration(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	platform := fmt.Sprintf("hold-quota-%d", now.UnixNano())
	accountRepo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	holdRepo := NewAccountSchedulingHoldRepository(integrationDB, accountRepo, nil)
	target := mustCreateAccount(t, integrationEntClient, &service.Account{Name: platform + "-target", Platform: platform, Type: service.AccountTypeAPIKey, Schedulable: true})
	fallback := mustCreateAccount(t, integrationEntClient, &service.Account{
		Name: platform + "-fallback", Platform: platform, Type: service.AccountTypeAPIKey, Schedulable: true,
		Extra: map[string]any{"quota_limit": 1.0, "quota_used": 1.0},
	})
	cleanupSchedulingHoldAccounts(t, target.ID, fallback.ID)
	target, err := accountRepo.GetByID(ctx, target.ID)
	require.NoError(t, err)

	_, err = holdRepo.PutSchedulingHold(ctx, holdPutCommand(target, "hold-quota-capacity", now.Add(15*time.Minute)), now)
	require.Equal(t, "CAPACITY_GUARD_BLOCKED", errors.Reason(err))
}

func holdPutCommand(account *service.Account, decisionID string, leaseUntil time.Time) service.AccountSchedulingHoldPut {
	return service.AccountSchedulingHoldPut{
		AccountID: account.ID, Owner: service.AccountSchedulingHoldOwner, DecisionID: decisionID,
		ReasonCode: service.AccountSchedulingHoldReasonManualApproved, LeaseUntil: leaseUntil,
		ExpectedAccountUpdatedAt: account.UpdatedAt, RequestHash: decisionID + "-hash",
		MaximumCumulativeLease: service.AccountSchedulingHoldMaximumTotal,
	}
}

func accountHealthStateIDs(states []*service.AccountHealthState) []int64 {
	ids := make([]int64, 0, len(states))
	for _, state := range states {
		if state != nil {
			ids = append(ids, state.AccountID)
		}
	}
	return ids
}

func cleanupSchedulingHoldAccounts(t *testing.T, accountIDs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, accountID := range accountIDs {
			_, _ = integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE account_id = $1`, accountID)
			_, _ = integrationDB.Exec(`DELETE FROM account_health_states WHERE account_id = $1`, accountID)
			_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id = $1`, accountID)
		}
	})
}

func cleanupSchedulingHoldGroups(t *testing.T, accountIDs, groupIDs []int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, accountID := range accountIDs {
			_, _ = integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE account_id = $1`, accountID)
			_, _ = integrationDB.Exec(`DELETE FROM account_health_states WHERE account_id = $1`, accountID)
			_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id = $1`, accountID)
		}
		for _, groupID := range groupIDs {
			_, _ = integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE group_id = $1`, groupID)
			_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id = $1`, groupID)
		}
	})
}
