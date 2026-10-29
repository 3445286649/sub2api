package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) multiGroupGeminiModels(c *gin.Context, key *service.APIKey) {
	groups, err := h.apiKeyService.BillingGroups(c.Request.Context(), key)
	if err != nil {
		googleError(c, http.StatusServiceUnavailable, "Unable to list models")
		return
	}
	seen := make(map[string]struct{})
	models := make([]gemini.Model, 0)
	for _, selected := range groups {
		group := selected.Group
		if !group.IsActive() {
			continue
		}
		platforms := []string{group.Platform}
		if group.Platform == service.PlatformComposite {
			platforms = []string{service.PlatformGemini, service.PlatformAntigravity}
		} else if group.Platform != service.PlatformGemini && group.Platform != service.PlatformAntigravity {
			continue
		}
		for _, platform := range platforms {
			available := h.gatewayService.GetAvailableModels(c.Request.Context(), &group.ID, platform)
			if len(available) == 0 && platform == service.PlatformGemini {
				for _, model := range gemini.DefaultModels() {
					available = append(available, model.Name)
				}
			}
			if platform == service.PlatformAntigravity && h.geminiCompatService != nil {
				if declared, err := h.geminiCompatService.AntigravityGeminiModelIDs(c.Request.Context(), &group.ID, false); err == nil {
					available = append(available, declared...)
				}
			}
			for _, id := range available {
				id = strings.TrimSpace(id)
				if id == "" || strings.HasPrefix(strings.TrimPrefix(id, "models/"), "claude-") ||
					(group.ModelAllowlistEnabled() && !group.ModelAllowlist.Allows(id)) {
					continue
				}
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				models = append(models, gemini.FallbackModel(id))
			}
		}
	}
	c.JSON(http.StatusOK, gemini.ModelsListResponse{Models: models})
}
