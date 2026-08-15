package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

const (
	dailyCheckinSettingEnabled   = "daily_checkin_enabled"
	dailyCheckinSettingBase      = "daily_checkin_base_reward"
	dailyCheckinSettingCycleDays = "daily_checkin_cycle_days"
	dailyCheckinSetting7         = "daily_checkin_milestone_7"
	dailyCheckinSetting15        = "daily_checkin_milestone_15"
	dailyCheckinSetting30        = "daily_checkin_milestone_30"
	dailyCheckinSettingMinAge    = "daily_checkin_min_account_age_hours"
	dailyCheckinSettingVerified  = "daily_checkin_require_verified"
	dailyCheckinSettingBudget    = "daily_checkin_daily_budget"
	dailyCheckinSettingVersion   = "daily_checkin_rule_version"
)

type DailyCheckinConfig struct {
	Enabled            bool    `json:"enabled"`
	BaseReward         float64 `json:"base_reward"`
	CycleDays          int     `json:"cycle_days"`
	Milestone7         float64 `json:"milestone_7"`
	Milestone15        float64 `json:"milestone_15"`
	Milestone30        float64 `json:"milestone_30"`
	MinAccountAgeHours int     `json:"min_account_age_hours"`
	RequireVerified    bool    `json:"require_verified"`
	DailyBudget        float64 `json:"daily_budget"`
	RuleVersion        int     `json:"rule_version"`
}

type DailyCheckinCycle struct {
	ID                int64      `json:"id"`
	CycleNumber       int        `json:"cycle_number"`
	Status            string     `json:"status"`
	CycleDays         int        `json:"cycle_days"`
	CheckinCount      int        `json:"checkin_count"`
	ConsecutiveDays   int        `json:"consecutive_days"`
	StartedOn         time.Time  `json:"started_on"`
	LastCheckinOn     *time.Time `json:"last_checkin_on,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	BaseReward        float64    `json:"base_reward"`
	Milestone7Reward  float64    `json:"milestone_7_reward"`
	Milestone15Reward float64    `json:"milestone_15_reward"`
	Milestone30Reward float64    `json:"milestone_30_reward"`
	RuleVersion       int        `json:"rule_version"`
	TotalReward       float64    `json:"total_reward"`
}

type DailyCheckinRecord struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id,omitempty"`
	UserEmail       string    `json:"user_email,omitempty"`
	BusinessDate    time.Time `json:"business_date"`
	CycleDay        int       `json:"cycle_day"`
	BaseReward      float64   `json:"base_reward"`
	MilestoneReward float64   `json:"milestone_reward"`
	TotalReward     float64   `json:"total_reward"`
	BalanceBefore   float64   `json:"balance_before"`
	BalanceAfter    float64   `json:"balance_after"`
	RuleVersion     int       `json:"rule_version"`
	CreatedAt       time.Time `json:"created_at"`
}

type DailyCheckinStatus struct {
	Config              DailyCheckinConfig `json:"config"`
	Today               string             `json:"today"`
	TodayClaimed        bool               `json:"today_claimed"`
	Eligible            bool               `json:"eligible"`
	IneligibleReason    string             `json:"ineligible_reason,omitempty"`
	CurrentCycle        *DailyCheckinCycle `json:"current_cycle,omitempty"`
	HistoryReward       float64            `json:"history_reward"`
	CompletedCycles     int                `json:"completed_cycles"`
	NextMilestone       *int               `json:"next_milestone,omitempty"`
	DaysToNextMilestone int                `json:"days_to_next_milestone"`
}

