package service

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

const (
	accountHealthDefaultProbePageSize = 100
	accountProbeDefaultInterval       = 6 * time.Hour
	accountProbeMaxBatchAccounts      = 100
	accountProbeListPointLimit        = 10
)

var accountProbeBearerTokenPattern = regexp.MustCompile(`(?i)\bBearer\s+[^\s,;]+`)

const (
	AccountHealthEventSourceBackgroundProbe = "background_probe"
	AccountHealthEventSourceManualProbe     = "manual_probe"
	AccountHealthEventTypeSuccess           = "success"
	AccountHealthEventTypeFailure           = "failure"
)

type AccountHealthState struct {
	AccountID   int64      `json:"account_id"`
	NextProbeAt *time.Time `json:"next_probe_at,omitempty"`
}

type AccountHealthEvent struct {
	ID            int64     `json:"id"`
	AccountID     int64     `json:"account_id"`
	Source        string    `json:"source"`
	EventType     string    `json:"event_type"`
	ErrorCategory string    `json:"error_category,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	LatencyMs     *int64    `json:"latency_ms,omitempty"`
	ActorUserID   *int64    `json:"actor_user_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type AccountHealthEventList struct {
	Items      []AccountHealthEvent `json:"items"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	TotalPages int                  `json:"total_pages"`
}

type AccountProbePoint struct {
	Timestamp     time.Time `json:"timestamp"`
	LatencyMs     *int64    `json:"latency_ms,omitempty"`
	SuccessCount  int       `json:"success_count"`
	FailureCount  int       `json:"failure_count"`
	ErrorCategory string    `json:"error_category,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

type AccountProbeTrend struct {
	AccountID         int64               `json:"account_id"`
	Range             string              `json:"range"`
	From              time.Time           `json:"from"`
	To                time.Time           `json:"to"`
	Points            []AccountProbePoint `json:"points"`
	Total             int                 `json:"total"`
	SuccessCount      int                 `json:"success_count"`
	FailureCount      int                 `json:"failure_count"`
	SuccessRate       *float64            `json:"success_rate,omitempty"`
	P50LatencyMs      *int64              `json:"p50_latency_ms,omitempty"`
	P95LatencyMs      *int64              `json:"p95_latency_ms,omitempty"`
	LastResult        string              `json:"last_result,omitempty"`
	LastLatencyMs     *int64              `json:"last_latency_ms,omitempty"`
	LastProbedAt      *time.Time          `json:"last_probed_at,omitempty"`
	LastErrorCategory string              `json:"last_error_category,omitempty"`
	LastErrorMessage  string              `json:"last_error_message,omitempty"`
	NextProbeAt       *time.Time          `json:"next_probe_at,omitempty"`
}

type AccountProbeDetail struct {
	AccountProbeTrend
	CacheStats                  AccountProbeCacheStats `json:"cache_stats"`
	HealthProbeEnabled          bool                   `json:"health_probe_enabled"`
	HealthProbeIntervalMinutes  *int                   `json:"health_probe_interval_minutes,omitempty"`
	HealthProbeModel            *string                `json:"health_probe_model,omitempty"`
	HealthyProbeEnabled         bool                   `json:"healthy_probe_enabled"`
	HealthyProbeIntervalMinutes *int                   `json:"healthy_probe_interval_minutes,omitempty"`
	HealthyProbeIntervalHours   *int                   `json:"healthy_probe_interval_hours,omitempty"`
}

type AccountProbeCacheStats struct {
	Window              string   `json:"window"`
	RequestCount        int64    `json:"request_count"`
	InputTokens         int64    `json:"input_tokens"`
	CacheCreationTokens int64    `json:"cache_creation_tokens"`
	CacheReadTokens     int64    `json:"cache_read_tokens"`
	CacheRate           *float64 `json:"cache_rate,omitempty"`
}

type AccountProbeCacheUsage struct {
	RequestCount        int64
	InputTokens         int64
	CacheCreationTokens int64
	CacheReadTokens     int64
}

type AccountHealthRepository interface {
	ClaimDueProbe(ctx context.Context, accountID int64, now time.Time, leaseUntil time.Time) (bool, error)
	ScheduleNextProbe(ctx context.Context, accountID int64, nextProbeAt *time.Time, now time.Time) error
	GetNextProbeAt(ctx context.Context, accountID int64) (*time.Time, error)
	InsertEvent(ctx context.Context, event *AccountHealthEvent) error
	ListProbeEvents(ctx context.Context, accountIDs []int64, since time.Time) ([]AccountHealthEvent, error)
	ListEvents(ctx context.Context, accountID int64, eventType string, since time.Time, params pagination.PaginationParams) (*AccountHealthEventList, error)
	GetRecentCacheUsage(ctx context.Context, accountID int64, from, to time.Time) (*AccountProbeCacheUsage, error)
	DeleteEventsBefore(ctx context.Context, before time.Time) (int64, error)
}

type accountProbeAccountRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type accountProbeCandidateRepository interface {
	ListHealthyProbeCandidates(ctx context.Context, now time.Time, limit int) ([]Account, error)
}

type AccountHealthService struct {
	repo        AccountHealthRepository
	accountRepo accountProbeAccountRepository
	now         func() time.Time
}

func NewAccountHealthService(repo AccountHealthRepository, accountRepo AccountRepository) *AccountHealthService {
	return &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}
}

