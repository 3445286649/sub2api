package admin

import (
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountSchedulingHoldHandler struct {
	service *service.AccountSchedulingHoldService
}

func NewAccountSchedulingHoldHandler(svc *service.AccountSchedulingHoldService) *AccountSchedulingHoldHandler {
	return &AccountSchedulingHoldHandler{service: svc}
}

func (h *AccountSchedulingHoldHandler) Capabilities(c *gin.Context) {
	if h == nil || h.service == nil {
		response.ErrorFrom(c, service.ErrSchedulingHoldUnavailable)
		return
	}
	response.Success(c, h.service.Capabilities())
}

func (h *AccountSchedulingHoldHandler) GetState(c *gin.Context) {
	accountID, ok := schedulingHoldAccountID(c)
	if !ok {
		return
	}
	if h == nil || h.service == nil {
		response.ErrorFrom(c, service.ErrSchedulingHoldUnavailable)
		return
	}
	state, err := h.service.GetState(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *AccountSchedulingHoldHandler) Put(c *gin.Context) {
	accountID, ok := schedulingHoldAccountID(c)
	if !ok {
		return
	}
	if h == nil || h.service == nil {
		response.ErrorFrom(c, service.ErrSchedulingHoldUnavailable)
		return
	}
	var req service.PutAccountSchedulingHoldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrInvalidHoldRequest)
		return
	}
	state, err := h.service.Put(c.Request.Context(), accountID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *AccountSchedulingHoldHandler) Release(c *gin.Context) {
	accountID, ok := schedulingHoldAccountID(c)
	if !ok {
		return
	}
	if h == nil || h.service == nil {
		response.ErrorFrom(c, service.ErrSchedulingHoldUnavailable)
		return
	}
	var req service.ReleaseAccountSchedulingHoldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrInvalidHoldRequest)
		return
	}
	state, err := h.service.Release(c.Request.Context(), accountID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func schedulingHoldAccountID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id"))
		return 0, false
	}
	return id, true
}
