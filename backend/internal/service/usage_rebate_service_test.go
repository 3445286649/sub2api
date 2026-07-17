package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type usageRebateSettingsStub struct{ enabled bool }

func (s usageRebateSettingsStub) IsUsageRebateEnabled(context.Context) bool { return s.enabled }

type usageRebateRepositoryStub struct {
	ensureCalls   int
	claimQueue    []*UsageRebatePeriod
	sealed        []int64
	payable       map[int64][]UsageRebateReward
	credited      []int64
	unknown       []int64
	failedRewards []int64
	finalized     []int64
	failedPeriods []int64
	creditErr     map[int64]error
}

func (r *usageRebateRepositoryStub) EnsureOpenPeriod(context.Context, UsageRebatePeriodSeed) error {
	r.ensureCalls++
	return nil
}

func (r *usageRebateRepositoryStub) ClaimDuePeriod(context.Context, time.Time, time.Time) (*UsageRebatePeriod, error) {
	if len(r.claimQueue) == 0 {
		return nil, nil
	}
	period := r.claimQueue[0]
	r.claimQueue = r.claimQueue[1:]
	return period, nil
}

func (r *usageRebateRepositoryStub) SealClaimedPeriod(_ context.Context, periodID int64, _ []UsageRebateRate) error {
	r.sealed = append(r.sealed, periodID)
	return nil
}

func (r *usageRebateRepositoryStub) ListPayableRewards(_ context.Context, periodID int64) ([]UsageRebateReward, error) {
	return append([]UsageRebateReward(nil), r.payable[periodID]...), nil
}

func (r *usageRebateRepositoryStub) CreditReward(_ context.Context, rewardID int64) (int64, bool, error) {
	if err := r.creditErr[rewardID]; err != nil {
		return 0, false, err
	}
	r.credited = append(r.credited, rewardID)
	return rewardID + 1000, true, nil
}

func (r *usageRebateRepositoryStub) MarkRewardFailed(_ context.Context, rewardID int64, _ string) error {
	r.failedRewards = append(r.failedRewards, rewardID)
	return nil
}

func (r *usageRebateRepositoryStub) MarkRewardUnknown(_ context.Context, rewardID int64, _ string) error {
	r.unknown = append(r.unknown, rewardID)
	return nil
}

func (r *usageRebateRepositoryStub) FinalizePeriod(_ context.Context, periodID int64) error {
	r.finalized = append(r.finalized, periodID)
	return nil
}

func (r *usageRebateRepositoryStub) MarkPeriodFailed(_ context.Context, periodID int64, _ string) error {
	r.failedPeriods = append(r.failedPeriods, periodID)
	return nil
}

func (r *usageRebateRepositoryStub) GetLeaderboard(context.Context, time.Time, time.Time, int64, int) ([]UsageRebateCandidate, error) {
	return nil, nil
}

func (r *usageRebateRepositoryStub) GetUserPosition(context.Context, time.Time, time.Time, int64) (UsageRebatePosition, error) {
	return UsageRebatePosition{}, nil
}

func (r *usageRebateRepositoryStub) ListUserRewards(context.Context, int64, int) ([]UsageRebateReward, error) {
	return nil, nil
}

func (r *usageRebateRepositoryStub) ListRecentPeriods(context.Context, int) ([]UsageRebatePeriod, error) {
	return nil, nil
}

func (r *usageRebateRepositoryStub) ListPeriodRewards(context.Context, int64, int) ([]UsageRebateReward, error) {
	return nil, nil
}

type usageRebateCacheStub struct{ userIDs []int64 }

func (s *usageRebateCacheStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}

