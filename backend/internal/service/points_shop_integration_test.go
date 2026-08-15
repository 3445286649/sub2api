//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPointsShopInviteAwardRedeemIdempotencyAndRefund(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("sub2api_points"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.Eventually(t, func() bool { return db.PingContext(ctx) == nil }, 30*time.Second, 250*time.Millisecond)
	require.NoError(t, repository.ApplyMigrations(ctx, db))

	svc := service.NewPointsShopService(db, nil)
	_, err = svc.UpdateConfig(ctx, service.PointsConfig{Enabled: true, InviteThresholdAmount: 50, InviteRewardPoints: 1, QualificationWindowDays: 30, FreezeHours: 0})
	require.NoError(t, err)

	inviterID := insertPointsTestUser(t, db, "inviter@example.test")
	inviteeID := insertPointsTestUser(t, db, "invitee@example.test")
	insertPointsAffiliate(t, db, inviterID, nil, "INVITER")
	insertPointsAffiliate(t, db, inviteeID, &inviterID, "INVITEE")
	_, err = svc.UpdateConfig(ctx, service.PointsConfig{Enabled: true, InviteThresholdAmount: 5, InviteRewardPoints: 2, QualificationWindowDays: 30, FreezeHours: 48})
	require.NoError(t, err)
	orderID := insertPointsPaymentOrder(t, db, inviteeID, 50, "COMPLETED")

	// markCompleted updates the database before invoking the points hook, while
	// the in-memory Ent object can still carry the pre-update status.
	order := &dbent.PaymentOrder{ID: orderID, UserID: inviteeID, Amount: 50, PayAmount: 50, OrderType: payment.OrderTypeBalance, Status: "RECHARGING", ProviderSnapshot: map[string]any{"base_recharge_amount": float64(50)}}
	require.NoError(t, svc.RecordPaymentCompletion(ctx, order))
	require.NoError(t, svc.RecordPaymentCompletion(ctx, order), "payment retries must not duplicate points")
	account, err := svc.GetAccount(ctx, inviterID)
	require.NoError(t, err)
	require.EqualValues(t, 1, account.AvailablePoints)
	require.EqualValues(t, 1, account.LifetimeEarned)
	var snapshotThreshold float64
	var snapshotReward int64
	var snapshotFreeze int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT threshold_amount,points,freeze_hours FROM affiliate_point_awards WHERE invitee_user_id=$1`, inviteeID).
		Scan(&snapshotThreshold, &snapshotReward, &snapshotFreeze))
	require.InDelta(t, 50, snapshotThreshold, 1e-9)
	require.EqualValues(t, 1, snapshotReward)
	require.Zero(t, snapshotFreeze)

	_, err = svc.UpdateConfig(ctx, service.PointsConfig{Enabled: true, InviteThresholdAmount: 50, InviteRewardPoints: 1, QualificationWindowDays: 30, FreezeHours: 0})
	require.NoError(t, err)

	legacyInviteeID := insertPointsTestUser(t, db, "legacy-invitee@example.test")
	insertPointsAffiliate(t, db, legacyInviteeID, &inviterID, "LEGACYINVITEE")
	legacyOrderID := insertLegacyPointsPaymentOrder(t, db, legacyInviteeID, 50, "COMPLETED")
	legacyOrder := &dbent.PaymentOrder{ID: legacyOrderID, UserID: legacyInviteeID, Amount: 50, PayAmount: 50, OrderType: payment.OrderTypeBalance, Status: "COMPLETED"}
	require.NoError(t, svc.RecordPaymentCompletion(ctx, legacyOrder))
	var legacyAwardCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM affiliate_point_awards WHERE invitee_user_id=$1`, legacyInviteeID).Scan(&legacyAwardCount))
	require.Zero(t, legacyAwardCount, "orders without a base recharge snapshot must not qualify")

	product, err := svc.CreateProduct(ctx, service.PointsProductInput{ProductType: "balance", Name: "Balance 5", PointsPrice: 1, BalanceAmount: 5, ForSale: true})
	require.NoError(t, err)
	first, err := svc.Redeem(ctx, inviterID, product.ID, "redeem-once")
	require.NoError(t, err)
	second, err := svc.Redeem(ctx, inviterID, product.ID, "redeem-once")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	var balance float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, inviterID).Scan(&balance))
	require.Equal(t, 5.0, balance)

	_, err = db.ExecContext(ctx, `UPDATE user_points_accounts SET available_points=1 WHERE user_id=$1`, inviterID)
	require.NoError(t, err)
	concurrentResults := make(chan error, 2)
	for _, key := range []string{"concurrent-a", "concurrent-b"} {
		go func(idempotencyKey string) {
			_, redeemErr := svc.Redeem(ctx, inviterID, product.ID, idempotencyKey)
			concurrentResults <- redeemErr
		}(key)
	}
	successes := 0
	for range 2 {
		if <-concurrentResults == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "row locking must allow only one concurrent redemption")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, inviterID).Scan(&balance))
	require.Equal(t, 10.0, balance)

	_, err = db.ExecContext(ctx, `UPDATE user_points_accounts SET available_points=1 WHERE user_id=$1`, inviterID)
	require.NoError(t, err)
	type redeemResult struct {
		order service.PointsShopOrder
		err   error
	}
	sameKeyResults := make(chan redeemResult, 2)
	for range 2 {
		go func() {
			redeemed, redeemErr := svc.Redeem(ctx, inviterID, product.ID, "concurrent-same-key")
			sameKeyResults <- redeemResult{order: redeemed, err: redeemErr}
		}()
	}
	firstConcurrent := <-sameKeyResults
	secondConcurrent := <-sameKeyResults
	require.NoError(t, firstConcurrent.err)
	require.NoError(t, secondConcurrent.err)
	require.Equal(t, firstConcurrent.order.ID, secondConcurrent.order.ID)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, inviterID).Scan(&balance))
	require.Equal(t, 15.0, balance)

	invitee2ID := insertPointsTestUser(t, db, "invitee2@example.test")
	insertPointsAffiliate(t, db, invitee2ID, &inviterID, "INVITEE2")
	_ = insertLegacyPointsPaymentOrder(t, db, invitee2ID, 50, "COMPLETED")
	order2ID := insertPointsPaymentOrder(t, db, invitee2ID, 50, "COMPLETED")
	order2 := &dbent.PaymentOrder{ID: order2ID, UserID: invitee2ID, Amount: 50, PayAmount: 50, OrderType: payment.OrderTypeBalance, Status: "COMPLETED", ProviderSnapshot: map[string]any{"base_recharge_amount": float64(50)}}
	require.NoError(t, svc.RecordPaymentCompletion(ctx, order2))
	require.NoError(t, db.QueryRowContext(ctx, `UPDATE payment_orders SET status='REFUNDED',refund_amount=50,refund_at=NOW() WHERE id=$1 RETURNING id`, order2ID).Scan(&order2ID))
	require.NoError(t, svc.HandlePaymentRefund(ctx, invitee2ID))
	account, err = svc.GetAccount(ctx, inviterID)
	require.NoError(t, err)
	require.Zero(t, account.AvailablePoints)
	require.Zero(t, account.FrozenPoints)
	require.Zero(t, account.DebtPoints)

	_, err = svc.UpdateConfig(ctx, service.PointsConfig{Enabled: true, InviteThresholdAmount: 50, InviteRewardPoints: 1, QualificationWindowDays: 30, FreezeHours: 168})
	require.NoError(t, err)
	invitee3ID := insertPointsTestUser(t, db, "invitee3@example.test")
	insertPointsAffiliate(t, db, invitee3ID, &inviterID, "INVITEE3")
	_ = insertLegacyPointsPaymentOrder(t, db, invitee3ID, 50, "COMPLETED")
	order3ID := insertPointsPaymentOrder(t, db, invitee3ID, 50, "COMPLETED")
	order3 := &dbent.PaymentOrder{ID: order3ID, UserID: invitee3ID, Amount: 50, PayAmount: 50, OrderType: payment.OrderTypeBalance, Status: "COMPLETED", ProviderSnapshot: map[string]any{"base_recharge_amount": float64(50)}}
	require.NoError(t, svc.RecordPaymentCompletion(ctx, order3))
	_, err = db.ExecContext(ctx, `UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=50,refund_at=NOW() WHERE id=$1`, order3ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE affiliate_point_awards SET release_at=NOW()-INTERVAL '1 second' WHERE invitee_user_id=$1`, invitee3ID)
	require.NoError(t, err)
	account, err = svc.GetAccount(ctx, inviterID)
	require.NoError(t, err)
	require.Zero(t, account.AvailablePoints, "maturity checks must not fall back to legacy order amounts")
	require.Zero(t, account.FrozenPoints)
	var maturedStatus string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM affiliate_point_awards WHERE invitee_user_id=$1`, invitee3ID).Scan(&maturedStatus))
	require.Equal(t, "revoked", maturedStatus)

	userOrders, err := svc.ListOrders(ctx, inviterID, false, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 3, userOrders.Total)
	require.Len(t, userOrders.Items, 3)
	otherUserOrders, err := svc.ListOrders(ctx, inviteeID, false, 1, 20)
	require.NoError(t, err)
	require.Zero(t, otherUserOrders.Total, "users must not see another user's points redemptions")
	require.Empty(t, otherUserOrders.Items)

	var awardCount, ledgerCount, orderCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM affiliate_point_awards`).Scan(&awardCount))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_points_ledger`).Scan(&ledgerCount))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM points_shop_orders`).Scan(&orderCount))
	require.Equal(t, 3, awardCount)
	require.Equal(t, 10, ledgerCount)
	require.Equal(t, 3, orderCount)
}

func insertPointsTestUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO users(email,password_hash,role,status,balance,concurrency,signup_source) VALUES($1,'test','user','active',0,5,'email') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertPointsAffiliate(t *testing.T, db *sql.DB, userID int64, inviterID *int64, code string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO user_affiliates(user_id,aff_code,inviter_id) VALUES($1,$2,$3)`, userID, code, inviterID)
	require.NoError(t, err)
}

