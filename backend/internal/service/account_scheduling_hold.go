package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AccountSchedulingHoldContractVersion = "2026-07-15"
	AccountSchedulingHoldOwner           = "upstreamops"

	AccountSchedulingHoldDefaultLease = 15 * time.Minute
	AccountSchedulingHoldMinimumLease = time.Minute
	AccountSchedulingHoldMaximumLease = time.Hour
	AccountSchedulingHoldMaximumTotal = 4 * time.Hour

	AccountSchedulingHoldExpiryBatch = 200
)

const (
	AccountSchedulingHoldReasonAuthInvalid         = "auth_invalid"
	AccountSchedulingHoldReasonQuotaExhausted      = "quota_exhausted"
	AccountSchedulingHoldReasonUpstreamUnreachable = "upstream_unreachable"
	AccountSchedulingHoldReasonSustained5xx        = "sustained_5xx"
	AccountSchedulingHoldReasonSustainedTTFT       = "sustained_ttft"
	AccountSchedulingHoldReasonManualApproved      = "manual_approved"
)

var (
	ErrInvalidHoldRequest        = infraerrors.BadRequest("INVALID_HOLD_REQUEST", "invalid scheduling hold request")
	ErrAccountStateDrift         = infraerrors.Conflict("ACCOUNT_STATE_DRIFT", "account state changed")
	ErrHoldDecisionConflict      = infraerrors.Conflict("HOLD_DECISION_CONFLICT", "scheduling hold decision conflicts with a previous command")
	ErrHoldReleaseConflict       = infraerrors.Conflict("HOLD_RELEASE_CONFLICT", "active scheduling hold differs from the expected hold")
	ErrManualSchedulingDisabled  = infraerrors.Conflict("MANUAL_SCHEDULING_DISABLED", "account is manually unschedulable")
	ErrCapacityGuardBlocked      = infraerrors.Conflict("CAPACITY_GUARD_BLOCKED", "scheduling hold would violate minimum capacity")
	ErrLeaseOutOfRange           = infraerrors.New(http.StatusUnprocessableEntity, "LEASE_OUT_OF_RANGE", "scheduling hold lease is outside the allowed range")
	ErrInvalidHoldReason         = infraerrors.New(http.StatusUnprocessableEntity, "INVALID_REASON_CODE", "unsupported scheduling hold reason")
	ErrSchedulingHoldUnavailable = infraerrors.ServiceUnavailable("SCHEDULING_HOLD_UNAVAILABLE", "scheduling hold service unavailable")

	accountSchedulingDecisionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)
	accountSchedulingHoldReasons       = map[string]struct{}{
		AccountSchedulingHoldReasonAuthInvalid:         {},
		AccountSchedulingHoldReasonQuotaExhausted:      {},
		AccountSchedulingHoldReasonUpstreamUnreachable: {},
		AccountSchedulingHoldReasonSustained5xx:        {},
		AccountSchedulingHoldReasonSustainedTTFT:       {},
		AccountSchedulingHoldReasonManualApproved:      {},
	}
)

type AccountSchedulingHoldCapabilities struct {
	ContractVersion               string `json:"contract_version"`
	ExternalHolds                 bool   `json:"external_holds"`
	ExternalHoldOwner             string `json:"external_hold_owner"`
	DefaultLeaseSeconds           int    `json:"default_lease_seconds"`
	MinimumLeaseSeconds           int    `json:"minimum_lease_seconds"`
	MaximumLeaseSeconds           int    `json:"maximum_lease_seconds"`
	MaximumCumulativeLeaseSeconds int    `json:"maximum_cumulative_lease_seconds"`
	CapacityGuard                 bool   `json:"capacity_guard"`
	OptimisticConcurrency         bool   `json:"optimistic_concurrency"`
	Idempotency                   bool   `json:"idempotency"`
	LeaseExpiry                   bool   `json:"lease_expiry"`
	ProbeWhileHeld                bool   `json:"probe_while_held"`
	SchedulerOutbox               bool   `json:"scheduler_outbox"`
}

