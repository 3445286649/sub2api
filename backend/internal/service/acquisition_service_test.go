package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAcquisitionRecordPaymentCreatesParticipationAndTicketsOnce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	repo := newAcquisitionRepoStub(now)
	svc := NewAcquisitionService(repo, acquisitionAlwaysEnabled{})

	order := &AcquisitionPaymentEvent{
		OrderID:              101,
		UserID:               20,
		OrderType:            "balance",
		SubscriptionPlanType: "subscription",
		PayAmount:            10,
		CompletedAt:          now,
	}
	err := svc.RecordPaymentCompletion(ctx, order)
	require.NoError(t, err)
	require.Len(t, repo.participations, 1)
	require.Len(t, repo.tickets, 2)

	err = svc.RecordPaymentCompletion(ctx, order)
	require.NoError(t, err)
	require.Len(t, repo.participations, 1)
	require.Len(t, repo.tickets, 2)
}

func TestAcquisitionRecordPaymentSkipsNonFirstPayment(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	repo := newAcquisitionRepoStub(now)
	repo.hasPriorPayment = true
	svc := NewAcquisitionService(repo, acquisitionAlwaysEnabled{})

	err := svc.RecordPaymentCompletion(ctx, &AcquisitionPaymentEvent{
		OrderID:     102,
		UserID:      20,
		OrderType:   "balance",
		PayAmount:   10,
		CompletedAt: now,
	})
	require.NoError(t, err)
	require.Empty(t, repo.participations)
	require.Empty(t, repo.tickets)
}

func TestAcquisitionSettleCampaignCreatesLeaderboardAndLotteryRewards(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	repo := newAcquisitionRepoStub(now)
	repo.leaderboard = []AcquisitionLeaderboardRow{
		{UserID: 1, InviteCount: 3},
		{UserID: 2, InviteCount: 2},
		{UserID: 3, InviteCount: 1},
	}
	repo.tickets = []AcquisitionLotteryTicket{
		{ID: 1, CampaignID: 1, UserID: 1},
		{ID: 2, CampaignID: 1, UserID: 1},
		{ID: 3, CampaignID: 1, UserID: 2},
	}
	svc := NewAcquisitionService(repo, acquisitionAlwaysEnabled{})

	err := svc.SettleDueCampaigns(ctx, now)
	require.NoError(t, err)
	require.Len(t, repo.rewards, 5)
	require.Equal(t, 40.0, repo.rewards[0].Amount)
	require.Equal(t, AcquisitionRewardTypeLeaderboard, repo.rewards[0].RewardType)
	require.Equal(t, AcquisitionRewardTypeLottery, repo.rewards[3].RewardType)
	require.Equal(t, AcquisitionRewardStatusPaid, repo.rewards[0].Status)
	require.Equal(t, 120.0, repo.totalPaid)
}

func TestAcquisitionGetCurrentUserSummaryFillsLeaderboardRewardAmounts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	repo := newAcquisitionRepoStub(now)
	repo.leaderboard = []AcquisitionLeaderboardRow{
		{UserID: 10, InviteCount: 3, Rank: 1},
		{UserID: 20, InviteCount: 2, Rank: 2},
		{UserID: 30, InviteCount: 1, Rank: 3},
	}
	svc := NewAcquisitionService(repo, acquisitionAlwaysEnabled{})

	summary, err := svc.GetCurrentUserSummary(ctx, 10)

	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Len(t, summary.Leaderboard, 3)
	require.Equal(t, 40.0, summary.Leaderboard[0].RewardAmount)
	require.Equal(t, 30.0, summary.Leaderboard[1].RewardAmount)
	require.Equal(t, 20.0, summary.Leaderboard[2].RewardAmount)
}

func TestAcquisitionGetCurrentUserSummaryMasksLeaderboardEmailsAndBuildsInviteLink(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	repo := newAcquisitionRepoStub(now)
	repo.affCode = "AFF CODE"
	repo.leaderboard = []AcquisitionLeaderboardRow{
		{UserID: 10, Email: "alice@example.com", InviteCount: 3, Rank: 1},
		{UserID: 20, Email: "bob@example.net", InviteCount: 2, Rank: 2},
	}
	settings := acquisitionSettingStub{frontendURL: "https://subapi.example.com/app/"}
	svc := NewAcquisitionService(repo, settings)

	summary, err := svc.GetCurrentUserSummary(ctx, 10)

	require.NoError(t, err)
	require.Equal(t, "https://subapi.example.com/register?aff=AFF+CODE", summary.InviteLink)
	require.Equal(t, "a***@e***.com", summary.Leaderboard[0].Email)
	require.Equal(t, "b***@e***.net", summary.Leaderboard[1].Email)
}

