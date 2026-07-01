//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type memoryAccountHealthRepo struct {
	states map[int64]*AccountHealthState
}

func (r *memoryAccountHealthRepo) Get(_ context.Context, accountID int64) (*AccountHealthState, error) {
	if r.states == nil {
		r.states = map[int64]*AccountHealthState{}
	}
	if state := r.states[accountID]; state != nil {
		cp := *state
		return &cp, nil
	}
	return nil, nil
}

func (r *memoryAccountHealthRepo) ListByAccountIDs(_ context.Context, ids []int64) (map[int64]*AccountHealthState, error) {
	out := map[int64]*AccountHealthState{}
	for _, id := range ids {
		if state := r.states[id]; state != nil {
			cp := *state
			out[id] = &cp
		}
	}
	return out, nil
}

func (r *memoryAccountHealthRepo) Upsert(_ context.Context, state *AccountHealthState) error {
	if r.states == nil {
		r.states = map[int64]*AccountHealthState{}
	}
	cp := *state
	r.states[state.AccountID] = &cp
	return nil
}

func (r *memoryAccountHealthRepo) Delete(_ context.Context, accountID int64) error {
	delete(r.states, accountID)
	return nil
}

func (r *memoryAccountHealthRepo) ListDueForProbe(_ context.Context, now time.Time, _ int) ([]*AccountHealthState, error) {
	var out []*AccountHealthState
	for _, state := range r.states {
		if state.NextProbeAt != nil && !state.NextProbeAt.After(now) && (state.Status == AccountHealthStatusIsolated || state.Status == AccountHealthStatusRecovering || state.Status == AccountHealthStatusDegraded) {
			cp := *state
			out = append(out, &cp)
		}
	}
	return out, nil
}

type healthAccountRepoStub struct {
	accounts          map[int64]*Account
	tempSetCalls      int
	tempClearCalls    int
	lastTempAccountID int64
	lastTempUntil     time.Time
}

func (r *healthAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	return r.accounts[id], nil
}
func (r *healthAccountRepoStub) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	var out []*Account
	for _, id := range ids {
		if a := r.accounts[id]; a != nil {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *healthAccountRepoStub) List(_ context.Context, _ pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	var out []Account
	for _, a := range r.accounts {
		out = append(out, *a)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out))}, nil
}
func (r *healthAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, _ string) error {
	r.tempSetCalls++
	r.lastTempAccountID = id
	r.lastTempUntil = until
	return nil
}
func (r *healthAccountRepoStub) ClearTempUnschedulable(_ context.Context, id int64) error {
	r.tempClearCalls++
	r.lastTempAccountID = id
	return nil
}

func TestAccountHealthService_IsolatesByAccountID(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": "https://upstream.test", "api_key": "key-a"}},
		2: {ID: 2, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": "https://upstream.test", "api_key": "key-b"}},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))

	a, err := svc.Get(ctx, 1)
	require.NoError(t, err)
	b, err := svc.Get(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, AccountHealthStatusIsolated, a.Status)
	require.Equal(t, AccountHealthStatusHealthy, b.Status)
	require.GreaterOrEqual(t, accountRepo.tempSetCalls, 1)
	require.Equal(t, int64(1), accountRepo.lastTempAccountID)
}

func TestAccountHealthService_RecoversByClearingOnlyTemporaryIsolation(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: false},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	repo.states[1].Score = 70
	require.NoError(t, svc.RecordSuccess(ctx, 1, 120))
	require.NoError(t, svc.RecordSuccess(ctx, 1, 100))

	state := repo.states[1]
	require.Equal(t, AccountHealthStatusHealthy, state.Status)
	require.Equal(t, 1, accountRepo.tempClearCalls)
	require.False(t, accountRepo.accounts[1].Schedulable, "manual schedulable=false must not be flipped by auto recovery")
}

func TestAccountHealthService_ProbeSuccessFastRecoversIsolatedAccount(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	repo.states[1].Score = 25

	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 120))
	require.Equal(t, 50, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)

	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 100))
	require.Equal(t, AccountHealthStatusHealthy, repo.states[1].Status)
	require.Equal(t, 75, repo.states[1].Score)
	require.Nil(t, repo.states[1].NextProbeAt)
	require.Equal(t, 1, accountRepo.tempClearCalls)
}

func TestAccountHealthService_AuthErrorDoesNotUseFastProbeRecovery(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	repo.states[1].Score = 25
	repo.states[1].Status = AccountHealthStatusIsolated

	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 120))
	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 100))

	require.Equal(t, 35, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)
	require.Equal(t, 0, accountRepo.tempClearCalls)
}

