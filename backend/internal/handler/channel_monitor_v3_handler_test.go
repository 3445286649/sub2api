package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type v3SettingsStub struct {
	service.SettingRepository
	enabled string
}

type fusionDirectoryRepo struct {
	v3MonitorStub
	selected []*service.ChannelMonitor
}

func (r *fusionDirectoryRepo) ListEnabled(context.Context) ([]*service.ChannelMonitor, error) {
	one, two := int64(1), int64(2)
	return []*service.ChannelMonitor{
		{ID: 1, Name: "allowed", Enabled: true, V3FusionEnabled: true, V3GroupID: &one, PrimaryModel: "primary", APIKey: "secret-one"},
		{ID: 2, Name: "outside scope", Enabled: true, V3FusionEnabled: true, V3GroupID: &two, PrimaryModel: "other", APIKey: "secret-two"},
	}, nil
}
func (*fusionDirectoryRepo) TryBeginProbe(context.Context, int64, bool) (func(), bool, error) {
	panic("GET must never probe")
}
func (r *fusionDirectoryRepo) ProbeBuckets(_ context.Context, cards []*service.ChannelMonitor, _ service.ChannelMonitorV2Filter) (map[int64][]service.ChannelMonitorProbeBucket, error) {
	r.selected = cards
	out := map[int64][]service.ChannelMonitorProbeBucket{}
	for _, m := range cards {
		out[m.ID] = []service.ChannelMonitorProbeBucket{{SuccessCount: 1}}
	}
	return out, nil
}

type fusionGroupScope struct{}

func (fusionGroupScope) GetAvailableGroups(context.Context, int64) ([]service.Group, error) {
	return []service.Group{{ID: 1}}, nil
}

func TestChannelMonitorV3FusionPermissionAndReadOnly(t *testing.T) {
	repo := &fusionDirectoryRepo{}
	h := NewChannelMonitorUserHandler(service.NewChannelMonitorService(repo, nil), service.NewSettingService(v3SettingsStub{enabled: "true"}, &config.Config{}), nil)
	h.apiKeyService = fusionGroupScope{}
	router := gin.New()
	router.GET("/cards", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.V3Cards(c)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/cards?range=90m&group_id=2", nil))
	require.Equal(t, 200, w.Code)
	require.Len(t, repo.selected, 1)
	require.Equal(t, int64(1), repo.selected[0].ID)
	require.NotContains(t, w.Body.String(), "secret")
	require.Equal(t, 1, strings.Count(w.Body.String(), `"probe_buckets"`))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/cards?range=unbounded", nil))
	require.Equal(t, 400, w.Code)
}

func (s v3SettingsStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{service.SettingKeyChannelMonitorEnabled: s.enabled, service.SettingKeyChannelMonitorMode: "v3"}, nil
}

type v3MonitorStub struct {
	service.ChannelMonitorRepository
}

func (v3MonitorStub) ListEnabled(context.Context) ([]*service.ChannelMonitor, error) {
	id := int64(8)
	return []*service.ChannelMonitor{{ID: 3, Name: "Configured card", Provider: "openai", V3GroupID: &id, APIKey: "private-value", Endpoint: "https://private.invalid", ExtraHeaders: map[string]string{"Authorization": "private-value"}}}, nil
}
func TestChannelMonitorV3PublicProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []string{"true", "false"} {
		settings := service.NewSettingService(v3SettingsStub{enabled: enabled}, &config.Config{})
		h := NewChannelMonitorUserHandler(service.NewChannelMonitorService(v3MonitorStub{}, nil), settings, nil)
		router := gin.New()
		router.GET("/cards", h.V3Cards)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/cards", nil))
		if enabled == "false" {
			require.Equal(t, 403, w.Code)
			continue
		}
		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), `"v3_group_id":8`)
		require.Contains(t, w.Body.String(), "Configured card")
		require.NotContains(t, w.Body.String(), "private")
		require.NotContains(t, w.Body.String(), "api_key")
	}
}
