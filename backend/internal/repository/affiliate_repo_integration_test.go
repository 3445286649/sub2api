//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func querySingleFloat(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) float64 {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value float64
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func querySingleInt(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) int {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value int
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func TestAffiliateRepository_ListInvitees_IncludesPointsQualificationStatuses(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)
	now := time.Now().UTC().Truncate(time.Second)

	_, err := client.ExecContext(txCtx, `UPDATE settings SET value=CASE key
		WHEN 'points_invite_threshold_amount' THEN '50'
		WHEN 'points_invite_reward_points' THEN '1'
		WHEN 'points_invite_window_days' THEN '30'
		WHEN 'points_invite_freeze_hours' THEN '168'
		ELSE value END
		WHERE key IN ('points_invite_threshold_amount','points_invite_reward_points','points_invite_window_days','points_invite_freeze_hours')`)
	require.NoError(t, err)

	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("points-status-inviter-%d@example.com", now.UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	_, err = repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)

	createInvitee := func(label string, registeredAt time.Time) *service.User {
		t.Helper()
		u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("points-status-%s-%d@example.com", label, now.UnixNano()), Username: label, PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		_, createErr := repo.EnsureUserAffiliate(txCtx, u.ID)
		require.NoError(t, createErr)
		bound, bindErr := repo.BindInviter(txCtx, u.ID, inviter.ID)
		require.NoError(t, bindErr)
		require.True(t, bound)
		_, createErr = client.ExecContext(txCtx, `UPDATE users SET created_at=$1 WHERE id=$2`, registeredAt, u.ID)
		require.NoError(t, createErr)
		_, createErr = client.ExecContext(txCtx, `UPDATE user_affiliates SET created_at=$1 WHERE user_id=$2`, registeredAt, u.ID)
		require.NoError(t, createErr)
		return u
	}

	insertOrder := func(u *service.User, amount float64, completedAt time.Time, orderType, status string, refundAmount float64, withBaseSnapshot bool) int64 {
		t.Helper()
		var id int64
		var snapshot any
		if withBaseSnapshot {
			snapshot = fmt.Sprintf(`{"base_recharge_amount":%g}`, amount)
		}
		outTradeNo := fmt.Sprintf("points-status-%d-%d", u.ID, completedAt.UnixNano())
		rows, queryErr := client.QueryContext(txCtx, `INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,fee_rate,recharge_code,out_trade_no,payment_type,payment_trade_no,order_type,status,refund_amount,expires_at,completed_at,client_ip,src_host,provider_snapshot) VALUES($1,$2,$3,$4,$4,0,$5,$5,'test','trade',$6,$7,$8,$9::timestamptz+INTERVAL '1 hour',$9::timestamptz,'127.0.0.1','local',$10::jsonb) RETURNING id`, u.ID, u.Email, u.Username, amount, outTradeNo, orderType, status, refundAmount, completedAt, snapshot)
		require.NoError(t, queryErr)
		require.True(t, rows.Next())
		require.NoError(t, rows.Scan(&id))
		require.NoError(t, rows.Close())
		return id
	}

	notRecharged := createInvitee("not-recharged", now.Add(-24*time.Hour))
	progressing := createInvitee("progressing", now.Add(-5*24*time.Hour))
	pending := createInvitee("pending", now.Add(-2*24*time.Hour))
	available := createInvitee("available", now.Add(-10*24*time.Hour))
	revoked := createInvitee("revoked", now.Add(-4*24*time.Hour))
	expired := createInvitee("expired", now.Add(-40*24*time.Hour))

	_ = notRecharged
	insertOrder(progressing, 100, now.Add(-2*time.Hour), "balance", "COMPLETED", 0, false)
	insertOrder(progressing, 30, now.Add(-time.Hour), "balance", "COMPLETED", 0, true)
	pendingOrderID := insertOrder(pending, 50, now.Add(-24*time.Hour), "balance", "COMPLETED", 0, true)
	availableOrderID := insertOrder(available, 50, now.Add(-8*24*time.Hour), "balance", "COMPLETED", 0, true)
	revokedOrderID := insertOrder(revoked, 50, now.Add(-3*24*time.Hour), "balance", "PARTIALLY_REFUNDED", 50, true)
	insertOrder(expired, 50, now.Add(-5*24*time.Hour), "balance", "COMPLETED", 0, true)

	_, err = client.ExecContext(txCtx, `INSERT INTO affiliate_point_awards(inviter_user_id,invitee_user_id,source_order_id,status,points,threshold_amount,qualifying_amount,qualification_window_days,freeze_hours,release_at,created_at,updated_at) VALUES
		($1,$2,$3,'pending',1,50,50,30,168,$4,$5,$5),
		($1,$6,$7,'available',1,50,50,30,168,$8,$9,$9),
		($1,$10,$11,'revoked',1,50,0,30,168,$12,$13,$13)`,
		inviter.ID, pending.ID, pendingOrderID, now.Add(6*24*time.Hour), now.Add(-24*time.Hour),
		available.ID, availableOrderID, now.Add(-24*time.Hour), now.Add(-8*24*time.Hour),
		revoked.ID, revokedOrderID, now.Add(4*24*time.Hour), now.Add(-3*24*time.Hour))
	require.NoError(t, err)
	_, err = client.ExecContext(txCtx, `UPDATE affiliate_point_awards SET released_at=$1 WHERE invitee_user_id=$2`, now.Add(-24*time.Hour), available.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(txCtx, `UPDATE affiliate_point_awards SET revoked_at=$1 WHERE invitee_user_id=$2`, now.Add(-24*time.Hour), revoked.ID)
	require.NoError(t, err)

	items, err := repo.ListInvitees(txCtx, inviter.ID, 100)
	require.NoError(t, err)
	byName := make(map[string]service.AffiliateInvitee, len(items))
	for _, item := range items {
		byName[item.Username] = item
	}

	require.Equal(t, "not_recharged", byName[notRecharged.Username].PointsStatus)
	require.Equal(t, "progressing", byName[progressing.Username].PointsStatus)
	require.InDelta(t, 30, byName[progressing.Username].QualifyingAmount, 1e-9, "legacy orders without base snapshots must be excluded")
	require.Equal(t, "pending", byName[pending.Username].PointsStatus)
	require.EqualValues(t, 1, byName[pending.Username].RewardPoints)
	require.WithinDuration(t, now.Add(6*24*time.Hour), *byName[pending.Username].ReleaseAt, time.Second)
	require.Equal(t, "available", byName[available.Username].PointsStatus)
	require.NotNil(t, byName[available.Username].ReleasedAt)
	require.Equal(t, "revoked", byName[revoked.Username].PointsStatus)
	require.Equal(t, "refund_below_threshold", byName[revoked.Username].PointsStatusReason)
	require.Equal(t, "expired", byName[expired.Username].PointsStatus)
	require.Equal(t, "qualification_window_expired", byName[expired.Username].PointsStatusReason)
	require.Zero(t, byName[expired.Username].QualifyingAmount, "recharges after the qualification deadline must be excluded")
	require.InDelta(t, 50, byName[expired.Username].ThresholdAmount, 1e-9)
	require.Equal(t, 30, byName[expired.Username].QualificationWindowDays)
	require.NotNil(t, byName[expired.Username].QualificationDeadline)
}

func TestAffiliateRepository_TransferQuotaToBalance_UsesClaimedQuotaBeforeClear(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-transfer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      5.5,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $3, NOW(), NOW())`, u.ID, affCode, 12.34)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.34, transferred, 1e-9)
	require.InDelta(t, 17.84, balance, 1e-9)

	affQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID)
	require.InDelta(t, 0.0, affQuota, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 17.84, persistedBalance, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'transfer'", u.ID)
	require.Equal(t, 1, ledgerCount)

	rows, err := client.QueryContext(txCtx, `
SELECT amount::double precision,
       balance_after::double precision,
       aff_quota_after::double precision,
       aff_frozen_quota_after::double precision,
       aff_history_quota_after::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND action = 'transfer'
LIMIT 1`, u.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "expected transfer ledger")
	var amount, balanceAfter, quotaAfter, frozenAfter, historyAfter float64
	require.NoError(t, rows.Scan(&amount, &balanceAfter, &quotaAfter, &frozenAfter, &historyAfter))
	require.InDelta(t, 12.34, amount, 1e-9)
	require.InDelta(t, 17.84, balanceAfter, 1e-9)
	require.InDelta(t, 0.0, quotaAfter, 1e-9)
	require.InDelta(t, 0.0, frozenAfter, 1e-9)
	require.InDelta(t, 12.34, historyAfter, 1e-9)
}

// TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction guards the
// cross-layer tx propagation invariant: when AccrueQuota is called with a ctx
// that already carries a transaction (via dbent.NewTxContext), repo.withTx
// must reuse that tx rather than opening a nested one. If this invariant
// breaks, AccrueQuota would commit independently and survive a rollback of
// the outer tx, which would violate payment_fulfillment's all-or-nothing
// semantics.
func TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction(t *testing.T) {
	ctx := context.Background()

	outerTx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err, "begin outer tx")
	// Defensive cleanup: if any require.* below fires before the explicit
	// Rollback, this prevents the tx from leaking until container teardown.
	// Rollback is idempotent at the driver level (extra rollback returns an
	// error we ignore).
	t.Cleanup(func() { _ = outerTx.Rollback() })
	client := outerTx.Client()
	txCtx := dbent.NewTxContext(ctx, outerTx)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	repo := NewAffiliateRepository(client, integrationDB)
	_, err = repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound, "invitee must bind to inviter")

	applied, err := repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 3.5, 0, nil)
	require.NoError(t, err)
	require.True(t, applied, "AccrueQuota must report applied=true")

	// Visible inside the outer tx.
	innerQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 3.5, innerQuota, 1e-9)

	// Roll back the outer tx; if AccrueQuota had opened its own inner tx and
	// committed it, the rows would still be visible to the global client.
	require.NoError(t, outerTx.Rollback())

	rows, err := integrationEntClient.QueryContext(ctx,
		"SELECT COUNT(*) FROM user_affiliates WHERE user_id IN ($1, $2)",
		inviter.ID, invitee.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var postRollbackCount int
	require.NoError(t, rows.Scan(&postRollbackCount))
	require.Equal(t, 0, postRollbackCount,
		"AccrueQuota must propagate the outer tx — found persisted rows after rollback")
}

func TestAffiliateRepository_AccrueQuotaFromRedeemCode_Idempotent(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-redeem-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-redeem-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	redeemCode, err := client.RedeemCode.Create().
		SetCode(fmt.Sprintf("SUBAFF%09d", time.Now().UnixNano()%1_000_000_000)).
		SetType(service.RedeemTypeSubscription).
		SetValue(100).
		SetAffiliateRebateBaseAmount(100).
		SetStatus(service.StatusUsed).
		Save(txCtx)
	require.NoError(t, err)

	applied, err := repo.AccrueQuotaFromRedeemCode(txCtx, inviter.ID, invitee.ID, 5, 0, redeemCode.ID)
	require.NoError(t, err)
	require.True(t, applied)

	applied, err = repo.AccrueQuotaFromRedeemCode(txCtx, inviter.ID, invitee.ID, 5, 0, redeemCode.ID)
	require.NoError(t, err)
	require.False(t, applied, "same redeem code must not accrue twice")

	quota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 5.0, quota, 1e-9)
	history := querySingleFloat(t, txCtx, client,
		"SELECT aff_history_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 5.0, history, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND source_user_id = $2 AND source_redeem_code_id = $3 AND action = 'accrue'",
		inviter.ID, invitee.ID, redeemCode.ID)
	require.Equal(t, 1, ledgerCount)
}

func TestAffiliateRepository_AccrueQuotaFromPaymentOrder_Idempotent(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-order-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-order-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(invitee.ID).
		SetUserEmail(invitee.Email).
		SetUserName(invitee.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("AFF-ORDER-%d", time.Now().UnixNano())).
		SetOutTradeNo(fmt.Sprintf("sub2_affiliate_order_%d", time.Now().UnixNano())).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus("completed").
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(txCtx)
	require.NoError(t, err)

	sourceOrderID := order.ID
	applied, err := repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 5, 0, &sourceOrderID)
	require.NoError(t, err)
	require.True(t, applied)

	applied, err = repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 5, 0, &sourceOrderID)
	require.NoError(t, err)
	require.False(t, applied, "same payment order must not accrue twice")

	quota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 5.0, quota, 1e-9)
	history := querySingleFloat(t, txCtx, client,
		"SELECT aff_history_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 5.0, history, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND source_user_id = $2 AND source_order_id = $3 AND action = 'accrue'",
		inviter.ID, invitee.ID, order.ID)
	require.Equal(t, 1, ledgerCount)
}

func TestAffiliateRepository_TransferQuotaToBalance_EmptyQuota(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-empty-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      3.21,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, 0, 0, NOW(), NOW())`, u.ID, affCode)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaEmpty)
	require.InDelta(t, 0.0, transferred, 1e-9)
	require.InDelta(t, 0.0, balance, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 3.21, persistedBalance, 1e-9)
}

