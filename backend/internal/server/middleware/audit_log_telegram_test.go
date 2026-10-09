package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTelegramSettingsAuditNeverCapturesCredentialBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &auditCaptureRepository{}
	svc := service.NewAuditLogService(repo, nil)
	svc.Start()
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(svc)))
	handler := func(c *gin.Context) {
		var body map[string]any
		require.NoError(t, c.ShouldBindJSON(&body))
		require.Contains(t, body, "bot_token")
		c.Status(http.StatusOK)
	}
	router.PUT("/api/v1/admin/settings/telegram", handler)
	router.POST("/api/v1/admin/settings/telegram/test", handler)
	for _, method := range []string{http.MethodPut, http.MethodPost} {
		path := "/api/v1/admin/settings/telegram"
		if method == http.MethodPost {
			path += "/test"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(`{"bot_token":"telegram-secret-canary","chat_id":"-1001"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}
	svc.Stop()
	require.Len(t, repo.logs, 2)
	for _, entry := range repo.logs {
		require.NotContains(t, entry.RequestBody, "telegram-secret-canary")
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
	}
}
