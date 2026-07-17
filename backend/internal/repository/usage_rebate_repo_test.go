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

func TestUsageRebateUserPositionReturnsPrivateRankProgress(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM usage_logs ul.*ul\.billing_type=0 AND ul\.actual_cost > 0.*LEFT JOIN annotated target`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{
			"rank", "participant_count", "requests", "tokens", "spend_amount",
			"previous_spend", "top_20_spend",
		}).AddRow(21, 25, int64(7), int64(1200), "80.00000000", "85.00000000", "82.00000000"))

	repo := NewUsageRebateRepository(db)
	position, err := repo.GetUserPosition(context.Background(), time.Now().Add(-time.Hour), time.Now(), 21)
	require.NoError(t, err)
	require.NotNil(t, position.Rank)
	require.Equal(t, 21, *position.Rank)
	require.Equal(t, 25, position.ParticipantCount)
	require.False(t, position.Eligible)
	require.True(t, position.GapToPrevious.Equal(decimal.NewFromInt(5)))
	require.True(t, position.GapToTop20.Equal(decimal.NewFromInt(2)))
	require.True(t, position.EstimatedReward.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageRebateUserPositionHandlesNoEligibleSpend(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)LEFT JOIN annotated target`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{
			"rank", "participant_count", "requests", "tokens", "spend_amount",
			"previous_spend", "top_20_spend",
		}).AddRow(nil, 25, int64(0), int64(0), "0", nil, "82.00000000"))

	repo := NewUsageRebateRepository(db)
	position, err := repo.GetUserPosition(context.Background(), time.Now().Add(-time.Hour), time.Now(), 99)
	require.NoError(t, err)
	require.Nil(t, position.Rank)
	require.Equal(t, 25, position.ParticipantCount)
	require.False(t, position.Eligible)
	require.True(t, position.SpendAmount.IsZero())
	require.Nil(t, position.GapToPrevious)
	require.Nil(t, position.GapToTop20)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageRebateUserPositionEligibilityBoundaries(t *testing.T) {
	tests := []struct {
		name             string
		rank             int
		spend            string
		previousSpend    any
		wantRate         string
		wantReward       string
		wantPreviousGap  *string
		wantPreviousRank *int
	}{
		{name: "first place", rank: 1, spend: "100.00000000", wantRate: "10", wantReward: "10"},
		{name: "twentieth place", rank: 20, spend: "50.00000000", previousSpend: "51.00000000", wantRate: "2.5", wantReward: "1.25", wantPreviousGap: stringPointer("1"), wantPreviousRank: intPointer(19)},
		{name: "equal spend keeps zero gap", rank: 2, spend: "100.00000000", previousSpend: "100.00000000", wantRate: "9", wantReward: "9", wantPreviousGap: stringPointer("0"), wantPreviousRank: intPointer(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			mock.ExpectQuery(`(?s)LEFT JOIN annotated target`).
				WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{
					"rank", "participant_count", "requests", "tokens", "spend_amount",
					"previous_spend", "top_20_spend",
				}).AddRow(tt.rank, 25, int64(10), int64(2000), tt.spend, tt.previousSpend, "50.00000000"))

			repo := NewUsageRebateRepository(db)
			position, err := repo.GetUserPosition(context.Background(), time.Now().Add(-time.Hour), time.Now(), 7)
			require.NoError(t, err)
			require.True(t, position.Eligible)
			require.True(t, position.RebatePercent.Equal(decimal.RequireFromString(tt.wantRate)))
			require.True(t, position.EstimatedReward.Equal(decimal.RequireFromString(tt.wantReward)))
			require.Equal(t, tt.wantPreviousRank, position.PreviousRank)
			if tt.wantPreviousGap == nil {
				require.Nil(t, position.GapToPrevious)
			} else {
				require.True(t, position.GapToPrevious.Equal(decimal.RequireFromString(*tt.wantPreviousGap)))
			}
			require.Nil(t, position.GapToTop20)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func intPointer(value int) *int { return &value }

func stringPointer(value string) *string { return &value }

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
