package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

const (
	AccountHealthStatusHealthy    = "healthy"
	AccountHealthStatusDegraded   = "degraded"
	AccountHealthStatusIsolated   = "isolated"
	AccountHealthStatusRecovering = "recovering"

	defaultAccountHealthScore         = 80
	accountHealthFailurePenalty       = 25
	accountHealthSuccessReward        = 5
	accountHealthProbeRecoveryReward  = 25
	accountHealthIsolationScore       = 40
	accountHealthIsolationFailures    = 3
	accountHealthRecoverySuccesses    = 2
	accountHealthRecoveryScore        = 70
	accountHealthProbeRecoveryScore   = 50
	accountHealthRecoveredScore       = 70
	accountHealthIsolationReason      = "account_health_auto_isolation"
	accountHealthDefaultHealthyHours  = 6
	accountHealthDefaultProbePageSize = 100
	accountHealthOverviewAccountLimit = 10000
)

var accountHealthBackoffSteps = []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute}

type AccountHealthState struct {
	AccountID            int64      `json:"account_id"`
	Score                int        `json:"score"`
	ConsecutiveSuccesses int        `json:"consecutive_successes"`
	ConsecutiveFailures  int        `json:"consecutive_failures"`
	Status               string     `json:"status"`
	LastSuccessAt        *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt        *time.Time `json:"last_failure_at,omitempty"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	LastErrorCategory    string     `json:"last_error_category,omitempty"`
	LastErrorMessage     string     `json:"last_error_message,omitempty"`
	LatencyEWMAMs        *int       `json:"latency_ewma_ms,omitempty"`
	BackoffLevel         int        `json:"backoff_level"`
	NextProbeAt          *time.Time `json:"next_probe_at,omitempty"`
	IsolatedAt           *time.Time `json:"isolated_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type AccountHealthSummary struct {
	AccountHealthState
	BaseURL                    string     `json:"base_url,omitempty"`
	KeyFingerprint             string     `json:"key_fingerprint,omitempty"`
	AccountName                string     `json:"account_name,omitempty"`
	Platform                   string     `json:"platform,omitempty"`
	Type                       string     `json:"type,omitempty"`
	RateMultiplier             float64    `json:"rate_multiplier"`
	RateMultiplierConfigured   bool       `json:"rate_multiplier_configured"`
	Schedulable                bool       `json:"schedulable"`
	TempUnschedulableUntil     *time.Time `json:"temp_unschedulable_until,omitempty"`
	HealthProbeEnabled         bool       `json:"health_probe_enabled"`
	HealthProbeIntervalMinutes *int       `json:"health_probe_interval_minutes,omitempty"`
	HealthProbeModel           *string    `json:"health_probe_model,omitempty"`
	HealthyProbeEnabled        bool       `json:"healthy_probe_enabled"`
	HealthyProbeIntervalHours  *int       `json:"healthy_probe_interval_hours,omitempty"`
	GroupIDs                   []int64    `json:"group_ids,omitempty"`
	GroupNames                 []string   `json:"group_names,omitempty"`
}

type AccountHealthURLOverview struct {
	BaseURL                string                 `json:"base_url"`
	Accounts               []AccountHealthSummary `json:"accounts"`
	Risks                  []AccountHealthRisk    `json:"risks,omitempty"`
	InsufficientGroupIDs   []int64                `json:"insufficient_group_ids,omitempty"`
	InsufficientGroupNames []string               `json:"insufficient_group_names,omitempty"`
}

type AccountHealthOverview struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	URLs        []AccountHealthURLOverview `json:"urls"`
	Risks       []AccountHealthRisk        `json:"risks,omitempty"`
}

type AccountHealthRisk struct {
	Level     string `json:"level"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	BaseURL   string `json:"base_url,omitempty"`
	AccountID *int64 `json:"account_id,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
	Count     int    `json:"count,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
}

type AccountHealthEvent struct {
	ID               int64     `json:"id"`
	AccountID        int64     `json:"account_id"`
	Source           string    `json:"source"`
	EventType        string    `json:"event_type"`
	ScoreBefore      int       `json:"score_before"`
	ScoreAfter       int       `json:"score_after"`
	StatusBefore     string    `json:"status_before"`
	StatusAfter      string    `json:"status_after"`
	Delta            int       `json:"delta"`
	ErrorCategory    string    `json:"error_category,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	LatencyMs        *int64    `json:"latency_ms,omitempty"`
	AffectedGroupIDs []int64   `json:"affected_group_ids,omitempty"`
	ActorUserID      *int64    `json:"actor_user_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type AccountHealthEventList struct {
	Items      []AccountHealthEvent `json:"items"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	TotalPages int                  `json:"total_pages"`
}

