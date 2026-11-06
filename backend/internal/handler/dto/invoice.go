package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type InvoiceableOrder struct {
	ID                int64     `json:"id"`
	OutTradeNo        string    `json:"out_trade_no"`
	RechargeCode      string    `json:"recharge_code"`
	OrderType         string    `json:"order_type"`
	PaymentType       string    `json:"payment_type"`
	PayAmount         float64   `json:"pay_amount"`
	BonusAmount       float64   `json:"bonus_amount"`
	InvoiceableAmount float64   `json:"invoiceable_amount"`
	PaidAt            time.Time `json:"paid_at"`
	AlreadyInvoiced   bool      `json:"already_invoiced"`
	Excluded          bool      `json:"excluded"`
	InvoiceRequestID  *int64    `json:"invoice_request_id"`
}

type InvoiceableSummary struct {
	TotalInvoiceableAmount float64 `json:"total_invoiceable_amount"`
	InvoiceableCount       int     `json:"invoiceable_count"`
}

type InvoiceableOrdersResponse struct {
	Items   []InvoiceableOrder `json:"items"`
	Summary InvoiceableSummary `json:"summary"`
}

// InvoiceRequest 是用户可见的申请。身份证号只给打码值，管理员备注不返回。
type InvoiceRequest struct {
	ID                    int64              `json:"id"`
	InvoiceType           string             `json:"invoice_type"`
	Title                 string             `json:"title"`
	EnterpriseTaxNumber   *string            `json:"enterprise_tax_number"`
	EnterpriseAddress     *string            `json:"enterprise_address"`
	EnterprisePhone       *string            `json:"enterprise_phone"`
	EnterpriseBank        *string            `json:"enterprise_bank"`
	EnterpriseBankAccount *string            `json:"enterprise_bank_account"`
	IndividualIDNumber    *string            `json:"individual_id_number"`
	TotalAmount           float64            `json:"total_amount"`
	OrderCount            int                `json:"order_count"`
	Status                string             `json:"status"`
	ReceiverEmail         string             `json:"receiver_email"`
	InvoiceNumber         *string            `json:"invoice_number"`
	InvoiceFileURL        *string            `json:"invoice_file_url"`
	HasFile               bool               `json:"has_file"`
	InvoiceFileName       *string            `json:"invoice_file_name"`
	InvoiceFileSize       *int64             `json:"invoice_file_size"`
	EmailSentAt           *time.Time         `json:"email_sent_at"`
	RejectReason          *string            `json:"reject_reason"`
	CreatedAt             time.Time          `json:"created_at"`
	ProcessingAt          *time.Time         `json:"processing_at"`
	CompletedAt           *time.Time         `json:"completed_at"`
	RejectedAt            *time.Time         `json:"rejected_at"`
	Orders                []InvoiceableOrder `json:"orders,omitempty"`
	CategoryID            *int64             `json:"category_id"`
	CategoryName          *string            `json:"category_name"`
}

type AdminInvoiceRequest struct {
	InvoiceRequest
	UserID     int64   `json:"user_id"`
	UserEmail  string  `json:"user_email"`
	AdminNotes *string `json:"admin_notes"`
}

type InvoiceStatusCounts struct {
	Pending         int     `json:"pending"`
	Processing      int     `json:"processing"`
	Completed       int     `json:"completed"`
	Rejected        int     `json:"rejected"`
	OpenAmount      float64 `json:"open_amount"`
	CompletedAmount float64 `json:"completed_amount"`
	TodayNew        int     `json:"today_new"`
}

