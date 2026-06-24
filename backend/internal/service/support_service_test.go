package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

func TestSupportService_CreateTicketCreatesInitialMessage(t *testing.T) {
	repo := newMemorySupportRepo()
	svc := NewSupportService(repo, repo, &supportUserRepo{})

	ticket, err := svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
		UserID:   10,
		Title:    "  充值没有到账  ",
		Category: SupportTicketCategoryRecharge,
		Content:  "请帮我看下订单",
	})
	if err != nil {
		t.Fatalf("CreateTicket returned error: %v", err)
	}
	if ticket.Status != SupportTicketStatusPendingAdmin {
		t.Fatalf("status = %s, want %s", ticket.Status, SupportTicketStatusPendingAdmin)
	}
	if ticket.Priority != SupportTicketPriorityNormal {
		t.Fatalf("priority = %s, want normal", ticket.Priority)
	}
	messages, err := svc.ListMessagesForUser(context.Background(), 10, ticket.ID, SupportMessageListFilters{})
	if err != nil {
		t.Fatalf("ListMessagesForUser returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "请帮我看下订单" || messages[0].SenderRole != SupportMessageSenderUser {
		t.Fatalf("unexpected initial message: %#v", messages)
	}
}

func TestSupportService_StatusTransitionsAndUnread(t *testing.T) {
	repo := newMemorySupportRepo()
	svc := NewSupportService(repo, repo, &supportUserRepo{})
	ticket, err := svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
		UserID:   10,
		Title:    "API 异常",
		Category: SupportTicketCategoryAPIIssue,
		Content:  "接口一直报错",
	})
	if err != nil {
		t.Fatalf("CreateTicket returned error: %v", err)
	}

	if _, err := svc.AddAdminMessage(context.Background(), 1, ticket.ID, "已收到"); err != nil {
		t.Fatalf("AddAdminMessage returned error: %v", err)
	}
	got, err := svc.GetForUser(context.Background(), 10, ticket.ID)
	if err != nil {
		t.Fatalf("GetForUser returned error: %v", err)
	}
	if got.Status != SupportTicketStatusPendingUser || !got.UserUnread() {
		t.Fatalf("after admin reply ticket = %#v, want pending_user and user unread", got)
	}

	if err := svc.MarkReadForUser(context.Background(), 10, ticket.ID); err != nil {
		t.Fatalf("MarkReadForUser returned error: %v", err)
	}
	got, _ = svc.GetForUser(context.Background(), 10, ticket.ID)
	if got.UserUnread() {
		t.Fatalf("user unread should be cleared after mark read")
	}

	if _, err := svc.AddUserMessage(context.Background(), 10, ticket.ID, "补充日志"); err != nil {
		t.Fatalf("AddUserMessage returned error: %v", err)
	}
	got, _ = svc.GetForAdmin(context.Background(), ticket.ID)
	if got.Status != SupportTicketStatusPendingAdmin || !got.AdminUnread() {
		t.Fatalf("after user reply ticket = %#v, want pending_admin and admin unread", got)
	}
}

func TestSupportService_UserCannotAccessOthersTicket(t *testing.T) {
	repo := newMemorySupportRepo()
	svc := NewSupportService(repo, repo, &supportUserRepo{})
	ticket, err := svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
		UserID:   10,
		Title:    "账号问题",
		Category: SupportTicketCategoryAccount,
		Content:  "无法登录",
	})
	if err != nil {
		t.Fatalf("CreateTicket returned error: %v", err)
	}

	if _, err := svc.GetForUser(context.Background(), 11, ticket.ID); !errors.Is(err, ErrSupportTicketNotFound) {
		t.Fatalf("GetForUser err = %v, want ErrSupportTicketNotFound", err)
	}
	if _, err := svc.AddUserMessage(context.Background(), 11, ticket.ID, "越权回复"); !errors.Is(err, ErrSupportTicketNotFound) {
		t.Fatalf("AddUserMessage err = %v, want ErrSupportTicketNotFound", err)
	}
}

