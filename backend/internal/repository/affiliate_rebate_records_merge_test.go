package repository

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateRebateRecordsPreserveSourcesAndNullableFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*LEFT JOIN users invitee.*LEFT JOIN redeem_codes`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	rows := sqlmock.NewRows([]string{
		"order_id", "trade_no", "source_type", "redeem_id", "redeem_code",
		"inviter_id", "inviter_email", "inviter_name", "invitee_id", "invitee_email", "invitee_name",
		"order_amount", "pay_amount", "rebate_base", "rebate", "payment_type", "status", "created_at",
	})
	now := time.Now()
	rows.AddRow(9, "order-9", "payment_order", 0, "", 1, "a@example.com", "a", 2, "b@example.com", "b", 10, 70, 10, 1, "alipay", "COMPLETED", now)
	rows.AddRow(nil, "", "redeem_code", 73, "test-redeem-code", 1, "a@example.com", "a", nil, "", "", 20, nil, 20, 2, "redeem_code", "used", now)
	rows.AddRow(nil, "", "", 0, "", 1, "a@example.com", "a", nil, "", "", nil, nil, 0, 3, "", "", now)
	mock.ExpectQuery(`(?s)SELECT po.id,.*LEFT JOIN redeem_codes.*LIMIT \$1 OFFSET \$2`).WithArgs(20, 0).WillReturnRows(rows)
	repo := &affiliateRepository{client: client}
	got, total, err := repo.ListAffiliateRebateRecords(context.Background(), service.AffiliateRecordFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, got, 3)
	require.EqualValues(t, 9, *got[0].OrderID)
	require.Equal(t, 70.0, *got[0].PayAmount)
	require.Nil(t, got[1].OrderID)
	require.Nil(t, got[1].InviteeID)
	require.Equal(t, "redeem_code", got[1].SourceType)
	require.EqualValues(t, 73, got[1].RedeemCodeID)
	require.Equal(t, "test-redeem-code", got[1].RedeemCode)
	require.Equal(t, 20.0, got[1].RebateBaseAmount)
	require.Equal(t, 20.0, *got[1].OrderAmount)
	require.Nil(t, got[1].PayAmount)
	require.Nil(t, got[2].OrderAmount)
	require.Empty(t, got[2].SourceType)
	require.Equal(t, 3.0, got[2].RebateAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}