type DailyCheckinPage struct {
	Items    []DailyCheckinRecord `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type DailyCheckinStats struct {
	TodayClaims     int64   `json:"today_claims"`
	TodayReward     float64 `json:"today_reward"`
	MonthReward     float64 `json:"month_reward"`
	CompletedCycles int64   `json:"completed_cycles"`
}

type DailyCheckinService struct {
	db                  *sql.DB
	billingCacheService *BillingCacheService
}

func NewDailyCheckinService(db *sql.DB, billingCacheService *BillingCacheService) *DailyCheckinService {
	return &DailyCheckinService{db: db, billingCacheService: billingCacheService}
}

func defaultDailyCheckinConfig() DailyCheckinConfig {
	return DailyCheckinConfig{BaseReward: 0.13, CycleDays: 30, Milestone7: 2, Milestone15: 5, Milestone30: 8, RuleVersion: 1}
}

func (s *DailyCheckinService) GetConfig(ctx context.Context) (DailyCheckinConfig, error) {
	cfg := defaultDailyCheckinConfig()
	if s == nil || s.db == nil {
		return cfg, errors.New("daily check-in database unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key,value FROM settings WHERE key LIKE 'daily_checkin_%'`)
	if err != nil {
		return cfg, fmt.Errorf("load daily check-in settings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return cfg, err
		}
		value = strings.TrimSpace(value)
		switch key {
		case dailyCheckinSettingEnabled:
			cfg.Enabled, _ = strconv.ParseBool(value)
		case dailyCheckinSettingBase:
			cfg.BaseReward, _ = strconv.ParseFloat(value, 64)
		case dailyCheckinSettingCycleDays:
			cfg.CycleDays, _ = strconv.Atoi(value)
		case dailyCheckinSetting7:
			cfg.Milestone7, _ = strconv.ParseFloat(value, 64)
		case dailyCheckinSetting15:
			cfg.Milestone15, _ = strconv.ParseFloat(value, 64)
		case dailyCheckinSetting30:
			cfg.Milestone30, _ = strconv.ParseFloat(value, 64)
		case dailyCheckinSettingMinAge:
			cfg.MinAccountAgeHours, _ = strconv.Atoi(value)
		case dailyCheckinSettingVerified:
			cfg.RequireVerified, _ = strconv.ParseBool(value)
		case dailyCheckinSettingBudget:
			cfg.DailyBudget, _ = strconv.ParseFloat(value, 64)
		case dailyCheckinSettingVersion:
			cfg.RuleVersion, _ = strconv.Atoi(value)
		}
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}
	return cfg, validateDailyCheckinConfig(cfg)
}

func validateDailyCheckinConfig(cfg DailyCheckinConfig) error {
	amounts := []float64{cfg.BaseReward, cfg.Milestone7, cfg.Milestone15, cfg.Milestone30, cfg.DailyBudget}
	for _, amount := range amounts {
		if amount < 0 || amount > 1000000 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return infraerrors.BadRequest("DAILY_CHECKIN_CONFIG_INVALID", "daily check-in configuration is invalid")
		}
	}
	if cfg.CycleDays != 30 || cfg.MinAccountAgeHours < 0 || cfg.MinAccountAgeHours > 87600 || cfg.RuleVersion < 1 {
		return infraerrors.BadRequest("DAILY_CHECKIN_CONFIG_INVALID", "daily check-in configuration is invalid")
	}
	return nil
}

