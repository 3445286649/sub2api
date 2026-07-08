package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const (
	UpstreamBalanceStatusOK          = "ok"
	UpstreamBalanceStatusUnsupported = "unsupported"
	UpstreamBalanceStatusError       = "error"
	UpstreamBalanceStatusAuthError   = "auth_error"
	UpstreamBalanceStatusChecking    = "checking"

	accountUpstreamBalanceRefreshInterval = 30 * time.Minute
	accountUpstreamBalanceQueryTimeout    = 15 * time.Second
	accountUpstreamBalanceLease           = 2 * time.Minute
	accountUpstreamBalanceMaxBodyBytes    = 64 * 1024
)

var (
	upstreamBalanceLooseSKPattern     = regexp.MustCompile(`\bsk-[0-9A-Za-z_-]{8,}\b`)
	upstreamBalanceAuthorizationValue = regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*bearer\s+[^,\s]+`)
)

type AccountUpstreamBalanceSnapshot struct {
	BaseURL                 string     `json:"base_url"`
	RepresentativeAccountID *int64     `json:"representative_account_id,omitempty"`
	Status                  string     `json:"status"`
	Balance                 *float64   `json:"balance,omitempty"`
	Remaining               *float64   `json:"remaining,omitempty"`
	Unit                    string     `json:"unit,omitempty"`
	SourceEndpoint          string     `json:"source_endpoint,omitempty"`
	HTTPStatus              *int       `json:"http_status,omitempty"`
	ErrorMessage            string     `json:"error_message,omitempty"`
	CheckedAt               *time.Time `json:"checked_at,omitempty"`
	NextCheckAt             *time.Time `json:"next_check_at,omitempty"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type AccountUpstreamBalanceRepository interface {
	Get(ctx context.Context, baseURL string) (*AccountUpstreamBalanceSnapshot, error)
	ListByBaseURLs(ctx context.Context, baseURLs []string) (map[string]*AccountUpstreamBalanceSnapshot, error)
	Upsert(ctx context.Context, snapshot *AccountUpstreamBalanceSnapshot) error
	Ensure(ctx context.Context, baseURL string, nextCheckAt time.Time) error
	ListDue(ctx context.Context, now time.Time, limit int) ([]string, error)
	Claim(ctx context.Context, baseURL string, now, leaseUntil time.Time) (bool, error)
	ClaimRefresh(ctx context.Context, baseURL string, now, leaseUntil time.Time) (bool, error)
}

type accountUpstreamBalanceAccountRepository interface {
	List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error)
}

type AccountUpstreamBalanceService struct {
	repo        AccountUpstreamBalanceRepository
	accountRepo accountUpstreamBalanceAccountRepository
	now         func() time.Time
}

func NewAccountUpstreamBalanceService(repo AccountUpstreamBalanceRepository, accountRepo accountUpstreamBalanceAccountRepository) *AccountUpstreamBalanceService {
	return &AccountUpstreamBalanceService{repo: repo, accountRepo: accountRepo, now: func() time.Time { return time.Now().UTC() }}
}

func (s *AccountUpstreamBalanceService) SnapshotMap(ctx context.Context, baseURLs []string) (map[string]*AccountUpstreamBalanceSnapshot, error) {
	if s == nil || s.repo == nil {
		return map[string]*AccountUpstreamBalanceSnapshot{}, nil
	}
	normalized := make([]string, 0, len(baseURLs))
	seen := make(map[string]struct{}, len(baseURLs))
	for _, raw := range baseURLs {
		baseURL := normalizeUpstreamBalanceBaseURL(raw)
		if baseURL == "" {
			continue
		}
		if err := validateUpstreamBalanceBaseURL(baseURL); err != nil {
			continue
		}
		if _, ok := seen[baseURL]; ok {
			continue
		}
		seen[baseURL] = struct{}{}
		normalized = append(normalized, baseURL)
	}
	if len(normalized) == 0 {
		return map[string]*AccountUpstreamBalanceSnapshot{}, nil
	}
	return s.repo.ListByBaseURLs(ctx, normalized)
}

