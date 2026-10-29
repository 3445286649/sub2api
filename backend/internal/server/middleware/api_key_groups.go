package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func selectAPIKeyRequestGroup(c *gin.Context, keys *service.APIKeyService, subscriptions *service.SubscriptionService, cfg *config.Config, key *service.APIKey, google bool) (*service.APIKey, bool) {
	if key == nil || len(key.ConfiguredGroupIDs()) < 2 || c.Request.URL.Path == "/v1/usage" ||
		c.Request.URL.Path == "/v1/sub2api/billing" || isAsyncImageTaskRead(c.Request.Method, c.Request.URL.Path) {
		return key, true
	}
	var models []string
	if model := groupModelAllowlistModelFromParams(c); model != "" {
		models = []string{model}
	} else if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
		var ok bool
		models, ok = groupModelAllowlistModelsFromBody(c)
		if !ok {
			return nil, false
		}
	}
	if len(models) == 0 && c.Query("model") != "" {
		models = []string{c.Query("model")}
	}
	kind := "token"
	if strings.Contains(c.Request.URL.Path, "/images/") {
		kind = "image"
	} else if strings.Contains(c.Request.URL.Path, "/videos") {
		kind = "video"
	}
	checkBilling := cfg != nil && cfg.RunMode != config.RunModeSimple && c.Request.Method != http.MethodGet && !strings.HasSuffix(c.Request.URL.Path, "/count_tokens")
	ctx := context.WithValue(c.Request.Context(), ctxkey.UserID, key.UserID)
	selected, err := keys.SelectRequestGroup(ctx, key, models, kind, google, checkBilling, subscriptions)
	if err != nil {
		if google {
			abortWithGoogleError(c, http.StatusServiceUnavailable, "No selected group is available for this request")
		} else {
			AbortWithError(c, http.StatusServiceUnavailable, "NO_AVAILABLE_API_KEY_GROUP", "No selected group is available for this request")
		}
		return nil, false
	}
	SetOpsFallbackAPIKey(c, selected)
	return selected, true
}
