//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type memoryAccountHealthRepo struct {
	states map[int64]*AccountHealthState
	events []AccountHealthEvent
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

func (r *memoryAccountHealthRepo) ClaimDueProbe(_ context.Context, accountID int64, now time.Time, leaseUntil time.Time) (bool, error) {
	state := r.states[accountID]
	if state == nil {
		state = defaultAccountHealthState(accountID, now)
		if r.states == nil {
			r.states = map[int64]*AccountHealthState{}
		}
		r.states[accountID] = state
	}
	if state.Status == AccountHealthStatusHealthy {
		if state.NextProbeAt != nil && state.NextProbeAt.After(now) {
			return false, nil
		}
		state.NextProbeAt = &leaseUntil
		return true, nil
	}
	if state.NextProbeAt == nil || state.NextProbeAt.After(now) {
		return false, nil
	}
	if state.Status != AccountHealthStatusIsolated && state.Status != AccountHealthStatusRecovering && state.Status != AccountHealthStatusDegraded {
		return false, nil
	}
	state.NextProbeAt = &leaseUntil
	return true, nil
}

func (r *memoryAccountHealthRepo) InsertEvent(_ context.Context, event *AccountHealthEvent) error {
	if event == nil {
		return nil
	}
	cp := *event
	cp.ID = int64(len(r.events) + 1)
	r.events = append(r.events, cp)
	return nil
}

func (r *memoryAccountHealthRepo) ListEvents(_ context.Context, accountID int64, eventType string, params pagination.PaginationParams) (*AccountHealthEventList, error) {
	var filtered []AccountHealthEvent
	for _, event := range r.events {
		if event.AccountID != accountID {
			continue
		}
		if eventType != "" && event.EventType != eventType {
			continue
		}
		filtered = append(filtered, event)
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	start := (params.Page - 1) * params.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + params.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	totalPages := 0
	if params.PageSize > 0 {
		totalPages = (len(filtered) + params.PageSize - 1) / params.PageSize
	}
	return &AccountHealthEventList{Items: filtered[start:end], Total: int64(len(filtered)), Page: params.Page, PageSize: params.PageSize, TotalPages: totalPages}, nil
}

func (r *memoryAccountHealthRepo) DeleteEventsBefore(_ context.Context, before time.Time) (int64, error) {
	kept := r.events[:0]
	var deleted int64
	for _, event := range r.events {
		if event.CreatedAt.Before(before) {
			deleted++
			continue
		}
		kept = append(kept, event)
	}
	r.events = kept
	return deleted, nil
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
	if account := r.accounts[id]; account != nil {
		account.TempUnschedulableUntil = &until
		account.TempUnschedulableReason = accountHealthIsolationReason
	}
	return nil
}
func (r *healthAccountRepoStub) ClearTempUnschedulable(_ context.Context, id int64) error {
	r.tempClearCalls++
	r.lastTempAccountID = id
	if account := r.accounts[id]; account != nil {
		account.TempUnschedulableUntil = nil
		account.TempUnschedulableReason = ""
	}
	return nil
}

func TestOpenAIGatewayService_FilterAccountsByHealthSchedulable(t *testing.T) {
	ctx := context.Background()
	healthRepo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 100, Status: AccountHealthStatusHealthy},
		2: {AccountID: 2, Score: 0, Status: AccountHealthStatusIsolated},
		3: {AccountID: 3, Score: 55, Status: AccountHealthStatusRecovering},
		4: {AccountID: 4, Score: 55, Status: AccountHealthStatusDegraded},
	}}
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true},
		2: {ID: 2, Status: StatusActive, Schedulable: true},
		3: {ID: 3, Status: StatusActive, Schedulable: true},
		4: {ID: 4, Status: StatusActive, Schedulable: true},
	}}
	healthSvc := &AccountHealthService{repo: healthRepo, accountRepo: accountRepo}
	svc := &OpenAIGatewayService{accountHealthService: healthSvc}

	filtered := svc.filterAccountsByHealthSchedulable(ctx, []Account{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})

	require.Equal(t, []int64{1, 4}, []int64{filtered[0].ID, filtered[1].ID})
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

func TestAccountHealthService_HealthProbeModelUsesAccountSetting(t *testing.T) {
	ctx := context.Background()
	model := "gemini-3.5-flash"
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, HealthProbeModel: &model},
		2: {ID: 2, Status: StatusActive},
	}}
	svc := &AccountHealthService{accountRepo: accountRepo, now: time.Now}

	require.Equal(t, model, svc.HealthProbeModel(ctx, 1))
	require.Empty(t, svc.HealthProbeModel(ctx, 2))
	require.Empty(t, svc.HealthProbeModel(ctx, 404))
}

