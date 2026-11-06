package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type invoiceRepository struct {
	db *sql.DB
}

func NewInvoiceRepository(db *sql.DB) service.InvoiceRepository {
	return &invoiceRepository{db: db}
}

// 占用和排除一起查出来，让 service 用同一份数据判断可否开票。
const invoiceOrderRowSelect = `
	SELECT o.id, o.out_trade_no, o.recharge_code, o.order_type, o.payment_type, o.status,
	       o.amount, o.pay_amount, o.bonus_amount, o.refund_amount,
	       COALESCE(o.provider_snapshot->>'currency', ''),
	       COALESCE(o.paid_at, o.completed_at, o.created_at),
	       claim.request_id,
	       ex.order_id IS NOT NULL
	FROM payment_orders o
	LEFT JOIN invoice_request_orders claim ON claim.order_id = o.id AND claim.active
	LEFT JOIN invoice_order_exclusions ex ON ex.order_id = o.id`

type invoiceRowScanner interface {
	Scan(dest ...any) error
}

func scanInvoiceOrderRow(rows invoiceRowScanner) (service.InvoiceOrderRow, error) {
	var row service.InvoiceOrderRow
	var requestID sql.NullInt64
	err := rows.Scan(
		&row.ID, &row.OutTradeNo, &row.RechargeCode, &row.OrderType, &row.PaymentType, &row.Status,
		&row.Amount, &row.PayAmount, &row.BonusAmount, &row.RefundAmount,
		&row.Currency, &row.PaidAt, &requestID, &row.Excluded,
	)
	if err != nil {
		return row, err
	}
	if requestID.Valid {
		row.InvoiceRequestID = &requestID.Int64
	}
	return row, nil
}

func (r *invoiceRepository) ListOrdersForUser(ctx context.Context, userID int64, statuses []string) ([]service.InvoiceOrderRow, error) {
	rows, err := r.db.QueryContext(ctx, invoiceOrderRowSelect+`
		WHERE o.user_id = $1 AND o.status = ANY($2)
		ORDER BY COALESCE(o.paid_at, o.completed_at, o.created_at) DESC, o.id DESC`,
		userID, pq.Array(statuses))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return collectInvoiceOrderRows(rows)
}

