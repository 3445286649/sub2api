package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type InvoiceHandler struct {
	invoiceService *service.InvoiceService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService}
}

type createInvoiceRequest struct {
	InvoiceType           string  `json:"invoice_type" binding:"required"`
	OrderIDs              []int64 `json:"order_ids" binding:"required"`
	ReceiverEmail         string  `json:"receiver_email" binding:"required"`
	EnterpriseName        string  `json:"enterprise_name"`
	EnterpriseTaxNumber   string  `json:"enterprise_tax_number"`
	EnterpriseAddress     string  `json:"enterprise_address"`
	EnterprisePhone       string  `json:"enterprise_phone"`
	EnterpriseBank        string  `json:"enterprise_bank"`
	EnterpriseBankAccount string  `json:"enterprise_bank_account"`
	IndividualName        string  `json:"individual_name"`
	IndividualIDNumber    string  `json:"individual_id_number"`
	CategoryID            *int64  `json:"category_id"`
}

func (h *InvoiceHandler) ListOrders(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	items, summary, err := h.invoiceService.ListInvoiceableOrders(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.InvoiceableOrdersResponse{
		Items:   dto.InvoiceableOrdersFromService(items),
		Summary: dto.InvoiceableSummaryFromService(summary),
	})
}

func (h *InvoiceHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req createInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	created, err := h.invoiceService.CreateRequest(c.Request.Context(), service.CreateInvoiceRequestInput{
		UserID:                subject.UserID,
		InvoiceType:           req.InvoiceType,
		OrderIDs:              req.OrderIDs,
		ReceiverEmail:         req.ReceiverEmail,
		EnterpriseName:        req.EnterpriseName,
		EnterpriseTaxNumber:   req.EnterpriseTaxNumber,
		EnterpriseAddress:     req.EnterpriseAddress,
		EnterprisePhone:       req.EnterprisePhone,
		EnterpriseBank:        req.EnterpriseBank,
		EnterpriseBankAccount: req.EnterpriseBankAccount,
		IndividualName:        req.IndividualName,
		IndividualIDNumber:    req.IndividualIDNumber,
		CategoryID:            req.CategoryID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.InvoiceRequestFromService(created))
}

func (h *InvoiceHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.invoiceService.ListForUser(
		c.Request.Context(),
		subject.UserID,
		pagination.PaginationParams{Page: page, PageSize: pageSize},
		strings.TrimSpace(c.Query("status")),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, dto.InvoiceRequestsFromService(items), result.Total, page, pageSize)
}

// DownloadFile 只有申请人本人、且发票已开具时才能下载。
func (h *InvoiceHandler) DownloadFile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	name, data, err := h.invoiceService.DownloadFileForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PrivateFile(c, name, "application/pdf", data)
}

func (h *InvoiceHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	req, err := h.invoiceService.GetForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.InvoiceRequestFromService(req))
}