func TestAccountHealthService_HealthySuccessClearsStaleHealthTempUnschedulable(t *testing.T) {
	ctx := context.Background()
	until := time.Now().Add(30 * time.Minute)
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {
			ID:                         1,
			Status:                     StatusActive,
			Schedulable:                true,
			TempUnschedulableUntil:     &until,
			TempUnschedulableReason:    accountHealthIsolationReason,
			HealthProbeEnabled:         true,
			HealthProbeIntervalMinutes: nil,
		},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 90, Status: AccountHealthStatusHealthy, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 120, nil))

	require.Equal(t, 1, accountRepo.tempClearCalls)
	require.Nil(t, accountRepo.accounts[1].TempUnschedulableUntil)
	require.Empty(t, accountRepo.accounts[1].TempUnschedulableReason)
	require.Equal(t, AccountHealthStatusHealthy, repo.states[1].Status)
}

func TestAccountHealthService_HealthySuccessDoesNotClearNonHealthTempUnschedulable(t *testing.T) {
	ctx := context.Background()
	until := time.Now().Add(30 * time.Minute)
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {
			ID:                      1,
			Status:                  StatusActive,
			Schedulable:             true,
			TempUnschedulableUntil:  &until,
			TempUnschedulableReason: "token refresh retry exhausted: network timeout",
			HealthProbeEnabled:      true,
		},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 90, Status: AccountHealthStatusHealthy, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 120, nil))

	require.Zero(t, accountRepo.tempClearCalls)
	require.NotNil(t, accountRepo.accounts[1].TempUnschedulableUntil)
	require.Equal(t, "token refresh retry exhausted: network timeout", accountRepo.accounts[1].TempUnschedulableReason)
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

func TestAccountHealthService_ManualAuthErrorRecoveryAfterTwoSuccesses(t *testing.T) {
	base := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	current := base
	ctx := context.Background()
	until := base.Add(30 * time.Minute)
	oneMinute := 1
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {
			ID:                         1,
			Status:                     StatusActive,
			Schedulable:                true,
			HealthProbeEnabled:         true,
			TempUnschedulableUntil:     &until,
			TempUnschedulableReason:    accountHealthIsolationReason,
			HealthProbeIntervalMinutes: &oneMinute,
		},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return current }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	repo.states[1].Score = 25
	repo.states[1].Status = AccountHealthStatusIsolated

	current = base.Add(time.Minute)
	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 120, nil))
	require.Equal(t, 30, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)
	require.Equal(t, 0, accountRepo.tempClearCalls)

	current = base.Add(2 * time.Minute)
	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 100, nil))

	state := repo.states[1]
	require.Equal(t, AccountHealthStatusHealthy, state.Status)
	require.Equal(t, accountHealthRecoveredScore, state.Score)
	require.Empty(t, state.LastErrorCategory)
	require.Empty(t, state.LastErrorMessage)
	require.Nil(t, state.NextProbeAt)
	require.Nil(t, state.IsolatedAt)
	require.Equal(t, 1, accountRepo.tempClearCalls)
	require.Nil(t, accountRepo.accounts[1].TempUnschedulableUntil)
	require.Equal(t, AccountHealthEventTypeRecovered, repo.events[len(repo.events)-1].EventType)
	require.Equal(t, AccountHealthEventSourceManualProbe, repo.events[len(repo.events)-1].Source)
}

func TestAccountHealthService_BackgroundAuthErrorProbeDoesNotUseManualRecovery(t *testing.T) {
	base := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	current := base
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return current }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	repo.states[1].Score = 25
	repo.states[1].Status = AccountHealthStatusIsolated

	current = base.Add(time.Minute)
	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 120, nil))
	current = base.Add(2 * time.Minute)
	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 100))

	require.Equal(t, 35, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)
	require.Equal(t, 0, accountRepo.tempClearCalls)
}

