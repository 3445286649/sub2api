package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration223CapturesAffiliatePointsRulesAtBinding(t *testing.T) {
	content, err := FS.ReadFile("223_affiliate_points_rule_snapshots.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "points_rule_threshold_amount")
	require.Contains(t, sql, "FROM affiliate_point_awards award")
	require.Contains(t, sql, "points_rule_threshold_amount = 50")
	require.Contains(t, sql, "points_rule_freeze_hours = 168")
	require.Contains(t, sql, "capture_affiliate_points_rule_snapshot")
	require.Contains(t, sql, "BEFORE INSERT OR UPDATE OF inviter_id")
	require.Contains(t, sql, "points_invite_threshold_amount")
	require.Contains(t, sql, "user_affiliates_points_rule_complete")
}