func (r *invoiceRepository) GetOrderInvoiceState(ctx context.Context, orderID int64) (*service.InvoiceOrderState, error) {
	var state service.InvoiceOrderState
	var requestID sql.NullInt64
	var requestStatus, invoiceNumber sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT ex.order_id IS NOT NULL, req.id, req.status, req.invoice_number
		FROM (SELECT $1::bigint AS order_id) target
		LEFT JOIN invoice_order_exclusions ex ON ex.order_id = target.order_id
		LEFT JOIN invoice_request_orders claim ON claim.order_id = target.order_id AND claim.active
		LEFT JOIN invoice_requests req ON req.id = claim.request_id`, orderID,
	).Scan(&state.Excluded, &requestID, &requestStatus, &invoiceNumber)
	if err != nil {
		return nil, err
	}
	if requestID.Valid {
		state.RequestID = &requestID.Int64
	}
	state.RequestStatus = invoiceNullString(requestStatus)
	state.InvoiceNumber = invoiceNullString(invoiceNumber)
	return &state, nil
}

func collectInvoiceOrderRows(rows *sql.Rows) ([]service.InvoiceOrderRow, error) {
	items := make([]service.InvoiceOrderRow, 0)
	for rows.Next() {
		item, err := scanInvoiceOrderRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *invoiceRepository) CreateRequest(
	ctx context.Context,
	req *service.InvoiceRequest,
	orderIDs []int64,
	buildLines func([]service.InvoiceOrderRow) ([]service.InvoiceOrderLine, error),
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 锁住订单，防止校验到写入之间订单进入退款流程。
	rows, err := tx.QueryContext(ctx, invoiceOrderRowSelect+`
		WHERE o.user_id = $1 AND o.id = ANY($2)
		ORDER BY o.id
		FOR UPDATE OF o`,
		req.UserID, pq.Array(orderIDs))
	if err != nil {
		return err
	}
	locked, err := collectInvoiceOrderRows(rows)
	_ = rows.Close()
	if err != nil {
		return err
	}
	lines, err := buildLines(locked)
	if err != nil {
		return err
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO invoice_requests (
			user_id, user_email, invoice_type, title,
			enterprise_tax_number, enterprise_address, enterprise_phone, enterprise_bank, enterprise_bank_account,
			individual_id_number_encrypted, individual_id_number_masked,
			total_amount, order_count, status, receiver_email, category_id, category_name
		) VALUES (
			$1, COALESCE((SELECT email FROM users WHERE id = $1), ''), $2, $3,
			$4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) RETURNING id`,
		req.UserID, req.InvoiceType, req.Title,
		req.EnterpriseTaxNumber, req.EnterpriseAddress, req.EnterprisePhone, req.EnterpriseBank, req.EnterpriseBankAccount,
		req.IDNumberEncrypted, req.IDNumberMasked,
		req.TotalAmount, req.OrderCount, req.Status, req.ReceiverEmail, req.CategoryID, req.CategoryName,
	).Scan(&req.ID)
	if err != nil {
		return err
	}

	for _, line := range lines {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO invoice_request_orders (request_id, order_id, invoiced_amount) VALUES ($1, $2, $3)`,
			req.ID, line.OrderID, line.Amount)
		if err != nil {
			// 两个请求同时占用同一笔订单时，后到的撞上部分唯一索引。
			if isUniqueViolation(err) {
				return service.ErrInvoiceOrderNotInvoiceable
			}
			return err
		}
	}
	return tx.Commit()
}

const invoiceRequestColumns = `
	id, user_id, user_email, invoice_type, title,
	enterprise_tax_number, enterprise_address, enterprise_phone, enterprise_bank, enterprise_bank_account,
	individual_id_number_encrypted, individual_id_number_masked,
	total_amount, order_count, status, receiver_email,
	invoice_number, invoice_file_url, invoice_file_name, invoice_file_key, invoice_file_size,
	email_sent_at, admin_notes, reject_reason,
	created_at, processing_at, completed_at, rejected_at,
		category_id, category_name`

func scanInvoiceRequest(row invoiceRowScanner) (*service.InvoiceRequest, error) {
	var req service.InvoiceRequest
	var taxNumber, address, phone, bank, bankAccount, encrypted, masked sql.NullString
	var invoiceNumber, fileURL, fileName, fileKey, notes, reason sql.NullString
	var fileSize sql.NullInt64
	var emailSentAt, processingAt, completedAt, rejectedAt sql.NullTime
	var categoryID sql.NullInt64
	var categoryName sql.NullString
	err := row.Scan(
		&req.ID, &req.UserID, &req.UserEmail, &req.InvoiceType, &req.Title,
		&taxNumber, &address, &phone, &bank, &bankAccount,
		&encrypted, &masked,
		&req.TotalAmount, &req.OrderCount, &req.Status, &req.ReceiverEmail,
		&invoiceNumber, &fileURL, &fileName, &fileKey, &fileSize,
		&emailSentAt, &notes, &reason,
		&req.CreatedAt, &processingAt, &completedAt, &rejectedAt,
		&categoryID, &categoryName,
	)
	if err != nil {
		return nil, err
	}
	req.EnterpriseTaxNumber = invoiceNullString(taxNumber)
	req.EnterpriseAddress = invoiceNullString(address)
	req.EnterprisePhone = invoiceNullString(phone)
	req.EnterpriseBank = invoiceNullString(bank)
	req.EnterpriseBankAccount = invoiceNullString(bankAccount)
	req.IDNumberEncrypted = invoiceNullString(encrypted)
	req.IDNumberMasked = invoiceNullString(masked)
	req.InvoiceNumber = invoiceNullString(invoiceNumber)
	req.InvoiceFileURL = invoiceNullString(fileURL)
	req.InvoiceFileName = invoiceNullString(fileName)
	req.InvoiceFileKey = invoiceNullString(fileKey)
	if fileSize.Valid {
		size := fileSize.Int64
		req.InvoiceFileSize = &size
	}
	req.AdminNotes = invoiceNullString(notes)
	req.RejectReason = invoiceNullString(reason)
	req.EmailSentAt = invoiceNullTime(emailSentAt)
	req.ProcessingAt = invoiceNullTime(processingAt)
	req.CompletedAt = invoiceNullTime(completedAt)
	req.RejectedAt = invoiceNullTime(rejectedAt)
	if categoryID.Valid {
		id := categoryID.Int64
		req.CategoryID = &id
	}
	req.CategoryName = invoiceNullString(categoryName)
	return &req, nil
}

func invoiceNullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func invoiceNullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func (r *invoiceRepository) GetRequest(ctx context.Context, id int64) (*service.InvoiceRequest, error) {
	req, err := scanInvoiceRequest(r.db.QueryRowContext(ctx,
		`SELECT `+invoiceRequestColumns+` FROM invoice_requests WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInvoiceRequestNotFound
		}
		return nil, err
	}
	req.Orders, err = r.listRequestOrders(ctx, id)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// 明细金额取申请时的快照，事后的部分退款不改变已提交申请的金额。