func TestAccountHealthService_ManualAuthErrorRecoveryRequiresConsecutiveSuccesses(t *testing.T) {
	base := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	current := base
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return current }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	repo.states[1].Score = 25
	repo.states[1].Status = AccountHealthStatusIsolated

	current = base.Add(time.Minute)
	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 120, nil))
	current = base.Add(2 * time.Minute)
	require.NoError(t, svc.RecordManualProbeFailure(ctx, 1, "auth_error", "invalid api key", nil))
	current = base.Add(3 * time.Minute)
	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 100, nil))

	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)
	require.Equal(t, 0, accountRepo.tempClearCalls)
}

func TestAccountHealthService_TemporaryFailuresUseProgressivePenaltyAndThirdFailureIsolates(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return base }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "forward_error", "connection reset"))
	require.Equal(t, 72, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusDegraded, repo.states[1].Status)

	require.NoError(t, svc.RecordFailure(ctx, 1, "timeout", "deadline exceeded"))
	require.Equal(t, 60, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusDegraded, repo.states[1].Status)

	require.NoError(t, svc.RecordFailure(ctx, 1, "network_error", "connection refused"))
	require.Equal(t, 40, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusIsolated, repo.states[1].Status)
	require.Equal(t, base.Add(5*time.Minute), *repo.states[1].NextProbeAt)
	require.Equal(t, *repo.states[1].NextProbeAt, accountRepo.lastTempUntil)
}

func TestAccountHealthService_RateLimitAndModelConfigFailuresAreGentle(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
		2: {ID: 2, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "rate_limited", "returned 429"))
	require.Equal(t, 70, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusDegraded, repo.states[1].Status)

	for i := 0; i < 10; i++ {
		require.NoError(t, svc.RecordFailure(ctx, 2, "model_not_found", "model does not exist"))
	}
	require.Equal(t, 30, repo.states[2].Score)
	require.Equal(t, AccountHealthStatusDegraded, repo.states[2].Status)
	require.Greater(t, accountRepo.tempSetCalls, 0, "repeated model config errors should be soft-unscheduled")
	require.Equal(t, int64(2), accountRepo.lastTempAccountID)
}

func TestAccountHealthService_DegradedRealRequestSuccessesRaiseFloorToSixty(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 35, Status: AccountHealthStatusDegraded, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordSuccess(ctx, 1, 120))
	require.Equal(t, 40, repo.states[1].Score)
	require.NoError(t, svc.RecordSuccess(ctx, 1, 110))
	require.Equal(t, 45, repo.states[1].Score)
	require.NoError(t, svc.RecordSuccess(ctx, 1, 100))
	require.Equal(t, 60, repo.states[1].Score)
	require.Equal(t, AccountHealthStatusDegraded, repo.states[1].Status)
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

func TestAccountHealthService_IsolatedTemporaryErrorUsesFixedFiveMinuteRecoveryProbe(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return base }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))

	require.Equal(t, AccountHealthStatusIsolated, repo.states[1].Status)
	require.Equal(t, base.Add(5*time.Minute), *repo.states[1].NextProbeAt)
	require.Equal(t, *repo.states[1].NextProbeAt, accountRepo.lastTempUntil)
}

func TestAccountHealthService_AuthErrorKeepsBackoffInsteadOfFixedRecoveryProbe(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return base }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid key"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid key"))

	require.Equal(t, AccountHealthStatusIsolated, repo.states[1].Status)
	require.Equal(t, base.Add(5*time.Minute), *repo.states[1].NextProbeAt, "second failure uses normal degraded backoff level 2, not fixed recovery override")
}

func TestAccountHealthService_RecordsEventsAndRedactsMessages(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, GroupIDs: []int64{10}},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", `{"api_key":"sk-secret-value","cookie":"sid=secret"}`))
	require.Len(t, repo.events, 1)
	require.Equal(t, AccountHealthEventTypeFailure, repo.events[0].EventType)
	require.Equal(t, AccountHealthEventSourceRealRequest, repo.events[0].Source)
	require.Equal(t, []int64{10}, repo.events[0].AffectedGroupIDs)
	require.NotContains(t, repo.events[0].ErrorMessage, "sk-secret-value")
	require.NotContains(t, repo.events[0].ErrorMessage, "sid=secret")
	require.Contains(t, repo.events[0].ErrorMessage, "***")
}