func TestUsageRebateRatesAndOwnSpendFormula(t *testing.T) {
	rates := UsageRebateRates()
	require.Len(t, rates, 20)

	total := decimal.Zero
	for _, rate := range rates {
		total = total.Add(rate.Percent)
	}
	require.True(t, total.Equal(decimal.NewFromInt(100)))
	require.True(t, rates[0].Percent.Equal(decimal.NewFromInt(10)))
	require.True(t, rates[19].Percent.Equal(decimal.RequireFromString("2.5")))

	reward, ok := CalculateUsageRebate(decimal.NewFromInt(36), 1)
	require.True(t, ok)
	require.True(t, reward.Equal(decimal.RequireFromString("3.60000000")))

	_, ok = CalculateUsageRebate(decimal.NewFromInt(36), 21)
	require.False(t, ok)
}

func TestUsageRebateRunOnceDisabledDoesNotCreateNewPeriod(t *testing.T) {
	repo := &usageRebateRepositoryStub{
		claimQueue: nil,
		payable:    map[int64][]UsageRebateReward{},
		creditErr:  map[int64]error{},
	}
	svc := NewUsageRebateService(repo, usageRebateSettingsStub{enabled: false})

	require.NoError(t, svc.RunOnce(context.Background(), time.Date(2026, 7, 18, 0, 16, 0, 0, time.FixedZone("CST", 8*60*60))))
	require.Zero(t, repo.ensureCalls)
	require.Empty(t, repo.sealed)
}

func TestUsageRebateRunOnceDisabledStillCompletesExistingPeriod(t *testing.T) {
	repo := &usageRebateRepositoryStub{
		claimQueue: []*UsageRebatePeriod{{ID: 6}},
		payable:    map[int64][]UsageRebateReward{6: {{ID: 10}}},
		creditErr:  map[int64]error{},
	}
	svc := NewUsageRebateService(repo, usageRebateSettingsStub{enabled: false})

	require.NoError(t, svc.RunOnce(context.Background(), time.Date(2026, 7, 18, 0, 16, 0, 0, time.FixedZone("CST", 8*60*60))))
	require.Zero(t, repo.ensureCalls)
	require.Equal(t, []int64{6}, repo.sealed)
	require.Equal(t, []int64{10}, repo.credited)
}

func TestUsageRebateRunOnceSettlesClaimedPeriodAndInvalidatesCreditedUsers(t *testing.T) {
	repo := &usageRebateRepositoryStub{
		claimQueue: []*UsageRebatePeriod{{ID: 7}},
		payable: map[int64][]UsageRebateReward{
			7: {{ID: 11}, {ID: 12}},
		},
		creditErr: map[int64]error{},
	}
	cache := &usageRebateCacheStub{}
	svc := NewUsageRebateService(repo, usageRebateSettingsStub{enabled: true})
	svc.SetAuthCacheInvalidator(cache)

	now := time.Date(2026, 7, 18, 0, 16, 0, 0, time.FixedZone("CST", 8*60*60))
	require.NoError(t, svc.RunOnce(context.Background(), now))
	require.Equal(t, 1, repo.ensureCalls)
	require.Equal(t, []int64{7}, repo.sealed)
	require.Equal(t, []int64{11, 12}, repo.credited)
	require.Equal(t, []int64{1011, 1012}, cache.userIDs)
	require.Equal(t, []int64{7}, repo.finalized)
}

func TestUsageRebateRunOnceFreezesAmbiguousCommit(t *testing.T) {
	repo := &usageRebateRepositoryStub{
		claimQueue: []*UsageRebatePeriod{{ID: 8}},
		payable: map[int64][]UsageRebateReward{
			8: {{ID: 21}},
		},
		creditErr: map[int64]error{21: errors.Join(ErrUsageRebateCommitUnknown, errors.New("connection reset"))},
	}
	svc := NewUsageRebateService(repo, usageRebateSettingsStub{enabled: true})

	require.NoError(t, svc.RunOnce(context.Background(), time.Date(2026, 7, 18, 0, 16, 0, 0, time.FixedZone("CST", 8*60*60))))
	require.Equal(t, []int64{21}, repo.unknown)
	require.Empty(t, repo.failedRewards)
	require.Equal(t, []int64{8}, repo.finalized)
}
