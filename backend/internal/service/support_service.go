package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	supportTitleMaxLength   = 120
	supportContentMaxLength = 2000
	supportMessageRateLimit = 20
	supportCreateRateLimit  = 10
)

type SupportService struct {
	ticketRepo  SupportTicketRepository
	messageRepo SupportTicketMessageRepository
	userRepo    UserRepository

	mu             sync.Mutex
	messageWindows map[int64][]time.Time
	createWindows  map[int64][]time.Time
}

func NewSupportService(
	ticketRepo SupportTicketRepository,
	messageRepo SupportTicketMessageRepository,
	userRepo UserRepository,
) *SupportService {
	return &SupportService{
		ticketRepo:     ticketRepo,
		messageRepo:    messageRepo,
		userRepo:       userRepo,
		messageWindows: make(map[int64][]time.Time),
		createWindows:  make(map[int64][]time.Time),
	}
}

type CreateSupportTicketInput struct {
	UserID   int64
	Title    string
	Category string
	Content  string
}

type CreateSupportMessageInput struct {
	UserID     int64
	TicketID   int64
	SenderRole string
	Content    string
}

type UpdateSupportTicketInput struct {
	Status          *string
	Category        *string
	Priority        *string
	AssignedAdminID **int64
}

type ReopenSupportTicketInput struct {
	UserID     int64
	TicketID   int64
	SenderRole string
	Content    string
}

func (s *SupportService) CreateTicket(ctx context.Context, input *CreateSupportTicketInput) (*SupportTicket, error) {
	if input == nil {
		return nil, ErrSupportInputRequired
	}
	if err := s.checkRate(ctx, input.UserID, supportCreateRateLimit, 10*time.Minute, s.createWindows, true); err != nil {
		return nil, err
	}

	title, err := normalizeSupportTitle(input.Title)
	if err != nil {
		return nil, err
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = SupportTicketCategoryGeneral
	}
	if !isValidSupportCategory(category) {
		return nil, ErrSupportCategoryInvalid
	}
	content, err := normalizeSupportContent(input.Content)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ticket := &SupportTicket{
		UserID:            input.UserID,
		Title:             title,
		Category:          category,
		Status:            SupportTicketStatusPendingAdmin,
		Priority:          SupportTicketPriorityNormal,
		LastMessageAt:     &now,
		LastUserMessageAt: &now,
		UserLastReadAt:    &now,
	}
	message := &SupportTicketMessage{
		SenderID:   input.UserID,
		SenderRole: SupportMessageSenderUser,
		Content:    content,
		CreatedAt:  now,
	}

	if err := s.ticketRepo.CreateWithMessage(ctx, ticket, message); err != nil {
		return nil, fmt.Errorf("create support ticket: %w", err)
	}
	return ticket, nil
}

func (s *SupportService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	filters.UserID = userID
	if filters.Status != "" && !isValidSupportStatus(filters.Status) {
		return nil, nil, ErrSupportStatusInvalid
	}
	return s.ticketRepo.List(ctx, params, filters)
}

func (s *SupportService) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	if filters.Status != "" && !isValidSupportStatus(filters.Status) {
		return nil, nil, ErrSupportStatusInvalid
	}
	if filters.Category != "" && !isValidSupportCategory(filters.Category) {
		return nil, nil, ErrSupportCategoryInvalid
	}
	if filters.Priority != "" && !isValidSupportPriority(filters.Priority) {
		return nil, nil, ErrSupportPriorityInvalid
	}
	return s.ticketRepo.List(ctx, params, filters)
}

func (s *SupportService) GetForUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	return s.ticketRepo.GetByIDForUser(ctx, userID, ticketID)
}

func (s *SupportService) GetForAdmin(ctx context.Context, ticketID int64) (*SupportTicket, error) {
	return s.ticketRepo.GetByID(ctx, ticketID)
}

func (s *SupportService) ListMessagesForUser(ctx context.Context, userID, ticketID int64, filters SupportMessageListFilters) ([]SupportTicketMessage, error) {
	if _, err := s.ticketRepo.GetByIDForUser(ctx, userID, ticketID); err != nil {
		return nil, err
	}
	return s.listMessages(ctx, ticketID, filters)
}

func (s *SupportService) ListMessagesForAdmin(ctx context.Context, ticketID int64, filters SupportMessageListFilters) ([]SupportTicketMessage, error) {
	if _, err := s.ticketRepo.GetByID(ctx, ticketID); err != nil {
		return nil, err
	}
	return s.listMessages(ctx, ticketID, filters)
}

