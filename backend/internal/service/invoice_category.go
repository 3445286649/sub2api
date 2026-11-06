package service

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	invoiceCategoryNameMax    = 100
	invoiceCategoryDescMax    = 500
	invoiceCategoryReorderMax = 200
)

var (
	ErrInvoiceCategoryRequired       = infraerrors.BadRequest("INVOICE_CATEGORY_REQUIRED", "select an invoice category")
	ErrInvoiceCategoryInvalid        = infraerrors.BadRequest("INVALID_INVOICE_CATEGORY", "invoice category is invalid or disabled")
	ErrInvoiceCategoryNotFound       = infraerrors.NotFound("INVOICE_CATEGORY_NOT_FOUND", "invoice category not found")
	ErrInvoiceCategoryNameRequired   = infraerrors.BadRequest("INVOICE_CATEGORY_NAME_REQUIRED", "invoice category name is required")
	ErrInvoiceCategoryTooLong        = infraerrors.BadRequest("INVOICE_CATEGORY_FIELD_TOO_LONG", "invoice category name or description is too long")
	ErrInvoiceCategoryDuplicate      = infraerrors.Conflict("INVOICE_CATEGORY_DUPLICATE", "an invoice category with this name already exists")
	ErrInvoiceCategoryInUse          = infraerrors.Conflict("INVOICE_CATEGORY_IN_USE", "this category is used by invoice requests and can only be disabled")
	ErrInvoiceCategoryReorderInvalid = infraerrors.BadRequest("INVOICE_CATEGORY_REORDER_INVALID", "category order is invalid")
)

// InvoiceCategory 是管理员配置的开票类目。RequestCount 只在管理端列表里填充，用来判断能否删除。
type InvoiceCategory struct {
	ID           int64
	Name         string
	Description  string
	Enabled      bool
	SortOrder    int
	RequestCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// InvoiceCategoryInput 的指针字段为 nil 表示不修改；创建时 Enabled 缺省为启用。
type InvoiceCategoryInput struct {
	Name        *string
	Description *string
	Enabled     *bool
	SortOrder   *int
}

func applyInvoiceCategoryInput(category *InvoiceCategory, input InvoiceCategoryInput) error {
	if input.Name != nil {
		category.Name = strings.TrimSpace(*input.Name)
		if category.Name == "" {
			return ErrInvoiceCategoryNameRequired
		}
	}
	if input.Description != nil {
		category.Description = strings.TrimSpace(*input.Description)
	}
	if utf8.RuneCountInString(category.Name) > invoiceCategoryNameMax || utf8.RuneCountInString(category.Description) > invoiceCategoryDescMax {
		return ErrInvoiceCategoryTooLong
	}
	if input.Enabled != nil {
		category.Enabled = *input.Enabled
	}
	if input.SortOrder != nil {
		category.SortOrder = *input.SortOrder
	}
	return nil
}

// ListCategoriesForUser 只返回已启用的类目，按排序值和 ID 排列。
func (s *InvoiceService) ListCategoriesForUser(ctx context.Context) ([]InvoiceCategory, error) {
	return s.repo.ListCategories(ctx, true)
}

func (s *InvoiceService) ListCategoriesForAdmin(ctx context.Context) ([]InvoiceCategory, error) {
	return s.repo.ListCategories(ctx, false)
}

func (s *InvoiceService) CreateCategory(ctx context.Context, input InvoiceCategoryInput) (*InvoiceCategory, error) {
	category := &InvoiceCategory{Enabled: true}
	if err := applyInvoiceCategoryInput(category, input); err != nil {
		return nil, err
	}
	if category.Name == "" {
		return nil, ErrInvoiceCategoryNameRequired
	}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return s.repo.GetCategory(ctx, category.ID)
}

func (s *InvoiceService) UpdateCategory(ctx context.Context, id int64, input InvoiceCategoryInput) (*InvoiceCategory, error) {
	category, err := s.repo.GetCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := applyInvoiceCategoryInput(category, input); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCategory(ctx, category); err != nil {
		return nil, err
	}
	return s.repo.GetCategory(ctx, id)
}

// DeleteCategory 只能删除没被任何申请引用的类目，已使用的由 repo 返回 ErrInvoiceCategoryInUse。
func (s *InvoiceService) DeleteCategory(ctx context.Context, id int64) error {
	return s.repo.DeleteCategory(ctx, id)
}

// ReorderCategories 按传入顺序重写排序值，ID 必须为正且不重复。
func (s *InvoiceService) ReorderCategories(ctx context.Context, ids []int64) error {
	if len(ids) == 0 || len(ids) > invoiceCategoryReorderMax {
		return ErrInvoiceCategoryReorderInvalid
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup || id <= 0 {
			return ErrInvoiceCategoryReorderInvalid
		}
		seen[id] = struct{}{}
	}
	return s.repo.ReorderCategories(ctx, ids)
}

// resolveInvoiceCategory 决定一次申请要记录的类目：
// 没有任何启用类目时保持旧流程（返回 nil）；有启用类目时必须选择其中一个。
func (s *InvoiceService) resolveInvoiceCategory(ctx context.Context, categoryID *int64) (*InvoiceCategory, error) {
	enabled, err := s.repo.ListCategories(ctx, true)
	if err != nil {
		return nil, err
	}
	if len(enabled) == 0 {
		if categoryID != nil {
			return nil, ErrInvoiceCategoryInvalid
		}
		return nil, nil
	}
	if categoryID == nil {
		return nil, ErrInvoiceCategoryRequired
	}
	idx := slices.IndexFunc(enabled, func(c InvoiceCategory) bool { return c.ID == *categoryID })
	if idx < 0 {
		return nil, ErrInvoiceCategoryInvalid
	}
	return &enabled[idx], nil
}