type AccountHealthRepository interface {
	Get(ctx context.Context, accountID int64) (*AccountHealthState, error)
	ListByAccountIDs(ctx context.Context, ids []int64) (map[int64]*AccountHealthState, error)
	Upsert(ctx context.Context, state *AccountHealthState) error
	Delete(ctx context.Context, accountID int64) error
	ListDueForProbe(ctx context.Context, now time.Time, limit int) ([]*AccountHealthState, error)
	InsertEvent(ctx context.Context, event *AccountHealthEvent) error
	ListEvents(ctx context.Context, accountID int64, eventType string, params pagination.PaginationParams) (*AccountHealthEventList, error)
	DeleteEventsBefore(ctx context.Context, before time.Time) (int64, error)
}

const (
	AccountHealthEventSourceRealRequest     = "real_request"
	AccountHealthEventSourceBackgroundProbe = "background_probe"
	AccountHealthEventSourceManualProbe     = "manual_probe"
	AccountHealthEventSourceSystem          = "system"

	AccountHealthEventTypeSuccess         = "success"
	AccountHealthEventTypeFailure         = "failure"
	AccountHealthEventTypeIsolated        = "isolated"
	AccountHealthEventTypeRecovering      = "recovering"
	AccountHealthEventTypeRecovered       = "recovered"
	AccountHealthEventTypeSettingsChanged = "settings_changed"
)

type accountHealthAccountRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
	List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error)
	SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error
	ClearTempUnschedulable(ctx context.Context, id int64) error
}

type accountHealthHealthyProbeCandidateRepository interface {
	ListHealthyProbeCandidates(ctx context.Context, now time.Time, limit int) ([]Account, error)
}

type AccountHealthService struct {
	repo        AccountHealthRepository
	accountRepo accountHealthAccountRepository
	now         func() time.Time
}

func NewAccountHealthService(repo AccountHealthRepository, accountRepo AccountRepository) *AccountHealthService {
	return &AccountHealthService{repo: repo, accountRepo: accountRepo, now: time.Now}
}