func TestSupportService_ClosedTicketRejectsReplyAndReopenAllowsReply(t *testing.T) {
	repo := newMemorySupportRepo()
	svc := NewSupportService(repo, repo, &supportUserRepo{})
	ticket, err := svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
		UserID:   10,
		Title:    "普通问题",
		Category: SupportTicketCategoryGeneral,
		Content:  "先关闭",
	})
	if err != nil {
		t.Fatalf("CreateTicket returned error: %v", err)
	}
	if _, err := svc.CloseForUser(context.Background(), 10, ticket.ID); err != nil {
		t.Fatalf("CloseForUser returned error: %v", err)
	}
	if _, err := svc.AddUserMessage(context.Background(), 10, ticket.ID, "关闭后回复"); !errors.Is(err, ErrSupportTicketClosed) {
		t.Fatalf("AddUserMessage err = %v, want ErrSupportTicketClosed", err)
	}
	reopened, msg, err := svc.ReopenForUser(context.Background(), 10, ticket.ID, "继续处理")
	if err != nil {
		t.Fatalf("ReopenForUser returned error: %v", err)
	}
	if reopened.Status != SupportTicketStatusPendingAdmin || msg == nil || msg.Content != "继续处理" {
		t.Fatalf("unexpected reopen result: ticket=%#v msg=%#v", reopened, msg)
	}
}

func TestSupportService_RejectsInvalidInputAndRateLimit(t *testing.T) {
	repo := newMemorySupportRepo()
	svc := NewSupportService(repo, repo, &supportUserRepo{})
	_, err := svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
		UserID:   10,
		Title:    strings.Repeat("x", 121),
		Category: SupportTicketCategoryGeneral,
		Content:  "内容",
	})
	if !errors.Is(err, ErrSupportTitleInvalid) {
		t.Fatalf("long title err = %v, want ErrSupportTitleInvalid", err)
	}

	for i := 0; i < supportCreateRateLimit; i++ {
		_, err := svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
			UserID:   20,
			Title:    "标题",
			Category: SupportTicketCategoryGeneral,
			Content:  "内容",
		})
		if err != nil {
			t.Fatalf("CreateTicket #%d returned error: %v", i, err)
		}
	}
	_, err = svc.CreateTicket(context.Background(), &CreateSupportTicketInput{
		UserID:   20,
		Title:    "标题",
		Category: SupportTicketCategoryGeneral,
		Content:  "内容",
	})
	if !infraerrors.IsTooManyRequests(err) {
		t.Fatalf("rate limit err = %v, want 429", err)
	}
}

type memorySupportRepo struct {
	mu            sync.Mutex
	nextTicketID  int64
	nextMessageID int64
	tickets       map[int64]*SupportTicket
	messages      map[int64][]SupportTicketMessage
}

func newMemorySupportRepo() *memorySupportRepo {
	return &memorySupportRepo{
		nextTicketID:  1,
		nextMessageID: 1,
		tickets:       make(map[int64]*SupportTicket),
		messages:      make(map[int64][]SupportTicketMessage),
	}
}

func (r *memorySupportRepo) CreateWithMessage(_ context.Context, ticket *SupportTicket, message *SupportTicketMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	ticket.ID = r.nextTicketID
	r.nextTicketID++
	ticket.CreatedAt = now
	ticket.UpdatedAt = now
	message.ID = r.nextMessageID
	r.nextMessageID++
	message.TicketID = ticket.ID
	message.CreatedAt = now
	r.tickets[ticket.ID] = cloneTicket(ticket)
	r.messages[ticket.ID] = append(r.messages[ticket.ID], *message)
	return nil
}

func (r *memorySupportRepo) GetByID(_ context.Context, id int64) (*SupportTicket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ticket, ok := r.tickets[id]
	if !ok {
		return nil, ErrSupportTicketNotFound
	}
	return cloneTicket(ticket), nil
}

func (r *memorySupportRepo) GetByIDForUser(_ context.Context, userID, id int64) (*SupportTicket, error) {
	ticket, err := r.GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrSupportTicketNotFound
	}
	return ticket, nil
}

func (r *memorySupportRepo) List(_ context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SupportTicket, 0, len(r.tickets))
	for _, ticket := range r.tickets {
		if filters.UserID > 0 && ticket.UserID != filters.UserID {
			continue
		}
		if filters.Status != "" && ticket.Status != filters.Status {
			continue
		}
		out = append(out, *cloneTicket(ticket))
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.PageSize}, nil
}

func (r *memorySupportRepo) Update(_ context.Context, ticket *SupportTicket) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tickets[ticket.ID]; !ok {
		return ErrSupportTicketNotFound
	}
	ticket.UpdatedAt = time.Now()
	r.tickets[ticket.ID] = cloneTicket(ticket)
	return nil
}