func TestAccountHealthService_SkipsNoopRealRequestSuccessEventAtFullScore(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
		2: {ID: 2, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 100, Status: AccountHealthStatusHealthy, CreatedAt: now, UpdatedAt: now},
		2: {AccountID: 2, Score: 100, Status: AccountHealthStatusHealthy, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordSuccess(ctx, 1, 1234))
	require.Empty(t, repo.events)
	require.Equal(t, now, *repo.states[1].LastSuccessAt)
	require.Equal(t, 1234, *repo.states[1].LatencyEWMAMs)

	require.NoError(t, svc.RecordProbeSuccess(ctx, 2, 567))
	require.Len(t, repo.events, 1)
	require.Equal(t, AccountHealthEventSourceBackgroundProbe, repo.events[0].Source)
	require.Equal(t, AccountHealthEventTypeSuccess, repo.events[0].EventType)
}

func TestAccountHealthService_RecordsRecoveryEvent(t *testing.T) {
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
	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 100))

	require.Equal(t, AccountHealthEventTypeRecovered, repo.events[len(repo.events)-1].EventType)
	require.Equal(t, AccountHealthEventSourceBackgroundProbe, repo.events[len(repo.events)-1].Source)
}

func TestAccountHealthService_CleanupEventsDeletesOnlyOldRows(t *testing.T) {
	now := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	repo := &memoryAccountHealthRepo{events: []AccountHealthEvent{
		{ID: 1, AccountID: 1, CreatedAt: now.Add(-31 * 24 * time.Hour)},
		{ID: 2, AccountID: 1, CreatedAt: now.Add(-29 * 24 * time.Hour)},
	}}
	svc := &AccountHealthService{repo: repo, now: func() time.Time { return now }}

	deleted, err := svc.CleanupEvents(context.Background(), now.Add(-30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.Len(t, repo.events, 1)
	require.Equal(t, int64(2), repo.events[0].ID)
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

func TestAccountHealthService_ProbeSuccessSchedulesNextHealthyProbe(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	oldNext := now.Add(-time.Hour)
	interval := 2
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthyProbeEnabled: true, HealthyProbeIntervalHours: &interval},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 95, Status: AccountHealthStatusHealthy, NextProbeAt: &oldNext, CreatedAt: oldNext, UpdatedAt: oldNext},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordManualProbeSuccess(ctx, 1, 120, nil))

	require.NotNil(t, repo.states[1].NextProbeAt)
	require.Equal(t, now.Add(2*time.Hour), *repo.states[1].NextProbeAt)
}

func TestAccountHealthService_ProbeSuccessClearsNextProbeWhenHealthyProbeDisabled(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	oldNext := now.Add(-time.Hour)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthyProbeEnabled: false},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 95, Status: AccountHealthStatusHealthy, NextProbeAt: &oldNext, CreatedAt: oldNext, UpdatedAt: oldNext},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 120))

	require.Nil(t, repo.states[1].NextProbeAt)
}

func TestAccountHealthService_DegradedProbeSuccessReschedulesCustomInterval(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(5 * time.Minute)
	interval := 1
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthProbeIntervalMinutes: &interval},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 55, Status: AccountHealthStatusDegraded, NextProbeAt: &leaseUntil, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 120))

	require.Equal(t, AccountHealthStatusDegraded, repo.states[1].Status)
	require.Equal(t, 60, repo.states[1].Score)
	require.Equal(t, now.Add(time.Minute), *repo.states[1].NextProbeAt)
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
	require.Equal(t, 72, state.Score)
	require.Equal(t, AccountHealthStatusDegraded, state.Status)
	require.NotNil(t, state.NextProbeAt)
	require.True(t, state.NextProbeAt.After(now))
}

func TestAccountHealthService_DegradedLowScoreSoftUnschedulesUntilNextProbe(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	interval := 1
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthProbeIntervalMinutes: &interval},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 65, Status: AccountHealthStatusHealthy, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))

	state := repo.states[1]
	require.Equal(t, AccountHealthStatusDegraded, state.Status)
	require.Less(t, state.Score, accountHealthRecoveryScore)
	require.Equal(t, now.Add(time.Minute), *state.NextProbeAt)
	require.Equal(t, now.Add(time.Minute), accountRepo.lastTempUntil)
	require.Equal(t, int64(1), accountRepo.lastTempAccountID)
}

func TestAccountHealthService_IsolatedTemporaryFailureRespectsCustomProbeInterval(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	interval := 1
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthProbeIntervalMinutes: &interval},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 30, Status: AccountHealthStatusIsolated, ConsecutiveFailures: 3, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordFailure(ctx, 1, "upstream_5xx", "bad gateway"))

	require.Equal(t, AccountHealthStatusIsolated, repo.states[1].Status)
	require.Equal(t, now.Add(time.Minute), *repo.states[1].NextProbeAt)
	require.Equal(t, now.Add(time.Minute), accountRepo.lastTempUntil)
}

