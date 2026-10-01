//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const qualityMonitorHandlerFixture = `{
  "api_version": "v1",
  "source": "self",
  "question_version": "candy-shapes-21",
  "generated_at": "2026-09-28T06:20:00.000Z",
  "groups": [
    {"id": "channel-1", "name": "Pro尊享分组", "model": "gpt-6-astra", "reasoning_effort": "medium",
     "interval_seconds": 1200, "next_run_at": "2026-09-28T06:40:00.000Z", "running": false,
     "stats": {
       "total": 8, "valid": 7, "passed": 7, "incorrect": 0, "errors": 1, "pass_rate": 100.0,
       "latest_status": "passed", "latest_finished_at": "2026-09-28T06:15:04.000Z"
     },
     "records": [
       {"id": "r1", "status": "passed", "finished_at": "2026-09-28T05:55:04Z", "parsed_answer": 21, "input_tokens": 4625, "duration_ms": 23000, "attempts": 1},
       {"id": "r2", "status": "error", "finished_at": "2026-09-28T06:15:04Z", "parsed_answer": null, "duration_ms": 5000, "attempts": 2}
     ]}
  ]
}`

func callQualityMonitor(t *testing.T, h *SettingHandler) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/quality-monitor", nil)

	h.GetQualityMonitor(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	return resp.Data
}

func newQualityMonitorHandler(values map[string]string, fetch func(context.Context, string) ([]byte, error)) *SettingHandler {
	repo := &settingHandlerPublicRepoStub{values: values}
	h := NewSettingHandler(nil, "test")
	h.SetQualityMonitorService(service.NewQualityMonitorServiceForTest(repo, fetch))
	return h
}

func TestSettingHandler_GetQualityMonitor_Disabled(t *testing.T) {
	h := newQualityMonitorHandler(map[string]string{}, func(context.Context, string) ([]byte, error) {
		t.Fatal("未配置时不应请求上游")
		return nil, nil
	})

	data := callQualityMonitor(t, h)
	require.Equal(t, false, data["enabled"])
	require.Equal(t, false, data["available"])
	require.NotContains(t, data, "data")
}

func TestSettingHandler_GetQualityMonitor_Available(t *testing.T) {
	h := newQualityMonitorHandler(map[string]string{
		service.SettingKeyQualityMonitorURL: "https://monitor.example.com",
	}, func(context.Context, string) ([]byte, error) {
		return []byte(qualityMonitorHandlerFixture), nil
	})

	data := callQualityMonitor(t, h)
	require.Equal(t, true, data["enabled"])
	require.Equal(t, true, data["available"])

	snap := data["data"].(map[string]any)
	require.Equal(t, "https://monitor.example.com", snap["detail_url"])
	groups := snap["groups"].([]any)
	require.Len(t, groups, 1)
	require.Equal(t, "Pro尊享分组", groups[0].(map[string]any)["name"])

	// 只透出状态与时间，不外泄答案、记录编号与用量
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	for _, secret := range []string{"answer", "r1", "input_tokens", "question_version"} {
		require.NotContains(t, string(raw), secret)
	}
}

func TestSettingHandler_GetQualityMonitor_UnavailableKeepsLinkOnly(t *testing.T) {
	h := newQualityMonitorHandler(map[string]string{
		service.SettingKeyQualityMonitorURL: "https://monitor.example.com",
	}, func(context.Context, string) ([]byte, error) {
		return nil, errors.New("upstream down")
	})

	data := callQualityMonitor(t, h)
	require.Equal(t, true, data["enabled"])
	require.Equal(t, false, data["available"])
	require.Equal(t, "https://monitor.example.com", data["detail_url"])
	require.NotContains(t, data, "data")
}
