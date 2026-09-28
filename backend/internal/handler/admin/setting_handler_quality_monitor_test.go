//go:build unit

package admin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 服务端会去请求这个地址，保存时必须拦下非 HTTPS 与内网地址（SSRF 防护）。
func TestUpdateSettings_QualityMonitorURLValidation(t *testing.T) {
	for _, raw := range []string{"http://monitor.example.com", "https://127.0.0.1", "https://localhost", "https://10.0.0.8", "javascript:alert(1)"} {
		code := updateSettingsCodexStatus(t, map[string]any{"quality_monitor_url": raw})
		require.Equal(t, http.StatusBadRequest, code, raw)
	}

	for _, raw := range []string{"", "https://manxue.codeplan.codes", "https://manxue.codeplan.codes/"} {
		code := updateSettingsCodexStatus(t, map[string]any{"quality_monitor_url": raw})
		require.Equal(t, http.StatusOK, code, raw)
	}
}

func TestUpdateSettings_QualityMonitorGroupNameLength(t *testing.T) {
	long := make([]rune, 65)
	for i := range long {
		long[i] = '满'
	}
	code := updateSettingsCodexStatus(t, map[string]any{"quality_monitor_group_name": string(long)})
	require.Equal(t, http.StatusBadRequest, code)

	code = updateSettingsCodexStatus(t, map[string]any{"quality_monitor_group_name": "OpenAI 满血分组"})
	require.Equal(t, http.StatusOK, code)
}