func TestAccountHealthService_SettingsChangedReschedulesExistingProbe(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	oldNext := now.Add(30 * time.Minute)
	interval := 1
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, HealthProbeIntervalMinutes: &interval},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 70, Status: AccountHealthStatusDegraded, NextProbeAt: &oldNext, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	require.NoError(t, svc.RecordSettingsChanged(ctx, 1, nil))

	require.Equal(t, now.Add(time.Minute), *repo.states[1].NextProbeAt)
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

func TestAccountHealthService_ClaimDueProbeLeasesDueState(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	due := now.Add(-time.Minute)
	leaseUntil := now.Add(accountHealthProbeLease)
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Status: AccountHealthStatusIsolated, NextProbeAt: &due},
	}}
	svc := &AccountHealthService{repo: repo}

	claimed, err := svc.ClaimDueProbe(ctx, 1, now, leaseUntil)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, leaseUntil, *repo.states[1].NextProbeAt)

	claimed, err = svc.ClaimDueProbe(ctx, 1, now, now.Add(2*accountHealthProbeLease))
	require.NoError(t, err)
	require.False(t, claimed)
	require.Equal(t, leaseUntil, *repo.states[1].NextProbeAt)
}

func TestAccountHealthService_ClaimDueProbeLeasesHealthyState(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(accountHealthProbeLease)
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Status: AccountHealthStatusHealthy},
	}}
	svc := &AccountHealthService{repo: repo}

	claimed, err := svc.ClaimDueProbe(ctx, 1, now, leaseUntil)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, leaseUntil, *repo.states[1].NextProbeAt)

	claimed, err = svc.ClaimDueProbe(ctx, 1, now, now.Add(2*accountHealthProbeLease))
	require.NoError(t, err)
	require.False(t, claimed)
	require.Equal(t, leaseUntil, *repo.states[1].NextProbeAt)
}

func TestAccountHealthService_ClaimDueProbeCreatesHealthyLeaseState(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(accountHealthProbeLease)
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo}

	claimed, err := svc.ClaimDueProbe(ctx, 9, now, leaseUntil)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NotNil(t, repo.states[9])
	require.Equal(t, AccountHealthStatusHealthy, repo.states[9].Status)
	require.Equal(t, leaseUntil, *repo.states[9].NextProbeAt)
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

func TestAccountHealthService_DedupesSameBackgroundProbeFailureWithinWindow(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	current := base
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return current }}

	require.NoError(t, svc.RecordProbeFailure(ctx, 1, "probe_failed", "upstream temporarily unavailable"))
	require.NoError(t, svc.RecordProbeFailure(ctx, 1, "timeout", "request timeout"))
	require.Equal(t, 60, repo.states[1].Score)
	require.Equal(t, "timeout", repo.states[1].LastErrorCategory)
	require.Len(t, repo.events, 2)

	require.NoError(t, svc.RecordProbeFailure(ctx, 1, "timeout", "request timeout"))
	require.Equal(t, 60, repo.states[1].Score)
	require.Len(t, repo.events, 2)

	current = base.Add(accountHealthProbeFailureDedupeWindow + time.Second)
	require.NoError(t, svc.RecordProbeFailure(ctx, 1, "timeout", "request timeout"))
	require.Equal(t, 40, repo.states[1].Score)
	require.Len(t, repo.events, 3)
}

func TestAccountHealthService_RecordRealSuccessClearsAuthError(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)

	require.NoError(t, svc.RecordSuccess(ctx, 1, 123))
	require.Empty(t, repo.states[1].LastErrorCategory)
	require.Empty(t, repo.states[1].LastErrorMessage)
}

func TestAccountHealthService_AuthErrorIsolationDoesNotFastRecoverAfterRealSuccess(t *testing.T) {
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true},
	}}
	repo := &memoryAccountHealthRepo{}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}

	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.NoError(t, svc.RecordFailure(ctx, 1, "auth_error", "invalid api key"))
	require.Equal(t, AccountHealthStatusIsolated, repo.states[1].Status)
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)

	require.NoError(t, svc.RecordSuccess(ctx, 1, 100))
	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)
	require.Equal(t, 25, repo.states[1].Score)
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)

	require.NoError(t, svc.RecordProbeSuccess(ctx, 1, 100))
	require.Equal(t, 30, repo.states[1].Score, "auth_error recovery probe should use normal +5 reward, not fast +25")
	require.Equal(t, AccountHealthStatusRecovering, repo.states[1].Status)
	require.Equal(t, "auth_error", repo.states[1].LastErrorCategory)
}

