package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"

	"github.com/gin-gonic/gin"
)

func TestRegisterAccountRoutes_HealthEnhancementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Account: &adminhandler.AccountHandler{},
			OAuth:   &adminhandler.OAuthHandler{},
		},
	}

	registerAccountRoutes(admin, handlers)

	wantRoutes := map[string]bool{
		http.MethodPatch + " /api/v1/admin/accounts/:id/rate-multiplier":       false,
		http.MethodPost + " /api/v1/admin/accounts/:id/health/probe":           false,
		http.MethodPatch + " /api/v1/admin/accounts/:id/health/probe-settings": false,
		http.MethodGet + " /api/v1/admin/accounts/health/overview":             false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantRoutes[key]; ok {
			wantRoutes[key] = true
		}
	}
	for route, found := range wantRoutes {
		if !found {
			t.Fatalf("route %s was not registered", route)
		}
	}
}