func (s *SupportService) AddUserMessage(ctx context.Context, userID, ticketID int64, content string) (*SupportTicketMessage, error) {
	if err := s.checkRate(ctx, userID, supportMessageRateLimit, time.Minute, s.messageWindows, true); err != nil {
		return nil, err
	}
	ticket, err := s.ticketRepo.GetByIDForUser(ctx, userID, ticketID)
	if err != nil {
		return nil, err
	}
	return s.addMessage(ctx, ticket, userID, SupportMessageSenderUser, content)
}

func (s *SupportService) AddAdminMessage(ctx context.Context, adminID, ticketID int64, content string) (*SupportTicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	return s.addMessage(ctx, ticket, adminID, SupportMessageSenderAdmin, content)
}

func (s *SupportService) MarkReadForUser(ctx context.Context, userID, ticketID int64) error {
	if _, err := s.ticketRepo.GetByIDForUser(ctx, userID, ticketID); err != nil {
		return err
	}
	if err := s.ticketRepo.UpdateReadAt(ctx, ticketID, SupportMessageSenderUser, time.Now()); err != nil {
		return fmt.Errorf("mark support ticket read: %w", err)
	}
	return nil
}

func (s *SupportService) MarkReadForAdmin(ctx context.Context, ticketID int64) error {
	if _, err := s.ticketRepo.GetByID(ctx, ticketID); err != nil {
		return err
	}
	if err := s.ticketRepo.UpdateReadAt(ctx, ticketID, SupportMessageSenderAdmin, time.Now()); err != nil {
		return fmt.Errorf("mark support ticket read: %w", err)
	}
	return nil
}

func (s *SupportService) CloseForUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.ticketRepo.GetByIDForUser(ctx, userID, ticketID)
	if err != nil {
		return nil, err
	}
	return s.closeTicket(ctx, ticket, userID)
}

func (s *SupportService) CloseForAdmin(ctx context.Context, adminID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	return s.closeTicket(ctx, ticket, adminID)
}

func (s *SupportService) ReopenForUser(ctx context.Context, userID, ticketID int64, content string) (*SupportTicket, *SupportTicketMessage, error) {
	ticket, err := s.ticketRepo.GetByIDForUser(ctx, userID, ticketID)
	if err != nil {
		return nil, nil, err
	}
	return s.reopenTicket(ctx, ticket, userID, SupportMessageSenderUser, content)
}

func (s *SupportService) ReopenForAdmin(ctx context.Context, adminID, ticketID int64, content string) (*SupportTicket, *SupportTicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, nil, err
	}
	return s.reopenTicket(ctx, ticket, adminID, SupportMessageSenderAdmin, content)
}

func (s *SupportService) UpdateByAdmin(ctx context.Context, ticketID int64, input *UpdateSupportTicketInput) (*SupportTicket, error) {
	if input == nil {
		return nil, ErrSupportInputRequired
	}
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if !isValidSupportStatus(status) {
			return nil, ErrSupportStatusInvalid
		}
		ticket.Status = status
		if status == SupportTicketStatusClosed {
			now := time.Now()
			ticket.ClosedAt = &now
		} else {
			ticket.ClosedAt = nil
			ticket.ClosedBy = nil
		}
	}
	if input.Category != nil {
		category := strings.TrimSpace(*input.Category)
		if !isValidSupportCategory(category) {
			return nil, ErrSupportCategoryInvalid
		}
		ticket.Category = category
	}
	if input.Priority != nil {
		priority := strings.TrimSpace(*input.Priority)
		if !isValidSupportPriority(priority) {
			return nil, ErrSupportPriorityInvalid
		}
		ticket.Priority = priority
	}
	if input.AssignedAdminID != nil {
		if *input.AssignedAdminID == nil {
			ticket.AssignedAdminID = nil
		} else {
			adminID := **input.AssignedAdminID
			if adminID <= 0 {
				return nil, ErrSupportAssignedAdminInvalid
			}
			user, err := s.userRepo.GetByID(ctx, adminID)
			if err != nil {
				return nil, ErrSupportAssignedAdminInvalid
			}
			if !user.IsAdmin() {
				return nil, ErrSupportAssignedAdminInvalid
			}
			ticket.AssignedAdminID = &adminID
		}
	}

	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("update support ticket: %w", err)
	}
	return ticket, nil
}

func (s *SupportService) addMessage(ctx context.Context, ticket *SupportTicket, senderID int64, senderRole, content string) (*SupportTicketMessage, error) {
	if ticket.Status == SupportTicketStatusClosed {
		return nil, ErrSupportTicketClosed
	}
	if !isValidSupportSenderRole(senderRole) || senderRole == SupportMessageSenderSystem {
		return nil, ErrSupportSenderRoleInvalid
	}
	normalized, err := normalizeSupportContent(content)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	message := &SupportTicketMessage{
		TicketID:   ticket.ID,
		SenderID:   senderID,
		SenderRole: senderRole,
		Content:    normalized,
		CreatedAt:  now,
	}
	ticket.LastMessageAt = &now
	if senderRole == SupportMessageSenderAdmin {
		ticket.Status = SupportTicketStatusPendingUser
		ticket.LastAdminMessageAt = &now
		ticket.AdminLastReadAt = &now
	} else {
		ticket.Status = SupportTicketStatusPendingAdmin
		ticket.LastUserMessageAt = &now
		ticket.UserLastReadAt = &now
	}

	if err := s.messageRepo.Create(ctx, ticket, message); err != nil {
		return nil, fmt.Errorf("create support message: %w", err)
	}
	return message, nil
}