func (s *AccountHealthService) Get(ctx context.Context, accountID int64) (*AccountHealthSummary, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	state, err := s.getOrDefault(ctx, accountID)
	if err != nil {
		return nil, err
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.enrichSummary(state, account), nil
}

func (s *AccountHealthService) HealthProbeModel(ctx context.Context, accountID int64) string {
	if s == nil || s.accountRepo == nil {
		return ""
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.HealthProbeModel == nil {
		return ""
	}
	return strings.TrimSpace(*account.HealthProbeModel)
}

func (s *AccountHealthService) ListByAccountIDs(ctx context.Context, ids []int64) (map[int64]*AccountHealthSummary, error) {
	out := make(map[int64]*AccountHealthSummary, len(ids))
	if s == nil || s.repo == nil || len(ids) == 0 {
		return out, nil
	}
	states, err := s.repo.ListByAccountIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		state := states[account.ID]
		if state == nil {
			state = defaultAccountHealthState(account.ID, s.now())
		}
		out[account.ID] = s.enrichSummary(state, account)
	}
	return out, nil
}

func (s *AccountHealthService) RecordSuccess(ctx context.Context, accountID int64, latencyMs int64) error {
	return s.recordSuccess(ctx, accountID, latencyMs, false, AccountHealthEventSourceRealRequest, nil)
}

func (s *AccountHealthService) RecordProbeSuccess(ctx context.Context, accountID int64, latencyMs int64) error {
	return s.recordSuccess(ctx, accountID, latencyMs, true, AccountHealthEventSourceBackgroundProbe, nil)
}

func (s *AccountHealthService) RecordManualProbeSuccess(ctx context.Context, accountID int64, latencyMs int64, actorUserID *int64) error {
	return s.recordSuccess(ctx, accountID, latencyMs, true, AccountHealthEventSourceManualProbe, actorUserID)
}

func (s *AccountHealthService) recordSuccess(ctx context.Context, accountID int64, latencyMs int64, probe bool, source string, actorUserID *int64) error {
	if s == nil || s.repo == nil {
		return nil
	}
	now := s.now()
	state, err := s.getOrDefault(ctx, accountID)
	if err != nil {
		return err
	}
	before := *state
	authRecovery := state.LastErrorCategory == "auth_error"
	reward := accountHealthSuccessReward
	if probe && canFastRecoverFromProbeSuccess(state) {
		reward = accountHealthProbeRecoveryReward
	}
	state.Score = clampHealthScore(state.Score + reward)
	state.ConsecutiveSuccesses++
	state.ConsecutiveFailures = 0
	state.LastSuccessAt = &now
	state.LastCheckedAt = &now
	if !authRecovery {
		state.LastErrorCategory = ""
		state.LastErrorMessage = ""
	}
	if latencyMs > 0 {
		next := int(latencyMs)
		if state.LatencyEWMAMs != nil {
			next = int(math.Round(float64(*state.LatencyEWMAMs)*0.7 + float64(latencyMs)*0.3))
		}
		state.LatencyEWMAMs = &next
	}
	if state.Status == AccountHealthStatusIsolated || state.Status == AccountHealthStatusRecovering {
		recoveryScore := accountHealthRecoveryScore
		if probe && reward == accountHealthProbeRecoveryReward {
			recoveryScore = accountHealthProbeRecoveryScore
		}
		if state.ConsecutiveSuccesses >= accountHealthRecoverySuccesses && state.Score >= recoveryScore {
			state.Status = AccountHealthStatusHealthy
			if probe && state.Score < accountHealthRecoveredScore {
				state.Score = accountHealthRecoveredScore
			}
			state.BackoffLevel = 0
			state.NextProbeAt = nil
			state.IsolatedAt = nil
			state.LastErrorCategory = ""
			state.LastErrorMessage = ""
			if err := s.accountRepo.ClearTempUnschedulable(ctx, accountID); err != nil {
				return err
			}
		} else {
			state.Status = AccountHealthStatusRecovering
			state.NextProbeAt = accountHealthPtrTime(now.Add(s.nextProbeDelayForState(ctx, accountID, state)))
		}
	} else if state.Score < accountHealthRecoveryScore {
		state.Status = AccountHealthStatusDegraded
	} else {
		state.Status = AccountHealthStatusHealthy
	}
	if state.Status == AccountHealthStatusHealthy {
		if err := s.clearStaleHealthTempUnschedulable(ctx, accountID); err != nil {
			return err
		}
	}
	state.UpdatedAt = now
	if err := s.repo.Upsert(ctx, state); err != nil {
		return err
	}
	_ = s.insertHealthEvent(ctx, &before, state, source, accountHealthSuccessEventType(&before, state), "", "", latencyMs, actorUserID)
	return nil
}

func (s *AccountHealthService) RecordFailure(ctx context.Context, accountID int64, category, message string) error {
	return s.recordFailure(ctx, accountID, category, message, AccountHealthEventSourceRealRequest, nil)
}

func (s *AccountHealthService) RecordProbeFailure(ctx context.Context, accountID int64, category, message string) error {
	return s.recordFailure(ctx, accountID, category, message, AccountHealthEventSourceBackgroundProbe, nil)
}

func (s *AccountHealthService) RecordManualProbeFailure(ctx context.Context, accountID int64, category, message string, actorUserID *int64) error {
	return s.recordFailure(ctx, accountID, category, message, AccountHealthEventSourceManualProbe, actorUserID)
}

func (s *AccountHealthService) recordFailure(ctx context.Context, accountID int64, category, message, source string, actorUserID *int64) error {
	if s == nil || s.repo == nil {
		return nil
	}
	now := s.now()
	state, err := s.getOrDefault(ctx, accountID)
	if err != nil {
		return err
	}
	before := *state
	state.Score = clampHealthScore(state.Score - accountHealthFailurePenalty)
	state.ConsecutiveFailures++
	state.ConsecutiveSuccesses = 0
	state.LastFailureAt = &now
	state.LastCheckedAt = &now
	state.LastErrorCategory = truncateAccountHealthString(strings.TrimSpace(category), 40)
	state.LastErrorMessage = truncateAccountHealthString(redactAccountHealthMessage(message), 1000)
	state.BackoffLevel = nextBackoffLevel(state.BackoffLevel)
	if state.ConsecutiveFailures >= accountHealthIsolationFailures || state.Score < accountHealthIsolationScore {
		state.Status = AccountHealthStatusIsolated
		state.IsolatedAt = &now
	} else {
		state.Status = AccountHealthStatusDegraded
	}
	nextProbe := now.Add(s.nextProbeDelayForState(ctx, accountID, state))
	state.NextProbeAt = &nextProbe
	if state.Status == AccountHealthStatusIsolated {
		if err := s.accountRepo.SetTempUnschedulable(ctx, accountID, nextProbe, accountHealthIsolationReason); err != nil {
			return err
		}
	}
	state.UpdatedAt = now
	if err := s.repo.Upsert(ctx, state); err != nil {
		return err
	}
	_ = s.insertHealthEvent(ctx, &before, state, source, accountHealthFailureEventType(&before, state), state.LastErrorCategory, state.LastErrorMessage, 0, actorUserID)
	return nil
}

func (s *AccountHealthService) Reset(ctx context.Context, accountID int64) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if err := s.repo.Delete(ctx, accountID); err != nil {
		return err
	}
	return s.accountRepo.ClearTempUnschedulable(ctx, accountID)
}

