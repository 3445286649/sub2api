package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *UserHandler) ConfigureAccountRestrictions(repo service.UserAccountRestrictionRepository, invalidator service.APIKeyAuthCacheInvalidator) {
	h.accountRestrictionRepo = repo
	h.authCacheInvalidator = invalidator
}

func (h *UserHandler) accountRestrictionUser(c *gin.Context) (int64, bool) {
	// Account restrictions are full administrator operations, not delegated-key operations.
	if adminaccess.FromContext(c.Request.Context()) != nil {
		response.Error(c, http.StatusForbidden, "Full administrator access required")
		return 0, false
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return 0, false
	}
	if h.accountRestrictionRepo == nil || h.authCacheInvalidator == nil {
		response.Error(c, http.StatusServiceUnavailable, "Account restriction service unavailable")
		return 0, false
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return 0, false
	}
	return id, true
}

func (h *UserHandler) GetAccountRestrictions(c *gin.Context) {
	userID, ok := h.accountRestrictionUser(c)
	if !ok {
		return
	}
	restrictions, err := h.accountRestrictionRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"restrictions": restrictions})
}

func (h *UserHandler) UpdateAccountRestrictions(c *gin.Context) {
	userID, ok := h.accountRestrictionUser(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	var req struct {
		AccountIDs *[]int64 `json:"account_ids"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if err := c.ShouldBindJSON(&req); err != nil || req.AccountIDs == nil || len(*req.AccountIDs) > 1000 {
		response.BadRequest(c, "account_ids must be an array of at most 1000 account IDs (use [] to clear)")
		return
	}
	if err := h.accountRestrictionRepo.Replace(c.Request.Context(), userID, groupID, *req.AccountIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.authCacheInvalidator.InvalidateAuthCacheByUserID(c.Request.Context(), userID)
	response.Success(c, gin.H{"account_ids": *req.AccountIDs})
}