func (s *DailyCheckinService) UpdateConfig(ctx context.Context, cfg DailyCheckinConfig) (DailyCheckinConfig, error) {
	current, err := s.GetConfig(ctx)
	if err != nil {
		return DailyCheckinConfig{}, err
	}
	if cfg.BaseReward != current.BaseReward || cfg.CycleDays != current.CycleDays || cfg.Milestone7 != current.Milestone7 || cfg.Milestone15 != current.Milestone15 || cfg.Milestone30 != current.Milestone30 {
		cfg.RuleVersion = current.RuleVersion + 1
	} else {
		cfg.RuleVersion = current.RuleVersion
	}
	if err := validateDailyCheckinConfig(cfg); err != nil {
		return DailyCheckinConfig{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DailyCheckinConfig{}, err
	}
	defer func() { _ = tx.Rollback() }()
	values := map[string]string{
		dailyCheckinSettingEnabled: strconv.FormatBool(cfg.Enabled), dailyCheckinSettingBase: formatMoney(cfg.BaseReward), dailyCheckinSettingCycleDays: strconv.Itoa(cfg.CycleDays), dailyCheckinSetting7: formatMoney(cfg.Milestone7), dailyCheckinSetting15: formatMoney(cfg.Milestone15), dailyCheckinSetting30: formatMoney(cfg.Milestone30), dailyCheckinSettingMinAge: strconv.Itoa(cfg.MinAccountAgeHours), dailyCheckinSettingVerified: strconv.FormatBool(cfg.RequireVerified), dailyCheckinSettingBudget: formatMoney(cfg.DailyBudget), dailyCheckinSettingVersion: strconv.Itoa(cfg.RuleVersion),
	}
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, key, value); err != nil {
			return DailyCheckinConfig{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return DailyCheckinConfig{}, err
	}
	return cfg, nil
}

func (s *DailyCheckinService) GetStatus(ctx context.Context, userID int64) (DailyCheckinStatus, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return DailyCheckinStatus{}, err
	}
	if userID <= 0 {
		return DailyCheckinStatus{}, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	today := timezone.Today()
	status := DailyCheckinStatus{Config: cfg, Today: today.Format("2006-01-02")}
	status.TodayClaimed, err = s.hasClaimed(ctx, userID, today)
	if err != nil {
		return status, err
	}
	status.Eligible, status.IneligibleReason, err = s.getEligibility(ctx, userID, cfg)
	if err != nil {
		return status, err
	}
	status.CurrentCycle, err = s.loadCycle(ctx, userID, true)
	if err != nil {
		return status, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_reward),0) FROM daily_checkins WHERE user_id=$1`, userID).Scan(&status.HistoryReward); err != nil {
		return status, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_checkin_cycles WHERE user_id=$1 AND status='completed'`, userID).Scan(&status.CompletedCycles); err != nil {
		return status, err
	}
	if status.CurrentCycle != nil {
		status.NextMilestone, status.DaysToNextMilestone = nextMilestone(status.CurrentCycle.CheckinCount, status.CurrentCycle.CycleDays)
	} else {
		status.NextMilestone, status.DaysToNextMilestone = nextMilestone(0, cfg.CycleDays)
	}
	return status, nil
}

