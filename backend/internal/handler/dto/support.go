package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type SupportTicketUser struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type SupportTicket struct {
	ID                 int64              `json:"id"`
	UserID             int64              `json:"user_id"`
	Title              string             `json:"title"`
	Category           string             `json:"category"`
	Status             string             `json:"status"`
	Priority           string             `json:"priority"`
	LastMessageAt      *time.Time         `json:"last_message_at,omitempty"`
	LastUserMessageAt  *time.Time         `json:"last_user_message_at,omitempty"`
	LastAdminMessageAt *time.Time         `json:"last_admin_message_at,omitempty"`
	UserLastReadAt     *time.Time         `json:"user_last_read_at,omitempty"`
	AdminLastReadAt    *time.Time         `json:"admin_last_read_at,omitempty"`
	AssignedAdminID    *int64             `json:"assigned_admin_id,omitempty"`
	ClosedAt           *time.Time         `json:"closed_at,omitempty"`
	ClosedBy           *int64             `json:"closed_by,omitempty"`
	UserUnread         bool               `json:"user_unread"`
	AdminUnread        bool               `json:"admin_unread"`
	User               *SupportTicketUser `json:"user,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

type SupportTicketMessage struct {
	ID         int64     `json:"id"`
	TicketID   int64     `json:"ticket_id"`
	SenderID   int64     `json:"sender_id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

func SupportTicketFromService(t *service.SupportTicket) *SupportTicket {
	if t == nil {
		return nil
	}
	out := &SupportTicket{
		ID:                 t.ID,
		UserID:             t.UserID,
		Title:              t.Title,
		Category:           t.Category,
		Status:             t.Status,
		Priority:           t.Priority,
		LastMessageAt:      t.LastMessageAt,
		LastUserMessageAt:  t.LastUserMessageAt,
		LastAdminMessageAt: t.LastAdminMessageAt,
		UserLastReadAt:     t.UserLastReadAt,
		AdminLastReadAt:    t.AdminLastReadAt,
		AssignedAdminID:    t.AssignedAdminID,
		ClosedAt:           t.ClosedAt,
		ClosedBy:           t.ClosedBy,
		UserUnread:         t.UserUnread(),
		AdminUnread:        t.AdminUnread(),
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
	}
	if t.User != nil {
		out.User = &SupportTicketUser{
			ID:       t.User.ID,
			Email:    t.User.Email,
			Username: t.User.Username,
		}
	}
	return out
}

func SupportTicketsFromService(items []service.SupportTicket) []SupportTicket {
	out := make([]SupportTicket, 0, len(items))
	for i := range items {
		if item := SupportTicketFromService(&items[i]); item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func SupportMessageFromService(m *service.SupportTicketMessage) *SupportTicketMessage {
	if m == nil {
		return nil
	}
	return &SupportTicketMessage{
		ID:         m.ID,
		TicketID:   m.TicketID,
		SenderID:   m.SenderID,
		SenderRole: m.SenderRole,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
	}
}

func SupportMessagesFromService(items []service.SupportTicketMessage) []SupportTicketMessage {
	out := make([]SupportTicketMessage, 0, len(items))
	for i := range items {
		if item := SupportMessageFromService(&items[i]); item != nil {
			out = append(out, *item)
		}
	}
	return out
}
