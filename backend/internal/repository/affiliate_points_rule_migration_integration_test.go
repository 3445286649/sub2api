//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration223BackfillsOldInvitesAndSnapshotsNewBindings(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	suffix := time.Now().UnixNano()

	_, err := tx.ExecContext(ctx, `
ALTER TABLE user_affiliates DROP CONSTRAINT user_affiliates_points_rule_complete;
ALTER TABLE user_affiliates DROP CONSTRAINT user_affiliates_points_rule_values;
DROP TRIGGER trg_user_affiliates_points_rule_snapshot ON user_affiliates;`)
	require.NoError(t, err)

	insertUser := func(label string) int64 {
		t.Helper()
		var id int64
		err := tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,balance,concurrency,signup_source) VALUES($1,'test','user','active',0,5,'email') RETURNING id`,
			fmt.Sprintf("snapshot-%s-%d@example.test", label, suffix)).Scan(&id)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO user_affiliates(user_id,aff_code,inviter_id) VALUES($1,$2,NULL)`, id, fmt.Sprintf("SNAP%s%d", label, id))
		require.NoError(t, err)
		return id
	}

	inviterID := insertUser("inviter")
	legacyNoAwardID := insertUser("legacy")
	legacyAwardID := insertUser("awarded")
	_, err = tx.ExecContext(ctx, `UPDATE user_affiliates SET inviter_id=$1,
		points_rule_threshold_amount=NULL,points_rule_reward_points=NULL,
		points_rule_window_days=NULL,points_rule_freeze_hours=NULL,points_rule_version=NULL
		WHERE user_id IN ($2,$3)`, inviterID, legacyNoAwardID, legacyAwardID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO affiliate_point_awards(
		inviter_user_id,invitee_user_id,status,points,threshold_amount,qualifying_amount,
		qualification_window_days,freeze_hours,release_at
	) VALUES($1,$2,'pending',2,40,40,20,12,NOW()+INTERVAL '12 hours')`, inviterID, legacyAwardID)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, `UPDATE settings SET value=CASE key
		WHEN 'points_invite_threshold_amount' THEN '5'
		WHEN 'points_invite_reward_points' THEN '1'
		WHEN 'points_invite_window_days' THEN '30'
		WHEN 'points_invite_freeze_hours' THEN '48'
		ELSE value END
		WHERE key IN ('points_invite_threshold_amount','points_invite_reward_points','points_invite_window_days','points_invite_freeze_hours')`)
	require.NoError(t, err)

	migrationSQL, err := migrations.FS.ReadFile("223_affiliate_points_rule_snapshots.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	assertSnapshot := func(userID int64, threshold float64, reward int64, window, freeze int) {
		t.Helper()
		var gotThreshold float64
		var gotReward int64
		var gotWindow, gotFreeze, version int
		err := tx.QueryRowContext(ctx, `SELECT points_rule_threshold_amount,points_rule_reward_points,
			points_rule_window_days,points_rule_freeze_hours,points_rule_version
			FROM user_affiliates WHERE user_id=$1`, userID).
			Scan(&gotThreshold, &gotReward, &gotWindow, &gotFreeze, &version)
		require.NoError(t, err)
		require.InDelta(t, threshold, gotThreshold, 1e-9)
		require.Equal(t, reward, gotReward)
		require.Equal(t, window, gotWindow)
		require.Equal(t, freeze, gotFreeze)
		require.Equal(t, 1, version)
	}

	assertSnapshot(legacyNoAwardID, 50, 1, 30, 168)
	assertSnapshot(legacyAwardID, 40, 2, 20, 12)

	newInviteeID := insertUser("new")
	_, err = tx.ExecContext(ctx, `UPDATE user_affiliates SET inviter_id=$1 WHERE user_id=$2`, inviterID, newInviteeID)
	require.NoError(t, err)
	assertSnapshot(newInviteeID, 5, 1, 30, 48)

	_, err = tx.ExecContext(ctx, `UPDATE settings SET value='9' WHERE key='points_invite_threshold_amount'`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE user_affiliates SET updated_at=NOW() WHERE user_id=$1`, newInviteeID)
	require.NoError(t, err)
	assertSnapshot(newInviteeID, 5, 1, 30, 48)
}
