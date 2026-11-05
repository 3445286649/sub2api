package admin

import (
	"io"
	"net/http"
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

type updateInvoiceStatusRequest struct {
	Status         string  `json:"status" binding:"required"`
	AdminNotes     *string `json:"admin_notes"`
	InvoiceNumber  string  `json:"invoice_number"`
	InvoiceFileURL string  `json:"invoice_file_url"`
	RejectReason   string  `json:"reject_reason"`
	SendEmail      bool    `json:"send_email"`
}

type addInvoiceExclusionsRequest struct {
	OrderNumbers []string `json:"order_numbers" binding:"required"`
}

func (h *InvoiceHandler) GetStats(c *gin.Context) {
	stats, err := h.invoiceService.GetStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.InvoiceStatusCountsFromService(stats))
}

func (h *InvoiceHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 200 {
		keyword = keyword[:200]
	}
	items, result, err := h.invoiceService.ListForAdmin(
		c.Request.Context(),
		pagination.PaginationParams{Page: page, PageSize: pageSize},
		service.InvoiceRequestListFilters{
			Status:      strings.TrimSpace(c.Query("status")),
			InvoiceType: strings.TrimSpace(c.Query("invoice_type")),
			Keyword:     keyword,
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, dto.AdminInvoiceRequestsFromService(items), result.Total, page, pageSize)
}

func (h *InvoiceHandler) GetByID(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	req, err := h.invoiceService.GetForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceRequestFromService(req))
}

func (h *InvoiceHandler) UpdateStatus(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	var req updateInvoiceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updated, err := h.invoiceService.UpdateStatus(c.Request.Context(), id, service.UpdateInvoiceStatusInput{
		Status:         req.Status,
		AdminNotes:     req.AdminNotes,
		InvoiceNumber:  req.InvoiceNumber,
		InvoiceFileURL: req.InvoiceFileURL,
		RejectReason:   req.RejectReason,
		SendEmail:      req.SendEmail,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceRequestFromService(updated))
}

// UploadFile 接收管理员上传的发票 PDF（multipart 字段名 file），重复上传会替换旧文件。
func (h *InvoiceHandler) UploadFile(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.InvoiceFileMaxBytes+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Invoice file is required (PDF, maximum 10 MB)")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, service.InvoiceFileMaxBytes+1))
	if err != nil {
		response.BadRequest(c, "Failed to read the uploaded file")
		return
	}
	updated, err := h.invoiceService.UploadFile(c.Request.Context(), id, header.Filename, data)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceRequestFromService(updated))
}

// DownloadFile 管理员下载已上传的发票文件，任何状态都可以，用于开票前核对。
func (h *InvoiceHandler) DownloadFile(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	name, data, err := h.invoiceService.DownloadFileForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PrivateFile(c, name, "application/pdf", data)
}

func (h *InvoiceHandler) SendEmail(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid invoice request ID")
	if !ok {
		return
	}
	req, err := h.invoiceService.SendEmail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceRequestFromService(req))
}

func (h *InvoiceHandler) GetOrderState(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid order ID")
	if !ok {
		return
	}
	state, err := h.invoiceService.GetOrderInvoiceState(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.InvoiceOrderStateFromService(state))
}

func (h *InvoiceHandler) ListExclusions(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 200 {
		keyword = keyword[:200]
	}
	items, result, err := h.invoiceService.ListExclusions(
		c.Request.Context(),
		pagination.PaginationParams{Page: page, PageSize: pageSize},
		keyword,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, dto.InvoiceExclusionsFromService(items), result.Total, page, pageSize)
}

func (h *InvoiceHandler) AddExclusions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req addInvoiceExclusionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.invoiceService.AddExclusions(c.Request.Context(), req.OrderNumbers, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AddInvoiceExclusionsResultFromService(result))
}

func (h *InvoiceHandler) DeleteExclusion(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid exclusion ID")
	if !ok {
		return
	}
	if err := h.invoiceService.DeleteExclusion(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}
