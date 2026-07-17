package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const (
	UsageRebatePeriodStatusOpen     = "open"
	UsageRebatePeriodStatusSettling = "settling"
	UsageRebatePeriodStatusSettled  = "settled"
	UsageRebatePeriodStatusFailed   = "failed"
	UsageRebatePeriodStatusUnknown  = "unknown"

	UsageRebateRewardStatusPending  = "pending"
	UsageRebateRewardStatusCredited = "credited"
	UsageRebateRewardStatusFailed   = "failed"
	UsageRebateRewardStatusUnknown  = "unknown"

	usageRebateRuleVersion = "v1"
	usageRebateLimit       = 20
	usageRebateSettleHour  = 0
	usageRebateSettleMin   = 15
)

var ErrUsageRebateCommitUnknown = errors.New("usage rebate transaction commit result is unknown")

type UsageRebateRate struct {
	Rank    int             `json:"rank"`
	Percent decimal.Decimal `json:"percent"`
}

type UsageRebatePeriod struct {
	ID              int64             `json:"id"`
	BusinessDate    string            `json:"business_date"`
	WindowStart     time.Time         `json:"window_start"`
	WindowEnd       time.Time         `json:"window_end"`
	SettleAfter     time.Time         `json:"settle_after"`
	Timezone        string            `json:"timezone"`
	RuleVersion     string            `json:"rule_version"`
	Rates           []UsageRebateRate `json:"rates"`
	Status          string            `json:"status"`
	TotalSpend      decimal.Decimal   `json:"total_spend"`
	TotalReward     decimal.Decimal   `json:"total_reward"`
	AttemptCount    int               `json:"attempt_count"`
	ErrorMessage    string            `json:"error_message,omitempty"`
	SettlementToken string            `json:"-"`
	LockedUntil     *time.Time        `json:"locked_until,omitempty"`
	SettledAt       *time.Time        `json:"settled_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type UsageRebatePeriodSeed struct {
	BusinessDate string
	WindowStart  time.Time
	WindowEnd    time.Time
	SettleAfter  time.Time
	Timezone     string
	RuleVersion  string
}

type UsageRebateReward struct {
	ID            int64           `json:"id"`
	PeriodID      int64           `json:"period_id"`
	BusinessDate  string          `json:"business_date"`
	UserID        int64           `json:"user_id"`
	Username      string          `json:"username,omitempty"`
	Rank          int             `json:"rank"`
	SpendAmount   decimal.Decimal `json:"spend_amount"`
	RebatePercent decimal.Decimal `json:"rebate_percent"`
	RewardAmount  decimal.Decimal `json:"reward_amount"`
	Status        string          `json:"status"`
	BusinessKey   string          `json:"-"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	BalanceBefore decimal.Decimal `json:"balance_before"`
	BalanceAfter  decimal.Decimal `json:"balance_after"`
	CreditedAt    *time.Time      `json:"credited_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type UsageRebateCandidate struct {
	UserID          int64           `json:"user_id"`
	Username        string          `json:"username"`
	Rank            int             `json:"rank"`
	Requests        int64           `json:"requests"`
	Tokens          int64           `json:"tokens"`
	SpendAmount     decimal.Decimal `json:"spend_amount"`
	RebatePercent   decimal.Decimal `json:"rebate_percent"`
	EstimatedReward decimal.Decimal `json:"estimated_reward"`
}

type UsageRebatePosition struct {
	Rank             *int
	ParticipantCount int
	Requests         int64
	Tokens           int64
	SpendAmount      decimal.Decimal
	RebatePercent    decimal.Decimal
	EstimatedReward  decimal.Decimal
	Eligible         bool
	PreviousRank     *int
	GapToPrevious    *decimal.Decimal
	GapToTop20       *decimal.Decimal
}

type UsageRebateRepository interface {
	EnsureOpenPeriod(ctx context.Context, seed UsageRebatePeriodSeed) error
	ClaimDuePeriod(ctx context.Context, now, lockUntil time.Time) (*UsageRebatePeriod, error)
	SealClaimedPeriod(ctx context.Context, periodID int64, rates []UsageRebateRate) error
	ListPayableRewards(ctx context.Context, periodID int64) ([]UsageRebateReward, error)
	CreditReward(ctx context.Context, rewardID int64) (userID int64, credited bool, err error)
	MarkRewardFailed(ctx context.Context, rewardID int64, reason string) error
	MarkRewardUnknown(ctx context.Context, rewardID int64, reason string) error
	FinalizePeriod(ctx context.Context, periodID int64) error
	MarkPeriodFailed(ctx context.Context, periodID int64, reason string) error
	GetLeaderboard(ctx context.Context, start, end time.Time, viewerUserID int64, limit int) ([]UsageRebateCandidate, error)
	GetUserPosition(ctx context.Context, start, end time.Time, userID int64) (UsageRebatePosition, error)
	ListUserRewards(ctx context.Context, userID int64, limit int) ([]UsageRebateReward, error)
	ListRecentPeriods(ctx context.Context, limit int) ([]UsageRebatePeriod, error)
	ListPeriodRewards(ctx context.Context, periodID int64, limit int) ([]UsageRebateReward, error)
}

type UsageRebateSettings interface {
	IsUsageRebateEnabled(ctx context.Context) bool
}

type UsageRebateAuthCacheInvalidator interface {
	InvalidateAuthCacheByUserID(ctx context.Context, userID int64)
}

type UsageRebateService struct {
	repo                 UsageRebateRepository
	settings             UsageRebateSettings
	authCacheInvalidator UsageRebateAuthCacheInvalidator
	billingCacheService  *BillingCacheService
	interval             time.Duration
	lockDuration         time.Duration
	stopCh               chan struct{}
	stopOnce             sync.Once
}

func NewUsageRebateService(repo UsageRebateRepository, settings UsageRebateSettings) *UsageRebateService {
	return &UsageRebateService{
		repo:         repo,
		settings:     settings,
		interval:     time.Minute,
		lockDuration: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}
}

func UsageRebateRates() []UsageRebateRate {
	values := []string{
		"10", "9", "8", "7", "6.5", "6", "5.5", "5", "4.5", "4.5",
		"4", "4", "4", "3.5", "3.5", "3.5", "3", "3", "3", "2.5",
	}
	rates := make([]UsageRebateRate, 0, len(values))
	for i, value := range values {
		rates = append(rates, UsageRebateRate{Rank: i + 1, Percent: decimal.RequireFromString(value)})
	}
	return rates
}

func CalculateUsageRebate(spend decimal.Decimal, rank int) (decimal.Decimal, bool) {
	if spend.IsNegative() || spend.IsZero() || rank < 1 || rank > usageRebateLimit {
		return decimal.Zero, false
	}
	rate := UsageRebateRates()[rank-1].Percent
	return spend.Mul(rate).Div(decimal.NewFromInt(100)).Round(8), true
}

func (s *UsageRebateService) SetAuthCacheInvalidator(invalidator UsageRebateAuthCacheInvalidator) {
	if s != nil {
		s.authCacheInvalidator = invalidator
	}
}

func (s *UsageRebateService) SetCacheInvalidators(invalidator UsageRebateAuthCacheInvalidator, billing *BillingCacheService) {
	if s != nil {
		s.authCacheInvalidator = invalidator
		s.billingCacheService = billing
	}
}

func (s *UsageRebateService) IsEnabled(ctx context.Context) bool {
	return s != nil && (s.settings == nil || s.settings.IsUsageRebateEnabled(ctx))
}

func (s *UsageRebateService) Start() {
	if s == nil || s.repo == nil || s.interval <= 0 {
		return
	}
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				_ = s.RunOnce(context.Background(), now)
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *UsageRebateService) Stop() {
	if s != nil {
		s.stopOnce.Do(func() { close(s.stopCh) })
	}
}

func (s *UsageRebateService) RunOnce(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil {
		return nil
	}
	localNow := now.In(usageRebateLocation())
	if s.settings == nil || s.settings.IsUsageRebateEnabled(ctx) {
		if err := s.repo.EnsureOpenPeriod(ctx, newUsageRebatePeriodSeed(localNow)); err != nil {
			return err
		}
	}

	period, err := s.repo.ClaimDuePeriod(ctx, localNow, localNow.Add(s.lockDuration))
	if err != nil || period == nil {
		return err
	}
	if err := s.settleClaimedPeriod(ctx, period); err != nil {
		_ = s.repo.MarkPeriodFailed(ctx, period.ID, truncateUsageRebateError(err))
		return err
	}
	return nil
}

func (s *UsageRebateService) settleClaimedPeriod(ctx context.Context, period *UsageRebatePeriod) error {
	if err := s.repo.SealClaimedPeriod(ctx, period.ID, UsageRebateRates()); err != nil {
		return err
	}
	rewards, err := s.repo.ListPayableRewards(ctx, period.ID)
	if err != nil {
		return err
	}
	for _, reward := range rewards {
		userID, credited, creditErr := s.repo.CreditReward(ctx, reward.ID)
		if creditErr != nil {
			reason := truncateUsageRebateError(creditErr)
			if errors.Is(creditErr, ErrUsageRebateCommitUnknown) {
				s.invalidateUserBalance(ctx, reward.UserID)
				if err := s.repo.MarkRewardUnknown(ctx, reward.ID, reason); err != nil {
					return err
				}
				continue
			}
			if err := s.repo.MarkRewardFailed(ctx, reward.ID, reason); err != nil {
				return err
			}
			continue
		}
		if credited && userID > 0 {
			s.invalidateUserBalance(ctx, userID)
		}
	}
	return s.repo.FinalizePeriod(ctx, period.ID)
}

func (s *UsageRebateService) invalidateUserBalance(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
	}
}

func (s *UsageRebateService) GetLeaderboard(ctx context.Context, now time.Time, viewerUserID int64) ([]UsageRebateCandidate, error) {
	start, end := usageRebateDayWindow(now)
	return s.repo.GetLeaderboard(ctx, start, end, viewerUserID, usageRebateLimit)
}

func (s *UsageRebateService) GetUserPosition(ctx context.Context, now time.Time, userID int64) (UsageRebatePosition, error) {
	start, end := usageRebateDayWindow(now)
	return s.repo.GetUserPosition(ctx, start, end, userID)
}

func (s *UsageRebateService) ListMyRewards(ctx context.Context, userID int64, limit int) ([]UsageRebateReward, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return s.repo.ListUserRewards(ctx, userID, limit)
}

func (s *UsageRebateService) ListRecentPeriods(ctx context.Context, limit int) ([]UsageRebatePeriod, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return s.repo.ListRecentPeriods(ctx, limit)
}

func (s *UsageRebateService) ListPeriodRewards(ctx context.Context, periodID int64, limit int) ([]UsageRebateReward, error) {
	if periodID <= 0 {
		return nil, fmt.Errorf("invalid usage rebate period id")
	}
	if limit <= 0 || limit > 100 {
		limit = usageRebateLimit
	}
	return s.repo.ListPeriodRewards(ctx, periodID, limit)
}

func newUsageRebatePeriodSeed(now time.Time) UsageRebatePeriodSeed {
	start, end := usageRebateDayWindow(now)
	return UsageRebatePeriodSeed{
		BusinessDate: start.Format("2006-01-02"),
		WindowStart:  start,
		WindowEnd:    end,
		SettleAfter:  time.Date(end.Year(), end.Month(), end.Day(), usageRebateSettleHour, usageRebateSettleMin, 0, 0, end.Location()),
		Timezone:     "Asia/Shanghai",
		RuleVersion:  usageRebateRuleVersion,
	}
}

func usageRebateDayWindow(now time.Time) (time.Time, time.Time) {
	local := now.In(usageRebateLocation())
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	return start, start.AddDate(0, 0, 1)
}

func usageRebateLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return location
}

func truncateUsageRebateError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
