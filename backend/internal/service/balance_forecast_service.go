package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	balanceForecastInterval            = 30 * time.Minute
	balanceForecastRunTimeout          = 2 * time.Minute
	balanceForecastWindow              = 7 * 24 * time.Hour
	balanceForecastHorizon             = 72 * time.Hour
	balanceForecastMinDailyBurn        = 0.01
	balanceForecastLeaderLockKey       = "balance:forecast:leader"
	balanceForecastLeaderLockTTL       = 5 * time.Minute
	balanceForecastSMTPWarningInterval = time.Minute
	balanceForecastSourceType          = "balance_forecast"
	balanceForecastReminderKey         = "open"
)

// BalanceForecastCandidate 是一个有近期余额消耗、且开启了提醒的用户。
type BalanceForecastCandidate struct {
	UserID      int64
	Email       string
	Username    string
	Balance     float64
	WindowCost  float64
	ExtraEmails string
}

// BalanceForecastEpisode 是某个用户当前未关闭的一轮预测告警。
type BalanceForecastEpisode struct {
	ID         int64
	NotifiedAt *time.Time
}

// BalanceForecastEpisodeInput 是开一轮预测告警时记录的快照。
type BalanceForecastEpisodeInput struct {
	UserID         int64
	Balance        float64
	DailyBurn      float64
	HoursRemaining float64
	OpenedAt       time.Time
}

// BalanceForecastRepository 负责预测所需的查询和轮次状态。
type BalanceForecastRepository interface {
	// CloseRecoveredEpisodes 关闭已经充值过（余额高于开轮时）或不再需要提醒的用户的轮次。
	CloseRecoveredEpisodes(ctx context.Context, now time.Time) (int64, error)
	// ListCandidates 返回 since 之后余额消耗合计不低于 minWindowCost 的候选用户。
	ListCandidates(ctx context.Context, since time.Time, minWindowCost float64) ([]BalanceForecastCandidate, error)
	// OpenEpisode 幂等开轮：同一用户已有未关闭的轮次时返回现有轮次。
	OpenEpisode(ctx context.Context, input BalanceForecastEpisodeInput) (*BalanceForecastEpisode, error)
	MarkEpisodeNotified(ctx context.Context, id int64, at time.Time) error
}

type balanceForecast struct {
	DailyBurn      float64
	HoursRemaining float64
}

// computeBalanceForecast 按窗口内的平均消耗估算余额还能撑多久。
// 返回 ok=false 表示不需要提醒：余额已耗尽、消耗太小，或预计耗尽时间超出提醒范围。
func computeBalanceForecast(balance, windowCost float64) (balanceForecast, bool) {
	if balance <= 0 || windowCost <= 0 {
		return balanceForecast{}, false
	}
	dailyBurn := windowCost / (balanceForecastWindow.Hours() / 24)
	if dailyBurn < balanceForecastMinDailyBurn {
		return balanceForecast{}, false
	}
	hours := balance / dailyBurn * 24
	if hours > balanceForecastHorizon.Hours() {
		return balanceForecast{}, false
	}
	return balanceForecast{DailyBurn: dailyBurn, HoursRemaining: hours}, true
}

// BalanceForecastService 定期预测余额耗尽时间，并向用户已验证的通知邮箱发提醒。
type BalanceForecastService struct {
	repo                     BalanceForecastRepository
	settingRepo              SettingRepository
	notificationEmailService *NotificationEmailService
	interval                 time.Duration
	stopCh                   chan struct{}
	stopOnce                 sync.Once
	wg                       sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
	now        func() time.Time

	smtpWarningMu   sync.Mutex
	lastSMTPWarning time.Time
}

func NewBalanceForecastService(repo BalanceForecastRepository, interval time.Duration) *BalanceForecastService {
	return &BalanceForecastService{
		repo:       repo,
		interval:   interval,
		stopCh:     make(chan struct{}),
		instanceID: uuid.NewString(),
		now:        time.Now,
	}
}

func (s *BalanceForecastService) SetSettingRepository(settingRepo SettingRepository) {
	s.settingRepo = settingRepo
}

func (s *BalanceForecastService) SetNotificationEmailService(notificationEmailService *NotificationEmailService) {
	s.notificationEmailService = notificationEmailService
}

// SetLeaderLock 注入选主锁，多实例部署时只有一个实例做扫描和发信。
func (s *BalanceForecastService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *BalanceForecastService) Start() {
	if s == nil || s.repo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *BalanceForecastService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *BalanceForecastService) runOnce() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BalanceForecast] panic in run: %v", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), balanceForecastRunTimeout)
	defer cancel()
	s.run(ctx)
}

