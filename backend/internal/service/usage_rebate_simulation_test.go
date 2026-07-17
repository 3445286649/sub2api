package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type simulatedUsageRebateRepo struct {
	mu         sync.Mutex
	period     UsageRebatePeriod
	candidates []UsageRebateCandidate
	rewards    map[int64]UsageRebateReward
	balances   map[int64]decimal.Decimal
	claimed    bool
	sealed     bool
}

func newSimulatedUsageRebateRepo() *simulatedUsageRebateRepo {
	repo := &simulatedUsageRebateRepo{
		period:   UsageRebatePeriod{ID: 1, BusinessDate: "2026-07-17"},
		rewards:  map[int64]UsageRebateReward{},
		balances: map[int64]decimal.Decimal{},
	}
	for userID := int64(1); userID <= 25; userID++ {
		spend := decimal.NewFromInt(26 - userID)
		repo.candidates = append(repo.candidates, UsageRebateCandidate{
			UserID: userID, Username: fmt.Sprintf("user-%02d", userID),
			Requests: userID * 2, Tokens: (26 - userID) * 1000, SpendAmount: spend,
		})
		repo.balances[userID] = decimal.NewFromInt(100)
	}
	return repo
}

func (r *simulatedUsageRebateRepo) EnsureOpenPeriod(context.Context, UsageRebatePeriodSeed) error {
	return nil
}

func (r *simulatedUsageRebateRepo) ClaimDuePeriod(context.Context, time.Time, time.Time) (*UsageRebatePeriod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return nil, nil
	}
	r.claimed = true
	period := r.period
	return &period, nil
}

func (r *simulatedUsageRebateRepo) SealClaimedPeriod(_ context.Context, periodID int64, rates []UsageRebateRate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return nil
	}
	for index, candidate := range r.candidates[:20] {
		rank := index + 1
		amount, _ := CalculateUsageRebate(candidate.SpendAmount, rank)
		id := int64(rank)
		r.rewards[id] = UsageRebateReward{
			ID: id, PeriodID: periodID, UserID: candidate.UserID, Rank: rank,
			SpendAmount: candidate.SpendAmount, RebatePercent: rates[index].Percent,
			RewardAmount: amount, Status: UsageRebateRewardStatusPending,
		}
	}
	r.sealed = true
	return nil
}

func (r *simulatedUsageRebateRepo) ListPayableRewards(_ context.Context, periodID int64) ([]UsageRebateReward, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var rewards []UsageRebateReward
	for _, reward := range r.rewards {
		if reward.PeriodID == periodID && (reward.Status == UsageRebateRewardStatusPending || reward.Status == UsageRebateRewardStatusFailed) {
			rewards = append(rewards, reward)
		}
	}
	sort.Slice(rewards, func(i, j int) bool { return rewards[i].Rank < rewards[j].Rank })
	return rewards, nil
}

func (r *simulatedUsageRebateRepo) CreditReward(_ context.Context, rewardID int64) (int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	reward, ok := r.rewards[rewardID]
	if !ok || reward.Status == UsageRebateRewardStatusCredited {
		return reward.UserID, false, nil
	}
	r.balances[reward.UserID] = r.balances[reward.UserID].Add(reward.RewardAmount)
	reward.Status = UsageRebateRewardStatusCredited
	r.rewards[rewardID] = reward
	return reward.UserID, true, nil
}

func (r *simulatedUsageRebateRepo) MarkRewardFailed(context.Context, int64, string) error {
	return nil
}
func (r *simulatedUsageRebateRepo) MarkRewardUnknown(context.Context, int64, string) error {
	return nil
}
func (r *simulatedUsageRebateRepo) FinalizePeriod(context.Context, int64) error { return nil }
func (r *simulatedUsageRebateRepo) MarkPeriodFailed(context.Context, int64, string) error {
	return nil
}
func (r *simulatedUsageRebateRepo) GetLeaderboard(_ context.Context, _, _ time.Time, _ int64, limit int) ([]UsageRebateCandidate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := append([]UsageRebateCandidate(nil), r.candidates...)
	if len(items) > limit {
		items = items[:limit]
	}
	for index := range items {
		items[index].Rank = index + 1
		items[index].RebatePercent = UsageRebateRates()[index].Percent
		items[index].EstimatedReward, _ = CalculateUsageRebate(items[index].SpendAmount, index+1)
	}
	return items, nil
}
func (r *simulatedUsageRebateRepo) GetUserPosition(context.Context, time.Time, time.Time, int64) (UsageRebatePosition, error) {
	return UsageRebatePosition{}, nil
}
func (r *simulatedUsageRebateRepo) ListUserRewards(_ context.Context, userID int64, _ int) ([]UsageRebateReward, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []UsageRebateReward
	for _, reward := range r.rewards {
		if reward.UserID == userID {
			items = append(items, reward)
		}
	}
	return items, nil
}
func (r *simulatedUsageRebateRepo) ListRecentPeriods(context.Context, int) ([]UsageRebatePeriod, error) {
	return []UsageRebatePeriod{r.period}, nil
}
func (r *simulatedUsageRebateRepo) ListPeriodRewards(context.Context, int64, int) ([]UsageRebateReward, error) {
	return r.ListPayableRewards(context.Background(), 1)
}

func TestUsageRebateSimulationTwentyFiveUsersAndTwoRunnersRemainIdempotent(t *testing.T) {
	repo := newSimulatedUsageRebateRepo()
	runnerA := NewUsageRebateService(repo, usageRebateSettingsStub{enabled: false})
	runnerB := NewUsageRebateService(repo, usageRebateSettingsStub{enabled: false})
	now := time.Date(2026, 7, 18, 0, 16, 0, 0, time.FixedZone("CST", 8*60*60))

	var wg sync.WaitGroup
	for _, runner := range []*UsageRebateService{runnerA, runnerB} {
		wg.Add(1)
		go func(svc *UsageRebateService) {
			defer wg.Done()
			require.NoError(t, svc.RunOnce(context.Background(), now))
		}(runner)
	}
	wg.Wait()

	require.Len(t, repo.rewards, 20)
	require.True(t, repo.balances[1].Equal(decimal.RequireFromString("102.5")))
	require.True(t, repo.balances[20].Equal(decimal.RequireFromString("100.15")))
	for userID := int64(21); userID <= 25; userID++ {
		require.True(t, repo.balances[userID].Equal(decimal.NewFromInt(100)))
	}

	before := make(map[int64]decimal.Decimal, len(repo.balances))
	for userID, balance := range repo.balances {
		before[userID] = balance
	}
	require.NoError(t, runnerA.RunOnce(context.Background(), now.Add(time.Minute)))
	require.Equal(t, before, repo.balances)
}
