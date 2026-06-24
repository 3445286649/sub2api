package domain

import (
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SupportTicketStatusOpen         = "open"
	SupportTicketStatusPendingAdmin = "pending_admin"
	SupportTicketStatusPendingUser  = "pending_user"
	SupportTicketStatusClosed       = "closed"
)

const (
	SupportTicketCategoryGeneral      = "general"
	SupportTicketCategoryRecharge     = "recharge"
	SupportTicketCategorySubscription = "subscription"
	SupportTicketCategoryAPIIssue     = "api_issue"
	SupportTicketCategoryAccount      = "account"
)

const (
	SupportTicketPriorityLow    = "low"
	SupportTicketPriorityNormal = "normal"
	SupportTicketPriorityHigh   = "high"
	SupportTicketPriorityUrgent = "urgent"
)

const (
	SupportMessageSenderUser   = "user"
	SupportMessageSenderAdmin  = "admin"
	SupportMessageSenderSystem = "system"
)

var (
	ErrSupportTicketNotFound = infraerrors.NotFound("SUPPORT_TICKET_NOT_FOUND", "support ticket not found")
)

type SupportTicket struct {
	ID                 int64
	UserID             int64
	Title              string
	Category           string
	Status             string
	Priority           string
	LastMessageAt      *time.Time
	LastUserMessageAt  *time.Time
	LastAdminMessageAt *time.Time
	UserLastReadAt     *time.Time
	AdminLastReadAt    *time.Time
	AssignedAdminID    *int64
	ClosedAt           *time.Time
	ClosedBy           *int64
	CreatedAt          time.Time
	UpdatedAt          time.Time

	User *SupportTicketUser
}

type SupportTicketUser struct {
	ID       int64
	Email    string
	Username string
}

type SupportTicketMessage struct {
	ID         int64
	TicketID   int64
	SenderID   int64
	SenderRole string
	Content    string
	CreatedAt  time.Time
}

func (t *SupportTicket) UserUnread() bool {
	if t == nil || t.LastAdminMessageAt == nil {
		return false
	}
	return t.UserLastReadAt == nil || t.LastAdminMessageAt.After(*t.UserLastReadAt)
}

func (t *SupportTicket) AdminUnread() bool {
	if t == nil || t.LastUserMessageAt == nil {
		return false
	}
	return t.AdminLastReadAt == nil || t.LastUserMessageAt.After(*t.AdminLastReadAt)
}
