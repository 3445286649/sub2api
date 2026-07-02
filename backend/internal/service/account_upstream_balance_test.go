package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestParseUpstreamBalanceResponseCommonFields(t *testing.T) {
	result, err := parseUpstreamBalanceResponse([]byte(`{"balance":"238.84008877","remaining":5.5,"unit":"USD"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.balance)
	require.InDelta(t, 238.84008877, *result.balance, 0.00000001)
	require.NotNil(t, result.remaining)
	require.Equal(t, 5.5, *result.remaining)
	require.Equal(t, "USD", result.unit)

	result, err = parseUpstreamBalanceResponse([]byte(`{"quota":{"remaining":"31.14402462"}}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.balance)
	require.NotNil(t, result.remaining)
	require.InDelta(t, 31.14402462, *result.remaining, 0.00000001)
	require.Equal(t, "USD", result.unit)
}

func TestSelectUpstreamBalanceRepresentativePrefersActiveSchedulableWithKey(t *testing.T) {
	baseURL := "https://upstream.example.com/v1"
	accounts := []Account{
		{ID: 3, Status: StatusActive, Schedulable: false, Credentials: map[string]any{"base_url": baseURL, "api_key": "sk-three"}},
		{ID: 1, Status: StatusDisabled, Schedulable: true, Credentials: map[string]any{"base_url": baseURL, "api_key": "sk-one"}},
		{ID: 2, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": baseURL + "/", "api_key": "sk-two"}},
		{ID: 4, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": "https://other.example.com/v1", "api_key": "sk-four"}},
		{ID: 5, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": baseURL}},
	}

	account, matched := selectUpstreamBalanceRepresentative(accounts, normalizeUpstreamBalanceBaseURL(baseURL))
	require.True(t, matched)
	require.NotNil(t, account)
	require.Equal(t, int64(2), account.ID)
}

func TestQueryAccountBalanceRejectsPrivateResolvedHost(t *testing.T) {
	var sawAuthorization bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuthorization = strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance":238.84008877,"unit":"USD"}`))
	}))
	defer server.Close()

	svc := &AccountUpstreamBalanceService{}
	account := &Account{ID: 9, Credentials: map[string]any{"api_key": "sk-test-secret"}}
	now := time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC)

	snapshot := svc.queryAccountBalance(context.Background(), server.URL, account, now)
	require.False(t, sawAuthorization, "private localhost URL must be blocked before sending account credentials")
	require.Equal(t, UpstreamBalanceStatusError, snapshot.Status)
	require.Nil(t, snapshot.Balance)
	require.Contains(t, snapshot.ErrorMessage, "not allowed")
	require.Equal(t, now.Add(accountUpstreamBalanceRefreshInterval), *snapshot.NextCheckAt)
}

func TestSanitizeUpstreamBalanceErrorRedactsSecrets(t *testing.T) {
	message := sanitizeUpstreamBalanceError(`bad key sk-live-secret Authorization: Bearer should-not-leak`)
	require.NotContains(t, message, "sk-live-secret")
	require.NotContains(t, message, "should-not-leak")
	require.Contains(t, message, "***")
}

func TestQueryAccountBalanceMarksUnsupportedForNotFound(t *testing.T) {
	now := time.Now().UTC()
	status := http.StatusNotFound

	snapshot := &AccountUpstreamBalanceSnapshot{
		BaseURL:      "https://upstream.example.com",
		Status:       UpstreamBalanceStatusUnsupported,
		HTTPStatus:   &status,
		CheckedAt:    &now,
		NextCheckAt:  ptrUpstreamBalanceTime(now.Add(accountUpstreamBalanceRefreshInterval)),
		UpdatedAt:    now,
		ErrorMessage: sanitizeUpstreamBalanceError("404 page not found"),
	}

	require.Equal(t, UpstreamBalanceStatusUnsupported, snapshot.Status)
	require.NotNil(t, snapshot.HTTPStatus)
	require.Equal(t, http.StatusNotFound, *snapshot.HTTPStatus)
}

func TestRefreshByBaseURLRejectsUnknownURLWithoutWritingCache(t *testing.T) {
	ctx := context.Background()
	repo := &memoryUpstreamBalanceRepo{}
	svc := &AccountUpstreamBalanceService{
		repo: repo,
		accountRepo: &upstreamBalanceAccountRepoStub{accounts: []Account{
			{ID: 1, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": "https://known.example.com", "api_key": "sk-known"}},
		}},
		now: func() time.Time { return time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC) },
	}

	_, err := svc.RefreshByBaseURL(ctx, "https://unknown.example.com")
	require.True(t, infraerrors.IsNotFound(err))
	require.Empty(t, repo.items)
	require.Zero(t, repo.upsertCalls)
}