func (s *AccountUpstreamBalanceService) EnsureKnownBaseURLs(ctx context.Context, baseURLs []string) {
	if s == nil || s.repo == nil {
		return
	}
	now := s.now()
	for _, raw := range baseURLs {
		baseURL := normalizeUpstreamBalanceBaseURL(raw)
		if baseURL == "" {
			continue
		}
		if err := validateUpstreamBalanceBaseURL(baseURL); err != nil {
			continue
		}
		_ = s.repo.Ensure(ctx, baseURL, now)
	}
}

func (s *AccountUpstreamBalanceService) RefreshByBaseURL(ctx context.Context, rawBaseURL string) (*AccountUpstreamBalanceSnapshot, error) {
	return s.refreshByBaseURL(ctx, rawBaseURL, true, false)
}

func (s *AccountUpstreamBalanceService) refreshByBaseURL(ctx context.Context, rawBaseURL string, claim bool, allowOrphan bool) (*AccountUpstreamBalanceSnapshot, error) {
	if s == nil || s.repo == nil || s.accountRepo == nil {
		return nil, infraerrors.InternalServer("UPSTREAM_BALANCE_UNAVAILABLE", "upstream balance service unavailable")
	}
	baseURL := normalizeUpstreamBalanceBaseURL(rawBaseURL)
	if baseURL == "" {
		return nil, infraerrors.BadRequest("INVALID_BASE_URL", "invalid base url")
	}
	if err := validateUpstreamBalanceBaseURL(baseURL); err != nil {
		return nil, infraerrors.BadRequest("INVALID_BASE_URL", "invalid base url")
	}
	accounts, _, err := s.accountRepo.List(ctx, pagination.PaginationParams{Page: 1, PageSize: accountHealthOverviewAccountLimit})
	if err != nil {
		return nil, err
	}
	now := s.now()
	account, matched := selectUpstreamBalanceRepresentative(accounts, baseURL)
	if !matched && !allowOrphan {
		return nil, infraerrors.NotFound("UPSTREAM_BALANCE_URL_NOT_FOUND", "upstream url not found")
	}
	if claim {
		if err := s.repo.Ensure(ctx, baseURL, now); err != nil {
			return nil, err
		}
		claimed, err := s.repo.ClaimRefresh(ctx, baseURL, now, now.Add(accountUpstreamBalanceLease))
		if err != nil {
			return nil, err
		}
		if !claimed {
			if snapshot, getErr := s.repo.Get(ctx, baseURL); getErr == nil && snapshot != nil {
				return snapshot, nil
			}
			return nil, infraerrors.Conflict("UPSTREAM_BALANCE_REFRESH_IN_PROGRESS", "upstream balance refresh is already in progress")
		}
	}
	if account == nil {
		snapshot := &AccountUpstreamBalanceSnapshot{
			BaseURL:      baseURL,
			Status:       UpstreamBalanceStatusUnsupported,
			ErrorMessage: "no account with api key for upstream url",
			CheckedAt:    &now,
			NextCheckAt:  ptrUpstreamBalanceTime(now.Add(accountUpstreamBalanceRefreshInterval)),
			UpdatedAt:    now,
		}
		if err := s.repo.Upsert(ctx, snapshot); err != nil {
			return nil, err
		}
		return snapshot, nil
	}
	snapshot := s.queryAccountBalance(ctx, baseURL, account, now)
	if err := s.repo.Upsert(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *AccountUpstreamBalanceService) RefreshDue(ctx context.Context, limit int) {
	if s == nil || s.repo == nil {
		return
	}
	if limit <= 0 {
		limit = 100
	}
	now := s.now()
	baseURLs, err := s.repo.ListDue(ctx, now, limit)
	if err != nil {
		return
	}
	for _, baseURL := range baseURLs {
		claimed, err := s.repo.Claim(ctx, baseURL, now, now.Add(accountUpstreamBalanceLease))
		if err != nil || !claimed {
			continue
		}
		ctxOne, cancel := context.WithTimeout(ctx, accountUpstreamBalanceQueryTimeout+5*time.Second)
		_, _ = s.refreshByBaseURL(ctxOne, baseURL, false, true)
		cancel()
	}
}

func (s *AccountUpstreamBalanceService) queryAccountBalance(ctx context.Context, baseURL string, account *Account, now time.Time) *AccountUpstreamBalanceSnapshot {
	accountID := account.ID
	next := now.Add(accountUpstreamBalanceRefreshInterval)
	snapshot := &AccountUpstreamBalanceSnapshot{
		BaseURL:                 baseURL,
		RepresentativeAccountID: &accountID,
		Status:                  UpstreamBalanceStatusUnsupported,
		CheckedAt:               &now,
		NextCheckAt:             &next,
		UpdatedAt:               now,
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		snapshot.ErrorMessage = "representative account has no api key"
		return snapshot
	}
	endpoints := upstreamBalanceCandidateEndpoints(baseURL)
	var lastStatus int
	var lastMessage string
	for _, endpoint := range endpoints {
		status, result, message := s.fetchBalanceEndpoint(ctx, account, endpoint, apiKey)
		lastStatus = status
		lastMessage = message
		if result != nil {
			snapshot.Status = UpstreamBalanceStatusOK
			snapshot.Balance = result.balance
			snapshot.Remaining = result.remaining
			snapshot.Unit = result.unit
			snapshot.SourceEndpoint = endpoint
			snapshot.HTTPStatus = &status
			snapshot.ErrorMessage = ""
			return snapshot
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			snapshot.Status = UpstreamBalanceStatusAuthError
			snapshot.SourceEndpoint = endpoint
			snapshot.HTTPStatus = &status
			snapshot.ErrorMessage = sanitizeUpstreamBalanceError(message)
			return snapshot
		}
	}
	for _, pair := range upstreamBalanceNewAPIEndpointPairs(baseURL) {
		status, result, message := s.fetchNewAPIBalanceEndpointPair(ctx, account, pair, apiKey)
		lastStatus = status
		lastMessage = message
		if result != nil {
			snapshot.Status = UpstreamBalanceStatusOK
			snapshot.Balance = result.balance
			snapshot.Remaining = result.remaining
			snapshot.Unit = result.unit
			snapshot.SourceEndpoint = pair.subscriptionEndpoint + " + " + pair.usageEndpoint
			snapshot.HTTPStatus = &status
			snapshot.ErrorMessage = ""
			return snapshot
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			snapshot.Status = UpstreamBalanceStatusAuthError
			snapshot.SourceEndpoint = pair.subscriptionEndpoint
			snapshot.HTTPStatus = &status
			snapshot.ErrorMessage = sanitizeUpstreamBalanceError(message)
			return snapshot
		}
	}
	if lastStatus > 0 {
		snapshot.HTTPStatus = &lastStatus
	}
	if lastStatus == http.StatusNotFound || lastStatus == http.StatusMethodNotAllowed {
		snapshot.Status = UpstreamBalanceStatusUnsupported
	} else if lastStatus >= 400 || lastStatus == 0 {
		snapshot.Status = UpstreamBalanceStatusError
	}
	snapshot.ErrorMessage = sanitizeUpstreamBalanceError(lastMessage)
	return snapshot
}

type upstreamBalanceParseResult struct {
	balance   *float64
	remaining *float64
	unit      string
}

func (s *AccountUpstreamBalanceService) fetchBalanceEndpoint(ctx context.Context, account *Account, endpoint string, apiKey string) (int, *upstreamBalanceParseResult, string) {
	status, body, message := s.fetchBalanceEndpointRaw(ctx, account, endpoint, apiKey)
	if status < 200 || status >= 300 || len(body) == 0 {
		return status, nil, message
	}
	result, err := parseUpstreamBalanceResponse(body)
	if err != nil {
		return status, nil, err.Error()
	}
	if result == nil {
		return status, nil, "balance fields not found"
	}
	return status, result, ""
}

func (s *AccountUpstreamBalanceService) fetchBalanceEndpointRaw(ctx context.Context, account *Account, endpoint string, apiKey string) (int, []byte, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err.Error()
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("x-api-key", apiKey)
	proxyURL := ""
	if account != nil && account.Proxy != nil && account.Proxy.IsActive() {
		proxyURL = account.Proxy.URL()
	}
	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:              proxyURL,
		Timeout:               accountUpstreamBalanceQueryTimeout,
		ResponseHeaderTimeout: 10 * time.Second,
		ValidateResolvedIP:    true,
		AllowPrivateHosts:     false,
	})
	if err != nil {
		return 0, nil, err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, accountUpstreamBalanceMaxBodyBytes))
	if err != nil {
		return resp.StatusCode, nil, err.Error()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, nil, string(body)
	}
	return resp.StatusCode, body, ""
}

