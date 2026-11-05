//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var invoiceTestSeq atomic.Int64

func invoiceTestUnique(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), invoiceTestSeq.Add(1))
}

func mustCreateInvoiceOrder(t *testing.T, userID int64, email, status string, pay float64) *dbent.PaymentOrder {
	t.Helper()
	builder := integrationEntClient.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail(email).
		SetUserName("invoice-test").
		SetAmount(pay).
		SetPayAmount(pay).
		SetFeeRate(0).
		SetRechargeCode(invoiceTestUnique("PAY")).
		SetOutTradeNo(invoiceTestUnique("sub2")).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus(status).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com")
	if status == "COMPLETED" {
		builder = builder.SetPaidAt(time.Now())
	}
	order, err := builder.Save(context.Background())
	require.NoError(t, err)
	return order
}

func newInvoiceIntegrationService(t *testing.T) (*service.InvoiceService, service.InvoiceRepository, *service.User) {
	t.Helper()
	repo := NewInvoiceRepository(integrationDB)
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: invoiceTestUnique("invoice") + "@example.com"})
	// 这些用例不走加密和邮件分支，依赖传 nil 即可。
	return service.NewInvoiceService(repo, nil, nil, nil, nil), repo, user
}

func enterpriseInvoiceInput(userID int64, orderIDs ...int64) service.CreateInvoiceRequestInput {
	return service.CreateInvoiceRequestInput{
		UserID:              userID,
		InvoiceType:         service.InvoiceTypeEnterprise,
		OrderIDs:            orderIDs,
		EnterpriseName:      "杭州云栈科技有限公司",
		EnterpriseTaxNumber: "91330106MA2KXXXX7Q",
		ReceiverEmail:       "finance@example.com",
	}
}

func TestInvoiceRequestLifecycleReleasesOrdersOnlyWhenRejected(t *testing.T) {
	ctx := context.Background()
	svc, _, user := newInvoiceIntegrationService(t)
	first := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 100.10)
	second := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 200.20)
	mustCreateInvoiceOrder(t, user.ID, user.Email, "PENDING", 999)

	items, summary, err := svc.ListInvoiceableOrders(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, items, 2, "unpaid orders are not listed")
	require.InDelta(t, 300.30, summary.TotalInvoiceableAmount, 0.001)

	created, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, first.ID, second.ID))
	require.NoError(t, err)
	require.InDelta(t, 300.30, created.TotalAmount, 0.001)
	require.Equal(t, user.Email, created.UserEmail)
	require.Len(t, created.Orders, 2)

	items, summary, err = svc.ListInvoiceableOrders(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 0, summary.InvoiceableCount)
	for _, item := range items {
		require.True(t, item.AlreadyInvoiced)
		require.Equal(t, created.ID, *item.InvoiceRequestID)
	}

	_, err = svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, first.ID))
	require.ErrorIs(t, err, service.ErrInvoiceOrderNotInvoiceable, "claimed orders cannot be requested twice")

	notes := "已接手"
	processing, err := svc.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusProcessing, AdminNotes: &notes})
	require.NoError(t, err)
	require.Equal(t, service.InvoiceStatusProcessing, processing.Status)
	require.NotNil(t, processing.ProcessingAt)
	require.Equal(t, "已接手", *processing.AdminNotes)

	_, err = svc.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusProcessing})
	require.ErrorIs(t, err, service.ErrInvoiceStatusTransition)

	rejected, err := svc.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusRejected, RejectReason: "抬头有误"})
	require.NoError(t, err)
	require.NotNil(t, rejected.RejectedAt)
	require.Equal(t, "已接手", *rejected.AdminNotes, "notes survive when the caller does not send them")
	require.Len(t, rejected.Orders, 2, "history keeps the orders of a rejected request")

	_, err = svc.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusCompleted, InvoiceNumber: "1", InvoiceFileURL: "https://a.cn/x.pdf"})
	require.ErrorIs(t, err, service.ErrInvoiceStatusTransition, "rejected is terminal")

	_, summary, err = svc.ListInvoiceableOrders(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 2, summary.InvoiceableCount, "rejection releases the orders")

	again, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, first.ID))
	require.NoError(t, err)

	completed, err := svc.UpdateStatus(ctx, again.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusCompleted, InvoiceNumber: "26332000000184527761", InvoiceFileURL: "https://a.cn/x.pdf"})
	require.NoError(t, err)
	require.Equal(t, "26332000000184527761", *completed.InvoiceNumber)
	require.NotNil(t, completed.ProcessingAt, "direct completion also stamps processing")
	require.NotNil(t, completed.CompletedAt)

	_, err = svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, first.ID))
	require.ErrorIs(t, err, service.ErrInvoiceOrderNotInvoiceable, "issued orders stay claimed")
}