type AccountSchedulingExternalHold struct {
	Owner      string    `json:"owner"`
	DecisionID string    `json:"decision_id"`
	ReasonCode string    `json:"reason_code"`
	Status     string    `json:"status"`
	LeaseUntil time.Time `json:"lease_until"`
	Active     bool      `json:"active"`
	Version    int64     `json:"version,omitempty"`
}

type AccountSchedulingHealthEvidence struct {
	NextProbeAt  *time.Time `json:"next_probe_at,omitempty"`
	ProbeEnabled bool       `json:"probe_enabled"`
}

type AccountSchedulingState struct {
	AccountID            int64                            `json:"account_id"`
	AccountUpdatedAt     time.Time                        `json:"account_updated_at"`
	ManualSchedulable    bool                             `json:"manual_schedulable"`
	InternalBlocked      bool                             `json:"internal_blocked"`
	InternalReasonCodes  []string                         `json:"internal_reason_codes"`
	ExternalHold         *AccountSchedulingExternalHold   `json:"external_hold,omitempty"`
	EffectiveSchedulable bool                             `json:"effective_schedulable"`
	EffectiveReasonCodes []string                         `json:"effective_reason_codes"`
	Health               *AccountSchedulingHealthEvidence `json:"health,omitempty"`
	IdempotentReplay     bool                             `json:"idempotent_replay,omitempty"`
}

type PutAccountSchedulingHoldRequest struct {
	DecisionID               string    `json:"decision_id"`
	ReasonCode               string    `json:"reason_code"`
	LeaseUntil               time.Time `json:"lease_until"`
	ExpectedAccountUpdatedAt time.Time `json:"expected_account_updated_at"`
}

type ReleaseAccountSchedulingHoldRequest struct {
	DecisionID             string `json:"decision_id"`
	ExpectedHoldDecisionID string `json:"expected_hold_decision_id"`
}

type AccountSchedulingHoldPut struct {
	AccountID                int64
	Owner                    string
	DecisionID               string
	ReasonCode               string
	LeaseUntil               time.Time
	ExpectedAccountUpdatedAt time.Time
	RequestHash              string
	MaximumCumulativeLease   time.Duration
}

type AccountSchedulingHoldRelease struct {
	AccountID              int64
	Owner                  string
	DecisionID             string
	ExpectedHoldDecisionID string
	RequestHash            string
}

type AccountSchedulingHoldRepository interface {
	GetSchedulingState(ctx context.Context, accountID int64, now time.Time) (*AccountSchedulingState, error)
	PutSchedulingHold(ctx context.Context, command AccountSchedulingHoldPut, now time.Time) (*AccountSchedulingState, error)
	ReleaseSchedulingHold(ctx context.Context, command AccountSchedulingHoldRelease, now time.Time) (*AccountSchedulingState, error)
	ExpireSchedulingHolds(ctx context.Context, owner string, now time.Time, limit int) ([]int64, error)
}

type AccountSchedulingHoldService struct {
	repo AccountSchedulingHoldRepository
	now  func() time.Time
}

func NewAccountSchedulingHoldService(repo AccountSchedulingHoldRepository) *AccountSchedulingHoldService {
	return &AccountSchedulingHoldService{repo: repo, now: time.Now}
}

func (s *AccountSchedulingHoldService) Capabilities() AccountSchedulingHoldCapabilities {
	return AccountSchedulingHoldCapabilities{
		ContractVersion:               AccountSchedulingHoldContractVersion,
		ExternalHolds:                 true,
		ExternalHoldOwner:             AccountSchedulingHoldOwner,
		DefaultLeaseSeconds:           int(AccountSchedulingHoldDefaultLease.Seconds()),
		MinimumLeaseSeconds:           int(AccountSchedulingHoldMinimumLease.Seconds()),
		MaximumLeaseSeconds:           int(AccountSchedulingHoldMaximumLease.Seconds()),
		MaximumCumulativeLeaseSeconds: int(AccountSchedulingHoldMaximumTotal.Seconds()),
		CapacityGuard:                 true,
		OptimisticConcurrency:         true,
		Idempotency:                   true,
		LeaseExpiry:                   true,
		ProbeWhileHeld:                true,
		SchedulerOutbox:               true,
	}
}

