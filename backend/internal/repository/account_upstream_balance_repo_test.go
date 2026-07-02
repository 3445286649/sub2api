//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountUpstreamBalanceRepositoryClaimRefreshUsesLeaseWithoutDueFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewAccountUpstreamBalanceRepository(db)
	now := time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE account_upstream_balance_snapshots
		SET claim_until = $3, status = $4, updated_at = $2
		WHERE base_url = $1
		  AND (claim_until IS NULL OR claim_until <= $2)
	`)).
		WithArgs("https://upstream.example.com", now, leaseUntil, service.UpstreamBalanceStatusChecking).
		WillReturnResult(sqlmock.NewResult(0, 1))

	claimed, err := repo.ClaimRefresh(context.Background(), "https://upstream.example.com", now, leaseUntil)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountUpstreamBalanceRepositoryClaimRefreshReturnsFalseWhenBusy(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewAccountUpstreamBalanceRepository(db)
	now := time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE account_upstream_balance_snapshots
		SET claim_until = $3, status = $4, updated_at = $2
		WHERE base_url = $1
		  AND (claim_until IS NULL OR claim_until <= $2)
	`)).
		WithArgs("https://upstream.example.com", now, leaseUntil, service.UpstreamBalanceStatusChecking).
		WillReturnResult(sqlmock.NewResult(0, 0))

	claimed, err := repo.ClaimRefresh(context.Background(), "https://upstream.example.com", now, leaseUntil)
	require.NoError(t, err)
	require.False(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

var _ service.AccountUpstreamBalanceRepository = (*accountUpstreamBalanceRepository)(nil)
