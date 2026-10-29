package handler

import (
	"math"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type keyBillingGroup struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	Platform  string `json:"platform"`
	Status    string `json:"status"`
	keyBillingInfoResponse
}

type multiGroupKeyBillingInfo struct {
	Object            string            `json:"object"`
	SchemaVersion     int               `json:"schema_version"`
	BillingScope      string            `json:"billing_scope"`
	BillingMode       string            `json:"billing_mode"`
	Groups            []keyBillingGroup `json:"groups"`
	MinRateMultiplier float64           `json:"min_rate_multiplier"`
	MaxRateMultiplier float64           `json:"max_rate_multiplier"`
	ObservedAt        time.Time         `json:"observed_at"`
}

func (h *GatewayHandler) multiGroupKeyBilling(c *gin.Context, key *service.APIKey) {
	if h.apiKeyService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Billing information is unavailable")
		return
	}
	groups, err := h.apiKeyService.BillingGroups(c.Request.Context(), key)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Billing information is unavailable")
		return
	}
	if len(groups) == 0 {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "No permitted groups are assigned to this key")
		return
	}
	now := timezone.Now()
	entries := make([]keyBillingGroup, 0, len(groups))
	for _, groupKey := range groups {
		rate, ok := h.resolveKeyBillingRate(c, groupKey)
		if !ok {
			h.errorResponse(c, http.StatusInternalServerError, "api_error", "Billing information is unavailable")
			return
		}
		entries = append(entries, keyBillingGroup{
			GroupID: groupKey.Group.ID, GroupName: groupKey.Group.Name,
			Platform: groupKey.Group.Platform, Status: groupKey.Group.Status,
			keyBillingInfoResponse: buildKeyBillingInfo(groupKey, rate, now),
		})
	}
	minRate, maxRate := math.Inf(1), 0.0
	for _, entry := range entries {
		minRate = math.Min(minRate, entry.EffectiveRateMultiplier)
		maxRate = math.Max(maxRate, entry.EffectiveRateMultiplier)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, multiGroupKeyBillingInfo{
		Object: "sub2api.key_billing", SchemaVersion: 2, BillingScope: "token", BillingMode: "multi_group",
		Groups: entries, MinRateMultiplier: minRate, MaxRateMultiplier: maxRate, ObservedAt: now.UTC(),
	})
}