func TestAcquisitionGetCurrentUserSummaryUsesRelativeInviteLinkWithoutFrontendURL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	repo := newAcquisitionRepoStub(now)
	repo.affCode = "AFF123"
	svc := NewAcquisitionService(repo, acquisitionSettingStub{})

	summary, err := svc.GetCurrentUserSummary(ctx, 10)

	require.NoError(t, err)
	require.Equal(t, "/register?aff=AFF123", summary.InviteLink)
}

type acquisitionAlwaysEnabled struct{}

func (acquisitionAlwaysEnabled) IsAcquisitionEnabled(context.Context) bool            { return true }
func (acquisitionAlwaysEnabled) IsAcquisitionLeaderboardEnabled(context.Context) bool { return true }
func (acquisitionAlwaysEnabled) IsAcquisitionLotteryEnabled(context.Context) bool     { return true }

type acquisitionSettingStub struct {
	frontendURL string
}

func (s acquisitionSettingStub) IsAcquisitionEnabled(context.Context) bool            { return true }
func (s acquisitionSettingStub) IsAcquisitionLeaderboardEnabled(context.Context) bool { return true }
func (s acquisitionSettingStub) IsAcquisitionLotteryEnabled(context.Context) bool     { return true }
func (s acquisitionSettingStub) GetFrontendURL(context.Context) string                { return s.frontendURL }

type acquisitionRepoStub struct {
	campaign        *AcquisitionCampaign
	binding         *AcquisitionInviteBinding
	affCode         string
	hasPriorPayment bool
	participations  []AcquisitionParticipation
	tickets         []AcquisitionLotteryTicket
	leaderboard     []AcquisitionLeaderboardRow
	rewards         []AcquisitionReward
	totalPaid       float64
}

func newAcquisitionRepoStub(now time.Time) *acquisitionRepoStub {
	return &acquisitionRepoStub{
		campaign: &AcquisitionCampaign{
			ID:                  1,
			Name:                "weekly",
			Status:              AcquisitionCampaignStatusActive,
			StartsAt:            now.Add(-time.Hour),
			EndsAt:              now.Add(time.Hour),
			LeaderboardEnabled:  true,
			LotteryEnabled:      true,
			LeaderboardPoolUSD:  100,
			LeaderboardShares:   []float64{40, 30, 20, 5, 5},
			LotterySeed:         "seed",
			LotteryPrizeConfigs: []AcquisitionLotteryPrizeConfig{{Name: "big", AmountUSD: 20, Count: 1, PerUserCap: 1}, {Name: "small", AmountUSD: 10, Count: 1, PerUserCap: 0}},
		},
		binding: &AcquisitionInviteBinding{
			InviterID: 10,
			BoundAt:   now.Add(-30 * time.Minute),
		},
	}
}

func (r *acquisitionRepoStub) ListCampaigns(ctx context.Context, filter AcquisitionCampaignFilter) ([]AcquisitionCampaign, error) {
	return []AcquisitionCampaign{*r.campaign}, nil
}

func (r *acquisitionRepoStub) GetCampaign(ctx context.Context, id int64) (*AcquisitionCampaign, error) {
	return r.campaign, nil
}

func (r *acquisitionRepoStub) CreateCampaign(ctx context.Context, input AcquisitionCampaignInput) (*AcquisitionCampaign, error) {
	return r.campaign, nil
}

func (r *acquisitionRepoStub) UpdateCampaign(ctx context.Context, id int64, input AcquisitionCampaignInput) (*AcquisitionCampaign, error) {
	return r.campaign, nil
}

func (r *acquisitionRepoStub) ListActiveCampaignsForCompletion(ctx context.Context, completedAt time.Time) ([]AcquisitionCampaign, error) {
	return []AcquisitionCampaign{*r.campaign}, nil
}

func (r *acquisitionRepoStub) GetInviteBinding(ctx context.Context, inviteeUserID int64) (*AcquisitionInviteBinding, error) {
	return r.binding, nil
}

