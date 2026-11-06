package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const invoiceCategoryColumns = `c.id, c.name, c.description, c.enabled, c.sort_order, c.created_at, c.updated_at`

// 管理端列表要显示被申请引用的次数，用户端不需要，不必多查。
const invoiceCategoryRequestCount = `(SELECT COUNT(*) FROM invoice_requests r WHERE r.category_id = c.id)`

func scanInvoiceCategory(row invoiceRowScanner) (service.InvoiceCategory, error) {
	var item service.InvoiceCategory
	err := row.Scan(
		&item.ID, &item.Name, &item.Description, &item.Enabled, &item.SortOrder,
		&item.CreatedAt, &item.UpdatedAt, &item.RequestCount,
	)
	return item, err
}

// 外键 RESTRICT 拦住删除时的 SQLSTATE 23503。
func isInvoiceCategoryForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}

func (r *invoiceRepository) ListCategories(ctx context.Context, enabledOnly bool) ([]service.InvoiceCategory, error) {
	count := invoiceCategoryRequestCount
	where := ""
	if enabledOnly {
		count = "0"
		where = " WHERE c.enabled"
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+invoiceCategoryColumns+`, `+count+` FROM invoice_categories c`+where+` ORDER BY c.sort_order, c.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.InvoiceCategory, 0)
	for rows.Next() {
		item, err := scanInvoiceCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *invoiceRepository) GetCategory(ctx context.Context, id int64) (*service.InvoiceCategory, error) {
	item, err := scanInvoiceCategory(r.db.QueryRowContext(ctx,
		`SELECT `+invoiceCategoryColumns+`, `+invoiceCategoryRequestCount+` FROM invoice_categories c WHERE c.id = $1`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInvoiceCategoryNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *invoiceRepository) CreateCategory(ctx context.Context, category *service.InvoiceCategory) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO invoice_categories (name, description, enabled, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		category.Name, category.Description, category.Enabled, category.SortOrder,
	).Scan(&category.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ErrInvoiceCategoryDuplicate
		}
		return err
	}
	return nil
}

func (r *invoiceRepository) UpdateCategory(ctx context.Context, category *service.InvoiceCategory) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE invoice_categories SET
			name = $2, description = $3, enabled = $4, sort_order = $5, updated_at = NOW()
		WHERE id = $1`,
		category.ID, category.Name, category.Description, category.Enabled, category.SortOrder)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ErrInvoiceCategoryDuplicate
		}
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrInvoiceCategoryNotFound
	}
	return nil
}

// DeleteCategory 先查引用再删：并发时新申请恰好引用该类目，外键 RESTRICT 会兜底拦住。
func (r *invoiceRepository) DeleteCategory(ctx context.Context, id int64) error {
	var used bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM invoice_requests WHERE category_id = $1)`, id).Scan(&used); err != nil {
		return err
	}
	if used {
		return service.ErrInvoiceCategoryInUse
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM invoice_categories WHERE id = $1`, id)
	if err != nil {
		if isInvoiceCategoryForeignKeyViolation(err) {
			return service.ErrInvoiceCategoryInUse
		}
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrInvoiceCategoryNotFound
	}
	return nil
}

func (r *invoiceRepository) ReorderCategories(ctx context.Context, ids []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for index, id := range ids {
		res, err := tx.ExecContext(ctx,
			`UPDATE invoice_categories SET sort_order = $2, updated_at = NOW() WHERE id = $1`, id, index+1)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return service.ErrInvoiceCategoryNotFound
		}
	}
	return tx.Commit()
}
