package repository

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepository_ServiceTierReceiptRoundTrip(t *testing.T) {
	for _, receipt := range []*service.ServiceTierDetails{nil, {
		GroupTier: "ultrafast", RequestedTier: "ultrafast", UpstreamTier: "fast", BillingTier: "standard", Source: "free_fast",
		CustomerCosts: &service.ServiceTierCustomerCosts{InputCost: .01, OutputCost: .02, TotalCost: .03},
	}} {
		db, mock := newSQLMock(t)
		repo := &usageLogRepository{sql: db}
		log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, Model: "gpt-6-astra", RequestID: "tier-receipt", ServiceTierDetails: receipt, CreatedAt: time.Now().UTC()}
		prepared := prepareUsageLogInsert(log)
		args := anySliceToDriverValues(prepared.args)
		mock.ExpectQuery("INSERT INTO usage_logs").WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(4), log.CreatedAt))
		inserted, err := repo.Create(context.Background(), log)
		require.NoError(t, err)
		require.True(t, inserted)
		columns := strings.Split(usageLogSelectColumns, ", ")
		values := append([]driver.Value{int64(4)}, args...)
		mock.ExpectQuery("SELECT .* FROM usage_logs WHERE id = \\$1").WithArgs(int64(4)).WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
		read, err := repo.GetByID(context.Background(), 4)
		require.NoError(t, err)
		require.Equal(t, receipt, read.ServiceTierDetails)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
