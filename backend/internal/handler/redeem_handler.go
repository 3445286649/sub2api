package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RedeemHandler handles redeem code-related requests
type RedeemHandler struct {
	redeemService *service.RedeemService
}

// NewRedeemHandler creates a new RedeemHandler
func NewRedeemHandler(redeemService *service.RedeemService) *RedeemHandler {
	return &RedeemHandler{
		redeemService: redeemService,
	}
}

// RedeemRequest represents the redeem code request payload
type RedeemRequest struct {
	Code string `json:"code" binding:"required"`
}

// RedeemResponse represents the redeem response
type RedeemResponse struct {
	Message         string   `json:"message"`
	Type            string   `json:"type"`
	Value           float64  `json:"value"`
	GroupName       string   `json:"group_name,omitempty"`
	ValidityDays    int      `json:"validity_days,omitempty"`
	QuotaResetScope string   `json:"quota_reset_scope,omitempty"`
	NewBalance      *float64 `json:"new_balance,omitempty"`
	NewConcurrency  *int     `json:"new_concurrency,omitempty"`
}

// Redeem handles redeeming a code
// POST /api/v1/redeem
func (h *RedeemHandler) Redeem(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req RedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.redeemService.Redeem(c.Request.Context(), subject.UserID, req.Code)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, buildRedeemResponse(result, c.GetHeader("Accept-Language")))
}

// GetHistory returns the user's redemption history
// GET /api/v1/redeem/history
func (h *RedeemHandler) GetHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Keep the legacy array response for clients that do not request pagination.
	if c.Request.URL.Query().Has("page") || c.Request.URL.Query().Has("page_size") {
		page, pageSize := 1, 20
		for name, target := range map[string]*int{"page": &page, "page_size": &pageSize} {
			if raw, exists := c.GetQuery(name); exists {
				value, err := strconv.Atoi(raw)
				if err != nil || value < 1 {
					response.BadRequest(c, "Invalid "+name)
					return
				}
				*target = value
			}
		}
		if pageSize > 100 {
			pageSize = 100
		}
		if page-1 > int(^uint(0)>>1)/pageSize {
			response.BadRequest(c, "Invalid page")
			return
		}
		codes, result, err := h.redeemService.GetUserHistoryPaginated(c.Request.Context(), subject.UserID, pagination.PaginationParams{Page: page, PageSize: pageSize})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		out := make([]dto.RedeemCode, 0, len(codes))
		for i := range codes {
			out = append(out, *dto.RedeemCodeFromService(&codes[i]).Localized(c.GetHeader("Accept-Language")))
		}
		response.Paginated(c, out, result.Total, page, pageSize)
		return
	}

	// Default limit is 25
	limit := 25

	codes, err := h.redeemService.GetUserHistory(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.RedeemCode, 0, len(codes))
	for i := range codes {
		out = append(out, *dto.RedeemCodeFromService(&codes[i]).Localized(c.GetHeader("Accept-Language")))
	}
	response.Success(c, out)
}

func buildRedeemResponse(result *service.RedeemCode, acceptLanguage ...string) RedeemResponse {
	resp := RedeemResponse{
		Message: buildRedeemSuccessMessage(result),
	}
	if result == nil {
		return resp
	}

	resp.Type = result.Type
	resp.Value = result.Value
	resp.ValidityDays = result.ValidityDays
	resp.QuotaResetScope = result.QuotaResetScope
	if result.Group != nil {
		resp.GroupName = result.Group.Name
		if len(acceptLanguage) > 0 {
			if localized := dto.RedeemCodeFromService(result).Localized(acceptLanguage[0]); localized.Group != nil {
				resp.GroupName = localized.Group.Name
			}
		}
	}
	return resp
}

func buildRedeemSuccessMessage(result *service.RedeemCode) string {
	if result == nil {
		return "Redeem succeeded"
	}
	switch result.Type {
	case service.RedeemTypeBalance:
		return "Balance recharged successfully"
	case service.RedeemTypeConcurrency:
		return "Concurrency updated successfully"
	case service.RedeemTypeSubscription:
		return "Subscription renewed successfully"
	case service.RedeemTypeSubscriptionQuotaReset:
		switch result.QuotaResetScope {
		case service.QuotaResetScopeDaily:
			return "Daily quota reset successfully"
		case service.QuotaResetScopeWeekly:
			return "Weekly quota reset successfully"
		case service.QuotaResetScopeMonthly:
			return "Monthly quota reset successfully"
		case service.QuotaResetScopeAll:
			return "All quota windows reset successfully"
		default:
			return "Quota reset successfully"
		}
	default:
		return "Redeem succeeded"
	}
}
