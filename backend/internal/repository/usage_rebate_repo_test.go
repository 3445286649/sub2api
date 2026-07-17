package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUsageRebateLeaderboardUsesBalanceBillingAndOwnSpend(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM usage_logs ul.*ul\.billing_type=0 AND ul\.actual_cost > 0`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "requests", "tokens", "spend_amount", "rank"}).
			AddRow(int64(11), "alice", int64(4), int64(1000), "36.00000000", 1).
			AddRow(int64(12), "bob", int64(3), int64(900), "20.00000000", 2))

	repo := NewUsageRebateRepository(db)
	items, err := repo.GetLeaderboard(context.Background(), time.Now().Add(-time.Hour), time.Now(), 11, 20)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.True(t, items[0].EstimatedReward.Equal(decimal.RequireFromString("3.60000000")))
	require.True(t, items[1].EstimatedReward.Equal(decimal.RequireFromString("1.80000000")))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageRebateCreditCommitAmbiguityIsFrozen(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, status, reward_amount").WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "status", "reward_amount"}).
			AddRow(int64(7), service.UsageRebateRewardStatusPending, "3.60000000"))
	mock.ExpectQuery("SELECT balance FROM users").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("10.00000000"))
	mock.ExpectQuery("UPDATE users").WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("13.60000000"))
	mock.ExpectExec("UPDATE usage_rebate_rewards").WithArgs(int64(9), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("connection reset after commit"))

	repo := NewUsageRebateRepository(db)
	userID, credited, err := repo.CreditReward(context.Background(), 9)
	require.Equal(t, int64(7), userID)
	require.False(t, credited)
	require.ErrorIs(t, err, service.ErrUsageRebateCommitUnknown)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageRebateAlreadyCreditedNeverAddsBalanceAgain(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, status, reward_amount").WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "status", "reward_amount"}).
			AddRow(int64(7), service.UsageRebateRewardStatusCredited, "3.60000000"))
	mock.ExpectCommit()

	repo := NewUsageRebateRepository(db)
	userID, credited, err := repo.CreditReward(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, int64(7), userID)
	require.False(t, credited)
	require.NoError(t, mock.ExpectationsWereMet())
}
