package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	SupportTicketStatusOpen         = domain.SupportTicketStatusOpen
	SupportTicketStatusPendingAdmin = domain.SupportTicketStatusPendingAdmin
	SupportTicketStatusPendingUser  = domain.SupportTicketStatusPendingUser
	SupportTicketStatusClosed       = domain.SupportTicketStatusClosed
)

const (
	SupportTicketCategoryGeneral      = domain.SupportTicketCategoryGeneral
	SupportTicketCategoryRecharge     = domain.SupportTicketCategoryRecharge
	SupportTicketCategorySubscription = domain.SupportTicketCategorySubscription
	SupportTicketCategoryAPIIssue     = domain.SupportTicketCategoryAPIIssue
	SupportTicketCategoryAccount      = domain.SupportTicketCategoryAccount
)

const (
	SupportTicketPriorityLow    = domain.SupportTicketPriorityLow
	SupportTicketPriorityNormal = domain.SupportTicketPriorityNormal
	SupportTicketPriorityHigh   = domain.SupportTicketPriorityHigh
	SupportTicketPriorityUrgent = domain.SupportTicketPriorityUrgent
)

const (
	SupportMessageSenderUser   = domain.SupportMessageSenderUser
	SupportMessageSenderAdmin  = domain.SupportMessageSenderAdmin
	SupportMessageSenderSystem = domain.SupportMessageSenderSystem
)

var (
	ErrSupportTicketNotFound       = domain.ErrSupportTicketNotFound
	ErrSupportInputRequired        = infraerrors.BadRequest("SUPPORT_INPUT_REQUIRED", "support input is required")
	ErrSupportTitleInvalid         = infraerrors.BadRequest("SUPPORT_TITLE_INVALID", "support ticket title is invalid")
	ErrSupportContentInvalid       = infraerrors.BadRequest("SUPPORT_CONTENT_INVALID", "support ticket message content is invalid")
	ErrSupportCategoryInvalid      = infraerrors.BadRequest("SUPPORT_CATEGORY_INVALID", "support ticket category is invalid")
	ErrSupportStatusInvalid        = infraerrors.BadRequest("SUPPORT_STATUS_INVALID", "support ticket status is invalid")
	ErrSupportPriorityInvalid      = infraerrors.BadRequest("SUPPORT_PRIORITY_INVALID", "support ticket priority is invalid")
	ErrSupportSenderRoleInvalid    = infraerrors.BadRequest("SUPPORT_SENDER_ROLE_INVALID", "support ticket sender role is invalid")
	ErrSupportTicketClosed         = infraerrors.Conflict("SUPPORT_TICKET_CLOSED", "support ticket is closed")
	ErrSupportTicketAlreadyOpen    = infraerrors.Conflict("SUPPORT_TICKET_ALREADY_OPEN", "support ticket is not closed")
	ErrSupportRateLimited          = infraerrors.TooManyRequests("SUPPORT_RATE_LIMITED", "too many support requests")
	ErrSupportAssignedAdminInvalid = infraerrors.BadRequest("SUPPORT_ASSIGNED_ADMIN_INVALID", "assigned admin id is invalid")
)

type SupportTicket = domain.SupportTicket
type SupportTicketMessage = domain.SupportTicketMessage
type SupportTicketUser = domain.SupportTicketUser

type SupportTicketListFilters struct {
	UserID     int64
	Status     string
	Category   string
	Priority   string
	Search     string
	UnreadOnly bool
}

type SupportMessageListFilters struct {
	BeforeID int64
	Limit    int
}

type SupportTicketRepository interface {
	CreateWithMessage(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage) error
	GetByID(ctx context.Context, id int64) (*SupportTicket, error)
	GetByIDForUser(ctx context.Context, userID, id int64) (*SupportTicket, error)
	List(ctx context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	Update(ctx context.Context, ticket *SupportTicket) error
	UpdateReadAt(ctx context.Context, ticketID int64, role string, readAt time.Time) error
	CountCreatedSince(ctx context.Context, userID int64, since time.Time) (int, error)
	CountMessagesSince(ctx context.Context, userID int64, since time.Time) (int, error)
}

type SupportTicketMessageRepository interface {
	Create(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage) error
	ListByTicketID(ctx context.Context, ticketID int64, filters SupportMessageListFilters) ([]SupportTicketMessage, error)
}
