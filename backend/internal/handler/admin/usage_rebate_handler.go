package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UsageRebateHandler struct {
	service *service.UsageRebateService
}

func NewUsageRebateHandler(rebateService *service.UsageRebateService) *UsageRebateHandler {
	return &UsageRebateHandler{service: rebateService}
}

func (h *UsageRebateHandler) ListPeriods(c *gin.Context) {
	periods, err := h.service.ListRecentPeriods(c.Request.Context(), 30)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to load usage rebate periods")
		return
	}
	response.Success(c, gin.H{
		"enabled": h.service.IsEnabled(c.Request.Context()),
		"rates":   service.UsageRebateRates(),
		"items":   periods,
	})
}

func (h *UsageRebateHandler) ListPeriodRewards(c *gin.Context) {
	periodID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || periodID <= 0 {
		response.BadRequest(c, "Invalid usage rebate period id")
		return
	}
	rewards, err := h.service.ListPeriodRewards(c.Request.Context(), periodID, 20)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to load usage rebate period rewards")
		return
	}
	response.Success(c, gin.H{"items": rewards})
}
