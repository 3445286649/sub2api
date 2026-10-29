package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) multiGroupModels(c *gin.Context, key *service.APIKey) {
	groups, err := h.apiKeyService.BillingGroups(c.Request.Context(), key)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Model list is unavailable")
		return
	}
	seen := make(map[string]struct{})
	models := make([]string, 0)
	for _, selected := range groups {
		group := selected.Group
		if !group.IsActive() {
			continue
		}
		var available []string
		if group.Platform == service.PlatformComposite {
			available = h.compositeAvailableModels(c.Request.Context(), &group.ID, "", true)
		} else {
			available = h.gatewayService.GetAvailableModels(c.Request.Context(), &group.ID, group.Platform)
		}
		if len(available) == 0 && (group.Platform == service.PlatformAnthropic ||
			group.Platform == service.PlatformOpenAI || group.Platform == service.PlatformGrok ||
			group.Platform == service.PlatformGemini || group.Platform == service.PlatformAntigravity) {
			available = defaultModelIDsForPlatform(group.Platform)
		}
		if group.ModelAllowlistEnabled() {
			available = group.ModelAllowlist.FilterForListing(available)
		}
		for _, model := range available {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if _, exists := seen[model]; exists {
				continue
			}
			seen[model] = struct{}{}
			models = append(models, model)
		}
	}
	platform := service.PlatformAnthropic
	if key.Group != nil {
		platform = key.Group.Platform
	}
	writeModelsList(c, platform, models)
}