func (s *AccountHealthService) RecordSettingsChanged(ctx context.Context, accountID int64, actorUserID *int64) error {
	if s == nil || s.repo == nil {
		return nil
	}
	now := s.now()
	state, err := s.getOrDefault(ctx, accountID)
	if err != nil {
		return err
	}
	event := &AccountHealthEvent{
		AccountID:        accountID,
		Source:           AccountHealthEventSourceSystem,
		EventType:        AccountHealthEventTypeSettingsChanged,
		ScoreBefore:      state.Score,
		ScoreAfter:       state.Score,
		StatusBefore:     state.Status,
		StatusAfter:      state.Status,
		ActorUserID:      actorUserID,
		CreatedAt:        now,
		AffectedGroupIDs: s.affectedGroupIDs(ctx, accountID),
	}
	_ = s.repo.InsertEvent(ctx, event)
	return nil
}

func (s *AccountHealthService) ListEvents(ctx context.Context, accountID int64, eventType string, params pagination.PaginationParams) (*AccountHealthEventList, error) {
	if s == nil || s.repo == nil {
		return &AccountHealthEventList{Items: []AccountHealthEvent{}, Page: params.Page, PageSize: params.PageSize}, nil
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
	return s.repo.ListEvents(ctx, accountID, strings.TrimSpace(eventType), params)
}

func (s *AccountHealthService) CleanupEvents(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	return s.repo.DeleteEventsBefore(ctx, before)
}

func (s *AccountHealthService) clearStaleHealthTempUnschedulable(ctx context.Context, accountID int64) error {
	if s == nil || s.accountRepo == nil {
		return nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return err
	}
	if account.TempUnschedulableUntil == nil || account.TempUnschedulableReason != accountHealthIsolationReason {
		return nil
	}
	return s.accountRepo.ClearTempUnschedulable(ctx, accountID)
}

func (s *AccountHealthService) ListDueForProbe(ctx context.Context, now time.Time, limit int) ([]*AccountHealthState, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = accountHealthDefaultProbePageSize
	}
	states, err := s.repo.ListDueForProbe(ctx, now, limit)
	if err != nil || s.accountRepo == nil {
		return states, err
	}
	states = s.appendHealthyProbeDueStates(ctx, now, states, limit)
	if len(states) == 0 {
		return states, nil
	}
	ids := make([]int64, 0, len(states))
	for _, state := range states {
		if state != nil {
			ids = append(ids, state.AccountID)
		}
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	enabled := make(map[int64]bool, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if state := findAccountHealthState(states, account.ID); state != nil && state.Status == AccountHealthStatusHealthy {
			if shouldProbeHealthyAccount(account) {
				enabled[account.ID] = true
			}
			continue
		}
		if shouldProbeUnhealthyAccount(account) {
			enabled[account.ID] = true
		}
	}
	filtered := states[:0]
	for _, state := range states {
		if state != nil && enabled[state.AccountID] {
			filtered = append(filtered, state)
		}
	}
	return filtered, nil
}

func (s *AccountHealthService) appendHealthyProbeDueStates(ctx context.Context, now time.Time, states []*AccountHealthState, limit int) []*AccountHealthState {
	if s == nil || s.accountRepo == nil || len(states) >= limit {
		return states
	}
	var accounts []Account
	if candidateRepo, ok := s.accountRepo.(accountHealthHealthyProbeCandidateRepository); ok {
		var err error
		accounts, err = candidateRepo.ListHealthyProbeCandidates(ctx, now, limit-len(states))
		if err != nil || len(accounts) == 0 {
			return states
		}
	} else {
		listed, _, err := s.accountRepo.List(ctx, pagination.PaginationParams{Page: 1, PageSize: accountHealthOverviewAccountLimit})
		if err != nil || len(listed) == 0 {
			return states
		}
		accounts = listed
	}
	existing := make(map[int64]struct{}, len(states))
	for _, state := range states {
		if state != nil {
			existing[state.AccountID] = struct{}{}
		}
	}
	candidates := make([]int64, 0)
	for i := range accounts {
		account := accounts[i]
		if _, ok := existing[account.ID]; ok {
			continue
		}
		if !shouldProbeHealthyAccount(&account) {
			continue
		}
		candidates = append(candidates, account.ID)
	}
	if len(candidates) == 0 {
		return states
	}
	healthByID, err := s.repo.ListByAccountIDs(ctx, candidates)
	if err != nil {
		return states
	}
	for i := range accounts {
		if len(states) >= limit {
			break
		}
		account := accounts[i]
		if _, ok := existing[account.ID]; ok {
			continue
		}
		if !shouldProbeHealthyAccount(&account) {
			continue
		}
		state := healthByID[account.ID]
		if state == nil {
			states = append(states, defaultAccountHealthState(account.ID, now))
			existing[account.ID] = struct{}{}
			continue
		}
		if state.Status != AccountHealthStatusHealthy {
			continue
		}
		if healthyProbeDueAt(state, &account).After(now) {
			continue
		}
		states = append(states, state)
		existing[account.ID] = struct{}{}
	}
	return states
}

func findAccountHealthState(states []*AccountHealthState, accountID int64) *AccountHealthState {
	for _, state := range states {
		if state != nil && state.AccountID == accountID {
			return state
		}
	}
	return nil
}

func (s *AccountHealthService) Overview(ctx context.Context) (*AccountHealthOverview, error) {
	if s == nil {
		return &AccountHealthOverview{GeneratedAt: time.Now()}, nil
	}
	accounts, _, err := s.accountRepo.List(ctx, pagination.PaginationParams{Page: 1, PageSize: accountHealthOverviewAccountLimit})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	healthByID, err := s.ListByAccountIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byURL := make(map[string][]AccountHealthSummary)
	availableByGroup := make(map[int64]int)
	boundByGroup := make(map[int64]int)
	groupNames := make(map[int64]string)
	for i := range accounts {
		summary := healthByID[accounts[i].ID]
		if summary == nil {
			continue
		}
		baseURL := summary.BaseURL
		if baseURL == "" {
			baseURL = "(no upstream url)"
		}
		byURL[baseURL] = append(byURL[baseURL], *summary)
		if summary.Schedulable && summary.Status != AccountHealthStatusIsolated {
			for _, gid := range summary.GroupIDs {
				availableByGroup[gid]++
			}
		}
		for i, gid := range summary.GroupIDs {
			boundByGroup[gid]++
			if i < len(summary.GroupNames) {
				groupNames[gid] = summary.GroupNames[i]
			}
		}
	}
	urls := make([]AccountHealthURLOverview, 0, len(byURL))
	var overviewRisks []AccountHealthRisk
	for baseURL, items := range byURL {
		sort.SliceStable(items, func(i, j int) bool { return items[i].AccountID < items[j].AccountID })
		insufficient := make(map[int64]struct{})
		for _, item := range items {
			for _, gid := range item.GroupIDs {
				if availableByGroup[gid] <= 0 {
					insufficient[gid] = struct{}{}
				}
			}
		}
		ids := make([]int64, 0, len(insufficient))
		names := make([]string, 0, len(insufficient))
		for gid := range insufficient {
			ids = append(ids, gid)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, gid := range ids {
			if name := groupNames[gid]; name != "" {
				names = append(names, name)
			}
		}
		risks := buildAccountHealthURLRisks(baseURL, items, availableByGroup, boundByGroup, groupNames)
		overviewRisks = append(overviewRisks, risks...)
		urls = append(urls, AccountHealthURLOverview{BaseURL: baseURL, Accounts: items, Risks: risks, InsufficientGroupIDs: ids, InsufficientGroupNames: names})
	}
	sort.SliceStable(urls, func(i, j int) bool { return urls[i].BaseURL < urls[j].BaseURL })
	sortAccountHealthRisks(overviewRisks)
	return &AccountHealthOverview{GeneratedAt: s.now(), URLs: urls, Risks: overviewRisks}, nil
}

func (s *AccountHealthService) getOrDefault(ctx context.Context, accountID int64) (*AccountHealthState, error) {
	state, err := s.repo.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return defaultAccountHealthState(accountID, s.now()), nil
	}
	return state, nil
}

func (s *AccountHealthService) enrichSummary(state *AccountHealthState, account *Account) *AccountHealthSummary {
	if state == nil || account == nil {
		return nil
	}
	summary := &AccountHealthSummary{AccountHealthState: *state}
	summary.AccountName = account.Name
	summary.Platform = account.Platform
	summary.Type = account.Type
	summary.RateMultiplier = account.BillingRateMultiplier()
	summary.RateMultiplierConfigured = account.RateMultiplier != nil && math.Abs(account.BillingRateMultiplier()-1) > 1e-9
	summary.Schedulable = account.IsSchedulable()
	summary.TempUnschedulableUntil = account.TempUnschedulableUntil
	summary.HealthProbeEnabled = account.HealthProbeEnabled
	summary.HealthProbeIntervalMinutes = account.HealthProbeIntervalMinutes
	summary.HealthProbeModel = account.HealthProbeModel
	summary.HealthyProbeEnabled = account.HealthyProbeEnabled
	summary.HealthyProbeIntervalHours = account.HealthyProbeIntervalHours
	summary.BaseURL = accountHealthBaseURL(account)
	summary.KeyFingerprint = accountHealthKeyFingerprint(account)
	summary.GroupIDs = append([]int64(nil), account.GroupIDs...)
	for _, g := range account.Groups {
		if g != nil {
			summary.GroupNames = append(summary.GroupNames, g.Name)
		}
	}
	if len(summary.GroupIDs) == 0 && len(account.AccountGroups) > 0 {
		for _, ag := range account.AccountGroups {
			summary.GroupIDs = append(summary.GroupIDs, ag.GroupID)
			if ag.Group != nil {
				summary.GroupNames = append(summary.GroupNames, ag.Group.Name)
			}
		}
	}
	return summary
}

func defaultAccountHealthState(accountID int64, now time.Time) *AccountHealthState {
	return &AccountHealthState{AccountID: accountID, Score: defaultAccountHealthScore, Status: AccountHealthStatusHealthy, CreatedAt: now, UpdatedAt: now}
}

func AccountHealthSortValue(states map[int64]*AccountHealthSummary, accountID int64) (score int, latency int) {
	if state := states[accountID]; state != nil {
		latency = math.MaxInt
		if state.LatencyEWMAMs != nil {
			latency = *state.LatencyEWMAMs
		}
		return state.Score, latency
	}
	return defaultAccountHealthScore, math.MaxInt
}

func accountHealthBaseURL(account *Account) string {
	if account == nil {
		return ""
	}
	for _, key := range []string{"base_url", "custom_base_url", "upstream_url", "api_url", "endpoint"} {
		if value, ok := account.Credentials[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimRight(strings.TrimSpace(value), "/")
		}
	}
	if value, ok := account.Extra["custom_base_url"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimRight(strings.TrimSpace(value), "/")
	}
	return ""
}

func accountHealthKeyFingerprint(account *Account) string {
	if account == nil {
		return ""
	}
	for _, key := range []string{"api_key", "key", "access_token", "refresh_token", "session_key", "cookie"} {
		if value, ok := account.Credentials[key].(string); ok && strings.TrimSpace(value) != "" {
			sum := sha256.Sum256([]byte(value))
			return hex.EncodeToString(sum[:])[:12]
		}
	}
	return ""
}

func clampHealthScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func nextBackoffLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level >= len(accountHealthBackoffSteps)-1 {
		return len(accountHealthBackoffSteps) - 1
	}
	return level + 1
}