func (s *DailyCheckinService) Claim(ctx context.Context, userID int64, clientIP, userAgent string) (DailyCheckinRecord, error) {
	if userID <= 0 {
		return DailyCheckinRecord{}, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return DailyCheckinRecord{}, err
	}
	if !cfg.Enabled {
		return DailyCheckinRecord{}, infraerrors.Forbidden("DAILY_CHECKIN_DISABLED", "daily check-in is disabled")
	}
	today := timezone.Today()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DailyCheckinRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var accountStatus string
	var createdAt time.Time
	var balanceBefore float64
	if err := tx.QueryRowContext(ctx, `SELECT status,created_at,balance FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&accountStatus, &createdAt, &balanceBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DailyCheckinRecord{}, infraerrors.NotFound("USER_NOT_FOUND", "user not found")
		}
		return DailyCheckinRecord{}, err
	}
	var existing DailyCheckinRecord
	err = tx.QueryRowContext(ctx, `SELECT id,business_date,cycle_day,base_reward,milestone_reward,total_reward,balance_before,balance_after,rule_version,created_at FROM daily_checkins WHERE user_id=$1 AND business_date=$2`, userID, today).Scan(&existing.ID, &existing.BusinessDate, &existing.CycleDay, &existing.BaseReward, &existing.MilestoneReward, &existing.TotalReward, &existing.BalanceBefore, &existing.BalanceAfter, &existing.RuleVersion, &existing.CreatedAt)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return DailyCheckinRecord{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return DailyCheckinRecord{}, err
	}
	if accountStatus != "active" {
		return DailyCheckinRecord{}, infraerrors.Forbidden("DAILY_CHECKIN_NOT_ELIGIBLE", "account is not eligible for daily check-in")
	}
	if cfg.MinAccountAgeHours > 0 && timezone.Now().Sub(createdAt) < time.Duration(cfg.MinAccountAgeHours)*time.Hour {
		return DailyCheckinRecord{}, infraerrors.Forbidden("DAILY_CHECKIN_NOT_ELIGIBLE", "account is too new for daily check-in")
	}
	if cfg.RequireVerified {
		var verified bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM auth_identities WHERE user_id=$1 AND verified_at IS NOT NULL)`, userID).Scan(&verified); err != nil {
			return DailyCheckinRecord{}, err
		}
		if !verified {
			return DailyCheckinRecord{}, infraerrors.Forbidden("DAILY_CHECKIN_NOT_ELIGIBLE", "account verification is required")
		}
	}
	cycle, err := lockOrCreateCycle(ctx, tx, userID, today, cfg)
	if err != nil {
		return DailyCheckinRecord{}, err
	}
	cycleDay := cycle.CheckinCount + 1
	milestone := 0.0
	if cycleDay == 7 {
		milestone = cycle.Milestone7Reward
	} else if cycleDay == 15 {
		milestone = cycle.Milestone15Reward
	} else if cycleDay == 30 {
		milestone = cycle.Milestone30Reward
	}
	baseAmount := decimal.NewFromFloat(cycle.BaseReward).Round(8)
	milestoneAmount := decimal.NewFromFloat(milestone).Round(8)
	totalAmount := baseAmount.Add(milestoneAmount).Round(8)
	total, _ := totalAmount.Float64()
	if _, err := tx.ExecContext(ctx, `INSERT INTO daily_checkin_daily_totals(business_date,budget_limit) VALUES($1,$2::numeric) ON CONFLICT(business_date) DO NOTHING`, today, formatMoney(cfg.DailyBudget)); err != nil {
		return DailyCheckinRecord{}, err
	}
	var budgetRaw, spentRaw string
	if err := tx.QueryRowContext(ctx, `SELECT budget_limit::text,total_reward::text FROM daily_checkin_daily_totals WHERE business_date=$1 FOR UPDATE`, today).Scan(&budgetRaw, &spentRaw); err != nil {
		return DailyCheckinRecord{}, err
	}
	budgetAmount, err := decimal.NewFromString(budgetRaw)
	if err != nil {
		return DailyCheckinRecord{}, fmt.Errorf("parse daily check-in budget: %w", err)
	}
	spentAmount, err := decimal.NewFromString(spentRaw)
	if err != nil {
		return DailyCheckinRecord{}, fmt.Errorf("parse daily check-in total: %w", err)
	}
	if budgetAmount.IsPositive() && spentAmount.Add(totalAmount).GreaterThan(budgetAmount) {
		return DailyCheckinRecord{}, infraerrors.Conflict("DAILY_CHECKIN_BUDGET_EXHAUSTED", "today's check-in budget has been exhausted")
	}
	var balanceAfter float64
	if err := tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1 RETURNING balance`, userID, formatMoney(total)).Scan(&balanceAfter); err != nil {
		return DailyCheckinRecord{}, err
	}
	record := DailyCheckinRecord{BusinessDate: today, CycleDay: cycleDay, BaseReward: cycle.BaseReward, MilestoneReward: milestone, TotalReward: total, BalanceBefore: balanceBefore, BalanceAfter: balanceAfter, RuleVersion: cycle.RuleVersion}
	key := fmt.Sprintf("checkin:%d:%s", userID, today.Format("2006-01-02"))
	if err := tx.QueryRowContext(ctx, `INSERT INTO daily_checkins(user_id,cycle_id,business_date,cycle_day,base_reward,milestone_reward,total_reward,balance_before,balance_after,rule_version,business_key,client_ip,user_agent_hash) VALUES($1,$2,$3,$4,$5::numeric,$6::numeric,$7::numeric,$8::numeric,$9::numeric,$10,$11,$12,$13) RETURNING id,created_at`, userID, cycle.ID, today, cycleDay, formatMoney(record.BaseReward), formatMoney(milestone), formatMoney(total), formatMoney(balanceBefore), formatMoney(balanceAfter), cycle.RuleVersion, key, parseIP(clientIP), hashUserAgent(userAgent)).Scan(&record.ID, &record.CreatedAt); err != nil {
		return DailyCheckinRecord{}, err
	}
	newConsecutive := 1
	if cycle.LastCheckinOn != nil && cycle.LastCheckinOn.AddDate(0, 0, 1).Equal(today) {
		newConsecutive = cycle.ConsecutiveDays + 1
	}
	newCount := cycleDay
	completed := newCount >= cycle.CycleDays
	if _, err := tx.ExecContext(ctx, `UPDATE daily_checkin_cycles SET checkin_count=$2,consecutive_days=$3,last_checkin_on=$4,total_reward=total_reward+$5::numeric,status=CASE WHEN $6 THEN 'completed' ELSE status END,completed_at=CASE WHEN $6 THEN NOW() ELSE completed_at END,updated_at=NOW() WHERE id=$1`, cycle.ID, newCount, newConsecutive, today, totalAmount.StringFixed(8), completed); err != nil {
		return DailyCheckinRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE daily_checkin_daily_totals SET claim_count=claim_count+1,total_reward=total_reward+$2::numeric,updated_at=NOW() WHERE business_date=$1`, today, formatMoney(total)); err != nil {
		return DailyCheckinRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return DailyCheckinRecord{}, err
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(context.Background(), userID)
	}
	return record, nil
}