// TestAffiliateRepository_AdminCustomCode covers the success path of admin
// invite-code rewrite + reset within a shared test transaction:
// - UpdateUserAffCode replaces aff_code, sets aff_code_custom=true, lookup works
// - the old code can no longer be found
// - ResetUserAffCode reverts aff_code_custom and assigns a new system-format code
//
// The conflict path (duplicate code → ErrAffiliateCodeTaken) lives in its own
// test because a unique-violation aborts the surrounding Postgres tx, which
// would poison subsequent assertions in the same transaction.
func TestAffiliateRepository_AdminCustomCode(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-custom-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	original, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.False(t, original.AffCodeCustom, "system-generated codes start as non-custom")
	originalCode := original.AffCode

	// Rewrite to a custom code
	customCode := fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, u.ID, customCode))

	updated, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, customCode, updated.AffCode)
	require.True(t, updated.AffCodeCustom)

	// Lookup by new custom code finds the user
	byCode, err := repo.GetAffiliateByCode(txCtx, customCode)
	require.NoError(t, err)
	require.Equal(t, u.ID, byCode.UserID)

	// Old system code should no longer match
	_, err = repo.GetAffiliateByCode(txCtx, originalCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)

	// Reset back to a fresh system code, clears custom flag
	newSysCode, err := repo.ResetUserAffCode(txCtx, u.ID)
	require.NoError(t, err)
	require.NotEqual(t, customCode, newSysCode)

	reset, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, newSysCode, reset.AffCode)
	require.False(t, reset.AffCodeCustom)

	// The old custom code is now free again
	_, err = repo.GetAffiliateByCode(txCtx, customCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)
}