func (s *AccountSchedulingHoldService) GetState(ctx context.Context, accountID int64) (*AccountSchedulingState, error) {
	if accountID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	if s == nil || s.repo == nil {
		return nil, ErrSchedulingHoldUnavailable
	}
	return s.repo.GetSchedulingState(ctx, accountID, s.currentTime())
}

func (s *AccountSchedulingHoldService) Put(ctx context.Context, accountID int64, req PutAccountSchedulingHoldRequest) (*AccountSchedulingState, error) {
	if accountID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	if s == nil || s.repo == nil {
		return nil, ErrSchedulingHoldUnavailable
	}
	req.DecisionID = strings.TrimSpace(req.DecisionID)
	req.ReasonCode = strings.TrimSpace(req.ReasonCode)
	if !accountSchedulingDecisionIDPattern.MatchString(req.DecisionID) || req.ExpectedAccountUpdatedAt.IsZero() || req.LeaseUntil.IsZero() {
		return nil, ErrInvalidHoldRequest
	}
	if _, ok := accountSchedulingHoldReasons[req.ReasonCode]; !ok {
		return nil, ErrInvalidHoldReason
	}
	now := s.currentTime()
	duration := req.LeaseUntil.Sub(now)
	if duration < AccountSchedulingHoldMinimumLease || duration > AccountSchedulingHoldMaximumLease {
		return nil, ErrLeaseOutOfRange
	}
	command := AccountSchedulingHoldPut{
		AccountID:                accountID,
		Owner:                    AccountSchedulingHoldOwner,
		DecisionID:               req.DecisionID,
		ReasonCode:               req.ReasonCode,
		LeaseUntil:               req.LeaseUntil.UTC(),
		ExpectedAccountUpdatedAt: req.ExpectedAccountUpdatedAt.UTC(),
		MaximumCumulativeLease:   AccountSchedulingHoldMaximumTotal,
	}
	command.RequestHash = schedulingHoldRequestHash("put", accountID, command.DecisionID, command.ReasonCode, command.LeaseUntil.Format(time.RFC3339Nano), command.ExpectedAccountUpdatedAt.Format(time.RFC3339Nano))
	return s.repo.PutSchedulingHold(ctx, command, now)
}

func (s *AccountSchedulingHoldService) Release(ctx context.Context, accountID int64, req ReleaseAccountSchedulingHoldRequest) (*AccountSchedulingState, error) {
	if accountID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	if s == nil || s.repo == nil {
		return nil, ErrSchedulingHoldUnavailable
	}
	req.DecisionID = strings.TrimSpace(req.DecisionID)
	req.ExpectedHoldDecisionID = strings.TrimSpace(req.ExpectedHoldDecisionID)
	if !accountSchedulingDecisionIDPattern.MatchString(req.DecisionID) || !accountSchedulingDecisionIDPattern.MatchString(req.ExpectedHoldDecisionID) {
		return nil, ErrInvalidHoldRequest
	}
	command := AccountSchedulingHoldRelease{
		AccountID:              accountID,
		Owner:                  AccountSchedulingHoldOwner,
		DecisionID:             req.DecisionID,
		ExpectedHoldDecisionID: req.ExpectedHoldDecisionID,
	}
	command.RequestHash = schedulingHoldRequestHash("release", accountID, command.DecisionID, command.ExpectedHoldDecisionID)
	return s.repo.ReleaseSchedulingHold(ctx, command, s.currentTime())
}

func (s *AccountSchedulingHoldService) ExpireDue(ctx context.Context) ([]int64, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSchedulingHoldUnavailable
	}
	return s.repo.ExpireSchedulingHolds(ctx, AccountSchedulingHoldOwner, s.currentTime(), AccountSchedulingHoldExpiryBatch)
}

func (s *AccountSchedulingHoldService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func schedulingHoldRequestHash(parts ...any) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, fmt.Sprint(part))
	}
	payload := strings.Join(values, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