func backoffDuration(level int) time.Duration {
	if level < 0 {
		level = 0
	}
	if level >= len(accountHealthBackoffSteps) {
		level = len(accountHealthBackoffSteps) - 1
	}
	return accountHealthBackoffSteps[level]
}

func (s *AccountHealthService) nextProbeDelay(ctx context.Context, accountID int64, backoffLevel int) time.Duration {
	if s != nil && s.accountRepo != nil {
		if account, err := s.accountRepo.GetByID(ctx, accountID); err == nil && account != nil && account.HealthProbeIntervalMinutes != nil && *account.HealthProbeIntervalMinutes > 0 {
			return time.Duration(*account.HealthProbeIntervalMinutes) * time.Minute
		}
	}
	return backoffDuration(backoffLevel)
}

func (s *AccountHealthService) nextProbeDelayForState(ctx context.Context, accountID int64, state *AccountHealthState) time.Duration {
	if state == nil {
		return s.nextProbeDelay(ctx, accountID, 0)
	}
	if state.LastErrorCategory != "auth_error" && (state.Status == AccountHealthStatusIsolated || state.Status == AccountHealthStatusRecovering) {
		return 5 * time.Minute
	}
	return s.nextProbeDelay(ctx, accountID, state.BackoffLevel)
}

func (s *AccountHealthService) insertHealthEvent(ctx context.Context, before, after *AccountHealthState, source, eventType, category, message string, latencyMs int64, actorUserID *int64) error {
	if s == nil || s.repo == nil || before == nil || after == nil {
		return nil
	}
	latency := (*int64)(nil)
	if latencyMs > 0 {
		latency = &latencyMs
	}
	event := &AccountHealthEvent{
		AccountID:        after.AccountID,
		Source:           source,
		EventType:        eventType,
		ScoreBefore:      before.Score,
		ScoreAfter:       after.Score,
		StatusBefore:     before.Status,
		StatusAfter:      after.Status,
		Delta:            after.Score - before.Score,
		ErrorCategory:    truncateAccountHealthString(strings.TrimSpace(category), 40),
		ErrorMessage:     truncateAccountHealthString(redactAccountHealthMessage(message), 1000),
		LatencyMs:        latency,
		AffectedGroupIDs: s.affectedGroupIDs(ctx, after.AccountID),
		ActorUserID:      actorUserID,
		CreatedAt:        s.now(),
	}
	return s.repo.InsertEvent(ctx, event)
}

