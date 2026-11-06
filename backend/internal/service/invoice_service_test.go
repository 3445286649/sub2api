//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type fakeInvoiceRepo struct {
	InvoiceRepository
	rows       []InvoiceOrderRow
	request    *InvoiceRequest
	lines      []InvoiceOrderLine
	updateOK   bool
	lastAdded  []int64
	matches    []InvoiceOrderMatch
	emailedAt  *time.Time
	categories []InvoiceCategory
	reordered  []int64
}

func (f *fakeInvoiceRepo) ListOrdersForUser(context.Context, int64, []string) ([]InvoiceOrderRow, error) {
	return f.rows, nil
}

func (f *fakeInvoiceRepo) CreateRequest(_ context.Context, req *InvoiceRequest, _ []int64, buildLines func([]InvoiceOrderRow) ([]InvoiceOrderLine, error)) error {
	lines, err := buildLines(f.rows)
	if err != nil {
		return err
	}
	f.lines = lines
	req.ID = 1001
	f.request = req
	return nil
}

func (f *fakeInvoiceRepo) GetRequest(_ context.Context, id int64) (*InvoiceRequest, error) {
	if f.request == nil || f.request.ID != id {
		return nil, ErrInvoiceRequestNotFound
	}
	copied := *f.request
	return &copied, nil
}

func (f *fakeInvoiceRepo) ListRequests(context.Context, pagination.PaginationParams, InvoiceRequestListFilters) ([]InvoiceRequest, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}

func (f *fakeInvoiceRepo) UpdateStatus(_ context.Context, _ int64, update InvoiceStatusUpdate) (bool, error) {
	if !f.updateOK {
		return false, nil
	}
	f.request.Status = update.Status
	f.request.InvoiceNumber = update.InvoiceNumber
	f.request.InvoiceFileURL = update.InvoiceFileURL
	return true, nil
}

func (f *fakeInvoiceRepo) MarkEmailSent(_ context.Context, _ int64, at time.Time) error {
	f.emailedAt = &at
	return nil
}