func (s *AccountHealthService) HealthProbeModel(ctx context.Context, accountID int64) string {
	account, err := s.getAccount(ctx, accountID)
	if err != nil || account.HealthProbeModel == nil {
		return ""
	}
	return strings.TrimSpace(*account.HealthProbeModel)
}

func (s *AccountHealthService) RecordProbeSuccess(ctx context.Context, accountID int64, latencyMs int64) error {
	return s.recordProbeResult(ctx, accountID, AccountHealthEventSourceBackgroundProbe, true, latencyMs, "", "", nil, true)
}

func (s *AccountHealthService) RecordManualProbeSuccess(ctx context.Context, accountID int64, latencyMs int64, actorUserID *int64) error {
	return s.recordProbeResult(ctx, accountID, AccountHealthEventSourceManualProbe, true, latencyMs, "", "", actorUserID, false)
}

func (s *AccountHealthService) RecordProbeFailure(ctx context.Context, accountID int64, category, message string) error {
	return s.recordProbeResult(ctx, accountID, AccountHealthEventSourceBackgroundProbe, false, 0, category, message, nil, true)
}

func (s *AccountHealthService) RecordManualProbeFailure(ctx context.Context, accountID int64, category, message string, actorUserID *int64) error {
	return s.recordProbeResult(ctx, accountID, AccountHealthEventSourceManualProbe, false, 0, category, message, actorUserID, false)
}

func (s *AccountHealthService) recordProbeResult(ctx context.Context, accountID int64, source string, success bool, latencyMs int64, category, message string, actorUserID *int64, reschedule bool) error {
	if s == nil || s.repo == nil || accountID <= 0 {
		return nil
	}
	now := s.nowTime()
	eventType := AccountHealthEventTypeFailure
	var latency *int64
	if success {
		eventType = AccountHealthEventTypeSuccess
		if latencyMs > 0 {
			value := latencyMs
			latency = &value
		}
	}
	eventErr := s.repo.InsertEvent(ctx, &AccountHealthEvent{
		AccountID:     accountID,
		Source:        source,
		EventType:     eventType,
		ErrorCategory: truncateAccountProbeString(category, 40),
		ErrorMessage:  truncateAccountProbeString(redactAccountProbeMessage(message), 2000),
		LatencyMs:     latency,
		ActorUserID:   actorUserID,
		CreatedAt:     now,
	})
	if !reschedule {
		return eventErr
	}
	account, accountErr := s.getAccount(ctx, accountID)
	if accountErr != nil {
		if eventErr != nil {
			return fmt.Errorf("record probe event: %v; load account for reschedule: %w", eventErr, accountErr)
		}
		return accountErr
	}
	next := nextScheduledAccountProbe(account, now)
	scheduleErr := s.repo.ScheduleNextProbe(ctx, accountID, next, now)
	if eventErr != nil {
		return eventErr
	}
	return scheduleErr
}

