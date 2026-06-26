package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AcquisitionCampaignStatusDraft    = "draft"
	AcquisitionCampaignStatusActive   = "active"
	AcquisitionCampaignStatusSettling = "settling"
	AcquisitionCampaignStatusSettled  = "settled"

	AcquisitionRewardTypeLeaderboard = "leaderboard"
	AcquisitionRewardTypeLottery     = "lottery"

	AcquisitionRewardStatusPending = "pending"
	AcquisitionRewardStatusPaid    = "paid"
	AcquisitionRewardStatusFailed  = "failed"

	AcquisitionTicketRoleInviter = "inviter"
	AcquisitionTicketRoleInvitee = "invitee"

	defaultAcquisitionSettlementInterval = time.Minute
)

var (
	ErrAcquisitionDisabled = infraerrors.Forbidden("ACQUISITION_DISABLED", "acquisition campaign is disabled")
	ErrAcquisitionNotFound = infraerrors.NotFound("ACQUISITION_NOT_FOUND", "acquisition campaign not found")
)

type AcquisitionCampaign struct {
	ID                  int64                           `json:"id"`
	Name                string                          `json:"name"`
	Status              string                          `json:"status"`
	StartsAt            time.Time                       `json:"starts_at"`
	EndsAt              time.Time                       `json:"ends_at"`
	LeaderboardEnabled  bool                            `json:"leaderboard_enabled"`
	LotteryEnabled      bool                            `json:"lottery_enabled"`
	LeaderboardPoolUSD  float64                         `json:"leaderboard_pool_usd"`
	LeaderboardShares   []float64                       `json:"leaderboard_shares"`
	LotteryPrizeConfigs []AcquisitionLotteryPrizeConfig `json:"lottery_prize_configs"`
	LotterySeed         string                          `json:"lottery_seed"`
	SettledAt           *time.Time                      `json:"settled_at,omitempty"`
	CreatedAt           time.Time                       `json:"created_at"`
	UpdatedAt           time.Time                       `json:"updated_at"`
}

type AcquisitionCampaignInput struct {
	Name                string                          `json:"name"`
	Status              string                          `json:"status"`
	StartsAt            time.Time                       `json:"starts_at"`
	EndsAt              time.Time                       `json:"ends_at"`
	LeaderboardEnabled  bool                            `json:"leaderboard_enabled"`
	LotteryEnabled      bool                            `json:"lottery_enabled"`
	LeaderboardPoolUSD  float64                         `json:"leaderboard_pool_usd"`
	LeaderboardShares   []float64                       `json:"leaderboard_shares"`
	LotteryPrizeConfigs []AcquisitionLotteryPrizeConfig `json:"lottery_prize_configs"`
	LotterySeed         string                          `json:"lottery_seed"`
}

type AcquisitionCampaignFilter struct {
	Status string
	Limit  int
}

type AcquisitionLotteryPrizeConfig struct {
	Name       string  `json:"name"`
	AmountUSD  float64 `json:"amount_usd"`
	Count      int     `json:"count"`
	PerUserCap int     `json:"per_user_cap"`
}

type AcquisitionPaymentEvent struct {
	OrderID              int64
	UserID               int64
	OrderType            string
	SubscriptionPlanType string
	PayAmount            float64
	RefundAmount         float64
	CompletedAt          time.Time
}

type AcquisitionInviteBinding struct {
	InviterID int64
	BoundAt   time.Time
}

type AcquisitionParticipation struct {
	ID            int64     `json:"id"`
	CampaignID    int64     `json:"campaign_id"`
	InviterID     int64     `json:"inviter_id"`
	InviteeID     int64     `json:"invitee_id"`
	SourceOrderID int64     `json:"source_order_id"`
	CompletedAt   time.Time `json:"completed_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type AcquisitionLotteryTicket struct {
	ID            int64     `json:"id"`
	CampaignID    int64     `json:"campaign_id"`
	UserID        int64     `json:"user_id"`
	InviteeID     int64     `json:"invitee_id"`
	SourceOrderID int64     `json:"source_order_id"`
	TicketRole    string    `json:"ticket_role"`
	CreatedAt     time.Time `json:"created_at"`
}

type AcquisitionLeaderboardRow struct {
	UserID          int64      `json:"user_id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	InviteCount     int        `json:"invite_count"`
	LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`
	Rank            int        `json:"rank"`
	RewardAmount    float64    `json:"reward_amount"`
}

type AcquisitionReward struct {
	ID           int64      `json:"id"`
	CampaignID   int64      `json:"campaign_id"`
	UserID       int64      `json:"user_id"`
	RewardType   string     `json:"reward_type"`
	RewardKey    string     `json:"reward_key"`
	Amount       float64    `json:"amount"`
	Rank         int        `json:"rank,omitempty"`
	PrizeName    string     `json:"prize_name,omitempty"`
	TicketID     *int64     `json:"ticket_id,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type AcquisitionUserSummary struct {
	Campaign     *AcquisitionCampaign        `json:"campaign,omitempty"`
	AffCode      string                      `json:"aff_code,omitempty"`
	InviteLink   string                      `json:"invite_link,omitempty"`
	ValidInvites int                         `json:"valid_invites"`
	Rank         int                         `json:"rank"`
	TicketCount  int                         `json:"ticket_count"`
	Leaderboard  []AcquisitionLeaderboardRow `json:"leaderboard"`
	Rewards      []AcquisitionReward         `json:"rewards"`
}