func TestInvoiceRequestConcurrentSubmissionsClaimAnOrderOnce(t *testing.T) {
	ctx := context.Background()
	svc, _, user := newInvoiceIntegrationService(t)
	order := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 100)

	const attempts = 8
	var wg sync.WaitGroup
	var succeeded, refused atomic.Int64
	errs := make(chan error, attempts)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, order.ID))
			switch {
			case err == nil:
				succeeded.Add(1)
			case errors.Is(err, service.ErrInvoiceOrderNotInvoiceable):
				refused.Add(1)
			default:
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, succeeded.Load())
	require.EqualValues(t, attempts-1, refused.Load())
}

func TestInvoiceRequestCannotClaimAnotherUsersOrder(t *testing.T) {
	ctx := context.Background()
	svc, _, owner := newInvoiceIntegrationService(t)
	other := mustCreateUser(t, integrationEntClient, &service.User{Email: invoiceTestUnique("other") + "@example.com"})
	order := mustCreateInvoiceOrder(t, owner.ID, owner.Email, "COMPLETED", 100)

	_, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(other.ID, order.ID))
	require.ErrorIs(t, err, service.ErrInvoiceOrderNotInvoiceable)
}

func TestInvoiceExclusionBlocksRequestsUntilRemoved(t *testing.T) {
	ctx := context.Background()
	svc, _, user := newInvoiceIntegrationService(t)
	byOutTradeNo := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 100)
	byRechargeCode := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 200)
	byID := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 300)
	open := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 400)
	unpaid := mustCreateInvoiceOrder(t, user.ID, user.Email, "PENDING", 500)

	result, err := svc.AddExclusions(ctx, []string{
		byOutTradeNo.OutTradeNo, byRechargeCode.RechargeCode, fmt.Sprint(byID.ID), unpaid.OutTradeNo, "no-such-order",
	}, user.ID)
	require.NoError(t, err)
	require.Len(t, result.Added, 3)
	require.Equal(t, []string{unpaid.OutTradeNo}, result.Unpaid)
	require.Equal(t, []string{"no-such-order"}, result.NotFound)

	again, err := svc.AddExclusions(ctx, []string{byOutTradeNo.OutTradeNo}, user.ID)
	require.NoError(t, err)
	require.Empty(t, again.Added)
	require.Equal(t, []string{byOutTradeNo.OutTradeNo}, again.AlreadyExcluded)

	items, summary, err := svc.ListInvoiceableOrders(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, items, 4, "excluded orders stay visible but marked")
	require.Equal(t, 1, summary.InvoiceableCount)
	require.InDelta(t, 400, summary.TotalInvoiceableAmount, 0.001)
	for _, item := range items {
		require.Equal(t, item.ID != open.ID, item.Excluded)
		require.Equal(t, item.ID != open.ID, item.AlreadyInvoiced)
		require.Nil(t, item.InvoiceRequestID)
	}

	_, err = svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, byOutTradeNo.ID))
	require.ErrorIs(t, err, service.ErrInvoiceOrderNotInvoiceable)

	params := pagination.PaginationParams{Page: 1, PageSize: 10}
	listed, page, err := svc.ListExclusions(ctx, params, byRechargeCode.RechargeCode)
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, byRechargeCode.ID, listed[0].OrderID)
	require.Equal(t, user.Email, listed[0].UserEmail)

	require.NoError(t, svc.DeleteExclusion(ctx, listed[0].ID))
	require.ErrorIs(t, svc.DeleteExclusion(ctx, listed[0].ID), service.ErrInvoiceExclusionNotFound)

	_, err = svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, byRechargeCode.ID))
	require.NoError(t, err, "removing the exclusion makes the order requestable again")
}

