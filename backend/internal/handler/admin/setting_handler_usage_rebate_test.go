package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageRebateSettingOmissionPreservesStoredState(t *testing.T) {
	var omitted UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.UsageRebateEnabled)
	require.True(t, resolveUsageRebateEnabled(omitted.UsageRebateEnabled, true))
	require.False(t, resolveUsageRebateEnabled(omitted.UsageRebateEnabled, false))

	var disabled UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"usage_rebate_enabled":false}`), &disabled))
	require.NotNil(t, disabled.UsageRebateEnabled)
	require.False(t, resolveUsageRebateEnabled(disabled.UsageRebateEnabled, true))
}
