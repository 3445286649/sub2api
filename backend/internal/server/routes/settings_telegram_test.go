package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTelegramSettingsRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminRoutes := router.Group("/api/v1/admin")
	adminRoutes.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	registerSettingsRoutes(adminRoutes, &handler.Handlers{Admin: &handler.AdminHandlers{Setting: &admin.SettingHandler{}}})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/settings/telegram"},
		{http.MethodPut, "/api/v1/admin/settings/telegram"},
		{http.MethodPost, "/api/v1/admin/settings/telegram/test"},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
		require.Equal(t, http.StatusUnauthorized, w.Code, tc.path)
	}
}