func TestInvoiceRejectionDoesNotReleaseAnExcludedOrder(t *testing.T) {
	ctx := context.Background()
	svc, _, user := newInvoiceIntegrationService(t)
	order := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 100)

	created, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, order.ID))
	require.NoError(t, err)
	_, err = svc.AddExclusions(ctx, []string{order.OutTradeNo}, user.ID)
	require.NoError(t, err)
	_, err = svc.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusRejected, RejectReason: "已线下开票"})
	require.NoError(t, err)

	_, err = svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, order.ID))
	require.ErrorIs(t, err, service.ErrInvoiceOrderNotInvoiceable)
}

func TestInvoiceRequestListFiltersAndStats(t *testing.T) {
	ctx := context.Background()
	svc, repo, user := newInvoiceIntegrationService(t)

	before, err := repo.GetStats(ctx, time.Now().Add(-time.Minute))
	require.NoError(t, err)

	firstOrder := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 100)
	secondOrder := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 50.5)
	first, err := svc.CreateRequest(ctx, enterpriseInvoiceInput(user.ID, firstOrder.ID))
	require.NoError(t, err)
	individual := service.CreateInvoiceRequestInput{UserID: user.ID, InvoiceType: service.InvoiceTypeIndividual, OrderIDs: []int64{secondOrder.ID}, IndividualName: "张_三%", ReceiverEmail: "a@b.cn"}
	second, err := svc.CreateRequest(ctx, individual)
	require.NoError(t, err)
	_, err = svc.UpdateStatus(ctx, second.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusCompleted, InvoiceNumber: "1", InvoiceFileURL: "https://a.cn/x.pdf"})
	require.NoError(t, err)

	after, err := repo.GetStats(ctx, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.Equal(t, before.Pending+1, after.Pending)
	require.Equal(t, before.Completed+1, after.Completed)
	require.Equal(t, before.TodayNew+2, after.TodayNew)
	require.InDelta(t, before.OpenAmount+100, after.OpenAmount, 0.001)
	require.InDelta(t, before.CompletedAmount+50.5, after.CompletedAmount, 0.001)

	params := pagination.PaginationParams{Page: 1, PageSize: 1}
	mine, page, err := svc.ListForUser(ctx, user.ID, params, "")
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Len(t, mine, 1, "page size applies")
	require.Equal(t, second.ID, mine[0].ID, "newest first")

	mine, page, err = svc.ListForUser(ctx, user.ID, pagination.PaginationParams{Page: 2, PageSize: 1}, "")
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Equal(t, first.ID, mine[0].ID, "offset applies")

	wide := pagination.PaginationParams{Page: 1, PageSize: 20}
	found, _, err := svc.ListForAdmin(ctx, wide, service.InvoiceRequestListFilters{Keyword: user.Email, Status: service.InvoiceStatusPending, InvoiceType: service.InvoiceTypeEnterprise})
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, first.ID, found[0].ID)

	found, _, err = svc.ListForAdmin(ctx, wide, service.InvoiceRequestListFilters{Keyword: fmt.Sprint(second.ID)})
	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, item := range found {
		ids = append(ids, item.ID)
	}
	require.Contains(t, ids, second.ID, "keyword also matches the request id")

	found, _, err = svc.ListForAdmin(ctx, wide, service.InvoiceRequestListFilters{Keyword: "张_三%"})
	require.NoError(t, err)
	require.NotEmpty(t, found, "like wildcards in the keyword are matched literally")
	found, _, err = svc.ListForAdmin(ctx, wide, service.InvoiceRequestListFilters{Keyword: "张x三"})
	require.NoError(t, err)
	require.Empty(t, found)
}