func (s *AccountHealthService) ListDueForProbe(ctx context.Context, now time.Time, limit int) ([]*AccountHealthState, error) {
	if s == nil || s.accountRepo == nil {
		return nil, nil
	}
	if limit <= 0 || limit > accountHealthDefaultProbePageSize {
		limit = accountHealthDefaultProbePageSize
	}
	candidateRepo, ok := s.accountRepo.(accountProbeCandidateRepository)
	if !ok {
		return nil, fmt.Errorf("account probe candidate repository unavailable")
	}
	accounts, err := candidateRepo.ListHealthyProbeCandidates(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	states := make([]*AccountHealthState, 0, len(accounts))
	for i := range accounts {
		states = append(states, &AccountHealthState{AccountID: accounts[i].ID})
	}
	return states, nil
}

func (s *AccountHealthService) ClaimDueProbe(ctx context.Context, accountID int64, now, leaseUntil time.Time) (bool, error) {
	if s == nil || s.repo == nil || accountID <= 0 || !leaseUntil.After(now) {
		return false, nil
	}
	return s.repo.ClaimDueProbe(ctx, accountID, now, leaseUntil)
}

func (s *AccountHealthService) RescheduleProbe(ctx context.Context, accountID int64) error {
	if s == nil || s.repo == nil {
		return nil
	}
	now := s.nowTime()
	account, err := s.getAccount(ctx, accountID)
	if err != nil {
		return err
	}
	return s.repo.ScheduleNextProbe(ctx, accountID, nextScheduledAccountProbe(account, now), now)
}

func (s *AccountHealthService) GetProbeTrends(ctx context.Context, accountIDs []int64) ([]AccountProbeTrend, error) {
	ids := normalizeAccountProbeIDs(accountIDs)
	if len(ids) == 0 {
		return []AccountProbeTrend{}, nil
	}
	if len(ids) > accountProbeMaxBatchAccounts {
		return nil, fmt.Errorf("too many account ids: maximum is %d", accountProbeMaxBatchAccounts)
	}
	now := s.nowTime()
	trends, err := s.buildProbeTrends(ctx, ids, "24h", now.Add(-24*time.Hour), now, 0)
	if err != nil {
		return nil, err
	}
	for i := range trends {
		if len(trends[i].Points) > accountProbeListPointLimit {
			trends[i].Points = trends[i].Points[len(trends[i].Points)-accountProbeListPointLimit:]
		}
	}
	return trends, nil
}

func (s *AccountHealthService) GetProbeDetail(ctx context.Context, accountID int64) (*AccountProbeDetail, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("invalid account id")
	}
	account, err := s.getAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	now := s.nowTime()
	trends, err := s.buildProbeTrends(ctx, []int64{accountID}, "24h", now.Add(-24*time.Hour), now, 0)
	if err != nil {
		return nil, err
	}
	cacheUsage, err := s.repo.GetRecentCacheUsage(ctx, accountID, now.Add(-time.Hour), now)
	if err != nil {
		return nil, err
	}
	cacheStats := AccountProbeCacheStats{Window: "1h"}
	if cacheUsage != nil {
		cacheStats.RequestCount = cacheUsage.RequestCount
		cacheStats.InputTokens = cacheUsage.InputTokens
		cacheStats.CacheCreationTokens = cacheUsage.CacheCreationTokens
		cacheStats.CacheReadTokens = cacheUsage.CacheReadTokens
		denominator := cacheUsage.InputTokens + cacheUsage.CacheCreationTokens + cacheUsage.CacheReadTokens
		if denominator > 0 {
			rate := float64(cacheUsage.CacheReadTokens) / float64(denominator) * 100
			cacheStats.CacheRate = &rate
		}
	}
	detail := &AccountProbeDetail{
		AccountProbeTrend:           trends[0],
		CacheStats:                  cacheStats,
		HealthProbeEnabled:          account.HealthProbeEnabled,
		HealthProbeIntervalMinutes:  account.HealthProbeIntervalMinutes,
		HealthProbeModel:            account.HealthProbeModel,
		HealthyProbeEnabled:         account.HealthyProbeEnabled,
		HealthyProbeIntervalMinutes: account.HealthyProbeIntervalMinutes,
		HealthyProbeIntervalHours:   account.HealthyProbeIntervalHours,
	}
	return detail, nil
}