func (f *fakeInvoiceRepo) ListCategories(_ context.Context, enabledOnly bool) ([]InvoiceCategory, error) {
	out := make([]InvoiceCategory, 0, len(f.categories))
	for _, c := range f.categories {
		if enabledOnly && !c.Enabled {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeInvoiceRepo) GetCategory(_ context.Context, id int64) (*InvoiceCategory, error) {
	for i := range f.categories {
		if f.categories[i].ID == id {
			c := f.categories[i]
			return &c, nil
		}
	}
	return nil, ErrInvoiceCategoryNotFound
}

func (f *fakeInvoiceRepo) CreateCategory(_ context.Context, c *InvoiceCategory) error {
	c.ID = int64(len(f.categories) + 1)
	f.categories = append(f.categories, *c)
	return nil
}

func (f *fakeInvoiceRepo) UpdateCategory(_ context.Context, c *InvoiceCategory) error {
	for i := range f.categories {
		if f.categories[i].ID == c.ID {
			f.categories[i] = *c
			return nil
		}
	}
	return ErrInvoiceCategoryNotFound
}

func (f *fakeInvoiceRepo) ReorderCategories(_ context.Context, ids []int64) error {
	f.reordered = ids
	return nil
}

func (f *fakeInvoiceRepo) MatchOrders(context.Context, []string, []int64) ([]InvoiceOrderMatch, error) {
	return f.matches, nil
}

func (f *fakeInvoiceRepo) AddExclusions(_ context.Context, ids []int64, _ int64) ([]InvoiceExclusion, error) {
	f.lastAdded = ids
	out := make([]InvoiceExclusion, 0, len(ids))
	for _, id := range ids {
		out = append(out, InvoiceExclusion{ID: id, OrderID: id})
	}
	return out, nil
}

type fakeInvoiceEncryptor struct{ failDecrypt bool }

func (f fakeInvoiceEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (f fakeInvoiceEncryptor) Decrypt(ciphertext string) (string, error) {
	if f.failDecrypt {
		return "", errors.New("wrong key")
	}
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}

type fakeInvoiceMailer struct {
	err error
	to  string
}

func (f *fakeInvoiceMailer) SendEmail(_ context.Context, to, _, _ string) error {
	f.to = to
	return f.err
}

type fakeInvoiceSite struct{}

func (fakeInvoiceSite) GetSiteName(context.Context) string { return "CodePlan" }

func newInvoiceTestService(repo *fakeInvoiceRepo, mailer *fakeInvoiceMailer) *InvoiceService {
	if mailer == nil {
		mailer = &fakeInvoiceMailer{}
	}
	return &InvoiceService{repo: repo, email: mailer, site: fakeInvoiceSite{}, encryptor: fakeInvoiceEncryptor{}}
}

func paidInvoiceRow(id int64, pay float64) InvoiceOrderRow {
	return InvoiceOrderRow{ID: id, Status: payment.OrderStatusCompleted, Amount: pay, PayAmount: pay}
}

func validEnterpriseInvoiceInput(orderIDs ...int64) CreateInvoiceRequestInput {
	return CreateInvoiceRequestInput{
		UserID:              7,
		InvoiceType:         InvoiceTypeEnterprise,
		OrderIDs:            orderIDs,
		EnterpriseName:      " 杭州云栈科技有限公司 ",
		EnterpriseTaxNumber: "91330106ma2kxxxx7q",
		ReceiverEmail:       " finance@example.com ",
	}
}

func TestInvoiceableAmount(t *testing.T) {
	cases := []struct {
		name string
		row  InvoiceOrderRow
		want float64
	}{
		{"completed uses the paid amount, not the credited amount with bonus", InvoiceOrderRow{Status: payment.OrderStatusCompleted, Amount: 110, PayAmount: 100, BonusAmount: 10}, 100},
		{"stale refund amount on a completed order is ignored", InvoiceOrderRow{Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100, RefundAmount: 100}, 100},
		{"partial refund converts credited refund back to paid money", InvoiceOrderRow{Status: payment.OrderStatusPartiallyRefunded, Amount: 200, PayAmount: 100, RefundAmount: 50}, 75},
		{"fully refunded order is not invoiceable", InvoiceOrderRow{Status: payment.OrderStatusRefunded, Amount: 100, PayAmount: 100, RefundAmount: 100}, 0},
		{"order in refund flow is not invoiceable", InvoiceOrderRow{Status: payment.OrderStatusRefundRequested, Amount: 100, PayAmount: 100}, 0},
		{"unpaid order is not invoiceable", InvoiceOrderRow{Status: payment.OrderStatusPending, Amount: 100, PayAmount: 100}, 0},
		{"foreign currency order is not invoiceable", InvoiceOrderRow{Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100, Currency: "USD"}, 0},
		{"order without recorded currency counts as CNY", InvoiceOrderRow{Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100, Currency: ""}, 100},
		{"lowercase CNY is accepted", InvoiceOrderRow{Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100, Currency: "cny"}, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.want, invoiceableAmount(tc.row), 0.001)
		})
	}
}

func TestListInvoiceableOrdersMarksClaimedAndExcludedAndLeavesThemOutOfTheSummary(t *testing.T) {
	requestID := int64(55)
	claimed := paidInvoiceRow(2, 200)
	claimed.InvoiceRequestID = &requestID
	excluded := paidInvoiceRow(3, 300)
	excluded.Excluded = true
	refunded := paidInvoiceRow(4, 400)
	refunded.Status = payment.OrderStatusRefunded

	svc := newInvoiceTestService(&fakeInvoiceRepo{rows: []InvoiceOrderRow{paidInvoiceRow(1, 100.10), claimed, excluded, refunded}}, nil)
	items, summary, err := svc.ListInvoiceableOrders(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, items, 3, "fully refunded orders disappear from the list")
	require.False(t, items[0].AlreadyInvoiced)
	require.True(t, items[1].AlreadyInvoiced)
	require.False(t, items[1].Excluded)
	require.True(t, items[2].AlreadyInvoiced)
	require.True(t, items[2].Excluded)
	require.InDelta(t, 100.10, summary.TotalInvoiceableAmount, 0.001)
	require.Equal(t, 1, summary.InvoiceableCount)
}

func TestCreateRequestComputesTotalOnTheServerAndNormalizesTitle(t *testing.T) {
	repo := &fakeInvoiceRepo{rows: []InvoiceOrderRow{paidInvoiceRow(1, 0.1), paidInvoiceRow(2, 0.2)}}
	req, err := newInvoiceTestService(repo, nil).CreateRequest(context.Background(), validEnterpriseInvoiceInput(1, 2, 2))

	require.NoError(t, err)
	require.Equal(t, 0.3, req.TotalAmount)
	require.Equal(t, 2, req.OrderCount)
	require.Equal(t, "杭州云栈科技有限公司", req.Title)
	require.Equal(t, "91330106MA2KXXXX7Q", *req.EnterpriseTaxNumber)
	require.Equal(t, "finance@example.com", req.ReceiverEmail)
	require.Equal(t, InvoiceStatusPending, req.Status)
}

func TestCreateRequestRejectsOrdersThatCannotBeInvoiced(t *testing.T) {
	requestID := int64(9)
	claimed := paidInvoiceRow(1, 100)
	claimed.InvoiceRequestID = &requestID
	excluded := paidInvoiceRow(1, 100)
	excluded.Excluded = true
	refunding := paidInvoiceRow(1, 100)
	refunding.Status = payment.OrderStatusRefundRequested

	cases := map[string][]InvoiceOrderRow{
		"claimed by another request":                {claimed},
		"manually excluded by admin":                {excluded},
		"entered the refund flow":                   {refunding},
		"belongs to another user (no row returned)": {},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeInvoiceRepo{rows: rows}
			_, err := newInvoiceTestService(repo, nil).CreateRequest(context.Background(), validEnterpriseInvoiceInput(1))
			require.ErrorIs(t, err, ErrInvoiceOrderNotInvoiceable)
			require.Nil(t, repo.request, "nothing may be written")
		})
	}
}

