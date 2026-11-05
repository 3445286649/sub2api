//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type invoiceRefundFixture struct {
	invoices *service.InvoiceService
	payments *service.PaymentService
	repo     service.InvoiceRepository
	user     *service.User
	instance string
}

func newInvoiceRefundFixture(t *testing.T) *invoiceRefundFixture {
	t.Helper()
	ctx := context.Background()
	repo := NewInvoiceRepository(integrationDB)
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email:   invoiceTestUnique("refund") + "@example.com",
		Balance: 100000,
	})
	instance, err := integrationEntClient.PaymentProviderInstance.Create().
		SetProviderKey("alipay").
		SetName(invoiceTestUnique("refund-provider")).
		SetConfig("{}").
		SetRefundEnabled(true).
		SetAllowUserRefund(true).
		Save(ctx)
	require.NoError(t, err)

	payments := service.NewPaymentService(integrationEntClient, nil, nil, nil, nil, nil, NewUserRepository(integrationEntClient, integrationDB), nil, nil)
	payments.SetInvoiceRepository(repo)
	return &invoiceRefundFixture{
		invoices: service.NewInvoiceService(repo, nil, nil, nil, nil),
		payments: payments,
		repo:     repo,
		user:     user,
		instance: fmt.Sprint(instance.ID),
	}
}

func (f *invoiceRefundFixture) order(t *testing.T) *dbent.PaymentOrder {
	t.Helper()
	order := mustCreateInvoiceOrder(t, f.user.ID, f.user.Email, "COMPLETED", 100)
	order, err := integrationEntClient.PaymentOrder.UpdateOneID(order.ID).SetProviderInstanceID(f.instance).Save(context.Background())
	require.NoError(t, err)
	return order
}

func (f *invoiceRefundFixture) orderStatus(t *testing.T, id int64) *dbent.PaymentOrder {
	t.Helper()
	order, err := integrationEntClient.PaymentOrder.Get(context.Background(), id)
	require.NoError(t, err)
	return order
}

func TestUserRefundIsAllowedForOrdersWithoutInvoice(t *testing.T) {
	f := newInvoiceRefundFixture(t)
	order := f.order(t)

	require.NoError(t, f.payments.RequestRefund(context.Background(), order.ID, f.user.ID, "不用了"))

	got := f.orderStatus(t, order.ID)
	require.Equal(t, "REFUND_REQUESTED", got.Status)
	require.Equal(t, "不用了", *got.RefundRequestReason)
}

func TestUserRefundIsBlockedWhileOrderIsClaimedByARequest(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceRefundFixture(t)
	order := f.order(t)
	created, err := f.invoices.CreateRequest(ctx, enterpriseInvoiceInput(f.user.ID, order.ID))
	require.NoError(t, err)

	assertBlocked := func(stage string) {
		t.Helper()
		err := f.payments.RequestRefund(ctx, order.ID, f.user.ID, "想退款")
		require.ErrorIs(t, err, service.ErrOrderInvoicedRefundBlocked, stage)
		got := f.orderStatus(t, order.ID)
		require.Equal(t, "COMPLETED", got.Status, "%s: the refund update must roll back", stage)
		require.Nil(t, got.RefundRequestedAt, stage)
		require.Zero(t, got.RefundAmount, stage)
	}

	assertBlocked("pending")
	_, err = f.invoices.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusProcessing})
	require.NoError(t, err)
	assertBlocked("processing")
	_, err = f.invoices.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusCompleted, InvoiceNumber: "26332", InvoiceFileURL: "https://a.cn/x.pdf"})
	require.NoError(t, err)
	assertBlocked("completed")

	state, err := f.invoices.GetOrderInvoiceState(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, state.Invoiced())
	require.Equal(t, created.ID, *state.RequestID)
	require.Equal(t, service.InvoiceStatusCompleted, *state.RequestStatus)
	require.Equal(t, "26332", *state.InvoiceNumber)
}

func TestUserRefundIsAllowedAgainAfterTheRequestIsRejected(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceRefundFixture(t)
	order := f.order(t)
	created, err := f.invoices.CreateRequest(ctx, enterpriseInvoiceInput(f.user.ID, order.ID))
	require.NoError(t, err)
	_, err = f.invoices.UpdateStatus(ctx, created.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusRejected, RejectReason: "抬头有误"})
	require.NoError(t, err)

	state, err := f.invoices.GetOrderInvoiceState(ctx, order.ID)
	require.NoError(t, err)
	require.False(t, state.Invoiced())

	require.NoError(t, f.payments.RequestRefund(ctx, order.ID, f.user.ID, "想退款"))
}

func TestUserRefundIsBlockedForManuallyExcludedOrders(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceRefundFixture(t)
	order := f.order(t)
	_, err := f.invoices.AddExclusions(ctx, []string{order.OutTradeNo}, f.user.ID)
	require.NoError(t, err)

	state, err := f.invoices.GetOrderInvoiceState(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, state.Excluded)
	require.Nil(t, state.RequestID)

	require.ErrorIs(t, f.payments.RequestRefund(ctx, order.ID, f.user.ID, "想退款"), service.ErrOrderInvoicedRefundBlocked)
}

func TestOrderInRefundFlowCannotBeInvoiced(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceRefundFixture(t)
	order := f.order(t)
	require.NoError(t, f.payments.RequestRefund(ctx, order.ID, f.user.ID, "想退款"))

	_, err := f.invoices.CreateRequest(ctx, enterpriseInvoiceInput(f.user.ID, order.ID))
	require.ErrorIs(t, err, service.ErrInvoiceOrderNotInvoiceable)
}

func TestGetOrderInvoiceStateForAnUnknownOrderIsNotInvoiced(t *testing.T) {
	state, err := NewInvoiceRepository(integrationDB).GetOrderInvoiceState(context.Background(), 987654321)
	require.NoError(t, err)
	require.False(t, state.Invoiced())
}

// 开票和退款同时发生时只能成功一个，这是两边各查一次防不住的竞争。
func TestConcurrentInvoiceAndRefundNeverBothSucceed(t *testing.T) {
	ctx := context.Background()
	f := newInvoiceRefundFixture(t)

	for round := range 40 {
		order := f.order(t)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var invoiceErr, refundErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, invoiceErr = f.invoices.CreateRequest(ctx, enterpriseInvoiceInput(f.user.ID, order.ID))
		}()
		go func() {
			defer wg.Done()
			<-start
			refundErr = f.payments.RequestRefund(ctx, order.ID, f.user.ID, "想退款")
		}()
		close(start)
		wg.Wait()

		if invoiceErr != nil {
			require.ErrorIs(t, invoiceErr, service.ErrInvoiceOrderNotInvoiceable, "round %d", round)
		}
		if refundErr != nil {
			require.True(t, errors.Is(refundErr, service.ErrOrderInvoicedRefundBlocked), "round %d: %v", round, refundErr)
		}
		require.False(t, invoiceErr == nil && refundErr == nil, "round %d: both invoice and refund succeeded", round)
		require.False(t, invoiceErr != nil && refundErr != nil, "round %d: neither succeeded", round)
	}
}