// TestAffiliateRepository_AdminCustomCode_Conflict isolates the unique-violation
// path. PostgreSQL aborts the enclosing tx when a unique constraint fires, so
// this test must be the only assertion and run in its own tx — production
// callers each have their own outer tx, so this matches real behavior.
func TestAffiliateRepository_AdminCustomCode_Conflict(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	taker := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-taker-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	requester := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-req-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})

	takenCode := fmt.Sprintf("HOT%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, taker.ID, takenCode))

	// Now requester tries to grab the same code → conflict.
	err := repo.UpdateUserAffCode(txCtx, requester.ID, takenCode)
	require.ErrorIs(t, err, service.ErrAffiliateCodeTaken)
}

// TestAffiliateRepository_AdminRebateRate covers per-user exclusive rate
// set/clear and the Batch variant including NULL semantics.
func TestAffiliateRepository_AdminRebateRate(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u1 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-a@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})
	u2 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-b@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	// Set exclusive rate for u1
	rate := 42.5
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, &rate))

	got, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRatePercent)
	require.InDelta(t, 42.5, *got.AffRebateRatePercent, 1e-9)

	// Clear exclusive rate
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, nil))
	cleared, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.AffRebateRatePercent)

	// Batch set both users
	batchRate := 15.0
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, &batchRate))

	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.NotNil(t, v.AffRebateRatePercent)
		require.InDelta(t, 15.0, *v.AffRebateRatePercent, 1e-9)
	}

	// Batch clear
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, nil))
	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.Nil(t, v.AffRebateRatePercent)
	}
}

