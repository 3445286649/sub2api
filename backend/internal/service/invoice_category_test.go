//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func categoryTestService(categories ...InvoiceCategory) (*InvoiceService, *fakeInvoiceRepo) {
	repo := &fakeInvoiceRepo{rows: []InvoiceOrderRow{paidInvoiceRow(1, 100)}, categories: categories}
	return newInvoiceTestService(repo, nil), repo
}

func categoryID(id int64) *int64 { return &id }

func TestCreateRequestKeepsTheOldFlowWhenNoCategoryIsConfigured(t *testing.T) {
	svc, repo := categoryTestService()

	created, err := svc.CreateRequest(context.Background(), validEnterpriseInvoiceInput(1))
	require.NoError(t, err)
	require.Nil(t, created.CategoryID)
	require.Nil(t, created.CategoryName)
	require.NotNil(t, repo.request)

	// 没有任何启用类目时，传入类目 ID 一律视为非法，不能写入悬空引用。
	input := validEnterpriseInvoiceInput(1)
	input.CategoryID = categoryID(1)
	_, err = svc.CreateRequest(context.Background(), input)
	require.ErrorIs(t, err, ErrInvoiceCategoryInvalid)
}

func TestCreateRequestRequiresAnEnabledCategoryOnceOneIsConfigured(t *testing.T) {
	svc, _ := categoryTestService(
		InvoiceCategory{ID: 1, Name: "技术服务费", Enabled: true},
		InvoiceCategory{ID: 2, Name: "已停用类目", Enabled: false},
	)

	_, err := svc.CreateRequest(context.Background(), validEnterpriseInvoiceInput(1))
	require.ErrorIs(t, err, ErrInvoiceCategoryRequired)

	for name, id := range map[string]int64{"disabled": 2, "unknown": 99} {
		input := validEnterpriseInvoiceInput(1)
		input.CategoryID = categoryID(id)
		_, err := svc.CreateRequest(context.Background(), input)
		require.ErrorIs(t, err, ErrInvoiceCategoryInvalid, name)
	}
}

func TestCreateRequestStoresTheCategoryNameSnapshot(t *testing.T) {
	svc, repo := categoryTestService(
		InvoiceCategory{ID: 1, Name: "技术服务费", Enabled: true},
		InvoiceCategory{ID: 2, Name: "软件使用费", Enabled: true},
	)
	input := validEnterpriseInvoiceInput(1)
	input.CategoryID = categoryID(2)

	created, err := svc.CreateRequest(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, created.CategoryID)
	require.Equal(t, int64(2), *created.CategoryID)
	require.NotNil(t, created.CategoryName)
	require.Equal(t, "软件使用费", *created.CategoryName)

	// 申请记录的是名称快照：类目之后改名，已提交的申请不变。
	repo.categories[1].Name = "改名后的类目"
	after, err := svc.GetForAdmin(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "软件使用费", *after.CategoryName)
}

func TestUserSeesOnlyEnabledCategoriesWhileAdminSeesAll(t *testing.T) {
	svc, _ := categoryTestService(
		InvoiceCategory{ID: 1, Name: "技术服务费", Enabled: true},
		InvoiceCategory{ID: 2, Name: "已停用类目", Enabled: false},
	)

	user, err := svc.ListCategoriesForUser(context.Background())
	require.NoError(t, err)
	require.Len(t, user, 1)
	require.Equal(t, "技术服务费", user[0].Name)

	admin, err := svc.ListCategoriesForAdmin(context.Background())
	require.NoError(t, err)
	require.Len(t, admin, 2)
}

func TestCreateCategoryTrimsAndValidatesInput(t *testing.T) {
	svc, _ := categoryTestService()
	str := func(v string) *string { return &v }

	created, err := svc.CreateCategory(context.Background(), InvoiceCategoryInput{Name: str("  技术服务费 "), Description: str(" 软件技术服务 ")})
	require.NoError(t, err)
	require.Equal(t, "技术服务费", created.Name)
	require.Equal(t, "软件技术服务", created.Description)
	require.True(t, created.Enabled, "new categories are enabled by default")

	_, err = svc.CreateCategory(context.Background(), InvoiceCategoryInput{})
	require.ErrorIs(t, err, ErrInvoiceCategoryNameRequired)
	_, err = svc.CreateCategory(context.Background(), InvoiceCategoryInput{Name: str("   ")})
	require.ErrorIs(t, err, ErrInvoiceCategoryNameRequired)
	_, err = svc.CreateCategory(context.Background(), InvoiceCategoryInput{Name: str(strings.Repeat("类", invoiceCategoryNameMax+1))})
	require.ErrorIs(t, err, ErrInvoiceCategoryTooLong)
	_, err = svc.CreateCategory(context.Background(), InvoiceCategoryInput{Name: str("技术服务费"), Description: str(strings.Repeat("说", invoiceCategoryDescMax+1))})
	require.ErrorIs(t, err, ErrInvoiceCategoryTooLong)

	// 刚好等于上限的名称可以通过。
	_, err = svc.CreateCategory(context.Background(), InvoiceCategoryInput{Name: str(strings.Repeat("类", invoiceCategoryNameMax))})
	require.NoError(t, err)
}

func TestUpdateCategoryOnlyChangesProvidedFields(t *testing.T) {
	svc, _ := categoryTestService(InvoiceCategory{ID: 1, Name: "技术服务费", Description: "原说明", Enabled: true, SortOrder: 3})
	disabled := false

	updated, err := svc.UpdateCategory(context.Background(), 1, InvoiceCategoryInput{Enabled: &disabled})
	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.Equal(t, "技术服务费", updated.Name)
	require.Equal(t, "原说明", updated.Description)
	require.Equal(t, 3, updated.SortOrder)

	blank := " "
	_, err = svc.UpdateCategory(context.Background(), 1, InvoiceCategoryInput{Name: &blank})
	require.ErrorIs(t, err, ErrInvoiceCategoryNameRequired)

	_, err = svc.UpdateCategory(context.Background(), 404, InvoiceCategoryInput{Enabled: &disabled})
	require.ErrorIs(t, err, ErrInvoiceCategoryNotFound)
}

func TestReorderCategoriesValidatesIDs(t *testing.T) {
	svc, repo := categoryTestService()

	for name, ids := range map[string][]int64{
		"empty":     {},
		"duplicate": {1, 2, 1},
		"zero":      {1, 0},
		"negative":  {-3, 2},
	} {
		require.ErrorIs(t, svc.ReorderCategories(context.Background(), ids), ErrInvoiceCategoryReorderInvalid, name)
	}
	require.Nil(t, repo.reordered, "invalid input never reaches the repository")

	require.NoError(t, svc.ReorderCategories(context.Background(), []int64{3, 1, 2}))
	require.Equal(t, []int64{3, 1, 2}, repo.reordered)
}