func insertPointsPaymentOrder(t *testing.T, db *sql.DB, userID int64, amount float64, status string) int64 {
	t.Helper()
	var id int64
	outTradeNo := fmt.Sprintf("points-%d-%d", userID, time.Now().UnixNano())
	err := db.QueryRow(`INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,fee_rate,recharge_code,out_trade_no,payment_type,payment_trade_no,order_type,status,expires_at,completed_at,client_ip,src_host,provider_snapshot) VALUES($1,$2,'test',$3::numeric,$3::numeric,0,$4,$4,'test','trade','balance',$5,NOW()+INTERVAL '1 hour',NOW(),'127.0.0.1','local',jsonb_build_object('base_recharge_amount',$3::numeric)) RETURNING id`, userID, fmt.Sprintf("user-%d@example.test", userID), amount, outTradeNo, status).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertLegacyPointsPaymentOrder(t *testing.T, db *sql.DB, userID int64, amount float64, status string) int64 {
	t.Helper()
	var id int64
	outTradeNo := fmt.Sprintf("legacy-points-%d-%d", userID, time.Now().UnixNano())
	err := db.QueryRow(`INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,fee_rate,recharge_code,out_trade_no,payment_type,payment_trade_no,order_type,status,expires_at,completed_at,client_ip,src_host) VALUES($1,$2,'test',$3::numeric,$3::numeric,0,$4,$4,'test','trade','balance',$5,NOW()+INTERVAL '1 hour',NOW(),'127.0.0.1','local') RETURNING id`, userID, fmt.Sprintf("user-%d@example.test", userID), amount, outTradeNo, status).Scan(&id)
	require.NoError(t, err)
	return id
}
