package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	InvoiceTypeEnterprise = "enterprise"
	InvoiceTypeIndividual = "individual"

	InvoiceStatusPending    = "pending"
	InvoiceStatusProcessing = "processing"
	InvoiceStatusCompleted  = "completed"
	InvoiceStatusRejected   = "rejected"
)

var (
	ErrInvoiceRequestNotFound     = infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	ErrInvoiceExclusionNotFound   = infraerrors.NotFound("INVOICE_EXCLUSION_NOT_FOUND", "invoice exclusion not found")
	ErrInvoiceNoOrders            = infraerrors.BadRequest("INVOICE_NO_ORDERS", "select at least one order")
	ErrInvoiceTooManyOrders       = infraerrors.BadRequest("INVOICE_TOO_MANY_ORDERS", "too many orders in one request")
	ErrInvoiceOrderNotInvoiceable = infraerrors.Conflict("ORDER_NOT_INVOICEABLE", "an order was already invoiced or cannot be invoiced")
	ErrInvoiceTypeInvalid         = infraerrors.BadRequest("INVALID_INVOICE_TYPE", "invoice type is invalid")
	ErrInvoiceTitleRequired       = infraerrors.BadRequest("INVOICE_TITLE_REQUIRED", "invoice title is required")
	ErrInvoiceFieldTooLong        = infraerrors.BadRequest("INVOICE_FIELD_TOO_LONG", "an invoice field is too long")
	ErrInvoiceTaxNumberInvalid    = infraerrors.BadRequest("INVALID_TAX_NUMBER", "tax number is invalid")
	ErrInvoiceIDNumberInvalid     = infraerrors.BadRequest("INVALID_ID_NUMBER", "id number is invalid")
	ErrInvoiceBankAccountInvalid  = infraerrors.BadRequest("INVALID_BANK_ACCOUNT", "bank account is invalid")
	ErrInvoiceEmailInvalid        = infraerrors.BadRequest("INVALID_RECEIVER_EMAIL", "receiver email is invalid")
	ErrInvoiceStatusTransition    = infraerrors.Conflict("INVALID_STATUS_TRANSITION", "invoice request cannot move to this status")
	ErrInvoiceNumberRequired      = infraerrors.BadRequest("INVOICE_NUMBER_REQUIRED", "invoice number is required")
	ErrInvoiceFileURLInvalid      = infraerrors.BadRequest("INVOICE_FILE_URL_INVALID", "invoice file url must be http or https")
	ErrInvoiceRejectReason        = infraerrors.BadRequest("INVOICE_REJECT_REASON_REQUIRED", "reject reason is required")
	ErrInvoiceEmailNotCompleted   = infraerrors.Conflict("CANNOT_SEND_EMAIL", "invoice is not issued yet")
	ErrInvoiceEmailFailed         = infraerrors.ServiceUnavailable("INVOICE_EMAIL_FAILED", "failed to send invoice email")
	ErrInvoiceExclusionEmpty      = infraerrors.BadRequest("INVOICE_EXCLUSION_EMPTY", "enter at least one order number")
	ErrInvoiceExclusionTooMany    = infraerrors.BadRequest("INVOICE_EXCLUSION_TOO_MANY", "too many order numbers in one submission")
	ErrOrderInvoicedRefundBlocked = infraerrors.Conflict("ORDER_INVOICED_REFUND_BLOCKED", "this order has been invoiced and cannot be refunded by the user")
)

// InvoiceOrderRow 是 payment_orders 上与开票相关的字段，加上占用与排除情况。
// 是否可开票、可开多少由 service 判断，列表和提交共用同一套规则。
type InvoiceOrderRow struct {
	ID               int64
	OutTradeNo       string
	RechargeCode     string
	OrderType        string
	PaymentType      string
	Status           string
	Amount           float64
	PayAmount        float64
	BonusAmount      float64
	RefundAmount     float64
	Currency         string
	PaidAt           time.Time
	InvoiceRequestID *int64
	Excluded         bool
}

// InvoiceableOrder 是展示给用户的订单。AlreadyInvoiced 同时覆盖申请占用和手动排除。
type InvoiceableOrder struct {
	ID                int64
	OutTradeNo        string
	RechargeCode      string
	OrderType         string
	PaymentType       string
	PayAmount         float64
	BonusAmount       float64
	InvoiceableAmount float64
	PaidAt            time.Time
	AlreadyInvoiced   bool
	Excluded          bool
	InvoiceRequestID  *int64
}

type InvoiceableSummary struct {
	TotalInvoiceableAmount float64
	InvoiceableCount       int
}

type CreateInvoiceRequestInput struct {
	UserID                int64
	InvoiceType           string
	OrderIDs              []int64
	EnterpriseName        string
	EnterpriseTaxNumber   string
	EnterpriseAddress     string
	EnterprisePhone       string
	EnterpriseBank        string
	EnterpriseBankAccount string
	IndividualName        string
	IndividualIDNumber    string
	ReceiverEmail         string
}

