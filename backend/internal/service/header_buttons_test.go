//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHeaderButtonsSettingsRoundTrip(t *testing.T) {
	repo := &bmUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	buttons := []HeaderButton{{Action: "help", Name: "Help", Enabled: true}, {Action: "external", Name: "Status", URL: "https://status.example.com", Enabled: true}}
	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{HeaderButtons: buttons}))
	require.Equal(t, buttons, svc.parseSettings(repo.updates).HeaderButtons)
	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{}))
	_, written := repo.updates[SettingKeyHeaderButtons]
	require.False(t, written, "old clients must not overwrite header buttons")
	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{HeaderButtons: []HeaderButton{}}))
	require.Equal(t, "[]", repo.updates[SettingKeyHeaderButtons])
}

func TestHeaderButtonsRejectUnsafeSettings(t *testing.T) {
	for _, target := range []string{"javascript:alert(1)", "data:text/html,test", "//example.com", "https://user:pass@example.com", "https://", "https://example.com\\@evil.test"} {
		t.Run(target, func(t *testing.T) {
			repo := &bmUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})
			err := svc.UpdateSettings(context.Background(), &SystemSettings{HeaderButtons: []HeaderButton{{Action: "external", Name: "Link", URL: target, Enabled: true}}})
			require.Error(t, err)
			require.Empty(t, repo.updates)
		})
	}
	_, err := normalizeHeaderButtons([]HeaderButton{{Action: "help", Name: "Help"}, {Action: "help", Name: "Again"}})
	require.Error(t, err)
}