func (s *DailyCheckinService) History(ctx context.Context, userID int64, page, pageSize int) (DailyCheckinPage, error) {
	page, pageSize = normalizePage(page, pageSize)
	var result DailyCheckinPage
	result.Page, result.PageSize = page, pageSize
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_checkins WHERE user_id=$1`, userID).Scan(&result.Total); err != nil {
		return result, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.QueryContext(ctx, `SELECT id,business_date,cycle_day,base_reward,milestone_reward,total_reward,balance_before,balance_after,rule_version,created_at FROM daily_checkins WHERE user_id=$1 ORDER BY business_date DESC LIMIT $2 OFFSET $3`, userID, pageSize, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item DailyCheckinRecord
		if err := rows.Scan(&item.ID, &item.BusinessDate, &item.CycleDay, &item.BaseReward, &item.MilestoneReward, &item.TotalReward, &item.BalanceBefore, &item.BalanceAfter, &item.RuleVersion, &item.CreatedAt); err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

func (s *DailyCheckinService) Stats(ctx context.Context) (DailyCheckinStats, error) {
	var out DailyCheckinStats
	today := timezone.Today()
	month := timezone.StartOfMonth(today)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(total_reward),0) FROM daily_checkins WHERE business_date=$1`, today).Scan(&out.TodayClaims, &out.TodayReward); err != nil {
		return out, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_reward),0) FROM daily_checkins WHERE business_date >= $1`, month).Scan(&out.MonthReward); err != nil {
		return out, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM daily_checkin_cycles WHERE status='completed' AND completed_at >= $1`, month).Scan(&out.CompletedCycles); err != nil {
		return out, err
	}
	return out, nil
}

func (s *DailyCheckinService) AdminRecords(ctx context.Context, page, pageSize int, date string) (DailyCheckinPage, error) {
	page, pageSize = normalizePage(page, pageSize)
	var result DailyCheckinPage
	result.Page, result.PageSize = page, pageSize
	where := ""
	args := []any{}
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return result, infraerrors.BadRequest("DAILY_CHECKIN_DATE_INVALID", "invalid date")
		}
		where = " WHERE business_date=$1"
		args = append(args, date)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM daily_checkins"+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := "SELECT c.id,c.user_id,u.email,c.business_date,c.cycle_day,c.base_reward,c.milestone_reward,c.total_reward,c.balance_before,c.balance_after,c.rule_version,c.created_at FROM daily_checkins c JOIN users u ON u.id=c.user_id"
	if date != "" {
		query += " WHERE c.business_date=$1"
	}
	query += " ORDER BY c.created_at DESC LIMIT $" + strconv.Itoa(len(args)-1) + " OFFSET $" + strconv.Itoa(len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item DailyCheckinRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.BusinessDate, &item.CycleDay, &item.BaseReward, &item.MilestoneReward, &item.TotalReward, &item.BalanceBefore, &item.BalanceAfter, &item.RuleVersion, &item.CreatedAt); err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