func (s *AccountHealthService) buildProbeTrends(ctx context.Context, accountIDs []int64, rangeName string, from, to time.Time, bucket time.Duration) ([]AccountProbeTrend, error) {
	events, err := s.repo.ListProbeEvents(ctx, accountIDs, from)
	if err != nil {
		return nil, err
	}
	byAccount := make(map[int64][]AccountHealthEvent, len(accountIDs))
	for _, event := range events {
		if event.CreatedAt.After(to) {
			continue
		}
		byAccount[event.AccountID] = append(byAccount[event.AccountID], event)
	}
	result := make([]AccountProbeTrend, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		trend := buildAccountProbeTrend(accountID, rangeName, from, to, byAccount[accountID], bucket)
		if next, nextErr := s.repo.GetNextProbeAt(ctx, accountID); nextErr == nil {
			trend.NextProbeAt = next
		}
		result = append(result, trend)
	}
	return result, nil
}

func (s *AccountHealthService) ListEvents(ctx context.Context, accountID int64, eventType string, params pagination.PaginationParams) (*AccountHealthEventList, error) {
	if s == nil || s.repo == nil {
		return &AccountHealthEventList{Items: []AccountHealthEvent{}}, nil
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	eventType = strings.TrimSpace(eventType)
	if eventType != "" && eventType != AccountHealthEventTypeSuccess && eventType != AccountHealthEventTypeFailure {
		return nil, fmt.Errorf("invalid probe event type")
	}
	return s.repo.ListEvents(ctx, accountID, eventType, s.nowTime().Add(-24*time.Hour), params)
}

func (s *AccountHealthService) CleanupEvents(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	return s.repo.DeleteEventsBefore(ctx, before)
}

func (s *AccountHealthService) getAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("account probe service unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("account not found")
	}
	return account, nil
}

func (s *AccountHealthService) nowTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func buildAccountProbeTrend(accountID int64, rangeName string, from, to time.Time, events []AccountHealthEvent, bucket time.Duration) AccountProbeTrend {
	trend := AccountProbeTrend{AccountID: accountID, Range: rangeName, From: from, To: to, Points: []AccountProbePoint{}}
	latencies := make([]int64, 0, len(events))
	for i := range events {
		event := events[i]
		trend.Total++
		if event.EventType == AccountHealthEventTypeSuccess {
			trend.SuccessCount++
			if event.LatencyMs != nil {
				latencies = append(latencies, *event.LatencyMs)
			}
		} else {
			trend.FailureCount++
			trend.LastErrorCategory = event.ErrorCategory
			trend.LastErrorMessage = event.ErrorMessage
		}
		trend.LastResult = event.EventType
		trend.LastLatencyMs = event.LatencyMs
		createdAt := event.CreatedAt
		trend.LastProbedAt = &createdAt
	}
	if trend.Total > 0 {
		rate := float64(trend.SuccessCount) * 100 / float64(trend.Total)
		trend.SuccessRate = &rate
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		trend.P50LatencyMs = percentileLatency(latencies, 0.50)
		trend.P95LatencyMs = percentileLatency(latencies, 0.95)
	}
	trend.Points = aggregateAccountProbePoints(events, bucket)
	return trend
}