func TestAccountHealthService_ListDueForProbeSkipsDisabledAccounts(t *testing.T) {
	now := time.Now()
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: false},
		2: {ID: 2, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Status: AccountHealthStatusIsolated, NextProbeAt: &now},
		2: {AccountID: 2, Status: AccountHealthStatusIsolated, NextProbeAt: &now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	due, err := svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, int64(2), due[0].AccountID)
}

func TestAccountHealthService_FixedProbeIntervalOverridesBackoff(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	interval := 3
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, HealthProbeEnabled: true, HealthProbeIntervalMinutes: &interval},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return base }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NotNil(t, repo.states[1].NextProbeAt)
	require.Equal(t, base.Add(3*time.Minute), *repo.states[1].NextProbeAt)
}

func TestAccountHealthService_HealthyAccountsAreNotProbedByDefault(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastChecked := now.Add(-24 * time.Hour)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 80, Status: AccountHealthStatusHealthy, LastCheckedAt: &lastChecked, CreatedAt: lastChecked, UpdatedAt: lastChecked},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	due, err := svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Empty(t, due)
}

func TestAccountHealthService_HealthyProbeUsesDefaultSixHourInterval(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastChecked := now.Add(-6*time.Hour - time.Minute)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthyProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 80, Status: AccountHealthStatusHealthy, LastCheckedAt: &lastChecked, CreatedAt: lastChecked, UpdatedAt: lastChecked},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	due, err := svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, int64(1), due[0].AccountID)
}

func TestAccountHealthService_HealthyProbeUsesCustomHourInterval(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastChecked := now.Add(-2 * time.Hour)
	interval := 3
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthyProbeEnabled: true, HealthyProbeIntervalHours: &interval},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 80, Status: AccountHealthStatusHealthy, LastCheckedAt: &lastChecked, CreatedAt: lastChecked, UpdatedAt: lastChecked},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	due, err := svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Empty(t, due)

	repo.states[1].LastCheckedAt = accountHealthPtrTime(now.Add(-3*time.Hour - time.Second))
	due, err = svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, int64(1), due[0].AccountID)
}

func TestAccountHealthService_HealthyProbeSkipsManualUnschedulableAccount(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastChecked := now.Add(-24 * time.Hour)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: false, HealthProbeEnabled: true, HealthyProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 80, Status: AccountHealthStatusHealthy, LastCheckedAt: &lastChecked, CreatedAt: lastChecked, UpdatedAt: lastChecked},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	due, err := svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Empty(t, due)
}

func TestAccountHealthService_HealthyProbeFailureEntersDegradedBackoff(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthyProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 80, Status: AccountHealthStatusHealthy, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))

	state := repo.states[1]
	require.Equal(t, 55, state.Score)
	require.Equal(t, AccountHealthStatusDegraded, state.Status)
	require.NotNil(t, state.NextProbeAt)
	require.True(t, state.NextProbeAt.After(now))
}

func TestAccountHealthService_HealthyProbeWithoutStateIsDueImmediately(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthyProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	due, err := svc.ListDueForProbe(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, int64(1), due[0].AccountID)
	require.Equal(t, AccountHealthStatusHealthy, due[0].Status)
}

func TestAccountHealthService_IsolatedFailureExtendsTemporaryIsolation(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	current := base
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return current }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	firstUntil := accountRepo.lastTempUntil
	firstSetCalls := accountRepo.tempSetCalls
	require.GreaterOrEqual(t, firstSetCalls, 1)

	current = base.Add(2 * time.Minute)
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "still bad"))

	require.Equal(t, AccountHealthStatusIsolated, repo.states[1].Status)
	require.Equal(t, firstSetCalls+1, accountRepo.tempSetCalls)
	require.True(t, accountRepo.lastTempUntil.After(firstUntil), "isolated failures must extend the temporary unschedulable window")
	require.Equal(t, *repo.states[1].NextProbeAt, accountRepo.lastTempUntil)
}

func TestAccountHealthRunnerClaimPreventsDuplicateInFlightProbe(t *testing.T) {
	runner := NewAccountHealthRunner(nil, nil)

	require.True(t, runner.tryClaim(1))
	require.False(t, runner.tryClaim(1))

	runner.release(1)
	require.True(t, runner.tryClaim(1))
}

func TestAccountHealthService_RecordFailureRedactsSensitiveMessage(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", `{"api_key":"sk-secret-value","authorization":"Bearer abc","cookie":"sid=secret"}`))

	message := repo.states[1].LastErrorMessage
	require.NotContains(t, message, "sk-secret-value")
	require.NotContains(t, message, "Bearer abc")
	require.NotContains(t, message, "sid=secret")
	require.Contains(t, message, "***")
}
