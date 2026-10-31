//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCustomMenuOpenModesAndPersistence(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	for _, mode := range []string{"", "embed", "new_tab", "redirect"} {
		item := map[string]any{"id": "docs", "label": "Docs", "url": "https://docs.example.com/", "visibility": "user", "open_mode": mode, "enabled": false}
		rec := doUpdateSettings(t, h, map[string]any{"custom_menu_items": []any{item}}, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		stored := repo.values[service.SettingKeyCustomMenuItems]
		var items []dto.CustomMenuItem
		require.NoError(t, json.Unmarshal([]byte(stored), &items))
		require.Equal(t, mode, items[0].OpenMode)
		require.NotNil(t, items[0].Enabled)
		require.False(t, *items[0].Enabled)
		require.Empty(t, dto.ParseUserVisibleMenuItems(stored))
		rec = doUpdateSettings(t, h, map[string]any{"site_name": "keep menus"}, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, stored, repo.values[service.SettingKeyCustomMenuItems])
	}
	for _, item := range []map[string]any{
		{"id": "bad", "label": "Bad", "url": "https://example.com", "visibility": "user", "open_mode": "bad"},
		{"id": "bad", "label": "Bad", "url": "javascript:alert(1)", "visibility": "user"},
		{"id": "bad", "label": "Bad", "url": "https://user:pass@example.com/", "visibility": "user"},
		{"id": "bad", "label": "Bad", "url": "md:docs", "visibility": "user", "open_mode": "new_tab"},
	} {
		before := repo.values[service.SettingKeyCustomMenuItems]
		rec := doUpdateSettings(t, h, map[string]any{"custom_menu_items": []any{item}}, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Equal(t, before, repo.values[service.SettingKeyCustomMenuItems])
	}
}