type InvoiceRequest struct {
	ID                    int64
	UserID                int64
	UserEmail             string
	InvoiceType           string
	Title                 string
	EnterpriseTaxNumber   *string
	EnterpriseAddress     *string
	EnterprisePhone       *string
	EnterpriseBank        *string
	EnterpriseBankAccount *string
	// IDNumberEncrypted 只在 service 内部使用，不进响应。
	IDNumberEncrypted *string
	IDNumberMasked    *string
	// IDNumberPlain 只有管理端详情才会解密填充。
	IDNumberPlain  *string
	TotalAmount    float64
	OrderCount     int
	Status         string
	ReceiverEmail  string
	InvoiceNumber  *string
	InvoiceFileURL *string
	// 管理员上传的 PDF 的元数据。Key 是磁盘上的存储名，只在 service 内部使用，不进响应。
	InvoiceFileName *string
	InvoiceFileKey  *string
	InvoiceFileSize *int64
	EmailSentAt     *time.Time
	AdminNotes      *string
	RejectReason    *string
	CreatedAt       time.Time
	ProcessingAt    *time.Time
	CompletedAt     *time.Time
	RejectedAt      *time.Time
	Orders          []InvoiceableOrder
}

type InvoiceRequestListFilters struct {
	UserID      int64
	Status      string
	InvoiceType string
	Keyword     string
}

type InvoiceStatusCounts struct {
	Pending         int
	Processing      int
	Completed       int
	Rejected        int
	OpenAmount      float64
	CompletedAmount float64
	TodayNew        int
}

// InvoiceOrderLine 是申请占用的一笔订单及其开票金额快照。
type InvoiceOrderLine struct {
	OrderID int64
	Amount  float64
}

// InvoiceStatusUpdate 描述一次状态流转，时间戳由 Now 统一给出。
type InvoiceStatusUpdate struct {
	FromStatuses   []string
	Status         string
	AdminNotes     *string
	InvoiceNumber  *string
	InvoiceFileURL *string
	RejectReason   *string
	Now            time.Time
}

type InvoiceExclusion struct {
	ID         int64
	OrderID    int64
	OutTradeNo string
	UserEmail  string
	PayAmount  float64
	PaidAt     *time.Time
	CreatedBy  int64
	CreatedAt  time.Time
}

// InvoiceOrderMatch 是订单号查询命中的一笔订单。
type InvoiceOrderMatch struct {
	ID             int64
	OutTradeNo     string
	RechargeCode   string
	PaymentTradeNo string
	Paid           bool
	Excluded       bool
}

// InvoiceOrderState 描述一笔订单当前的开票占用情况。驳回的申请已释放订单，不算在内。
type InvoiceOrderState struct {
	Excluded      bool
	RequestID     *int64
	RequestStatus *string
	InvoiceNumber *string
}

// Invoiced 表示订单已开票或正在开票，用户不能再自助退款。
func (s InvoiceOrderState) Invoiced() bool {
	return s.Excluded || s.RequestID != nil
}

// AddInvoiceExclusionsResult 逐类告诉管理员每个订单号的处理结果，粘贴里的错号不会被静默吞掉。
type AddInvoiceExclusionsResult struct {
	Added           []InvoiceExclusion
	AlreadyExcluded []string
	NotFound        []string
	Unpaid          []string
	Ambiguous       []string
}

type InvoiceRepository interface {
	ListOrdersForUser(ctx context.Context, userID int64, statuses []string) ([]InvoiceOrderRow, error)
	GetOrderInvoiceState(ctx context.Context, orderID int64) (*InvoiceOrderState, error)
	// CreateRequest 在事务内锁定属于该用户的订单后调用 buildLines；它返回错误时整个事务回滚。
	CreateRequest(ctx context.Context, req *InvoiceRequest, orderIDs []int64, buildLines func([]InvoiceOrderRow) ([]InvoiceOrderLine, error)) error
	GetRequest(ctx context.Context, id int64) (*InvoiceRequest, error)
	ListRequests(ctx context.Context, params pagination.PaginationParams, filters InvoiceRequestListFilters) ([]InvoiceRequest, *pagination.PaginationResult, error)
	// UpdateStatus 只在当前状态属于 FromStatuses 时生效，返回是否更新；驳回时同时释放订单。
	UpdateStatus(ctx context.Context, id int64, update InvoiceStatusUpdate) (bool, error)
	MarkEmailSent(ctx context.Context, id int64, at time.Time) error
	// SetFile 记录上传的发票文件，返回被替换的旧文件 key（没有则为 nil），由调用方删除旧文件。
	// 只有 pending、processing、completed 的申请可以上传；申请不存在或状态不允许时返回对应错误。
	SetFile(ctx context.Context, id int64, meta InvoiceFileMeta) (*string, error)
	GetStats(ctx context.Context, todayStart time.Time) (*InvoiceStatusCounts, error)
	MatchOrders(ctx context.Context, orderNumbers []string, orderIDs []int64) ([]InvoiceOrderMatch, error)
	AddExclusions(ctx context.Context, orderIDs []int64, adminID int64) ([]InvoiceExclusion, error)
	ListExclusions(ctx context.Context, params pagination.PaginationParams, keyword string) ([]InvoiceExclusion, *pagination.PaginationResult, error)
	DeleteExclusion(ctx context.Context, id int64) error
}