func TestCreateRequestValidatesTitle(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CreateInvoiceRequestInput)
		want   error
	}{
		{"no orders", func(in *CreateInvoiceRequestInput) { in.OrderIDs = nil }, ErrInvoiceNoOrders},
		{"bad type", func(in *CreateInvoiceRequestInput) { in.InvoiceType = "vat" }, ErrInvoiceTypeInvalid},
		{"bad email", func(in *CreateInvoiceRequestInput) { in.ReceiverEmail = "a@b\r\nBcc: x@y.cn" }, ErrInvoiceEmailInvalid},
		{"missing enterprise name", func(in *CreateInvoiceRequestInput) { in.EnterpriseName = " " }, ErrInvoiceTitleRequired},
		{"short tax number", func(in *CreateInvoiceRequestInput) { in.EnterpriseTaxNumber = "123" }, ErrInvoiceTaxNumberInvalid},
		{"bad bank account", func(in *CreateInvoiceRequestInput) { in.EnterpriseBankAccount = "62-22" }, ErrInvoiceBankAccountInvalid},
		{"missing individual name", func(in *CreateInvoiceRequestInput) { in.InvoiceType = InvoiceTypeIndividual; in.IndividualName = "" }, ErrInvoiceTitleRequired},
		{"bad id number", func(in *CreateInvoiceRequestInput) {
			in.InvoiceType = InvoiceTypeIndividual
			in.IndividualName = "张三"
			in.IndividualIDNumber = "123"
		}, ErrInvoiceIDNumberInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validEnterpriseInvoiceInput(1)
			tc.mutate(&input)
			_, err := newInvoiceTestService(&fakeInvoiceRepo{rows: []InvoiceOrderRow{paidInvoiceRow(1, 100)}}, nil).CreateRequest(context.Background(), input)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestIndividualIDNumberIsEncryptedAndOnlyAdminDetailDecrypts(t *testing.T) {
	repo := &fakeInvoiceRepo{rows: []InvoiceOrderRow{paidInvoiceRow(1, 100)}}
	svc := newInvoiceTestService(repo, nil)
	input := CreateInvoiceRequestInput{UserID: 7, InvoiceType: InvoiceTypeIndividual, OrderIDs: []int64{1}, IndividualName: "张三", IndividualIDNumber: "33010619900101123x", ReceiverEmail: "a@b.cn"}

	created, err := svc.CreateRequest(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "enc:33010619900101123X", *created.IDNumberEncrypted)
	require.Equal(t, "3301**********123X", *created.IDNumberMasked)
	require.Nil(t, created.IDNumberPlain)

	forAdmin, err := svc.GetForAdmin(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "33010619900101123X", *forAdmin.IDNumberPlain)

	// 加密密钥变了也不能让详情页报错，退回打码值
	svc.encryptor = fakeInvoiceEncryptor{failDecrypt: true}
	forAdmin, err = svc.GetForAdmin(context.Background(), created.ID)
	require.NoError(t, err)
	require.Nil(t, forAdmin.IDNumberPlain)
	require.Equal(t, "3301**********123X", *forAdmin.IDNumberMasked)
}

func TestGetForUserHidesOtherUsersRequests(t *testing.T) {
	repo := &fakeInvoiceRepo{request: &InvoiceRequest{ID: 1001, UserID: 7}}
	svc := newInvoiceTestService(repo, nil)

	_, err := svc.GetForUser(context.Background(), 8, 1001)
	require.ErrorIs(t, err, ErrInvoiceRequestNotFound)

	req, err := svc.GetForUser(context.Background(), 7, 1001)
	require.NoError(t, err)
	require.Equal(t, int64(1001), req.ID)
}

func TestUpdateStatusValidatesRequiredFieldsPerTarget(t *testing.T) {
	cases := []struct {
		name  string
		input UpdateInvoiceStatusInput
		want  error
	}{
		{"cannot move back to pending", UpdateInvoiceStatusInput{Status: InvoiceStatusPending}, ErrInvoiceStatusTransition},
		{"completion needs a number", UpdateInvoiceStatusInput{Status: InvoiceStatusCompleted, InvoiceFileURL: "https://a.cn/x.pdf"}, ErrInvoiceNumberRequired},
		{"completion needs a url", UpdateInvoiceStatusInput{Status: InvoiceStatusCompleted, InvoiceNumber: "1"}, ErrInvoiceFileURLInvalid},
		{"script urls are refused", UpdateInvoiceStatusInput{Status: InvoiceStatusCompleted, InvoiceNumber: "1", InvoiceFileURL: "javascript:alert(1)"}, ErrInvoiceFileURLInvalid},
		{"rejection needs a reason", UpdateInvoiceStatusInput{Status: InvoiceStatusRejected, RejectReason: "  "}, ErrInvoiceRejectReason},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeInvoiceRepo{request: &InvoiceRequest{ID: 1001}, updateOK: true}
			_, err := newInvoiceTestService(repo, nil).UpdateStatus(context.Background(), 1001, tc.input)
			require.ErrorIs(t, err, tc.want)
			require.Empty(t, repo.request.Status, "invalid input must not reach the repository")
		})
	}
}