type AcquisitionRepository interface {
	ListCampaigns(ctx context.Context, filter AcquisitionCampaignFilter) ([]AcquisitionCampaign, error)
	GetCampaign(ctx context.Context, id int64) (*AcquisitionCampaign, error)
	CreateCampaign(ctx context.Context, input AcquisitionCampaignInput) (*AcquisitionCampaign, error)
	UpdateCampaign(ctx context.Context, id int64, input AcquisitionCampaignInput) (*AcquisitionCampaign, error)
	ListActiveCampaignsForCompletion(ctx context.Context, completedAt time.Time) ([]AcquisitionCampaign, error)
	GetInviteBinding(ctx context.Context, inviteeUserID int64) (*AcquisitionInviteBinding, error)
	HasPriorEligiblePayment(ctx context.Context, userID, orderID int64, completedAt time.Time) (bool, error)
	CreateParticipationWithTickets(ctx context.Context, p AcquisitionParticipation) (bool, error)
	ClaimDueCampaign(ctx context.Context, now time.Time) (*AcquisitionCampaign, error)
	ListLeaderboard(ctx context.Context, campaignID int64) ([]AcquisitionLeaderboardRow, error)
	ListLotteryTickets(ctx context.Context, campaignID int64) ([]AcquisitionLotteryTicket, error)
	CreateReward(ctx context.Context, reward AcquisitionReward) (bool, error)
	ListPendingRewards(ctx context.Context, campaignID int64) ([]AcquisitionReward, error)
	PayReward(ctx context.Context, reward AcquisitionReward) error
	MarkRewardFailed(ctx context.Context, rewardID int64, reason string) error
	MarkCampaignSettled(ctx context.Context, campaignID int64, settledAt time.Time) error
	GetUserSummary(ctx context.Context, campaignID, userID int64) (*AcquisitionUserSummary, error)
	ListRewards(ctx context.Context, campaignID int64, userID *int64) ([]AcquisitionReward, error)
}

type AcquisitionSettings interface {
	IsAcquisitionEnabled(ctx context.Context) bool
	IsAcquisitionLeaderboardEnabled(ctx context.Context) bool
	IsAcquisitionLotteryEnabled(ctx context.Context) bool
}

type AcquisitionService struct {
	repo                 AcquisitionRepository
	settings             AcquisitionSettings
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
	interval             time.Duration
	stopCh               chan struct{}
	stopOnce             sync.Once
}

func NewAcquisitionService(repo AcquisitionRepository, settings AcquisitionSettings) *AcquisitionService {
	return &AcquisitionService{
		repo:     repo,
		settings: settings,
		interval: defaultAcquisitionSettlementInterval,
		stopCh:   make(chan struct{}),
	}
}

func (s *AcquisitionService) SetCacheInvalidators(auth APIKeyAuthCacheInvalidator, billing *BillingCacheService) {
	s.authCacheInvalidator = auth
	s.billingCacheService = billing
}

func (s *AcquisitionService) Start() {
	if s == nil || s.repo == nil || s.interval <= 0 {
		return
	}
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = s.SettleDueCampaigns(context.Background(), time.Now().UTC())
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *AcquisitionService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *AcquisitionService) EnsureEnabled(ctx context.Context) error {
	if s != nil && s.settings != nil && !s.settings.IsAcquisitionEnabled(ctx) {
		return ErrAcquisitionDisabled
	}
	return nil
}

