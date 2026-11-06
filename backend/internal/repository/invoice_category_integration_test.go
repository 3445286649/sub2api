//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 集成测试共用一个数据库：只要库里有启用的类目，其他不带类目的申请用例就会被要求选类目。
// 所以每个用例开始前和结束后都把所有类目停用，只让自己创建的类目处于启用状态。
func resetInvoiceCategories(t *testing.T) {
	t.Helper()
	disableAll := func() {
		_, err := integrationDB.ExecContext(context.Background(), `UPDATE invoice_categories SET enabled = FALSE`)
		require.NoError(t, err)
	}
	disableAll()
	t.Cleanup(disableAll)
}

func mustCreateInvoiceCategory(t *testing.T, repo service.InvoiceRepository, name string, sortOrder int) *service.InvoiceCategory {
	t.Helper()
	category := &service.InvoiceCategory{Name: invoiceTestUnique(name), Description: name + "说明", Enabled: true, SortOrder: sortOrder}
	require.NoError(t, repo.CreateCategory(context.Background(), category))
	require.NotZero(t, category.ID)
	return category
}

func findInvoiceCategory(t *testing.T, items []service.InvoiceCategory, id int64) service.InvoiceCategory {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	require.FailNow(t, "category not found in list", "id %d", id)
	return service.InvoiceCategory{}
}

func TestInvoiceCategoryRepositoryEnforcesUniqueNamesAndReportsMissingRows(t *testing.T) {
	ctx := context.Background()
	resetInvoiceCategories(t)
	_, repo, _ := newInvoiceIntegrationService(t)
	first := mustCreateInvoiceCategory(t, repo, "技术服务费", 1)
	second := mustCreateInvoiceCategory(t, repo, "软件使用费", 2)

	duplicate := &service.InvoiceCategory{Name: first.Name, Enabled: true}
	require.ErrorIs(t, repo.CreateCategory(ctx, duplicate), service.ErrInvoiceCategoryDuplicate)

	second.Name = first.Name
	require.ErrorIs(t, repo.UpdateCategory(ctx, second), service.ErrInvoiceCategoryDuplicate, "renaming onto an existing name is refused")

	missing := &service.InvoiceCategory{ID: 999999999, Name: invoiceTestUnique("missing")}
	require.ErrorIs(t, repo.UpdateCategory(ctx, missing), service.ErrInvoiceCategoryNotFound)
	_, err := repo.GetCategory(ctx, 999999999)
	require.ErrorIs(t, err, service.ErrInvoiceCategoryNotFound)
	require.ErrorIs(t, repo.DeleteCategory(ctx, 999999999), service.ErrInvoiceCategoryNotFound)
}

func TestInvoiceCategoryListFiltersDisabledOnesForUsersAndOrdersBySortThenID(t *testing.T) {
	ctx := context.Background()
	resetInvoiceCategories(t)
	_, repo, _ := newInvoiceIntegrationService(t)
	late := mustCreateInvoiceCategory(t, repo, "后排", 20)
	early := mustCreateInvoiceCategory(t, repo, "前排", 10)
	off := mustCreateInvoiceCategory(t, repo, "已停用", 15)
	off.Enabled = false
	require.NoError(t, repo.UpdateCategory(ctx, off))

	enabled, err := repo.ListCategories(ctx, true)
	require.NoError(t, err)
	require.Len(t, enabled, 2, "only the categories enabled by this test are returned to users")
	require.Equal(t, early.ID, enabled[0].ID)
	require.Equal(t, late.ID, enabled[1].ID)

	all, err := repo.ListCategories(ctx, false)
	require.NoError(t, err)
	require.False(t, findInvoiceCategory(t, all, off.ID).Enabled, "admins also see disabled categories")
}