func TestAccountHealthService_OverviewBuildsRiskSummary(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	accountRepo := &healthAccountRepoStub{accounts: map[int64]*Account{
		1: {ID: 1, Name: "a", Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, Credentials: map[string]any{"base_url": "https://u1", "api_key": "k1"}, GroupIDs: []int64{10}},
		2: {ID: 2, Name: "b", Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, Credentials: map[string]any{"base_url": "https://u2", "api_key": "k2"}, GroupIDs: []int64{20}},
		3: {ID: 3, Name: "c", Status: StatusActive, Schedulable: true, HealthProbeEnabled: true, Credentials: map[string]any{"base_url": "https://u2", "api_key": "k3"}, GroupIDs: []int64{20}},
	}}
	repo := &memoryAccountHealthRepo{states: map[int64]*AccountHealthState{
		1: {AccountID: 1, Score: 80, Status: AccountHealthStatusHealthy, CreatedAt: now, UpdatedAt: now},
		2: {AccountID: 2, Score: 30, Status: AccountHealthStatusIsolated, CreatedAt: now, UpdatedAt: now},
		3: {AccountID: 3, Score: 30, Status: AccountHealthStatusIsolated, CreatedAt: now, UpdatedAt: now},
	}}
	svc := &AccountHealthService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return now }}

	overview, err := svc.Overview(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, overview.Risks)
	require.Contains(t, riskTypes(overview.Risks), "group_single_available_account")
	require.Contains(t, riskTypes(overview.Risks), "url_all_isolated")
	require.Contains(t, riskTypes(overview.Risks), "group_no_available_accounts")
}

func TestSortAccountPointersByHealthCostAndLRU_CostBeforeHealthThenLatency(t *testing.T) {
	cheap := 0.1
	expensive := 0.2
	accounts := []*Account{
		{ID: 1, RateMultiplier: &expensive},
		{ID: 2, RateMultiplier: &cheap},
		{ID: 3, RateMultiplier: &cheap},
	}
	latency3 := 300
	latency2 := 100
	health := map[int64]*AccountHealthSummary{
		1: {AccountHealthState: AccountHealthState{AccountID: 1, Score: 100}},
		2: {AccountHealthState: AccountHealthState{AccountID: 2, Score: 90, LatencyEWMAMs: &latency2}},
		3: {AccountHealthState: AccountHealthState{AccountID: 3, Score: 90, LatencyEWMAMs: &latency3}},
	}

	sortAccountPointersByHealthCostAndLRU(accounts, health, false, testHealthSortConfig())

	require.Equal(t, []int64{2, 3, 1}, []int64{accounts[0].ID, accounts[1].ID, accounts[2].ID})
}

func TestSortAccountPointersByHealthCostAndLRU_StableHealthBeforeDegradedCost(t *testing.T) {
	cheap := 0.1
	expensive := 0.2
	accounts := []*Account{
		{ID: 1, RateMultiplier: &cheap},
		{ID: 2, RateMultiplier: &expensive},
	}
	health := map[int64]*AccountHealthSummary{
		1: {AccountHealthState: AccountHealthState{AccountID: 1, Score: 65, Status: AccountHealthStatusDegraded}},
		2: {AccountHealthState: AccountHealthState{AccountID: 2, Score: 90, Status: AccountHealthStatusHealthy}},
	}

	sortAccountPointersByHealthCostAndLRU(accounts, health, false, testHealthSortConfig())

	require.Equal(t, []int64{2, 1}, []int64{accounts[0].ID, accounts[1].ID})
}

func TestSortAccountPointersByHealthCostAndLRU_HealthTierBeforeCost(t *testing.T) {
	cheap := 0.5
	mid := 1.0
	expensive := 1.5
	accounts := []*Account{
		{ID: 1, RateMultiplier: &cheap},
		{ID: 2, RateMultiplier: &mid},
		{ID: 3, RateMultiplier: &expensive},
	}
	health := map[int64]*AccountHealthSummary{
		1: {AccountHealthState: AccountHealthState{AccountID: 1, Score: 65}},
		2: {AccountHealthState: AccountHealthState{AccountID: 2, Score: 90}},
		3: {AccountHealthState: AccountHealthState{AccountID: 3, Score: 95}},
	}

	sortAccountPointersByHealthCostAndLRU(accounts, health, false, testHealthSortConfig())

	require.Equal(t, []int64{2, 3, 1}, []int64{accounts[0].ID, accounts[1].ID, accounts[2].ID})
}