func (s *AcquisitionService) ListCampaigns(ctx context.Context, filter AcquisitionCampaignFilter) ([]AcquisitionCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "acquisition service unavailable")
	}
	return s.repo.ListCampaigns(ctx, filter)
}

func (s *AcquisitionService) GetCampaign(ctx context.Context, id int64) (*AcquisitionCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "acquisition service unavailable")
	}
	return s.repo.GetCampaign(ctx, id)
}

func (s *AcquisitionService) CreateCampaign(ctx context.Context, input AcquisitionCampaignInput) (*AcquisitionCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "acquisition service unavailable")
	}
	normalized, err := normalizeAcquisitionCampaignInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateCampaign(ctx, normalized)
}

func (s *AcquisitionService) UpdateCampaign(ctx context.Context, id int64, input AcquisitionCampaignInput) (*AcquisitionCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "acquisition service unavailable")
	}
	normalized, err := normalizeAcquisitionCampaignInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateCampaign(ctx, id, normalized)
}

func (s *AcquisitionService) RecordPaymentCompletion(ctx context.Context, event *AcquisitionPaymentEvent) error {
	if s == nil || s.repo == nil || event == nil {
		return nil
	}
	if s.settings != nil && !s.settings.IsAcquisitionEnabled(ctx) {
		return nil
	}
	if !event.IsEligible() {
		return nil
	}
	hasPrior, err := s.repo.HasPriorEligiblePayment(ctx, event.UserID, event.OrderID, event.CompletedAt)
	if err != nil || hasPrior {
		return err
	}
	binding, err := s.repo.GetInviteBinding(ctx, event.UserID)
	if err != nil || binding == nil || binding.InviterID <= 0 || binding.InviterID == event.UserID {
		return err
	}
	campaigns, err := s.repo.ListActiveCampaignsForCompletion(ctx, event.CompletedAt)
	if err != nil {
		return err
	}
	for _, campaign := range campaigns {
		if !campaign.Contains(binding.BoundAt) || !campaign.Contains(event.CompletedAt) {
			continue
		}
		_, err = s.repo.CreateParticipationWithTickets(ctx, AcquisitionParticipation{
			CampaignID:    campaign.ID,
			InviterID:     binding.InviterID,
			InviteeID:     event.UserID,
			SourceOrderID: event.OrderID,
			CompletedAt:   event.CompletedAt,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *AcquisitionPaymentEvent) IsEligible() bool {
	if e == nil || e.OrderID <= 0 || e.UserID <= 0 || e.PayAmount <= 0 || e.RefundAmount > 0 {
		return false
	}
	if strings.EqualFold(e.OrderType, "subscription") && strings.EqualFold(e.SubscriptionPlanType, "quota_reset") {
		return false
	}
	return strings.EqualFold(e.OrderType, "balance") || strings.EqualFold(e.OrderType, "subscription")
}

func (c AcquisitionCampaign) Contains(ts time.Time) bool {
	return !ts.Before(c.StartsAt) && ts.Before(c.EndsAt)
}

func (s *AcquisitionService) SettleDueCampaigns(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil {
		return nil
	}
	for {
		campaign, err := s.repo.ClaimDueCampaign(ctx, now)
		if err != nil || campaign == nil {
			return err
		}
		if err := s.settleCampaign(ctx, campaign, now); err != nil {
			return err
		}
	}
}

func (s *AcquisitionService) settleCampaign(ctx context.Context, campaign *AcquisitionCampaign, now time.Time) error {
	if campaign == nil {
		return nil
	}
	if campaign.LeaderboardEnabled && (s.settings == nil || s.settings.IsAcquisitionLeaderboardEnabled(ctx)) {
		if err := s.createLeaderboardRewards(ctx, campaign); err != nil {
			return err
		}
	}
	if campaign.LotteryEnabled && (s.settings == nil || s.settings.IsAcquisitionLotteryEnabled(ctx)) {
		if err := s.createLotteryRewards(ctx, campaign); err != nil {
			return err
		}
	}
	if err := s.payPendingRewards(ctx, campaign.ID); err != nil {
		return err
	}
	return s.repo.MarkCampaignSettled(ctx, campaign.ID, now)
}

func (s *AcquisitionService) createLeaderboardRewards(ctx context.Context, campaign *AcquisitionCampaign) error {
	rows, err := s.repo.ListLeaderboard(ctx, campaign.ID)
	if err != nil {
		return err
	}
	shares := normalizeLeaderboardShares(campaign.LeaderboardShares)
	limit := len(shares)
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		amount := roundTo(campaign.LeaderboardPoolUSD*shares[i]/100, 8)
		if amount <= 0 {
			continue
		}
		_, err = s.repo.CreateReward(ctx, AcquisitionReward{
			CampaignID: campaign.ID,
			UserID:     rows[i].UserID,
			RewardType: AcquisitionRewardTypeLeaderboard,
			RewardKey:  fmt.Sprintf("rank:%d", i+1),
			Amount:     amount,
			Rank:       i + 1,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *AcquisitionService) createLotteryRewards(ctx context.Context, campaign *AcquisitionCampaign) error {
	tickets, err := s.repo.ListLotteryTickets(ctx, campaign.ID)
	if err != nil {
		return err
	}
	if len(tickets) == 0 {
		return nil
	}
	prizes := normalizeLotteryPrizes(campaign.LotteryPrizeConfigs)
	sort.SliceStable(prizes, func(i, j int) bool {
		if prizes[i].AmountUSD == prizes[j].AmountUSD {
			return prizes[i].Name < prizes[j].Name
		}
		return prizes[i].AmountUSD > prizes[j].AmountUSD
	})

	available := append([]AcquisitionLotteryTicket(nil), tickets...)
	winsByPrizeUser := map[string]map[int64]int{}
	for prizeIdx, prize := range prizes {
		if prize.Count <= 0 || prize.AmountUSD <= 0 {
			continue
		}
		if _, ok := winsByPrizeUser[prize.Name]; !ok {
			winsByPrizeUser[prize.Name] = map[int64]int{}
		}
		for slot := 0; slot < prize.Count && len(available) > 0; slot++ {
			winnerIdx := deterministicEligibleTicketIndex(campaign, prizeIdx, slot, available, func(ticket AcquisitionLotteryTicket) bool {
				return prize.PerUserCap <= 0 || winsByPrizeUser[prize.Name][ticket.UserID] < prize.PerUserCap
			})
			if winnerIdx < 0 {
				break
			}
			winner := available[winnerIdx]
			ticketID := winner.ID
			_, err = s.repo.CreateReward(ctx, AcquisitionReward{
				CampaignID: campaign.ID,
				UserID:     winner.UserID,
				RewardType: AcquisitionRewardTypeLottery,
				RewardKey:  fmt.Sprintf("ticket:%d:prize:%s:%d", winner.ID, prize.Name, slot+1),
				Amount:     roundTo(prize.AmountUSD, 8),
				PrizeName:  prize.Name,
				TicketID:   &ticketID,
			})
			if err != nil {
				return err
			}
			winsByPrizeUser[prize.Name][winner.UserID]++
			available = append(available[:winnerIdx], available[winnerIdx+1:]...)
		}
	}
	return nil
}

func deterministicTicketIndex(campaign *AcquisitionCampaign, prizeIdx, slot int, tickets []AcquisitionLotteryTicket) int {
	return deterministicTicketStartIndex(campaign, prizeIdx, slot, len(tickets))
}

func deterministicEligibleTicketIndex(campaign *AcquisitionCampaign, prizeIdx, slot int, tickets []AcquisitionLotteryTicket, eligible func(AcquisitionLotteryTicket) bool) int {
	if len(tickets) == 0 {
		return -1
	}
	start := deterministicTicketStartIndex(campaign, prizeIdx, slot, len(tickets))
	for offset := 0; offset < len(tickets); offset++ {
		idx := (start + offset) % len(tickets)
		if eligible == nil || eligible(tickets[idx]) {
			return idx
		}
	}
	return -1
}

func deterministicTicketStartIndex(campaign *AcquisitionCampaign, prizeIdx, slot, ticketCount int) int {
	seed := strings.TrimSpace(campaign.LotterySeed)
	if seed == "" {
		seed = fmt.Sprintf("campaign-%d", campaign.ID)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%d", seed, campaign.ID, prizeIdx, slot)))
	r := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(sum[:8]))))
	return r.Intn(ticketCount)
}

func (s *AcquisitionService) payPendingRewards(ctx context.Context, campaignID int64) error {
	rewards, err := s.repo.ListPendingRewards(ctx, campaignID)
	if err != nil {
		return err
	}
	for _, reward := range rewards {
		if reward.Amount <= 0 {
			continue
		}
		if err := s.repo.PayReward(ctx, reward); err != nil {
			_ = s.repo.MarkRewardFailed(ctx, reward.ID, err.Error())
			continue
		}
		s.invalidateUserBalance(ctx, reward.UserID)
	}
	return nil
}

func (s *AcquisitionService) invalidateUserBalance(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
		}()
	}
}

func (s *AcquisitionService) GetCurrentUserSummary(ctx context.Context, userID int64) (*AcquisitionUserSummary, error) {
	if err := s.EnsureEnabled(ctx); err != nil {
		return nil, err
	}
	campaigns, err := s.repo.ListActiveCampaignsForCompletion(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if len(campaigns) == 0 {
		return &AcquisitionUserSummary{}, nil
	}
	summary, err := s.repo.GetUserSummary(ctx, campaigns[0].ID, userID)
	if err != nil {
		return nil, err
	}
	applyLeaderboardRewardAmounts(summary)
	return summary, nil
}

func (s *AcquisitionService) GetCampaignDetail(ctx context.Context, campaignID int64, userID *int64) (*AcquisitionUserSummary, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.GetUserSummary(ctx, campaign.ID, derefInt64(userID))
	if err != nil {
		return nil, err
	}
	summary.Campaign = campaign
	applyLeaderboardRewardAmounts(summary)
	return summary, nil
}

func (s *AcquisitionService) SettleCampaignNow(ctx context.Context, campaignID int64, now time.Time) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "acquisition service unavailable")
	}
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}
	if campaign.Status == AcquisitionCampaignStatusSettled {
		if err := s.payPendingRewards(ctx, campaign.ID); err != nil {
			return err
		}
		return nil
	}
	return s.settleCampaign(ctx, campaign, now)
}