func aggregateAccountProbePoints(events []AccountHealthEvent, bucket time.Duration) []AccountProbePoint {
	if bucket <= 0 {
		points := make([]AccountProbePoint, 0, len(events))
		for _, event := range events {
			point := AccountProbePoint{Timestamp: event.CreatedAt, ErrorCategory: event.ErrorCategory, ErrorMessage: event.ErrorMessage}
			if event.EventType == AccountHealthEventTypeSuccess {
				point.SuccessCount = 1
				point.LatencyMs = event.LatencyMs
			} else {
				point.FailureCount = 1
			}
			points = append(points, point)
		}
		return points
	}
	type bucketValue struct {
		point     AccountProbePoint
		latencies []int64
	}
	values := make(map[time.Time]*bucketValue)
	order := make([]time.Time, 0)
	for _, event := range events {
		key := event.CreatedAt.UTC().Truncate(bucket)
		value := values[key]
		if value == nil {
			value = &bucketValue{point: AccountProbePoint{Timestamp: key}}
			values[key] = value
			order = append(order, key)
		}
		if event.EventType == AccountHealthEventTypeSuccess {
			value.point.SuccessCount++
			if event.LatencyMs != nil {
				value.latencies = append(value.latencies, *event.LatencyMs)
			}
		} else {
			value.point.FailureCount++
			value.point.ErrorCategory = event.ErrorCategory
			value.point.ErrorMessage = event.ErrorMessage
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Before(order[j]) })
	points := make([]AccountProbePoint, 0, len(order))
	for _, key := range order {
		value := values[key]
		if len(value.latencies) > 0 {
			sort.Slice(value.latencies, func(i, j int) bool { return value.latencies[i] < value.latencies[j] })
			value.point.LatencyMs = percentileLatency(value.latencies, 0.50)
		}
		points = append(points, value.point)
	}
	return points
}

func percentileLatency(sorted []int64, percentile float64) *int64 {
	if len(sorted) == 0 {
		return nil
	}
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	value := sorted[index]
	return &value
}

func normalizeAccountProbeIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func nextScheduledAccountProbe(account *Account, now time.Time) *time.Time {
	if account == nil || account.Status != StatusActive || !account.Schedulable || !account.HealthProbeEnabled || (account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(now)) {
		return nil
	}
	next := now.Add(accountProbeInterval(account))
	return &next
}

func accountProbeInterval(account *Account) time.Duration {
	if account != nil {
		if account.HealthProbeIntervalMinutes != nil && *account.HealthProbeIntervalMinutes > 0 {
			return time.Duration(*account.HealthProbeIntervalMinutes) * time.Minute
		}
		// Keep the previous healthy-probe cadence as a migration-free fallback.
		if account.HealthyProbeIntervalMinutes != nil && *account.HealthyProbeIntervalMinutes > 0 {
			return time.Duration(*account.HealthyProbeIntervalMinutes) * time.Minute
		}
		if account.HealthyProbeIntervalHours != nil && *account.HealthyProbeIntervalHours > 0 {
			return time.Duration(*account.HealthyProbeIntervalHours) * time.Hour
		}
	}
	return accountProbeDefaultInterval
}

func accountHealthProbeFailureCategory(message string) string {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "invalid_api_key"), strings.Contains(normalized, "invalid api key"), strings.Contains(normalized, "unauthorized"), strings.Contains(normalized, "authentication failed"), strings.Contains(normalized, "returned 401"), strings.Contains(normalized, "returned 403"):
		return "auth_error"
	case strings.Contains(normalized, "quota exceeded"), strings.Contains(normalized, "insufficient balance"), strings.Contains(normalized, "insufficient quota"), strings.Contains(normalized, "余额不足"):
		return "quota_exceeded"
	case strings.Contains(normalized, "model_not_found"), strings.Contains(normalized, "model not found"), strings.Contains(normalized, "model does not exist"), strings.Contains(normalized, "model unavailable"), strings.Contains(normalized, "unknown model"), strings.Contains(normalized, "unsupported model"):
		return "model_not_found"
	default:
		return "probe_failed"
	}
}

func AccountHealthProbeFailureCategory(message string) string {
	return accountHealthProbeFailureCategory(message)
}

func truncateAccountProbeString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	for max > 0 && !utf8.RuneStart(value[max]) {
		max--
	}
	return value[:max]
}

func redactAccountProbeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = accountProbeBearerTokenPattern.ReplaceAllString(message, "Bearer ***")
	return logredact.RedactText(message, "api_key", "key", "authorization", "cookie", "token", "session_key")
}