func TestSortAccountPointersByHealthCostAndLRU_UnknownHealthUsesConfiguredHealthyTier(t *testing.T) {
	cheap := 0.5
	expensive := 1.0
	accounts := []*Account{
		{ID: 1, RateMultiplier: &cheap},
		{ID: 2, RateMultiplier: &expensive},
	}
	cfg := testHealthSortConfig()
	cfg.HealthTierHealthyMin = 85
	health := map[int64]*AccountHealthSummary{
		1: {AccountHealthState: AccountHealthState{AccountID: 1, Score: 65}},
	}

	sortAccountPointersByHealthCostAndLRU(accounts, health, false, cfg)

	require.Equal(t, []int64{2, 1}, []int64{accounts[0].ID, accounts[1].ID})
	score, _ := accountHealthSortValueForScheduling(health, 2, cfg)
	require.Equal(t, 85, score)
	require.Equal(t, 0, accountHealthTier(score, cfg))
}

func TestSortAccountWithLoadByHealthCostAndLoad_ScoreLatencyThenLoad(t *testing.T) {
	rate := 0.1
	accounts := []accountWithLoad{
		{account: &Account{ID: 1, RateMultiplier: &rate}, loadInfo: &AccountLoadInfo{LoadRate: 90}},
		{account: &Account{ID: 2, RateMultiplier: &rate}, loadInfo: &AccountLoadInfo{LoadRate: 80}},
		{account: &Account{ID: 3, RateMultiplier: &rate}, loadInfo: &AccountLoadInfo{LoadRate: 20}},
		{account: &Account{ID: 4, RateMultiplier: &rate}, loadInfo: &AccountLoadInfo{LoadRate: 10}},
	}
	latency1 := 100
	latency2 := 300
	latency3 := 300
	latency4 := 300
	health := map[int64]*AccountHealthSummary{
		1: {AccountHealthState: AccountHealthState{AccountID: 1, Score: 95, LatencyEWMAMs: &latency1}},
		2: {AccountHealthState: AccountHealthState{AccountID: 2, Score: 95, LatencyEWMAMs: &latency2}},
		3: {AccountHealthState: AccountHealthState{AccountID: 3, Score: 90, LatencyEWMAMs: &latency3}},
		4: {AccountHealthState: AccountHealthState{AccountID: 4, Score: 90, LatencyEWMAMs: &latency4}},
	}

	sortAccountWithLoadByHealthCostAndLoad(accounts, health, false, testHealthSortConfig())

	require.Equal(t, []int64{1, 2, 4, 3}, []int64{accounts[0].account.ID, accounts[1].account.ID, accounts[2].account.ID, accounts[3].account.ID})
}

func TestSortAccountPointersByHealthCostAndLRU_DisabledUsesLegacyPriorityLRU(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	recent := now.Add(-time.Minute)
	cheap := 0.1
	expensive := 0.3
	accounts := []*Account{
		{ID: 1, Priority: 20, RateMultiplier: &cheap, LastUsedAt: &old},
		{ID: 2, Priority: 10, RateMultiplier: &expensive, LastUsedAt: &recent},
	}
	health := map[int64]*AccountHealthSummary{
		1: {AccountHealthState: AccountHealthState{AccountID: 1, Score: 99}},
		2: {AccountHealthState: AccountHealthState{AccountID: 2, Score: 10}},
	}

	cfg := testHealthSortConfig()
	cfg.HealthSortEnabled = false
	sortAccountPointersByHealthCostAndLRU(accounts, health, false, cfg)

	require.Equal(t, []int64{2, 1}, []int64{accounts[0].ID, accounts[1].ID})
}

func testHealthSortConfig() config.GatewaySchedulingConfig {
	return config.GatewaySchedulingConfig{
		HealthSortEnabled:     true,
		HealthTierHealthyMin:  80,
		HealthTierDegradedMin: 60,
	}
}

func riskTypes(risks []AccountHealthRisk) []string {
	out := make([]string, 0, len(risks))
	for _, risk := range risks {
		out = append(out, risk.Type)
	}
	return out
}