func normalizeAcquisitionCampaignInput(input AcquisitionCampaignInput) (AcquisitionCampaignInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "拉新活动"
	}
	if input.Status == "" {
		input.Status = AcquisitionCampaignStatusDraft
	}
	switch input.Status {
	case AcquisitionCampaignStatusDraft, AcquisitionCampaignStatusActive:
	default:
		return input, infraerrors.BadRequest("ACQUISITION_STATUS_INVALID", "campaign status must be draft or active")
	}
	if input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) {
		return input, infraerrors.BadRequest("ACQUISITION_TIME_INVALID", "campaign end time must be after start time")
	}
	if input.LeaderboardPoolUSD < 0 || math.IsNaN(input.LeaderboardPoolUSD) || math.IsInf(input.LeaderboardPoolUSD, 0) {
		return input, infraerrors.BadRequest("ACQUISITION_POOL_INVALID", "leaderboard pool is invalid")
	}
	input.LeaderboardShares = normalizeLeaderboardShares(input.LeaderboardShares)
	input.LotteryPrizeConfigs = normalizeLotteryPrizes(input.LotteryPrizeConfigs)
	if strings.TrimSpace(input.LotterySeed) == "" {
		input.LotterySeed = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return input, nil
}

func normalizeLeaderboardShares(in []float64) []float64 {
	if len(in) == 0 {
		return []float64{40, 25, 15, 12, 8}
	}
	out := make([]float64, 0, 5)
	for _, v := range in {
		if len(out) >= 5 {
			break
		}
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			v = 0
		}
		out = append(out, v)
	}
	for len(out) < 5 {
		out = append(out, 0)
	}
	return out
}

