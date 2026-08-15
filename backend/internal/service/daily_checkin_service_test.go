package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCheckinMilestones(t *testing.T) {
	tests := []struct {
		count     int
		milestone *int
		remaining int
	}{
		{count: 0, milestone: intPointer(7), remaining: 7},
		{count: 7, milestone: intPointer(15), remaining: 8},
		{count: 29, milestone: intPointer(30), remaining: 1},
		{count: 30, milestone: nil, remaining: 0},
	}
	for _, tt := range tests {
		milestone, remaining := nextMilestone(tt.count, 30)
		require.Equal(t, tt.milestone, milestone)
		require.Equal(t, tt.remaining, remaining)
	}
}

func TestValidateDailyCheckinConfig(t *testing.T) {
	valid := DailyCheckinConfig{BaseReward: 0.13, CycleDays: 30, Milestone7: 2, Milestone15: 5, Milestone30: 8, RuleVersion: 1}
	require.NoError(t, validateDailyCheckinConfig(valid))
	valid.DailyBudget = -1
	require.Error(t, validateDailyCheckinConfig(valid))
}

func intPointer(v int) *int { return &v }