func (r *memorySupportRepo) UpdateReadAt(_ context.Context, ticketID int64, role string, readAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ticket, ok := r.tickets[ticketID]
	if !ok {
		return ErrSupportTicketNotFound
	}
	if role == SupportMessageSenderAdmin {
		ticket.AdminLastReadAt = &readAt
	} else {
		ticket.UserLastReadAt = &readAt
	}
	return nil
}

func (r *memorySupportRepo) CountCreatedSince(_ context.Context, userID int64, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, ticket := range r.tickets {
		if ticket.UserID == userID && ticket.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

func (r *memorySupportRepo) CountMessagesSince(_ context.Context, userID int64, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, messages := range r.messages {
		for _, message := range messages {
			if message.SenderID == userID && message.SenderRole == SupportMessageSenderUser && message.CreatedAt.After(since) {
				count++
			}
		}
	}
	return count, nil
}

func (r *memorySupportRepo) Create(_ context.Context, ticket *SupportTicket, message *SupportTicketMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tickets[ticket.ID]; !ok {
		return ErrSupportTicketNotFound
	}
	message.ID = r.nextMessageID
	r.nextMessageID++
	message.CreatedAt = time.Now()
	r.messages[ticket.ID] = append(r.messages[ticket.ID], *message)
	ticket.UpdatedAt = time.Now()
	r.tickets[ticket.ID] = cloneTicket(ticket)
	return nil
}

func (r *memorySupportRepo) ListByTicketID(_ context.Context, ticketID int64, _ SupportMessageListFilters) ([]SupportTicketMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.messages[ticketID]
	out := make([]SupportTicketMessage, len(items))
	copy(out, items)
	return out, nil
}

func cloneTicket(ticket *SupportTicket) *SupportTicket {
	if ticket == nil {
		return nil
	}
	cp := *ticket
	return &cp
}

type supportUserRepo struct{}

func (r *supportUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if id == 1 {
		return &User{ID: id, Role: RoleAdmin, Status: StatusActive}, nil
	}
	return &User{ID: id, Role: RoleUser, Status: StatusActive}, nil
}

func (r *supportUserRepo) Create(context.Context, *User) error { return nil }
func (r *supportUserRepo) GetByIDIncludeDeleted(context.Context, int64) (*User, error) {
	return nil, ErrUserNotFound
}
func (r *supportUserRepo) GetByEmail(context.Context, string) (*User, error) {
	return nil, ErrUserNotFound
}
func (r *supportUserRepo) GetFirstAdmin(context.Context) (*User, error)              { return nil, ErrUserNotFound }
func (r *supportUserRepo) Update(context.Context, *User) error                       { return nil }
func (r *supportUserRepo) Delete(context.Context, int64) error                       { return nil }
func (r *supportUserRepo) GetUserAvatar(context.Context, int64) (*UserAvatar, error) { return nil, nil }
func (r *supportUserRepo) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	return nil, nil
}
func (r *supportUserRepo) DeleteUserAvatar(context.Context, int64) error { return nil }
func (r *supportUserRepo) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *supportUserRepo) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *supportUserRepo) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (r *supportUserRepo) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (r *supportUserRepo) UpdateUserLastActiveAt(context.Context, int64, time.Time) error { return nil }
func (r *supportUserRepo) UpdateBalance(context.Context, int64, float64) error            { return nil }
func (r *supportUserRepo) DeductBalance(context.Context, int64, float64) error            { return nil }
func (r *supportUserRepo) UpdateConcurrency(context.Context, int64, int) error            { return nil }
func (r *supportUserRepo) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (r *supportUserRepo) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (r *supportUserRepo) ExistsByEmail(context.Context, string) (bool, error) { return false, nil }
func (r *supportUserRepo) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *supportUserRepo) AddGroupToAllowedGroups(context.Context, int64, int64) error { return nil }
func (r *supportUserRepo) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (r *supportUserRepo) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, nil
}
func (r *supportUserRepo) UnbindUserAuthProvider(context.Context, int64, string) error { return nil }
func (r *supportUserRepo) UpdateTotpSecret(context.Context, int64, *string) error      { return nil }
func (r *supportUserRepo) EnableTotp(context.Context, int64) error                     { return nil }
func (r *supportUserRepo) DisableTotp(context.Context, int64) error                    { return nil }