func normalizeLotteryPrizes(in []AcquisitionLotteryPrizeConfig) []AcquisitionLotteryPrizeConfig {
	out := make([]AcquisitionLotteryPrizeConfig, 0, len(in))
	for _, p := range in {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = "奖品"
		}
		if p.Count <= 0 || p.AmountUSD <= 0 || math.IsNaN(p.AmountUSD) || math.IsInf(p.AmountUSD, 0) {
			continue
		}
		if p.PerUserCap < 0 {
			p.PerUserCap = 0
		}
		p.Name = name
		out = append(out, p)
	}
	return out
}

func applyLeaderboardRewardAmounts(summary *AcquisitionUserSummary) {
	if summary == nil || summary.Campaign == nil || !summary.Campaign.LeaderboardEnabled {
		return
	}
	shares := normalizeLeaderboardShares(summary.Campaign.LeaderboardShares)
	for i := range summary.Leaderboard {
		rank := summary.Leaderboard[i].Rank
		if rank <= 0 {
			rank = i + 1
		}
		if rank > len(shares) {
			summary.Leaderboard[i].RewardAmount = 0
			continue
		}
		summary.Leaderboard[i].RewardAmount = roundTo(summary.Campaign.LeaderboardPoolUSD*shares[rank-1]/100, 8)
	}
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
