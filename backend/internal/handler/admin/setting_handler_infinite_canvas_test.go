//go:build unit

package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestInfiniteCanvasSettingsPartialUpdate(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyInfiniteCanvasEnabled: "true"})
	rec := doUpdateSettings(t, h, map[string]any{"site_name": "Canvas host"}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyInfiniteCanvasEnabled])
	for _, enabled := range []bool{false, true} {
		rec = doUpdateSettings(t, h, map[string]any{"infinite_canvas_enabled": enabled}, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"infinite_canvas_enabled":`)
		require.Equal(t, enabled, repo.values[service.SettingKeyInfiniteCanvasEnabled] == "true")
	}
	rec = doUpdateSettings(t, h, map[string]any{"infinite_canvas_enabled": "yes"}, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyInfiniteCanvasEnabled])
}
