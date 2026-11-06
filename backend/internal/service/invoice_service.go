package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

const (
	invoiceMaxOrdersPerRequest  = 200
	invoiceMaxExclusionsPerCall = 1000
)

var (
	invoiceTaxNumberRE   = regexp.MustCompile(`^(?:[0-9A-Z]{15}|[0-9A-Z]{18}|[0-9A-Z]{20})$`)
	invoiceIDNumberRE    = regexp.MustCompile(`^\d{17}[\dX]$`)
	invoiceBankAccountRE = regexp.MustCompile(`^\d{8,30}$`)
	invoiceEmailRE       = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// 只有支付完成且没有进入退款流程的订单能开票。退款中的订单结果未定，退款完成后再开。
var invoiceableOrderStatuses = []string{payment.OrderStatusCompleted, payment.OrderStatusPartiallyRefunded}

// 状态流转：pending → processing / completed / rejected；processing → completed / rejected。
var invoiceAllowedFromStatuses = map[string][]string{
	InvoiceStatusProcessing: {InvoiceStatusPending},
	InvoiceStatusCompleted:  {InvoiceStatusPending, InvoiceStatusProcessing},
	InvoiceStatusRejected:   {InvoiceStatusPending, InvoiceStatusProcessing},
}

type invoiceEmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

type invoiceSiteNamer interface {
	GetSiteName(ctx context.Context) string
}

// invoiceAttachmentSender 是邮件发送方的可选能力：能带附件时发票 PDF 随邮件发出，否则只发正文。
type invoiceAttachmentSender interface {
	SendEmailWithAttachment(ctx context.Context, to, subject, body string, attachment *EmailAttachment) error
}

type InvoiceService struct {
	repo      InvoiceRepository
	email     invoiceEmailSender
	site      invoiceSiteNamer
	encryptor SecretEncryptor
	files     InvoiceFileStore
}

func NewInvoiceService(repo InvoiceRepository, emailService *EmailService, settingService *SettingService, encryptor SecretEncryptor, files InvoiceFileStore) *InvoiceService {
	return &InvoiceService{repo: repo, email: emailService, site: settingService, encryptor: encryptor, files: files}
}

// invoiceableAmount 返回订单可开票的人民币金额，0 表示不可开票。
//
// 只有实付能开票：bonus_amount 记在到账额度里，从不进 pay_amount。refund_amount 按到账额度计，
// 要换算回实付才能相减；只有部分退款状态才扣，退款被驳回后订单回到 COMPLETED 时该字段可能还留着旧值。
func invoiceableAmount(row InvoiceOrderRow) float64 {
	if !slices.Contains(invoiceableOrderStatuses, row.Status) || invoiceOrderCurrency(row.Currency) != payment.DefaultPaymentCurrency {
		return 0
	}
	amount := decimal.NewFromFloat(row.PayAmount)
	if row.Status == payment.OrderStatusPartiallyRefunded {
		refunded := calculateGatewayRefundAmount(row.Amount, row.PayAmount, row.RefundAmount, payment.DefaultPaymentCurrency)
		amount = amount.Sub(decimal.NewFromFloat(refunded))
	}
	if !amount.IsPositive() {
		return 0
	}
	return amount.Round(2).InexactFloat64()
}

// 没有记录币种的存量订单与 PaymentOrderCurrency 一样按默认币种处理。
func invoiceOrderCurrency(raw string) string {
	currency, err := payment.NormalizePaymentCurrency(raw)
	if err != nil {
		return payment.DefaultPaymentCurrency
	}
	return currency
}

func invoiceOrderFromRow(row InvoiceOrderRow, amount float64) InvoiceableOrder {
	return InvoiceableOrder{
		ID:                row.ID,
		OutTradeNo:        row.OutTradeNo,
		RechargeCode:      row.RechargeCode,
		OrderType:         row.OrderType,
		PaymentType:       row.PaymentType,
		PayAmount:         row.PayAmount,
		BonusAmount:       row.BonusAmount,
		InvoiceableAmount: amount,
		PaidAt:            row.PaidAt,
		AlreadyInvoiced:   row.InvoiceRequestID != nil || row.Excluded,
		Excluded:          row.Excluded,
		InvoiceRequestID:  row.InvoiceRequestID,
	}
}

func (s *InvoiceService) ListInvoiceableOrders(ctx context.Context, userID int64) ([]InvoiceableOrder, InvoiceableSummary, error) {
	rows, err := s.repo.ListOrdersForUser(ctx, userID, invoiceableOrderStatuses)
	if err != nil {
		return nil, InvoiceableSummary{}, fmt.Errorf("list invoiceable orders: %w", err)
	}
	items := make([]InvoiceableOrder, 0, len(rows))
	total := decimal.Zero
	count := 0
	for _, row := range rows {
		amount := invoiceableAmount(row)
		if amount <= 0 {
			continue
		}
		item := invoiceOrderFromRow(row, amount)
		items = append(items, item)
		if !item.AlreadyInvoiced {
			total = total.Add(decimal.NewFromFloat(amount))
			count++
		}
	}
	return items, InvoiceableSummary{TotalInvoiceableAmount: total.Round(2).InexactFloat64(), InvoiceableCount: count}, nil
}

func (s *InvoiceService) CreateRequest(ctx context.Context, input CreateInvoiceRequestInput) (*InvoiceRequest, error) {
	orderIDs := dedupePositiveIDs(input.OrderIDs)
	if len(orderIDs) == 0 {
		return nil, ErrInvoiceNoOrders
	}
	if len(orderIDs) > invoiceMaxOrdersPerRequest {
		return nil, ErrInvoiceTooManyOrders
	}
	req, err := s.buildRequest(input)
	if err != nil {
		return nil, err
	}
	// 类目在服务端重新校验，并把名称快照写进申请，之后类目改名不影响历史申请。
	category, err := s.resolveInvoiceCategory(ctx, input.CategoryID)
	if err != nil {
		return nil, err
	}
	if category != nil {
		req.CategoryID = &category.ID
		req.CategoryName = &category.Name
	}
	// 金额只取服务端算出的值，订单归属和可开票状态都在锁内重新判断。
	buildLines := func(rows []InvoiceOrderRow) ([]InvoiceOrderLine, error) {
		if len(rows) != len(orderIDs) {
			return nil, ErrInvoiceOrderNotInvoiceable
		}
		lines := make([]InvoiceOrderLine, 0, len(rows))
		total := decimal.Zero
		for _, row := range rows {
			amount := invoiceableAmount(row)
			if amount <= 0 || row.InvoiceRequestID != nil || row.Excluded {
				return nil, ErrInvoiceOrderNotInvoiceable
			}
			lines = append(lines, InvoiceOrderLine{OrderID: row.ID, Amount: amount})
			total = total.Add(decimal.NewFromFloat(amount))
		}
		req.TotalAmount = total.Round(2).InexactFloat64()
		req.OrderCount = len(lines)
		return lines, nil
	}
	if err := s.repo.CreateRequest(ctx, req, orderIDs, buildLines); err != nil {
		return nil, err
	}
	return s.repo.GetRequest(ctx, req.ID)
}

func (s *InvoiceService) buildRequest(input CreateInvoiceRequestInput) (*InvoiceRequest, error) {
	receiver := strings.TrimSpace(input.ReceiverEmail)
	if len(receiver) > 255 || !invoiceEmailRE.MatchString(receiver) {
		return nil, ErrInvoiceEmailInvalid
	}
	req := &InvoiceRequest{
		UserID:        input.UserID,
		InvoiceType:   input.InvoiceType,
		ReceiverEmail: receiver,
		Status:        InvoiceStatusPending,
	}
	var err error
	switch input.InvoiceType {
	case InvoiceTypeEnterprise:
		err = fillInvoiceEnterpriseTitle(req, input)
	case InvoiceTypeIndividual:
		err = s.fillInvoiceIndividualTitle(req, input)
	default:
		err = ErrInvoiceTypeInvalid
	}
	if err != nil {
		return nil, err
	}
	return req, nil
}

func fillInvoiceEnterpriseTitle(req *InvoiceRequest, input CreateInvoiceRequestInput) error {
	req.Title = strings.TrimSpace(input.EnterpriseName)
	if req.Title == "" {
		return ErrInvoiceTitleRequired
	}
	tax := strings.ToUpper(strings.TrimSpace(input.EnterpriseTaxNumber))
	if !invoiceTaxNumberRE.MatchString(tax) {
		return ErrInvoiceTaxNumberInvalid
	}
	account := strings.Join(strings.Fields(input.EnterpriseBankAccount), "")
	if account != "" && !invoiceBankAccountRE.MatchString(account) {
		return ErrInvoiceBankAccountInvalid
	}
	address := strings.TrimSpace(input.EnterpriseAddress)
	phone := strings.TrimSpace(input.EnterprisePhone)
	bank := strings.TrimSpace(input.EnterpriseBank)
	if utf8.RuneCountInString(req.Title) > 200 || utf8.RuneCountInString(address) > 255 || utf8.RuneCountInString(phone) > 50 || utf8.RuneCountInString(bank) > 100 {
		return ErrInvoiceFieldTooLong
	}
	req.EnterpriseTaxNumber = &tax
	req.EnterpriseAddress = invoiceNilIfEmpty(address)
	req.EnterprisePhone = invoiceNilIfEmpty(phone)
	req.EnterpriseBank = invoiceNilIfEmpty(bank)
	req.EnterpriseBankAccount = invoiceNilIfEmpty(account)
	return nil
}

func (s *InvoiceService) fillInvoiceIndividualTitle(req *InvoiceRequest, input CreateInvoiceRequestInput) error {
	req.Title = strings.TrimSpace(input.IndividualName)
	if req.Title == "" {
		return ErrInvoiceTitleRequired
	}
	if utf8.RuneCountInString(req.Title) > 200 {
		return ErrInvoiceFieldTooLong
	}
	idNumber := strings.ToUpper(strings.TrimSpace(input.IndividualIDNumber))
	if idNumber == "" {
		return nil
	}
	if !invoiceIDNumberRE.MatchString(idNumber) {
		return ErrInvoiceIDNumberInvalid
	}
	encrypted, err := s.encryptor.Encrypt(idNumber)
	if err != nil {
		return fmt.Errorf("encrypt id number: %w", err)
	}
	masked := maskInvoiceIDNumber(idNumber)
	req.IDNumberEncrypted = &encrypted
	req.IDNumberMasked = &masked
	return nil
}

func maskInvoiceIDNumber(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

func invoiceNilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func dedupePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok || id <= 0 {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *InvoiceService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, status string) ([]InvoiceRequest, *pagination.PaginationResult, error) {
	return s.repo.ListRequests(ctx, params, InvoiceRequestListFilters{UserID: userID, Status: status})
}

// GetForUser 查不到和不属于该用户返回同样的错误，避免暴露别人的申请是否存在。
func (s *InvoiceService) GetForUser(ctx context.Context, userID, id int64) (*InvoiceRequest, error) {
	req, err := s.repo.GetRequest(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.UserID != userID {
		return nil, ErrInvoiceRequestNotFound
	}
	return req, nil
}

func (s *InvoiceService) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters InvoiceRequestListFilters) ([]InvoiceRequest, *pagination.PaginationResult, error) {
	filters.UserID = 0
	return s.repo.ListRequests(ctx, params, filters)
}

// GetForAdmin 解密身份证号供管理员核对。加密密钥未持久化配置时重启后无法解密，退回打码值。
func (s *InvoiceService) GetForAdmin(ctx context.Context, id int64) (*InvoiceRequest, error) {
	req, err := s.repo.GetRequest(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.IDNumberEncrypted != nil {
		if plain, err := s.encryptor.Decrypt(*req.IDNumberEncrypted); err == nil {
			req.IDNumberPlain = &plain
		} else {
			slog.Warn("invoice id number decrypt failed", "request_id", id, "error", err)
		}
	}
	return req, nil
}

// GetOrderInvoiceState 供管理员退款前查看订单的开票情况。
func (s *InvoiceService) GetOrderInvoiceState(ctx context.Context, orderID int64) (*InvoiceOrderState, error) {
	return s.repo.GetOrderInvoiceState(ctx, orderID)
}

func (s *InvoiceService) GetStats(ctx context.Context) (*InvoiceStatusCounts, error) {
	return s.repo.GetStats(ctx, timezone.Today())
}

type UpdateInvoiceStatusInput struct {
	Status         string
	AdminNotes     *string
	InvoiceNumber  string
	InvoiceFileURL string
	RejectReason   string
	SendEmail      bool
}

func (s *InvoiceService) UpdateStatus(ctx context.Context, id int64, input UpdateInvoiceStatusInput) (*InvoiceRequest, error) {
	from, ok := invoiceAllowedFromStatuses[input.Status]
	if !ok {
		return nil, ErrInvoiceStatusTransition
	}
	update := InvoiceStatusUpdate{FromStatuses: from, Status: input.Status, Now: time.Now()}
	if input.AdminNotes != nil {
		notes := strings.TrimSpace(*input.AdminNotes)
		update.AdminNotes = &notes
	}
	switch input.Status {
	case InvoiceStatusCompleted:
		number := strings.TrimSpace(input.InvoiceNumber)
		if number == "" || len(number) > 64 {
			return nil, ErrInvoiceNumberRequired
		}
		// 发票文件二选一：填了外部链接就校验链接；没填则必须已经上传过 PDF。
		fileURL := strings.TrimSpace(input.InvoiceFileURL)
		if fileURL != "" {
			if !isInvoiceFileURL(fileURL) {
				return nil, ErrInvoiceFileURLInvalid
			}
			update.InvoiceFileURL = &fileURL
		} else {
			current, err := s.repo.GetRequest(ctx, id)
			if err != nil {
				return nil, err
			}
			if current.InvoiceFileKey == nil {
				return nil, ErrInvoiceFileURLInvalid
			}
		}
		update.InvoiceNumber = &number
	case InvoiceStatusRejected:
		reason := strings.TrimSpace(input.RejectReason)
		if reason == "" {
			return nil, ErrInvoiceRejectReason
		}
		update.RejectReason = &reason
	}

	updated, err := s.repo.UpdateStatus(ctx, id, update)
	if err != nil {
		return nil, err
	}
	if !updated {
		// 区分申请不存在和状态不允许，两者给管理员的提示不同。
		if _, err := s.repo.GetRequest(ctx, id); err != nil {
			return nil, err
		}
		return nil, ErrInvoiceStatusTransition
	}

	req, err := s.GetForAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Status == InvoiceStatusCompleted && input.SendEmail {
		// 发票已经开出，邮件失败不回滚状态；email_sent_at 留空，管理员能看到并重发。
		if err := s.deliverEmail(ctx, req); err != nil {
			slog.Warn("invoice email failed after completion", "request_id", id, "error", err)
		}
	}
	return req, nil
}

// 文件链接会渲染成 <a href>，只接受 http(s)，挡掉 javascript: 这类协议。
func isInvoiceFileURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func (s *InvoiceService) SendEmail(ctx context.Context, id int64) (*InvoiceRequest, error) {
	req, err := s.GetForAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != InvoiceStatusCompleted {
		return nil, ErrInvoiceEmailNotCompleted
	}
	if err := s.deliverEmail(ctx, req); err != nil {
		slog.Warn("invoice email failed", "request_id", id, "error", err)
		return nil, ErrInvoiceEmailFailed
	}
	return req, nil
}

func (s *InvoiceService) deliverEmail(ctx context.Context, req *InvoiceRequest) error {
	siteName := s.site.GetSiteName(ctx)
	subject := fmt.Sprintf("[%s] 您的电子发票已开具（申请 #%d）", siteName, req.ID)
	body := buildInvoiceEmailBody(siteName, req)

	// 上传过 PDF 就作为附件发出；读文件失败不挡邮件，退回只发正文，用户仍能登录后下载。
	var attachment *EmailAttachment
	if req.InvoiceFileKey != nil && s.files != nil {
		data, err := s.files.Read(ctx, *req.InvoiceFileKey)
		if err != nil {
			slog.Warn("invoice file unreadable for email attachment", "request_id", req.ID, "error", err)
		} else {
			name := fmt.Sprintf("invoice-%d.pdf", req.ID)
			if req.InvoiceFileName != nil && *req.InvoiceFileName != "" {
				name = *req.InvoiceFileName
			}
			attachment = &EmailAttachment{Filename: name, ContentType: "application/pdf", Data: data}
		}
	}

	var sendErr error
	if sender, ok := s.email.(invoiceAttachmentSender); ok && attachment != nil {
		sendErr = sender.SendEmailWithAttachment(ctx, req.ReceiverEmail, subject, body, attachment)
	} else {
		sendErr = s.email.SendEmail(ctx, req.ReceiverEmail, subject, body)
	}
	if sendErr != nil {
		return sendErr
	}
	now := time.Now()
	if err := s.repo.MarkEmailSent(ctx, req.ID, now); err != nil {
		return fmt.Errorf("mark invoice email sent: %w", err)
	}
	req.EmailSentAt = &now
	return nil
}

func buildInvoiceEmailBody(siteName string, req *InvoiceRequest) string {
	number := ""
	if req.InvoiceNumber != nil {
		number = *req.InvoiceNumber
	}
	fileURL := ""
	if req.InvoiceFileURL != nil {
		fileURL = *req.InvoiceFileURL
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f5f5f5;margin:0;padding:20px;">
<div style="max-width:600px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;color:#333;line-height:1.7;">
<h2 style="margin-top:0;">%s</h2>
<p>您申请的电子发票已开具：</p>
<ul>
<li>申请编号：#%d</li>
<li>发票抬头：%s</li>
<li>发票号码：%s</li>
<li>开票金额：¥%s</li>
</ul>
<p><a href="%s">点击下载发票文件</a></p>
<p style="color:#999;font-size:12px;">此邮件由系统自动发送，请勿回复。</p>
</div></body></html>`,
		html.EscapeString(siteName),
		req.ID,
		html.EscapeString(req.Title),
		html.EscapeString(number),
		decimal.NewFromFloat(req.TotalAmount).StringFixed(2),
		html.EscapeString(fileURL),
	)
}

// AddExclusions 把订单号登记为已在系统外开票。每个号可以是订单号、充值码、支付渠道交易号或订单 ID；
// 同一个号命中多笔订单时不猜测，交给管理员改用更精确的号。
func (s *InvoiceService) AddExclusions(ctx context.Context, rawNumbers []string, adminID int64) (*AddInvoiceExclusionsResult, error) {
	numbers := normalizeInvoiceOrderNumbers(rawNumbers)
	if len(numbers) == 0 {
		return nil, ErrInvoiceExclusionEmpty
	}
	if len(numbers) > invoiceMaxExclusionsPerCall {
		return nil, ErrInvoiceExclusionTooMany
	}
	ids := make([]int64, 0, len(numbers))
	for _, number := range numbers {
		if id, err := strconv.ParseInt(number, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	matches, err := s.repo.MatchOrders(ctx, numbers, ids)
	if err != nil {
		return nil, fmt.Errorf("match invoice orders: %w", err)
	}

	result := &AddInvoiceExclusionsResult{}
	toAdd := make([]int64, 0, len(numbers))
	pending := make(map[int64]struct{}, len(numbers))
	for _, number := range numbers {
		var hits []InvoiceOrderMatch
		for _, match := range matches {
			if match.matches(number) {
				hits = append(hits, match)
			}
		}
		switch {
		case len(hits) == 0:
			result.NotFound = append(result.NotFound, number)
		case len(hits) > 1:
			result.Ambiguous = append(result.Ambiguous, number)
		case !hits[0].Paid:
			result.Unpaid = append(result.Unpaid, number)
		case hits[0].Excluded:
			result.AlreadyExcluded = append(result.AlreadyExcluded, number)
		default:
			// 同一笔订单被粘贴了两个不同的号（如订单号和 ID）时只登记一次。
			if _, dup := pending[hits[0].ID]; dup {
				result.AlreadyExcluded = append(result.AlreadyExcluded, number)
				continue
			}
			pending[hits[0].ID] = struct{}{}
			toAdd = append(toAdd, hits[0].ID)
		}
	}
	if len(toAdd) > 0 {
		added, err := s.repo.AddExclusions(ctx, toAdd, adminID)
		if err != nil {
			return nil, fmt.Errorf("add invoice exclusions: %w", err)
		}
		result.Added = added
	}
	return result, nil
}

func (m InvoiceOrderMatch) matches(number string) bool {
	if number == m.OutTradeNo || number == m.RechargeCode || number == strconv.FormatInt(m.ID, 10) {
		return true
	}
	return m.PaymentTradeNo != "" && number == m.PaymentTradeNo
}

func normalizeInvoiceOrderNumbers(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		number := strings.TrimSpace(value)
		if number == "" {
			continue
		}
		if _, ok := seen[number]; ok {
			continue
		}
		seen[number] = struct{}{}
		out = append(out, number)
	}
	return out
}

func (s *InvoiceService) ListExclusions(ctx context.Context, params pagination.PaginationParams, keyword string) ([]InvoiceExclusion, *pagination.PaginationResult, error) {
	return s.repo.ListExclusions(ctx, params, strings.TrimSpace(keyword))
}

func (s *InvoiceService) DeleteExclusion(ctx context.Context, id int64) error {
	return s.repo.DeleteExclusion(ctx, id)
}