// TestAffiliateRepository_ListUsersWithCustomSettings verifies the admin list
// only includes users with at least one override applied.
func TestAffiliateRepository_ListUsersWithCustomSettings(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	// User without any custom config — should NOT appear in the list.
	plainEmail := fmt.Sprintf("affiliate-plain-%d@example.com", time.Now().UnixNano())
	uPlain := mustCreateUser(t, client, &service.User{
		Email: plainEmail, PasswordHash: "hash",
		Role: service.RoleUser, Status: service.StatusActive,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, uPlain.ID)
	require.NoError(t, err)

	// User with a custom code — should appear.
	uCode := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-codeonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	require.NoError(t, repo.UpdateUserAffCode(txCtx, uCode.ID, fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)))

	// User with only an exclusive rate — should appear.
	uRate := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rateonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	r := 33.3
	require.NoError(t, repo.SetUserRebateRate(txCtx, uRate.ID, &r))

	entries, total, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{
		Page: 1, PageSize: 100,
	})
	require.NoError(t, err)

	// Build a quick lookup to assert per-user attributes (other tests may have
	// inserted custom rows in the same DB; we only care about our 3).
	byUserID := make(map[int64]service.AffiliateAdminEntry, len(entries))
	for _, e := range entries {
		byUserID[e.UserID] = e
	}

	require.NotContains(t, byUserID, uPlain.ID, "users without overrides must not appear")

	codeEntry, ok := byUserID[uCode.ID]
	require.True(t, ok, "custom-code user missing from list")
	require.True(t, codeEntry.AffCodeCustom)
	require.Nil(t, codeEntry.AffRebateRatePercent)

	rateEntry, ok := byUserID[uRate.ID]
	require.True(t, ok, "custom-rate user missing from list")
	require.False(t, rateEntry.AffCodeCustom)
	require.NotNil(t, rateEntry.AffRebateRatePercent)
	require.InDelta(t, 33.3, *rateEntry.AffRebateRatePercent, 1e-9)

	require.GreaterOrEqual(t, total, int64(2), "total must include at least our 2 custom rows")
}