func (s *BalanceForecastService) run(ctx context.Context) {
	if s == nil || s.repo == nil || s.notificationEmailService == nil {
		return
	}
	if !s.enabled(ctx) {
		return
	}
	if !s.smtpConfigured(ctx) {
		return
	}
	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, balanceForecastLeaderLockKey, s.instanceID, balanceForecastLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	now := s.now()
	closed, err := s.repo.CloseRecoveredEpisodes(ctx, now)
	if err != nil {
		log.Printf("[BalanceForecast] Close recovered episodes failed: %v", err)
		return
	}
	if closed > 0 {
		log.Printf("[BalanceForecast] Closed %d recovered episodes", closed)
	}

	minWindowCost := balanceForecastMinDailyBurn * balanceForecastWindow.Hours() / 24
	candidates, err := s.repo.ListCandidates(ctx, now.Add(-balanceForecastWindow), minWindowCost)
	if err != nil {
		log.Printf("[BalanceForecast] List candidates failed: %v", err)
		return
	}
	rechargeURL := s.rechargeURL(ctx)
	for i := range candidates {
		if ctx.Err() != nil {
			return
		}
		s.forecastOne(ctx, &candidates[i], rechargeURL, now)
	}
}

func (s *BalanceForecastService) forecastOne(ctx context.Context, c *BalanceForecastCandidate, rechargeURL string, now time.Time) {
	forecast, ok := computeBalanceForecast(c.Balance, c.WindowCost)
	if !ok {
		return
	}
	recipients := filterVerifiedEmails(ParseNotifyEmails(c.ExtraEmails))
	if len(recipients) == 0 {
		return
	}
	episode, err := s.repo.OpenEpisode(ctx, BalanceForecastEpisodeInput{
		UserID:         c.UserID,
		Balance:        c.Balance,
		DailyBurn:      forecast.DailyBurn,
		HoursRemaining: forecast.HoursRemaining,
		OpenedAt:       now,
	})
	if err != nil {
		log.Printf("[BalanceForecast] Open episode failed: user=%d err=%v", c.UserID, err)
		return
	}
	if episode == nil || episode.NotifiedAt != nil {
		return
	}

	name := firstNonEmpty(c.Username, c.Email)
	variables := map[string]string{
		"current_balance": fmt.Sprintf("%.2f", c.Balance),
		"daily_burn":      fmt.Sprintf("%.2f", forecast.DailyBurn),
		"hours_remaining": fmt.Sprintf("%.0f", forecast.HoursRemaining),
		"days_remaining":  fmt.Sprintf("%.1f", forecast.HoursRemaining/24),
		"recharge_url":    rechargeURL,
	}
	allSent := true
	for _, to := range recipients {
		if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventBalanceForecast,
			RecipientEmail: to,
			RecipientName:  name,
			UserID:         c.UserID,
			SourceType:     balanceForecastSourceType,
			SourceID:       strconv.FormatInt(episode.ID, 10),
			ReminderKey:    balanceForecastReminderKey,
			Variables:      variables,
		}); err != nil {
			allSent = false
			log.Printf("[BalanceForecast] Send forecast email failed: user=%d episode=%d err=%v", c.UserID, episode.ID, err)
		}
	}
	if !allSent {
		// 保持未通知状态，下个周期重试；已发出的收件人由投递键去重，不会重复收到。
		return
	}
	if err := s.repo.MarkEpisodeNotified(ctx, episode.ID, now); err != nil {
		log.Printf("[BalanceForecast] Mark episode notified failed: episode=%d err=%v", episode.ID, err)
	}
}

// enabled 读取总开关。默认关闭：设置缺失或读取失败都按关闭处理。
func (s *BalanceForecastService) enabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyBalanceForecastNotifyEnabled)
	if err != nil {
		if !errors.Is(err, ErrSettingNotFound) {
			log.Printf("[BalanceForecast] Read forecast switch failed: %v", err)
		}
		return false
	}
	return value == "true"
}

func (s *BalanceForecastService) rechargeURL(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return ""
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyBalanceLowNotifyRechargeURL)
	if err != nil {
		return ""
	}
	return value
}

func (s *BalanceForecastService) smtpConfigured(ctx context.Context) bool {
	if s == nil || s.notificationEmailService == nil || s.notificationEmailService.emailService == nil {
		return false
	}
	_, err := s.notificationEmailService.emailService.GetSMTPConfig(ctx)
	if err == nil {
		return true
	}
	if errors.Is(err, ErrEmailNotConfigured) {
		s.smtpWarningMu.Lock()
		defer s.smtpWarningMu.Unlock()
		now := time.Now()
		if s.lastSMTPWarning.IsZero() || now.Sub(s.lastSMTPWarning) >= balanceForecastSMTPWarningInterval {
			log.Printf("[BalanceForecast] SMTP is not configured; skipping balance forecast")
			s.lastSMTPWarning = now
		}
		return false
	}
	log.Printf("[BalanceForecast] Read SMTP configuration failed; skipping balance forecast: %v", err)
	return false
}