func (s *AccountUpstreamBalanceService) fetchNewAPIBalanceEndpointPair(ctx context.Context, account *Account, pair upstreamBalanceNewAPIEndpointPair, apiKey string) (int, *upstreamBalanceParseResult, string) {
	subStatus, subBody, subMessage := s.fetchBalanceEndpointRaw(ctx, account, pair.subscriptionEndpoint, apiKey)
	if subStatus < 200 || subStatus >= 300 || len(subBody) == 0 {
		return subStatus, nil, subMessage
	}
	usageStatus, usageBody, usageMessage := s.fetchBalanceEndpointRaw(ctx, account, pair.usageEndpoint, apiKey)
	if usageStatus < 200 || usageStatus >= 300 || len(usageBody) == 0 {
		return usageStatus, nil, usageMessage
	}
	result, err := parseNewAPIUpstreamBalanceResponse(subBody, usageBody)
	if err != nil {
		return usageStatus, nil, err.Error()
	}
	return usageStatus, result, ""
}

func parseNewAPIUpstreamBalanceResponse(subscriptionBody, usageBody []byte) (*upstreamBalanceParseResult, error) {
	var subscription map[string]any
	if err := json.Unmarshal(subscriptionBody, &subscription); err != nil {
		return nil, err
	}
	if msg := upstreamBalanceEmbeddedError(subscription); msg != "" {
		return nil, fmt.Errorf("subscription error: %s", msg)
	}
	limit, ok := floatFromAny(subscription["hard_limit_usd"])
	if !ok {
		limit, ok = floatFromAny(subscription["system_hard_limit_usd"])
	}
	if !ok {
		limit, ok = floatFromAny(subscription["soft_limit_usd"])
	}
	if !ok {
		return nil, fmt.Errorf("subscription limit fields not found")
	}

	var usage map[string]any
	if err := json.Unmarshal(usageBody, &usage); err != nil {
		return nil, err
	}
	if msg := upstreamBalanceEmbeddedError(usage); msg != "" {
		return nil, fmt.Errorf("usage error: %s", msg)
	}
	totalUsage, ok := floatFromAny(usage["total_usage"])
	if !ok {
		return nil, fmt.Errorf("usage total_usage field not found")
	}
	remaining := limit - totalUsage/100
	return &upstreamBalanceParseResult{balance: &remaining, remaining: &remaining, unit: "upstream"}, nil
}

