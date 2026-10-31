package handler

import (
	"net/http"
	"strings"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) VideoCapabilities(c *gin.Context) {
	key, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || key == nil || key.Group == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "API key with a bound group is required"}})
		return
	}
	model := strings.TrimSpace(c.Query("model"))
	if model == "" || len(model) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "model is required (max 256 bytes)"}})
		return
	}
	profiles, err := h.gatewayService.VideoCapabilities(c.Request.Context(), key.Group, model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Unable to load video capabilities"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"model": model, "profiles": profiles})
}