func (s *DailyCheckinService) hasClaimed(ctx context.Context, userID int64, today time.Time) (bool, error) {
	var found bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM daily_checkins WHERE user_id=$1 AND business_date=$2)`, userID, today).Scan(&found)
	return found, err
}

func (s *DailyCheckinService) getEligibility(ctx context.Context, userID int64, cfg DailyCheckinConfig) (bool, string, error) {
	if !cfg.Enabled {
		return false, "disabled", nil
	}
	var status string
	var createdAt time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT status,created_at FROM users WHERE id=$1`, userID).Scan(&status, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", infraerrors.NotFound("USER_NOT_FOUND", "user not found")
		}
		return false, "", err
	}
	if status != "active" {
		return false, "inactive", nil
	}
	if cfg.MinAccountAgeHours > 0 && timezone.Now().Sub(createdAt) < time.Duration(cfg.MinAccountAgeHours)*time.Hour {
		return false, "account_too_new", nil
	}
	if cfg.RequireVerified {
		var verified bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM auth_identities WHERE user_id=$1 AND verified_at IS NOT NULL)`, userID).Scan(&verified); err != nil {
			return false, "", err
		}
		if !verified {
			return false, "verification_required", nil
		}
	}
	return true, "", nil
}
func (s *DailyCheckinService) loadCycle(ctx context.Context, userID int64, active bool) (*DailyCheckinCycle, error) {
	where := ""
	if active {
		where = " AND status='active'"
	}
	var c DailyCheckinCycle
	var last, completed *time.Time
	err := s.db.QueryRowContext(ctx, "SELECT id,cycle_number,status,cycle_days,checkin_count,consecutive_days,started_on,last_checkin_on,completed_at,base_reward,milestone_7_reward,milestone_15_reward,milestone_30_reward,rule_version,total_reward FROM daily_checkin_cycles WHERE user_id=$1"+where+" ORDER BY cycle_number DESC LIMIT 1", userID).Scan(&c.ID, &c.CycleNumber, &c.Status, &c.CycleDays, &c.CheckinCount, &c.ConsecutiveDays, &c.StartedOn, &last, &completed, &c.BaseReward, &c.Milestone7Reward, &c.Milestone15Reward, &c.Milestone30Reward, &c.RuleVersion, &c.TotalReward)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	c.LastCheckinOn = last
	c.CompletedAt = completed
	return &c, err
}

func lockOrCreateCycle(ctx context.Context, tx *sql.Tx, userID int64, today time.Time, cfg DailyCheckinConfig) (*DailyCheckinCycle, error) {
	var c DailyCheckinCycle
	var last, completed *time.Time
	err := tx.QueryRowContext(ctx, `SELECT id,cycle_number,status,cycle_days,checkin_count,consecutive_days,started_on,last_checkin_on,completed_at,base_reward,milestone_7_reward,milestone_15_reward,milestone_30_reward,rule_version,total_reward FROM daily_checkin_cycles WHERE user_id=$1 AND status='active' ORDER BY id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&c.ID, &c.CycleNumber, &c.Status, &c.CycleDays, &c.CheckinCount, &c.ConsecutiveDays, &c.StartedOn, &last, &completed, &c.BaseReward, &c.Milestone7Reward, &c.Milestone15Reward, &c.Milestone30Reward, &c.RuleVersion, &c.TotalReward)
	if errors.Is(err, sql.ErrNoRows) {
		var number int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(cycle_number),0)+1 FROM daily_checkin_cycles WHERE user_id=$1`, userID).Scan(&number); err != nil {
			return nil, err
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO daily_checkin_cycles(user_id,cycle_number,cycle_days,started_on,base_reward,milestone_7_reward,milestone_15_reward,milestone_30_reward,rule_version) VALUES($1,$2,$3,$4,$5::numeric,$6::numeric,$7::numeric,$8::numeric,$9) RETURNING id,status,started_on`, userID, number, cfg.CycleDays, today, formatMoney(cfg.BaseReward), formatMoney(cfg.Milestone7), formatMoney(cfg.Milestone15), formatMoney(cfg.Milestone30), cfg.RuleVersion).Scan(&c.ID, &c.Status, &c.StartedOn)
		if err != nil {
			return nil, err
		}
		c.CycleNumber = number
		c.CycleDays = cfg.CycleDays
		c.BaseReward = cfg.BaseReward
		c.Milestone7Reward = cfg.Milestone7
		c.Milestone15Reward = cfg.Milestone15
		c.Milestone30Reward = cfg.Milestone30
		c.RuleVersion = cfg.RuleVersion
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	c.LastCheckinOn = last
	c.CompletedAt = completed
	return &c, nil
}

func nextMilestone(count, cycleDays int) (*int, int) {
	for _, n := range []int{7, 15, 30} {
		if n <= cycleDays && count < n {
			return &n, n - count
		}
	}
	return nil, 0
}
func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func formatMoney(v float64) string { return decimal.NewFromFloat(v).Round(8).StringFixed(8) }
func parseIP(raw string) any {
	if ip := net.ParseIP(strings.TrimSpace(raw)); ip != nil {
		return ip.String()
	}
	return nil
}
func hashUserAgent(raw string) string {
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
