package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestFindDuplicatePaymentOrderUSDTBSCTradeNos(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT payment_trade_no, COUNT\\(\\*\\) AS duplicate_count FROM payment_orders").
		WillReturnRows(sqlmock.NewRows([]string{"payment_trade_no", "duplicate_count"}).AddRow("0xduplicate", 2))

	duplicates, err := findDuplicatePaymentOrderUSDTBSCTradeNos(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, []string{"0xduplicate (count=2)"}, duplicates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPreparePaymentOrdersUSDTBSCTradeNoUniqueMigrationRejectsDuplicates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT payment_trade_no, COUNT\\(\\*\\) AS duplicate_count FROM payment_orders").
		WillReturnRows(sqlmock.NewRows([]string{"payment_trade_no", "duplicate_count"}).AddRow("0xduplicate", 2))

	err = preparePaymentOrdersUSDTBSCTradeNoUniqueMigration(context.Background(), db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "0xduplicate (count=2)")
	require.NoError(t, mock.ExpectationsWereMet())
}