func TestInvoiceCategoryInUseKeepsItsSnapshotAndCannotBeDeleted(t *testing.T) {
	ctx := context.Background()
	resetInvoiceCategories(t)
	svc, repo, user := newInvoiceIntegrationService(t)
	used := mustCreateInvoiceCategory(t, repo, "技术服务费", 1)
	unused := mustCreateInvoiceCategory(t, repo, "闲置类目", 2)
	order := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 80)

	input := enterpriseInvoiceInput(user.ID, order.ID)
	input.CategoryID = &used.ID
	created, err := svc.CreateRequest(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, created.CategoryID)
	require.Equal(t, used.ID, *created.CategoryID)
	require.NotNil(t, created.CategoryName)
	require.Equal(t, used.Name, *created.CategoryName)

	listed, err := repo.ListCategories(ctx, false)
	require.NoError(t, err)
	require.Equal(t, 1, findInvoiceCategory(t, listed, used.ID).RequestCount)
	require.Equal(t, 0, findInvoiceCategory(t, listed, unused.ID).RequestCount)

	require.ErrorIs(t, repo.DeleteCategory(ctx, used.ID), service.ErrInvoiceCategoryInUse)
	require.NoError(t, repo.DeleteCategory(ctx, unused.ID), "an unused category can be deleted")
	_, err = repo.GetCategory(ctx, unused.ID)
	require.ErrorIs(t, err, service.ErrInvoiceCategoryNotFound)

	// 类目之后改名或停用，已提交申请里的名称快照不变。
	oldName := used.Name
	used.Name = invoiceTestUnique("改名后")
	used.Enabled = false
	require.NoError(t, repo.UpdateCategory(ctx, used))
	after, err := repo.GetRequest(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, after.CategoryName)
	require.Equal(t, oldName, *after.CategoryName)
	require.Equal(t, used.ID, *after.CategoryID)

	// 没有启用类目后恢复旧流程：不选类目也能提交，两列都留空。
	legacyOrder := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 30)
	legacy, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, legacyOrder.ID))
	require.NoError(t, err)
	require.Nil(t, legacy.CategoryID)
	require.Nil(t, legacy.CategoryName)
}

func TestInvoiceRequestsCanBeFilteredByCategory(t *testing.T) {
	ctx := context.Background()
	resetInvoiceCategories(t)
	svc, repo, user := newInvoiceIntegrationService(t)
	tech := mustCreateInvoiceCategory(t, repo, "技术服务费", 1)
	soft := mustCreateInvoiceCategory(t, repo, "软件使用费", 2)

	for _, category := range []*service.InvoiceCategory{tech, soft} {
		order := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 50)
		input := enterpriseInvoiceInput(user.ID, order.ID)
		input.CategoryID = &category.ID
		_, err := svc.CreateRequest(ctx, input)
		require.NoError(t, err)
	}

	items, result, err := repo.ListRequests(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, service.InvoiceRequestListFilters{CategoryID: tech.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].CategoryName)
	require.Equal(t, tech.Name, *items[0].CategoryName)

	_, result, err = repo.ListRequests(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, service.InvoiceRequestListFilters{CategoryID: soft.ID, UserID: user.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total, "the category filter combines with the other filters")
}

func TestInvoiceCategoryReorderIsAtomic(t *testing.T) {
	ctx := context.Background()
	resetInvoiceCategories(t)
	_, repo, _ := newInvoiceIntegrationService(t)
	a := mustCreateInvoiceCategory(t, repo, "甲", 1)
	b := mustCreateInvoiceCategory(t, repo, "乙", 2)
	c := mustCreateInvoiceCategory(t, repo, "丙", 3)

	require.NoError(t, repo.ReorderCategories(ctx, []int64{c.ID, a.ID, b.ID}))
	listed, err := repo.ListCategories(ctx, true)
	require.NoError(t, err)
	ordered := make([]int64, 0, 3)
	for _, item := range listed {
		if item.ID == a.ID || item.ID == b.ID || item.ID == c.ID {
			ordered = append(ordered, item.ID)
		}
	}
	require.Equal(t, []int64{c.ID, a.ID, b.ID}, ordered)

	// 列表里有不存在的 ID 时整个事务回滚，已更新的行也要恢复。
	before, err := repo.GetCategory(ctx, a.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.ReorderCategories(ctx, []int64{b.ID, a.ID, 999999999}), service.ErrInvoiceCategoryNotFound)
	after, err := repo.GetCategory(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, before.SortOrder, after.SortOrder, "a failed reorder leaves every row untouched")
}