func TestUpdateStatusDistinguishesMissingRequestFromBadTransition(t *testing.T) {
	input := UpdateInvoiceStatusInput{Status: InvoiceStatusProcessing}

	_, err := newInvoiceTestService(&fakeInvoiceRepo{}, nil).UpdateStatus(context.Background(), 1001, input)
	require.ErrorIs(t, err, ErrInvoiceRequestNotFound)

	repo := &fakeInvoiceRepo{request: &InvoiceRequest{ID: 1001, Status: InvoiceStatusCompleted}}
	_, err = newInvoiceTestService(repo, nil).UpdateStatus(context.Background(), 1001, input)
	require.ErrorIs(t, err, ErrInvoiceStatusTransition)
}

func TestCompletingWithEmailFailureKeepsTheIssuedStatus(t *testing.T) {
	repo := &fakeInvoiceRepo{request: &InvoiceRequest{ID: 1001, ReceiverEmail: "a@b.cn"}, updateOK: true}
	mailer := &fakeInvoiceMailer{err: errors.New("smtp down")}

	req, err := newInvoiceTestService(repo, mailer).UpdateStatus(context.Background(), 1001, UpdateInvoiceStatusInput{
		Status: InvoiceStatusCompleted, InvoiceNumber: "26332", InvoiceFileURL: "https://a.cn/x.pdf", SendEmail: true,
	})

	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCompleted, req.Status)
	require.Nil(t, req.EmailSentAt)
	require.Nil(t, repo.emailedAt)
}

