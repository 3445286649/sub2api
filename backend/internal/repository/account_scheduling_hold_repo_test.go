package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFinalizeAccountSchedulingStateCombinesIndependentBlocks(t *testing.T) {
	now := time.Now()
	state := buildAccountSchedulingState(&service.Account{
		ID:                     7,
		UpdatedAt:              now,
		Status:                 service.StatusActive,
		Schedulable:            false,
		TempUnschedulableUntil: schedulingHoldTimePtr(now.Add(time.Minute)),
	}, now)
	state.ExternalHold = &service.AccountSchedulingExternalHold{Active: true}
	finalizeAccountSchedulingState(state)

	require.False(t, state.EffectiveSchedulable)
	require.Equal(t, []string{"manual_unschedulable", "temp_unschedulable", "external_hold"}, state.EffectiveReasonCodes)
}

func TestSchedulingStateProbeEnabledRequiresRunnablePlan(t *testing.T) {
	account := &service.Account{
		Status:              service.StatusActive,
		Schedulable:         true,
		HealthProbeEnabled:  true,
		HealthyProbeEnabled: true,
	}

	require.True(t, schedulingStateProbeEnabled(account, service.AccountHealthStatusHealthy, false))
	account.Schedulable = false
	require.False(t, schedulingStateProbeEnabled(account, service.AccountHealthStatusHealthy, false))
	account.Schedulable = true
	account.HealthyProbeEnabled = false
	require.False(t, schedulingStateProbeEnabled(account, service.AccountHealthStatusHealthy, false))
	require.True(t, schedulingStateProbeEnabled(account, service.AccountHealthStatusDegraded, false))
}

func TestSchedulingHoldExpiryDecisionIDIsStableAndBounded(t *testing.T) {
	lease := time.Date(2026, 7, 15, 16, 30, 0, 0, time.UTC)
	first := schedulingHoldExpiryDecisionID(749, "ops-749-20260715-001", lease)
	second := schedulingHoldExpiryDecisionID(749, "ops-749-20260715-001", lease)
	require.Equal(t, first, second)
	require.LessOrEqual(t, len(first), 64)
}

func TestSchedulingHoldCommandReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	t.Run("same command replays", func(t *testing.T) {
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT account_id, request_hash
		FROM account_scheduling_hold_events
		WHERE owner = $1 AND decision_id = $2
	`)).WithArgs(service.AccountSchedulingHoldOwner, "decision-1").
			WillReturnRows(sqlmock.NewRows([]string{"account_id", "request_hash"}).AddRow(int64(7), "hash"))
		replayed, err := schedulingHoldCommandReplay(context.Background(), tx, service.AccountSchedulingHoldOwner, "decision-1", 7, "hash")
		require.NoError(t, err)
		require.True(t, replayed)
		mock.ExpectRollback()
		require.NoError(t, tx.Rollback())
	})

	t.Run("different payload conflicts", func(t *testing.T) {
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
		require.NoError(t, err)
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT account_id, request_hash
		FROM account_scheduling_hold_events
		WHERE owner = $1 AND decision_id = $2
	`)).WithArgs(service.AccountSchedulingHoldOwner, "decision-2").
			WillReturnRows(sqlmock.NewRows([]string{"account_id", "request_hash"}).AddRow(int64(7), "old"))
		_, err = schedulingHoldCommandReplay(context.Background(), tx, service.AccountSchedulingHoldOwner, "decision-2", 7, "new")
		require.Equal(t, "HOLD_DECISION_CONFLICT", infraerrors.Reason(err))
		mock.ExpectRollback()
		require.NoError(t, tx.Rollback())
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func schedulingHoldTimePtr(value time.Time) *time.Time { return &value }
