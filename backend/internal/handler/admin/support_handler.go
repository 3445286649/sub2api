package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type SupportHandler struct {
	supportService *service.SupportService
	settingService *service.SettingService
}

func NewSupportHandler(supportService *service.SupportService, settingService *service.SettingService) *SupportHandler {
	return &SupportHandler{supportService: supportService, settingService: settingService}
}

type createSupportMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type updateSupportTicketRequest struct {
	Status          *string `json:"status"`
	Category        *string `json:"category"`
	Priority        *string `json:"priority"`
	AssignedAdminID *int64  `json:"assigned_admin_id"`
}

type reopenSupportTicketRequest struct {
	Content string `json:"content"`
}

func (h *SupportHandler) List(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 200 {
		search = search[:200]
	}
	items, result, err := h.supportService.ListForAdmin(
		c.Request.Context(),
		pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortBy:    c.DefaultQuery("sort_by", "last_message_at"),
			SortOrder: c.DefaultQuery("sort_order", "desc"),
		},
		service.SupportTicketListFilters{
			Status:     strings.TrimSpace(c.Query("status")),
			Category:   strings.TrimSpace(c.Query("category")),
			Priority:   strings.TrimSpace(c.Query("priority")),
			Search:     search,
			UnreadOnly: parseBoolQuery(c.Query("unread_only")),
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, dto.SupportTicketsFromService(items), result.Total, page, pageSize)
}

func (h *SupportHandler) GetByID(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	ticket, err := h.supportService.GetForAdmin(c.Request.Context(), ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketFromService(ticket))
}

func (h *SupportHandler) ListMessages(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	items, err := h.supportService.ListMessagesForAdmin(
		c.Request.Context(),
		ticketID,
		parseSupportMessageFilters(c),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportMessagesFromService(items))
}

func (h *SupportHandler) CreateMessage(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	var req createSupportMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	message, err := h.supportService.AddAdminMessage(c.Request.Context(), subject.UserID, ticketID, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.SupportMessageFromService(message))
}

func (h *SupportHandler) MarkRead(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	if err := h.supportService.MarkReadForAdmin(c.Request.Context(), ticketID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *SupportHandler) Update(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	var req updateSupportTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var assigned **int64
	if req.AssignedAdminID != nil {
		value := req.AssignedAdminID
		assigned = &value
	}
	ticket, err := h.supportService.UpdateByAdmin(c.Request.Context(), ticketID, &service.UpdateSupportTicketInput{
		Status:          req.Status,
		Category:        req.Category,
		Priority:        req.Priority,
		AssignedAdminID: assigned,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketFromService(ticket))
}

func (h *SupportHandler) Close(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	ticket, err := h.supportService.CloseForAdmin(c.Request.Context(), subject.UserID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketFromService(ticket))
}

func (h *SupportHandler) Reopen(c *gin.Context) {
	if !h.ensureEnabled(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, ok := parsePositiveID(c, "id", "Invalid support ticket ID")
	if !ok {
		return
	}
	var req reopenSupportTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ticket, message, err := h.supportService.ReopenForAdmin(c.Request.Context(), subject.UserID, ticketID, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"ticket":  dto.SupportTicketFromService(ticket),
		"message": dto.SupportMessageFromService(message),
	})
}

func (h *SupportHandler) ensureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return true
	}
	if err := h.settingService.EnsureSupportTicketsEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}

func parsePositiveID(c *gin.Context, param string, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(param), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, message)
		return 0, false
	}
	return id, true
}

func parseSupportMessageFilters(c *gin.Context) service.SupportMessageListFilters {
	beforeID, _ := strconv.ParseInt(c.Query("before_id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	return service.SupportMessageListFilters{BeforeID: beforeID, Limit: limit}
}

func parseBoolQuery(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
