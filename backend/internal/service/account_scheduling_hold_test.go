package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type schedulingHoldRepoStub struct {
	put     *AccountSchedulingHoldPut
	release *AccountSchedulingHoldRelease
}

func (r *schedulingHoldRepoStub) GetSchedulingState(context.Context, int64, time.Time) (*AccountSchedulingState, error) {
	return &AccountSchedulingState{}, nil
}

func (r *schedulingHoldRepoStub) PutSchedulingHold(_ context.Context, command AccountSchedulingHoldPut, _ time.Time) (*AccountSchedulingState, error) {
	r.put = &command
	return &AccountSchedulingState{AccountID: command.AccountID}, nil
}

func (r *schedulingHoldRepoStub) ReleaseSchedulingHold(_ context.Context, command AccountSchedulingHoldRelease, _ time.Time) (*AccountSchedulingState, error) {
	r.release = &command
	return &AccountSchedulingState{AccountID: command.AccountID}, nil
}

func (r *schedulingHoldRepoStub) ExpireSchedulingHolds(context.Context, string, time.Time, int) ([]int64, error) {
	return nil, nil
}

func TestAccountSchedulingHoldCapabilitiesContract(t *testing.T) {
	svc := NewAccountSchedulingHoldService(&schedulingHoldRepoStub{})
	got := svc.Capabilities()
	require.Equal(t, AccountSchedulingHoldContractVersion, got.ContractVersion)
	require.Equal(t, AccountSchedulingHoldOwner, got.ExternalHoldOwner)
	require.Equal(t, 900, got.DefaultLeaseSeconds)
	require.Equal(t, 60, got.MinimumLeaseSeconds)
	require.Equal(t, 3600, got.MaximumLeaseSeconds)
	require.Equal(t, 14400, got.MaximumCumulativeLeaseSeconds)
	require.True(t, got.ExternalHolds)
	require.True(t, got.CapacityGuard)
	require.True(t, got.OptimisticConcurrency)
	require.True(t, got.Idempotency)
	require.True(t, got.LeaseExpiry)
	require.True(t, got.ProbeWhileHeld)
	require.True(t, got.SchedulerOutbox)
}

func TestAccountSchedulingHoldPutValidatesAndNormalizes(t *testing.T) {
	now := time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
	repo := &schedulingHoldRepoStub{}
	svc := NewAccountSchedulingHoldService(repo)
	svc.now = func() time.Time { return now }

	_, err := svc.Put(context.Background(), 749, PutAccountSchedulingHoldRequest{
		DecisionID:               " ops-749-001 ",
		ReasonCode:               AccountSchedulingHoldReasonSustainedTTFT,
		LeaseUntil:               now.Add(15 * time.Minute),
		ExpectedAccountUpdatedAt: now.Add(-time.Minute),
	})
	require.NoError(t, err)
	require.NotNil(t, repo.put)
	require.Equal(t, AccountSchedulingHoldOwner, repo.put.Owner)
	require.Equal(t, "ops-749-001", repo.put.DecisionID)
	require.Len(t, repo.put.RequestHash, 64)
	require.Equal(t, AccountSchedulingHoldMaximumTotal, repo.put.MaximumCumulativeLease)
}

func TestAccountSchedulingHoldPutRejectsInvalidReasonAndLease(t *testing.T) {
	now := time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
	svc := NewAccountSchedulingHoldService(&schedulingHoldRepoStub{})
	svc.now = func() time.Time { return now }

	_, err := svc.Put(context.Background(), 1, PutAccountSchedulingHoldRequest{
		DecisionID: "ops-1", ReasonCode: "model_not_found", LeaseUntil: now.Add(15 * time.Minute), ExpectedAccountUpdatedAt: now,
	})
	require.Equal(t, "INVALID_REASON_CODE", infraerrors.Reason(err))

	_, err = svc.Put(context.Background(), 1, PutAccountSchedulingHoldRequest{
		DecisionID: "ops-2", ReasonCode: AccountSchedulingHoldReasonManualApproved, LeaseUntil: now.Add(59 * time.Second), ExpectedAccountUpdatedAt: now,
	})
	require.Equal(t, "LEASE_OUT_OF_RANGE", infraerrors.Reason(err))
}

func TestAccountSchedulingHoldReleaseRequiresExpectedHold(t *testing.T) {
	repo := &schedulingHoldRepoStub{}
	svc := NewAccountSchedulingHoldService(repo)
	_, err := svc.Release(context.Background(), 7, ReleaseAccountSchedulingHoldRequest{DecisionID: "release-7"})
	require.Equal(t, "INVALID_HOLD_REQUEST", infraerrors.Reason(err))
	require.Nil(t, repo.release)
}