func (r *acquisitionRepoStub) HasPriorEligiblePayment(ctx context.Context, userID, orderID int64, completedAt time.Time) (bool, error) {
	return r.hasPriorPayment, nil
}

func (r *acquisitionRepoStub) CreateParticipationWithTickets(ctx context.Context, p AcquisitionParticipation) (bool, error) {
	for _, existing := range r.participations {
		if existing.CampaignID == p.CampaignID && existing.InviteeID == p.InviteeID {
			return false, nil
		}
	}
	r.participations = append(r.participations, p)
	r.tickets = append(r.tickets,
		AcquisitionLotteryTicket{ID: int64(len(r.tickets) + 1), CampaignID: p.CampaignID, UserID: p.InviterID, InviteeID: p.InviteeID, SourceOrderID: p.SourceOrderID, TicketRole: AcquisitionTicketRoleInviter},
		AcquisitionLotteryTicket{ID: int64(len(r.tickets) + 2), CampaignID: p.CampaignID, UserID: p.InviteeID, InviteeID: p.InviteeID, SourceOrderID: p.SourceOrderID, TicketRole: AcquisitionTicketRoleInvitee},
	)
	return true, nil
}

func (r *acquisitionRepoStub) ClaimDueCampaign(ctx context.Context, now time.Time) (*AcquisitionCampaign, error) {
	if r.campaign.Status == AcquisitionCampaignStatusSettled {
		return nil, nil
	}
	r.campaign.Status = AcquisitionCampaignStatusSettling
	return r.campaign, nil
}

func (r *acquisitionRepoStub) ListLeaderboard(ctx context.Context, campaignID int64) ([]AcquisitionLeaderboardRow, error) {
	return r.leaderboard, nil
}

func (r *acquisitionRepoStub) ListLotteryTickets(ctx context.Context, campaignID int64) ([]AcquisitionLotteryTicket, error) {
	return r.tickets, nil
}

func (r *acquisitionRepoStub) CreateReward(ctx context.Context, reward AcquisitionReward) (bool, error) {
	for _, existing := range r.rewards {
		if existing.CampaignID == reward.CampaignID && existing.RewardType == reward.RewardType && existing.RewardKey == reward.RewardKey {
			return false, nil
		}
	}
	reward.ID = int64(len(r.rewards) + 1)
	reward.Status = AcquisitionRewardStatusPending
	r.rewards = append(r.rewards, reward)
	return true, nil
}

func (r *acquisitionRepoStub) ListPendingRewards(ctx context.Context, campaignID int64) ([]AcquisitionReward, error) {
	out := make([]AcquisitionReward, 0)
	for _, reward := range r.rewards {
		if reward.CampaignID == campaignID && reward.Status == AcquisitionRewardStatusPending {
			out = append(out, reward)
		}
	}
	return out, nil
}

func (r *acquisitionRepoStub) PayReward(ctx context.Context, reward AcquisitionReward) error {
	for i := range r.rewards {
		if r.rewards[i].ID == reward.ID && r.rewards[i].Status != AcquisitionRewardStatusPaid {
			r.rewards[i].Status = AcquisitionRewardStatusPaid
			r.totalPaid += r.rewards[i].Amount
		}
	}
	return nil
}

func (r *acquisitionRepoStub) MarkRewardFailed(ctx context.Context, rewardID int64, reason string) error {
	return nil
}

func (r *acquisitionRepoStub) MarkCampaignSettled(ctx context.Context, campaignID int64, settledAt time.Time) error {
	r.campaign.Status = AcquisitionCampaignStatusSettled
	return nil
}

func (r *acquisitionRepoStub) GetUserSummary(ctx context.Context, campaignID, userID int64) (*AcquisitionUserSummary, error) {
	summary := &AcquisitionUserSummary{
		Campaign:    r.campaign,
		AffCode:     r.affCode,
		Leaderboard: append([]AcquisitionLeaderboardRow(nil), r.leaderboard...),
		Rewards:     append([]AcquisitionReward(nil), r.rewards...),
	}
	for _, row := range r.leaderboard {
		if row.UserID == userID {
			summary.ValidInvites = row.InviteCount
			summary.Rank = row.Rank
			break
		}
	}
	return summary, nil
}

func (r *acquisitionRepoStub) ListRewards(ctx context.Context, campaignID int64, userID *int64) ([]AcquisitionReward, error) {
	return r.rewards, nil
}