type InvoiceExclusion struct {
	ID         int64      `json:"id"`
	OrderID    int64      `json:"order_id"`
	OutTradeNo string     `json:"out_trade_no"`
	UserEmail  string     `json:"user_email"`
	PayAmount  float64    `json:"pay_amount"`
	PaidAt     *time.Time `json:"paid_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type InvoiceOrderState struct {
	Invoiced      bool    `json:"invoiced"`
	Excluded      bool    `json:"excluded"`
	RequestID     *int64  `json:"request_id"`
	RequestStatus *string `json:"request_status"`
	InvoiceNumber *string `json:"invoice_number"`
}

func InvoiceOrderStateFromService(state *service.InvoiceOrderState) *InvoiceOrderState {
	if state == nil {
		return nil
	}
	return &InvoiceOrderState{
		Invoiced:      state.Invoiced(),
		Excluded:      state.Excluded,
		RequestID:     state.RequestID,
		RequestStatus: state.RequestStatus,
		InvoiceNumber: state.InvoiceNumber,
	}
}

type AddInvoiceExclusionsResult struct {
	Added           []InvoiceExclusion `json:"added"`
	AlreadyExcluded []string           `json:"already_excluded"`
	NotFound        []string           `json:"not_found"`
	Unpaid          []string           `json:"unpaid"`
	Ambiguous       []string           `json:"ambiguous"`
}

func InvoiceableOrdersFromService(items []service.InvoiceableOrder) []InvoiceableOrder {
	out := make([]InvoiceableOrder, 0, len(items))
	for _, item := range items {
		out = append(out, InvoiceableOrder{
			ID:                item.ID,
			OutTradeNo:        item.OutTradeNo,
			RechargeCode:      item.RechargeCode,
			OrderType:         item.OrderType,
			PaymentType:       item.PaymentType,
			PayAmount:         item.PayAmount,
			BonusAmount:       item.BonusAmount,
			InvoiceableAmount: item.InvoiceableAmount,
			PaidAt:            item.PaidAt,
			AlreadyInvoiced:   item.AlreadyInvoiced,
			Excluded:          item.Excluded,
			InvoiceRequestID:  item.InvoiceRequestID,
		})
	}
	return out
}

func InvoiceableSummaryFromService(summary service.InvoiceableSummary) InvoiceableSummary {
	return InvoiceableSummary{
		TotalInvoiceableAmount: summary.TotalInvoiceableAmount,
		InvoiceableCount:       summary.InvoiceableCount,
	}
}

func InvoiceRequestFromService(req *service.InvoiceRequest) *InvoiceRequest {
	if req == nil {
		return nil
	}
	out := &InvoiceRequest{
		ID:                    req.ID,
		InvoiceType:           req.InvoiceType,
		Title:                 req.Title,
		EnterpriseTaxNumber:   req.EnterpriseTaxNumber,
		EnterpriseAddress:     req.EnterpriseAddress,
		EnterprisePhone:       req.EnterprisePhone,
		EnterpriseBank:        req.EnterpriseBank,
		EnterpriseBankAccount: req.EnterpriseBankAccount,
		IndividualIDNumber:    req.IDNumberMasked,
		TotalAmount:           req.TotalAmount,
		OrderCount:            req.OrderCount,
		Status:                req.Status,
		ReceiverEmail:         req.ReceiverEmail,
		InvoiceNumber:         req.InvoiceNumber,
		InvoiceFileURL:        req.InvoiceFileURL,
		EmailSentAt:           req.EmailSentAt,
		RejectReason:          req.RejectReason,
		CreatedAt:             req.CreatedAt,
		ProcessingAt:          req.ProcessingAt,
		CompletedAt:           req.CompletedAt,
		RejectedAt:            req.RejectedAt,
		CategoryID:            req.CategoryID,
		CategoryName:          req.CategoryName,
	}
	// 用户只在发票已开具后才看到文件，开票前管理员上传的草稿对用户不可见。
	if req.InvoiceFileKey != nil && req.Status == service.InvoiceStatusCompleted {
		out.HasFile = true
		out.InvoiceFileName = req.InvoiceFileName
		out.InvoiceFileSize = req.InvoiceFileSize
	}
	if len(req.Orders) > 0 {
		out.Orders = InvoiceableOrdersFromService(req.Orders)
	}
	return out
}

func InvoiceRequestsFromService(items []service.InvoiceRequest) []InvoiceRequest {
	out := make([]InvoiceRequest, 0, len(items))
	for i := range items {
		out = append(out, *InvoiceRequestFromService(&items[i]))
	}
	return out
}

func AdminInvoiceRequestFromService(req *service.InvoiceRequest) *AdminInvoiceRequest {
	if req == nil {
		return nil
	}
	out := &AdminInvoiceRequest{
		InvoiceRequest: *InvoiceRequestFromService(req),
		UserID:         req.UserID,
		UserEmail:      req.UserEmail,
		AdminNotes:     req.AdminNotes,
	}
	// 管理员在任何状态下都能看到已上传的文件，开票前可以核对。
	if req.InvoiceFileKey != nil {
		out.HasFile = true
		out.InvoiceFileName = req.InvoiceFileName
		out.InvoiceFileSize = req.InvoiceFileSize
	}
	// 详情接口解密后给完整身份证号，列表和解密失败时仍是打码值。
	if req.IDNumberPlain != nil {
		out.IndividualIDNumber = req.IDNumberPlain
	}
	return out
}

func AdminInvoiceRequestsFromService(items []service.InvoiceRequest) []AdminInvoiceRequest {
	out := make([]AdminInvoiceRequest, 0, len(items))
	for i := range items {
		out = append(out, *AdminInvoiceRequestFromService(&items[i]))
	}
	return out
}

func InvoiceStatusCountsFromService(stats *service.InvoiceStatusCounts) *InvoiceStatusCounts {
	if stats == nil {
		return nil
	}
	return &InvoiceStatusCounts{
		Pending:         stats.Pending,
		Processing:      stats.Processing,
		Completed:       stats.Completed,
		Rejected:        stats.Rejected,
		OpenAmount:      stats.OpenAmount,
		CompletedAmount: stats.CompletedAmount,
		TodayNew:        stats.TodayNew,
	}
}

func InvoiceExclusionsFromService(items []service.InvoiceExclusion) []InvoiceExclusion {
	out := make([]InvoiceExclusion, 0, len(items))
	for _, item := range items {
		out = append(out, InvoiceExclusion{
			ID:         item.ID,
			OrderID:    item.OrderID,
			OutTradeNo: item.OutTradeNo,
			UserEmail:  item.UserEmail,
			PayAmount:  item.PayAmount,
			PaidAt:     item.PaidAt,
			CreatedAt:  item.CreatedAt,
		})
	}
	return out
}

func AddInvoiceExclusionsResultFromService(result *service.AddInvoiceExclusionsResult) *AddInvoiceExclusionsResult {
	if result == nil {
		return nil
	}
	return &AddInvoiceExclusionsResult{
		Added:           InvoiceExclusionsFromService(result.Added),
		AlreadyExcluded: emptyIfNilStrings(result.AlreadyExcluded),
		NotFound:        emptyIfNilStrings(result.NotFound),
		Unpaid:          emptyIfNilStrings(result.Unpaid),
		Ambiguous:       emptyIfNilStrings(result.Ambiguous),
	}
}

func emptyIfNilStrings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