func (s *AccountHealthService) affectedGroupIDs(ctx context.Context, accountID int64) []int64 {
	if s == nil || s.accountRepo == nil {
		return nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil
	}
	ids := append([]int64(nil), account.GroupIDs...)
	if len(ids) == 0 {
		for _, ag := range account.AccountGroups {
			ids = append(ids, ag.GroupID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func accountHealthSuccessEventType(before, after *AccountHealthState) string {
	if before == nil || after == nil {
		return AccountHealthEventTypeSuccess
	}
	if before.Status == AccountHealthStatusIsolated || before.Status == AccountHealthStatusRecovering {
		if after.Status == AccountHealthStatusHealthy {
			return AccountHealthEventTypeRecovered
		}
		return AccountHealthEventTypeRecovering
	}
	return AccountHealthEventTypeSuccess
}

func accountHealthFailureEventType(before, after *AccountHealthState) string {
	if after == nil {
		return AccountHealthEventTypeFailure
	}
	if after.Status == AccountHealthStatusIsolated && (before == nil || before.Status != AccountHealthStatusIsolated) {
		return AccountHealthEventTypeIsolated
	}
	return AccountHealthEventTypeFailure
}

func buildAccountHealthURLRisks(baseURL string, items []AccountHealthSummary, availableByGroup, boundByGroup map[int64]int, groupNames map[int64]string) []AccountHealthRisk {
	risks := make([]AccountHealthRisk, 0)
	healthyLike := 0
	for i := range items {
		item := items[i]
		if item.Schedulable && item.Status != AccountHealthStatusIsolated {
			healthyLike++
		}
		if item.ConsecutiveFailures > 0 && item.Status != AccountHealthStatusIsolated {
			id := item.AccountID
			risks = append(risks, AccountHealthRisk{Level: "warning", Type: "consecutive_failures", Message: "account has recent consecutive failures", BaseURL: baseURL, AccountID: &id, Count: item.ConsecutiveFailures, Threshold: accountHealthIsolationFailures})
		}
		if item.HealthProbeEnabled && !item.HealthyProbeEnabled && item.Status == AccountHealthStatusHealthy {
			id := item.AccountID
			risks = append(risks, AccountHealthRisk{Level: "info", Type: "healthy_probe_disabled", Message: "healthy low-frequency probe disabled", BaseURL: baseURL, AccountID: &id})
		}
	}
	if len(items) > 0 && healthyLike == 0 {
		risks = append(risks, AccountHealthRisk{Level: "critical", Type: "url_all_isolated", Message: "all accounts under this upstream URL are unavailable", BaseURL: baseURL, Count: len(items)})
	}
	seenGroups := map[int64]struct{}{}
	for _, item := range items {
		for _, gid := range item.GroupIDs {
			if _, ok := seenGroups[gid]; ok {
				continue
			}
			seenGroups[gid] = struct{}{}
			available := availableByGroup[gid]
			bound := boundByGroup[gid]
			name := groupNames[gid]
			groupID := gid
			if bound > 0 && available == 0 {
				risks = append(risks, AccountHealthRisk{Level: "critical", Type: "group_no_available_accounts", Message: "group has no available accounts", BaseURL: baseURL, GroupID: &groupID, GroupName: name, Count: available})
			} else if available == 1 {
				risks = append(risks, AccountHealthRisk{Level: "warning", Type: "group_single_available_account", Message: "group has only one available account", BaseURL: baseURL, GroupID: &groupID, GroupName: name, Count: available, Threshold: 1})
			}
		}
	}
	sortAccountHealthRisks(risks)
	return risks
}

func sortAccountHealthRisks(risks []AccountHealthRisk) {
	priority := map[string]int{"critical": 0, "warning": 1, "info": 2}
	sort.SliceStable(risks, func(i, j int) bool {
		if priority[risks[i].Level] != priority[risks[j].Level] {
			return priority[risks[i].Level] < priority[risks[j].Level]
		}
		if risks[i].Type != risks[j].Type {
			return risks[i].Type < risks[j].Type
		}
		if risks[i].BaseURL != risks[j].BaseURL {
			return risks[i].BaseURL < risks[j].BaseURL
		}
		return risks[i].Count > risks[j].Count
	})
}

func healthyProbeInterval(account *Account) time.Duration {
	if account != nil && account.HealthyProbeIntervalHours != nil && *account.HealthyProbeIntervalHours > 0 {
		return time.Duration(*account.HealthyProbeIntervalHours) * time.Hour
	}
	return accountHealthDefaultHealthyHours * time.Hour
}

func healthyProbeDueAt(state *AccountHealthState, account *Account) time.Time {
	base := time.Time{}
	if state != nil {
		if state.LastCheckedAt != nil {
			base = *state.LastCheckedAt
		} else if state.LastSuccessAt != nil {
			base = *state.LastSuccessAt
		} else if !state.UpdatedAt.IsZero() {
			base = state.UpdatedAt
		} else if !state.CreatedAt.IsZero() {
			base = state.CreatedAt
		}
	}
	return base.Add(healthyProbeInterval(account))
}

func shouldProbeUnhealthyAccount(account *Account) bool {
	if account == nil || !account.HealthProbeEnabled {
		return false
	}
	if account.IsSchedulable() {
		return true
	}
	return account.TempUnschedulableUntil != nil
}

func shouldProbeHealthyAccount(account *Account) bool {
	if account == nil || !account.HealthProbeEnabled || !account.HealthyProbeEnabled {
		return false
	}
	return account.Schedulable && account.IsSchedulable()
}

func canFastRecoverFromProbeSuccess(state *AccountHealthState) bool {
	if state == nil {
		return false
	}
	if state.Status != AccountHealthStatusIsolated && state.Status != AccountHealthStatusRecovering {
		return false
	}
	return state.LastErrorCategory != "auth_error"
}

func accountHealthPtrTime(t time.Time) *time.Time { return &t }

func truncateAccountHealthString(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func redactAccountHealthMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	return logredact.RedactText(message, "api_key", "key", "authorization", "cookie", "token", "session_key")
}
