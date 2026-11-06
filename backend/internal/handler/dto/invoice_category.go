package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// InvoiceCategoryOption 是用户申请页下拉框里的一项，只含展示需要的字段。
type InvoiceCategoryOption struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AdminInvoiceCategory 是管理端的类目，带启停、排序和被申请引用的次数。
type AdminInvoiceCategory struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Enabled      bool      `json:"enabled"`
	SortOrder    int       `json:"sort_order"`
	RequestCount int       `json:"request_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func InvoiceCategoryOptionsFromService(items []service.InvoiceCategory) []InvoiceCategoryOption {
	out := make([]InvoiceCategoryOption, 0, len(items))
	for _, item := range items {
		out = append(out, InvoiceCategoryOption{ID: item.ID, Name: item.Name, Description: item.Description})
	}
	return out
}

func AdminInvoiceCategoryFromService(item *service.InvoiceCategory) *AdminInvoiceCategory {
	if item == nil {
		return nil
	}
	return &AdminInvoiceCategory{
		ID:           item.ID,
		Name:         item.Name,
		Description:  item.Description,
		Enabled:      item.Enabled,
		SortOrder:    item.SortOrder,
		RequestCount: item.RequestCount,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func AdminInvoiceCategoriesFromService(items []service.InvoiceCategory) []AdminInvoiceCategory {
	out := make([]AdminInvoiceCategory, 0, len(items))
	for i := range items {
		out = append(out, *AdminInvoiceCategoryFromService(&items[i]))
	}
	return out
}