func TestRefreshByBaseURLUsesClaimRefreshAndReturnsExistingWhenBusy(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC)
	existing := &AccountUpstreamBalanceSnapshot{BaseURL: "https://known.example.com", Status: UpstreamBalanceStatusChecking, UpdatedAt: now}
	repo := &memoryUpstreamBalanceRepo{items: map[string]*AccountUpstreamBalanceSnapshot{existing.BaseURL: existing}, claimRefreshResult: false}
	svc := &AccountUpstreamBalanceService{
		repo: repo,
		accountRepo: &upstreamBalanceAccountRepoStub{accounts: []Account{
			{ID: 1, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": existing.BaseURL, "api_key": "sk-known"}},
		}},
		now: func() time.Time { return now },
	}

	snapshot, err := svc.RefreshByBaseURL(ctx, existing.BaseURL)
	require.NoError(t, err)
	require.Same(t, existing, snapshot)
	require.Equal(t, 1, repo.ensureCalls)
	require.Equal(t, 1, repo.claimRefreshCalls)
	require.Zero(t, repo.upsertCalls)
}

func TestRefreshByBaseURLCachesUnsupportedForKnownURLWithoutKey(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC)
	baseURL := "https://known.example.com"
	repo := &memoryUpstreamBalanceRepo{claimRefreshResult: true}
	svc := &AccountUpstreamBalanceService{
		repo: repo,
		accountRepo: &upstreamBalanceAccountRepoStub{accounts: []Account{
			{ID: 1, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"base_url": baseURL}},
		}},
		now: func() time.Time { return now },
	}

	snapshot, err := svc.RefreshByBaseURL(ctx, baseURL)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusUnsupported, snapshot.Status)
	require.Contains(t, snapshot.ErrorMessage, "no account with api key")
	require.Equal(t, 1, repo.ensureCalls)
	require.Equal(t, 1, repo.claimRefreshCalls)
	require.Equal(t, 1, repo.upsertCalls)
	require.Equal(t, snapshot, repo.items[baseURL])
}

type upstreamBalanceAccountRepoStub struct {
	accounts []Account
}

func (r *upstreamBalanceAccountRepoStub) List(_ context.Context, _ pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return append([]Account(nil), r.accounts...), &pagination.PaginationResult{Total: int64(len(r.accounts))}, nil
}

type memoryUpstreamBalanceRepo struct {
	items              map[string]*AccountUpstreamBalanceSnapshot
	ensureCalls        int
	upsertCalls        int
	claimCalls         int
	claimRefreshCalls  int
	claimRefreshResult bool
}

func (r *memoryUpstreamBalanceRepo) Get(_ context.Context, baseURL string) (*AccountUpstreamBalanceSnapshot, error) {
	if r.items == nil {
		return nil, nil
	}
	return r.items[baseURL], nil
}

func (r *memoryUpstreamBalanceRepo) ListByBaseURLs(_ context.Context, baseURLs []string) (map[string]*AccountUpstreamBalanceSnapshot, error) {
	out := make(map[string]*AccountUpstreamBalanceSnapshot, len(baseURLs))
	for _, baseURL := range baseURLs {
		if item := r.items[baseURL]; item != nil {
			out[baseURL] = item
		}
	}
	return out, nil
}

func (r *memoryUpstreamBalanceRepo) Upsert(_ context.Context, snapshot *AccountUpstreamBalanceSnapshot) error {
	r.upsertCalls++
	if r.items == nil {
		r.items = map[string]*AccountUpstreamBalanceSnapshot{}
	}
	r.items[snapshot.BaseURL] = snapshot
	return nil
}

func (r *memoryUpstreamBalanceRepo) Ensure(_ context.Context, baseURL string, nextCheckAt time.Time) error {
	r.ensureCalls++
	if r.items == nil {
		r.items = map[string]*AccountUpstreamBalanceSnapshot{}
	}
	if r.items[baseURL] == nil {
		r.items[baseURL] = &AccountUpstreamBalanceSnapshot{BaseURL: baseURL, Status: UpstreamBalanceStatusUnsupported, NextCheckAt: &nextCheckAt, UpdatedAt: nextCheckAt}
	}
	return nil
}

func (r *memoryUpstreamBalanceRepo) ListDue(_ context.Context, _ time.Time, _ int) ([]string, error) {
	return nil, nil
}

func (r *memoryUpstreamBalanceRepo) Claim(_ context.Context, _ string, _, _ time.Time) (bool, error) {
	r.claimCalls++
	return true, nil
}

func (r *memoryUpstreamBalanceRepo) ClaimRefresh(_ context.Context, _ string, _, _ time.Time) (bool, error) {
	r.claimRefreshCalls++
	return r.claimRefreshResult, nil
}