func (r *invoiceRepository) listRequestOrders(ctx context.Context, requestID int64) ([]service.InvoiceableOrder, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.out_trade_no, o.recharge_code, o.order_type, o.payment_type,
		       o.pay_amount, o.bonus_amount, line.invoiced_amount,
		       COALESCE(o.paid_at, o.completed_at, o.created_at)
		FROM invoice_request_orders line
		JOIN payment_orders o ON o.id = line.order_id
		WHERE line.request_id = $1
		ORDER BY o.id DESC`, requestID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.InvoiceableOrder, 0)
	for rows.Next() {
		var item service.InvoiceableOrder
		if err := rows.Scan(
			&item.ID, &item.OutTradeNo, &item.RechargeCode, &item.OrderType, &item.PaymentType,
			&item.PayAmount, &item.BonusAmount, &item.InvoiceableAmount, &item.PaidAt,
		); err != nil {
			return nil, err
		}
		item.AlreadyInvoiced = true
		requestIDCopy := requestID
		item.InvoiceRequestID = &requestIDCopy
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *invoiceRepository) ListRequests(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.InvoiceRequestListFilters,
) ([]service.InvoiceRequest, *pagination.PaginationResult, error) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	arg := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if filters.UserID > 0 {
		conditions = append(conditions, "user_id = "+arg(filters.UserID))
	}
	if filters.Status != "" {
		conditions = append(conditions, "status = "+arg(filters.Status))
	}
	if filters.InvoiceType != "" {
		conditions = append(conditions, "invoice_type = "+arg(filters.InvoiceType))
	}
	if filters.CategoryID > 0 {
		conditions = append(conditions, "category_id = "+arg(filters.CategoryID))
	}
	if keyword := strings.TrimSpace(filters.Keyword); keyword != "" {
		pattern := arg("%" + escapeLike(keyword) + "%")
		clause := fmt.Sprintf("(user_email ILIKE %[1]s OR title ILIKE %[1]s OR enterprise_tax_number ILIKE %[1]s", pattern)
		if id, err := strconv.ParseInt(keyword, 10, 64); err == nil && id > 0 {
			clause += " OR id = " + arg(id)
		}
		conditions = append(conditions, clause+")")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_requests`+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	// 先拼好 SQL 再传 args：同一表达式里既追加又读取 args，Go 不保证求值顺序。
	query := `SELECT ` + invoiceRequestColumns + ` FROM invoice_requests` + where +
		` ORDER BY created_at DESC, id DESC LIMIT ` + arg(params.Limit()) + ` OFFSET ` + arg(params.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.InvoiceRequest, 0)
	for rows.Next() {
		item, err := scanInvoiceRequest(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *invoiceRepository) UpdateStatus(ctx context.Context, id int64, update service.InvoiceStatusUpdate) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var notes any
	notesProvided := update.AdminNotes != nil
	if notesProvided && *update.AdminNotes != "" {
		notes = *update.AdminNotes
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE invoice_requests SET
			status = $3::varchar,
			admin_notes = CASE WHEN $4::boolean THEN $5::text ELSE admin_notes END,
			invoice_number = COALESCE($6::varchar, invoice_number),
			invoice_file_url = COALESCE($7::text, invoice_file_url),
			reject_reason = COALESCE($8::text, reject_reason),
			processing_at = CASE WHEN $3::varchar IN ('processing', 'completed') THEN COALESCE(processing_at, $9) ELSE processing_at END,
			completed_at = CASE WHEN $3::varchar = 'completed' THEN $9 ELSE completed_at END,
			rejected_at = CASE WHEN $3::varchar = 'rejected' THEN $9 ELSE rejected_at END,
			updated_at = $9
		WHERE id = $1 AND status = ANY($2)`,
		id, pq.Array(update.FromStatuses), update.Status, notesProvided, notes,
		update.InvoiceNumber, update.InvoiceFileURL, update.RejectReason, update.Now,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if update.Status == service.InvoiceStatusRejected {
		if _, err := tx.ExecContext(ctx, `UPDATE invoice_request_orders SET active = FALSE WHERE request_id = $1`, id); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (r *invoiceRepository) MarkEmailSent(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE invoice_requests SET email_sent_at = $2, updated_at = $2 WHERE id = $1`, id, at)
	return err
}

// SetFile 在事务里先锁住申请行并读出旧文件 key，再写入新文件。并发上传会排队，
// 每次都能读到上一次写入的 key，不会各自漏删一个文件。
// 不能把读旧 key 放进同一条 UPDATE 的 CTE：它会在更新之后才被求值，FOR UPDATE 会跳过已被本语句改过的行，读到空值。
func (r *invoiceRepository) SetFile(ctx context.Context, id int64, meta service.InvoiceFileMeta) (*string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var previous sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT status, invoice_file_key FROM invoice_requests WHERE id = $1 FOR UPDATE`, id,
	).Scan(&status, &previous)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrInvoiceRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == service.InvoiceStatusRejected {
		return nil, service.ErrInvoiceFileStatus
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE invoice_requests SET
			invoice_file_name = $2,
			invoice_file_key = $3,
			invoice_file_size = $4,
			updated_at = $5
		WHERE id = $1`,
		id, meta.Name, meta.Key, meta.Size, meta.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return invoiceNullString(previous), nil
}

func (r *invoiceRepository) GetStats(ctx context.Context, todayStart time.Time) (*service.InvoiceStatusCounts, error) {
	var stats service.InvoiceStatusCounts
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'processing'),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'rejected'),
			COALESCE(SUM(total_amount) FILTER (WHERE status IN ('pending', 'processing')), 0),
			COALESCE(SUM(total_amount) FILTER (WHERE status = 'completed'), 0),
			COUNT(*) FILTER (WHERE created_at >= $1)
		FROM invoice_requests`, todayStart,
	).Scan(&stats.Pending, &stats.Processing, &stats.Completed, &stats.Rejected, &stats.OpenAmount, &stats.CompletedAmount, &stats.TodayNew)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *invoiceRepository) MatchOrders(ctx context.Context, orderNumbers []string, orderIDs []int64) ([]service.InvoiceOrderMatch, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.out_trade_no, o.recharge_code, o.payment_trade_no,
		       o.paid_at IS NOT NULL OR o.status = ANY($3),
		       ex.order_id IS NOT NULL
		FROM payment_orders o
		LEFT JOIN invoice_order_exclusions ex ON ex.order_id = o.id
		WHERE o.out_trade_no = ANY($1)
		   OR o.recharge_code = ANY($1)
		   OR (o.payment_trade_no <> '' AND o.payment_trade_no = ANY($1))
		   OR o.id = ANY($2)`,
		pq.Array(orderNumbers), pq.Array(orderIDs), pq.Array([]string{"COMPLETED", "PARTIALLY_REFUNDED", "REFUNDED"}))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.InvoiceOrderMatch, 0, len(orderNumbers))
	for rows.Next() {
		var item service.InvoiceOrderMatch
		if err := rows.Scan(&item.ID, &item.OutTradeNo, &item.RechargeCode, &item.PaymentTradeNo, &item.Paid, &item.Excluded); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const invoiceExclusionSelect = `
	SELECT ex.id, ex.order_id, o.out_trade_no, o.user_email, o.pay_amount, o.paid_at, ex.created_by, ex.created_at
	FROM invoice_order_exclusions ex
	JOIN payment_orders o ON o.id = ex.order_id`

func scanInvoiceExclusion(row invoiceRowScanner) (service.InvoiceExclusion, error) {
	var item service.InvoiceExclusion
	var paidAt sql.NullTime
	err := row.Scan(&item.ID, &item.OrderID, &item.OutTradeNo, &item.UserEmail, &item.PayAmount, &paidAt, &item.CreatedBy, &item.CreatedAt)
	item.PaidAt = invoiceNullTime(paidAt)
	return item, err
}

func (r *invoiceRepository) AddExclusions(ctx context.Context, orderIDs []int64, adminID int64) ([]service.InvoiceExclusion, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH inserted AS (
			INSERT INTO invoice_order_exclusions (order_id, created_by)
			SELECT unnest($1::bigint[]), $2
			ON CONFLICT (order_id) DO NOTHING
			RETURNING id, order_id, created_by, created_at
		)
		SELECT ex.id, ex.order_id, o.out_trade_no, o.user_email, o.pay_amount, o.paid_at, ex.created_by, ex.created_at
		FROM inserted ex
		JOIN payment_orders o ON o.id = ex.order_id
		ORDER BY ex.id DESC`,
		pq.Array(orderIDs), adminID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return collectInvoiceExclusions(rows)
}

func collectInvoiceExclusions(rows *sql.Rows) ([]service.InvoiceExclusion, error) {
	items := make([]service.InvoiceExclusion, 0)
	for rows.Next() {
		item, err := scanInvoiceExclusion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *invoiceRepository) ListExclusions(ctx context.Context, params pagination.PaginationParams, keyword string) ([]service.InvoiceExclusion, *pagination.PaginationResult, error) {
	where := ""
	args := make([]any, 0, 3)
	if keyword != "" {
		args = append(args, "%"+escapeLike(keyword)+"%")
		where = ` WHERE o.out_trade_no ILIKE $1 OR o.recharge_code ILIKE $1 OR o.user_email ILIKE $1`
	}
	var total int64
	countSQL := `SELECT COUNT(*) FROM invoice_order_exclusions ex JOIN payment_orders o ON o.id = ex.order_id` + where
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, nil, err
	}
	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, invoiceExclusionSelect+where+
		fmt.Sprintf(` ORDER BY ex.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items, err := collectInvoiceExclusions(rows)
	if err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *invoiceRepository) DeleteExclusion(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM invoice_order_exclusions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrInvoiceExclusionNotFound
	}
	return nil
}