func upstreamBalanceEmbeddedError(data map[string]any) string {
	if data == nil {
		return ""
	}
	errorValue, ok := data["error"]
	if !ok || errorValue == nil {
		return ""
	}
	if message, ok := errorValue.(string); ok {
		return strings.TrimSpace(message)
	}
	if errorMap, ok := errorValue.(map[string]any); ok {
		if message, ok := errorMap["message"].(string); ok {
			return strings.TrimSpace(message)
		}
	}
	return "upstream returned error"
}

func parseUpstreamBalanceResponse(body []byte) (*upstreamBalanceParseResult, error) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	result := &upstreamBalanceParseResult{}
	if v, ok := floatFromAny(data["balance"]); ok {
		result.balance = &v
	}
	if v, ok := floatFromAny(data["remaining"]); ok {
		result.remaining = &v
	}
	if result.remaining == nil {
		if quota, ok := data["quota"].(map[string]any); ok {
			if v, ok := floatFromAny(quota["remaining"]); ok {
				result.remaining = &v
			}
		}
	}
	if result.balance == nil {
		if v, ok := floatFromAny(data["total_available"]); ok {
			result.balance = &v
		} else if v, ok := floatFromAny(data["total_granted"]); ok {
			result.balance = &v
		}
	}
	if unit, ok := data["unit"].(string); ok {
		result.unit = strings.TrimSpace(unit)
	}
	if result.unit == "" {
		result.unit = "USD"
	}
	if result.balance == nil && result.remaining == nil {
		return nil, nil
	}
	return result, nil
}

func floatFromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func selectUpstreamBalanceRepresentative(accounts []Account, baseURL string) (*Account, bool) {
	candidates := make([]Account, 0)
	matched := false
	for _, account := range accounts {
		if normalizeUpstreamBalanceBaseURL(accountHealthBaseURL(&account)) != baseURL {
			continue
		}
		matched = true
		if strings.TrimSpace(account.GetCredential("api_key")) == "" {
			continue
		}
		candidates = append(candidates, account)
	}
	if len(candidates) == 0 {
		return nil, matched
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		ai := upstreamBalanceRepresentativeRank(candidates[i])
		aj := upstreamBalanceRepresentativeRank(candidates[j])
		if ai != aj {
			return ai < aj
		}
		return candidates[i].ID < candidates[j].ID
	})
	return &candidates[0], matched
}

func upstreamBalanceRepresentativeRank(account Account) int {
	if account.Status == StatusActive && account.Schedulable {
		return 0
	}
	if account.Status == StatusActive {
		return 1
	}
	return 2
}

type upstreamBalanceNewAPIEndpointPair struct {
	subscriptionEndpoint string
	usageEndpoint        string
}

func upstreamBalanceCandidateEndpoints(baseURL string) []string {
	root, apiRoot := upstreamBalanceRoots(baseURL)
	endpoints := []string{
		apiRoot + "/usage",
		root + "/dashboard/billing/credit_grants",
		apiRoot + "/dashboard/billing/credit_grants",
	}
	return dedupeUpstreamBalanceEndpoints(endpoints)
}

func upstreamBalanceNewAPIEndpointPairs(baseURL string) []upstreamBalanceNewAPIEndpointPair {
	root, apiRoot := upstreamBalanceRoots(baseURL)
	pairs := []upstreamBalanceNewAPIEndpointPair{
		{subscriptionEndpoint: root + "/dashboard/billing/subscription", usageEndpoint: root + "/dashboard/billing/usage"},
		{subscriptionEndpoint: apiRoot + "/dashboard/billing/subscription", usageEndpoint: apiRoot + "/dashboard/billing/usage"},
	}
	out := make([]upstreamBalanceNewAPIEndpointPair, 0, len(pairs))
	seen := map[string]struct{}{}
	for _, pair := range pairs {
		key := pair.subscriptionEndpoint + "\n" + pair.usageEndpoint
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, pair)
	}
	return out
}

func upstreamBalanceRoots(baseURL string) (root string, apiRoot string) {
	root = strings.TrimRight(baseURL, "/")
	apiRoot = root
	if !strings.HasSuffix(strings.ToLower(apiRoot), "/v1") {
		apiRoot += "/v1"
	}
	if strings.HasSuffix(strings.ToLower(root), "/v1") {
		root = strings.TrimRight(root[:len(root)-3], "/")
	}
	return root, apiRoot
}

func dedupeUpstreamBalanceEndpoints(endpoints []string) []string {
	out := make([]string, 0, len(endpoints))
	seen := map[string]struct{}{}
	for _, endpoint := range endpoints {
		if _, ok := seen[endpoint]; ok {
			continue
		}
		seen[endpoint] = struct{}{}
		out = append(out, endpoint)
	}
	return out
}

func normalizeUpstreamBalanceBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func validateUpstreamBalanceBaseURL(raw string) error {
	_, err := urlvalidator.ValidateHTTPURL(raw, true, urlvalidator.ValidationOptions{AllowPrivate: false})
	return err
}

func sanitizeUpstreamBalanceError(message string) string {
	message = upstreamBalanceAuthorizationValue.ReplaceAllString(message, "Authorization: Bearer ***")
	message = strings.TrimSpace(logredact.RedactText(message, "api_key", "key", "authorization", "cookie", "token", "session_key"))
	message = upstreamBalanceLooseSKPattern.ReplaceAllString(message, "sk-***")
	if message == "" {
		return ""
	}
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func ptrUpstreamBalanceTime(t time.Time) *time.Time {
	return &t
}