func TestSendEmailRequiresIssuedInvoiceAndRecordsDelivery(t *testing.T) {
	repo := &fakeInvoiceRepo{request: &InvoiceRequest{ID: 1001, Status: InvoiceStatusProcessing, ReceiverEmail: "a@b.cn"}}
	mailer := &fakeInvoiceMailer{}
	svc := newInvoiceTestService(repo, mailer)

	_, err := svc.SendEmail(context.Background(), 1001)
	require.ErrorIs(t, err, ErrInvoiceEmailNotCompleted)

	repo.request.Status = InvoiceStatusCompleted
	req, err := svc.SendEmail(context.Background(), 1001)
	require.NoError(t, err)
	require.Equal(t, "a@b.cn", mailer.to)
	require.NotNil(t, req.EmailSentAt)
	require.NotNil(t, repo.emailedAt)

	mailer.err = errors.New("smtp down")
	_, err = svc.SendEmail(context.Background(), 1001)
	require.ErrorIs(t, err, ErrInvoiceEmailFailed)
}

func TestInvoiceEmailBodyEscapesUserInput(t *testing.T) {
	number := "1"
	fileURL := "https://a.cn/x.pdf?a=1&b=2"
	body := buildInvoiceEmailBody("CodePlan", &InvoiceRequest{ID: 1, Title: "<script>alert(1)</script>", InvoiceNumber: &number, InvoiceFileURL: &fileURL, TotalAmount: 12.5})

	require.NotContains(t, body, "<script>")
	require.Contains(t, body, "&lt;script&gt;")
	require.Contains(t, body, "a=1&amp;b=2")
	require.Contains(t, body, "¥12.50")
}

func TestAddExclusionsReportsEveryPastedNumber(t *testing.T) {
	repo := &fakeInvoiceRepo{matches: []InvoiceOrderMatch{
		{ID: 1, OutTradeNo: "sub2_a", RechargeCode: "PAY-1-1", Paid: true},
		{ID: 2, OutTradeNo: "sub2_b", RechargeCode: "PAY-2-2", Paid: true, Excluded: true},
		{ID: 3, OutTradeNo: "sub2_c", RechargeCode: "PAY-3-3", Paid: false},
		{ID: 4, OutTradeNo: "sub2_d", RechargeCode: "PAY-4-4", PaymentTradeNo: "shared", Paid: true},
		{ID: 5, OutTradeNo: "sub2_e", RechargeCode: "PAY-5-5", PaymentTradeNo: "shared", Paid: true},
		{ID: 6, OutTradeNo: "sub2_f", RechargeCode: "PAY-6-6", PaymentTradeNo: "2026110422001", Paid: true},
	}}
	numbers := []string{" sub2_a ", "sub2_a", "PAY-1-1", "sub2_b", "sub2_c", "shared", "2026110422001", "6", "typo", "", "  "}

	result, err := newInvoiceTestService(repo, nil).AddExclusions(context.Background(), numbers, 99)

	require.NoError(t, err)
	require.Equal(t, []int64{1, 6}, repo.lastAdded, "an order pasted under two different numbers is only added once")
	require.Len(t, result.Added, 2)
	require.ElementsMatch(t, []string{"PAY-1-1", "sub2_b", "6"}, result.AlreadyExcluded)
	require.Equal(t, []string{"sub2_c"}, result.Unpaid)
	require.Equal(t, []string{"shared"}, result.Ambiguous)
	require.Equal(t, []string{"typo"}, result.NotFound)
}

func TestAddExclusionsValidatesInput(t *testing.T) {
	svc := newInvoiceTestService(&fakeInvoiceRepo{}, nil)

	_, err := svc.AddExclusions(context.Background(), []string{" ", ""}, 99)
	require.ErrorIs(t, err, ErrInvoiceExclusionEmpty)

	tooMany := make([]string, 0, invoiceMaxExclusionsPerCall+1)
	for i := 0; i <= invoiceMaxExclusionsPerCall; i++ {
		tooMany = append(tooMany, "sub2_"+strings.Repeat("x", 3)+time.Duration(i).String())
	}
	_, err = svc.AddExclusions(context.Background(), tooMany, 99)
	require.ErrorIs(t, err, ErrInvoiceExclusionTooMany)
}