func (s *SupportService) closeTicket(ctx context.Context, ticket *SupportTicket, actorID int64) (*SupportTicket, error) {
	now := time.Now()
	ticket.Status = SupportTicketStatusClosed
	ticket.ClosedAt = &now
	ticket.ClosedBy = &actorID
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("close support ticket: %w", err)
	}
	return ticket, nil
}

func (s *SupportService) reopenTicket(ctx context.Context, ticket *SupportTicket, actorID int64, senderRole, content string) (*SupportTicket, *SupportTicketMessage, error) {
	if ticket.Status != SupportTicketStatusClosed {
		return nil, nil, ErrSupportTicketAlreadyOpen
	}
	ticket.ClosedAt = nil
	ticket.ClosedBy = nil
	if senderRole == SupportMessageSenderAdmin {
		ticket.Status = SupportTicketStatusPendingUser
	} else {
		ticket.Status = SupportTicketStatusPendingAdmin
	}

	if strings.TrimSpace(content) == "" {
		if err := s.ticketRepo.Update(ctx, ticket); err != nil {
			return nil, nil, fmt.Errorf("reopen support ticket: %w", err)
		}
		return ticket, nil, nil
	}

	message, err := s.addMessage(ctx, ticket, actorID, senderRole, content)
	if err != nil {
		return nil, nil, err
	}
	return ticket, message, nil
}

func (s *SupportService) listMessages(ctx context.Context, ticketID int64, filters SupportMessageListFilters) ([]SupportTicketMessage, error) {
	if filters.Limit <= 0 || filters.Limit > 100 {
		filters.Limit = 50
	}
	return s.messageRepo.ListByTicketID(ctx, ticketID, filters)
}

func (s *SupportService) checkRate(ctx context.Context, userID int64, max int, window time.Duration, cache map[int64][]time.Time, countDB bool) error {
	if userID <= 0 {
		return ErrSupportInputRequired
	}
	now := time.Now()
	cutoff := now.Add(-window)

	if countDB {
		var count int
		var err error
		if window == time.Minute {
			count, err = s.ticketRepo.CountMessagesSince(ctx, userID, cutoff)
		} else {
			count, err = s.ticketRepo.CountCreatedSince(ctx, userID, cutoff)
		}
		if err != nil {
			return fmt.Errorf("count support rate window: %w", err)
		}
		if count >= max {
			return ErrSupportRateLimited
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entries := cache[userID]
	kept := entries[:0]
	for _, ts := range entries {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= max {
		cache[userID] = kept
		return ErrSupportRateLimited
	}
	kept = append(kept, now)
	cache[userID] = kept
	return nil
}

func normalizeSupportTitle(v string) (string, error) {
	title := strings.TrimSpace(v)
	if title == "" || len([]rune(title)) > supportTitleMaxLength {
		return "", ErrSupportTitleInvalid
	}
	return title, nil
}

func normalizeSupportContent(v string) (string, error) {
	content := strings.TrimSpace(v)
	if content == "" || len([]rune(content)) > supportContentMaxLength {
		return "", ErrSupportContentInvalid
	}
	return content, nil
}

func isValidSupportStatus(v string) bool {
	switch v {
	case SupportTicketStatusOpen, SupportTicketStatusPendingAdmin, SupportTicketStatusPendingUser, SupportTicketStatusClosed:
		return true
	default:
		return false
	}
}

func isValidSupportCategory(v string) bool {
	switch v {
	case SupportTicketCategoryGeneral, SupportTicketCategoryRecharge, SupportTicketCategorySubscription, SupportTicketCategoryAPIIssue, SupportTicketCategoryAccount:
		return true
	default:
		return false
	}
}

func isValidSupportPriority(v string) bool {
	switch v {
	case SupportTicketPriorityLow, SupportTicketPriorityNormal, SupportTicketPriorityHigh, SupportTicketPriorityUrgent:
		return true
	default:
		return false
	}
}

func isValidSupportSenderRole(v string) bool {
	switch v {
	case SupportMessageSenderUser, SupportMessageSenderAdmin, SupportMessageSenderSystem:
		return true
	default:
		return false
	}
}
