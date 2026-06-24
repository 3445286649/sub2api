package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/supportticket"
	"github.com/Wei-Shaw/sub2api/ent/supportticketmessage"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type supportTicketRepository struct {
	client *dbent.Client
}

type supportTicketMessageRepository struct {
	client *dbent.Client
}

func NewSupportTicketRepository(client *dbent.Client) service.SupportTicketRepository {
	return &supportTicketRepository{client: client}
}

func NewSupportTicketMessageRepository(client *dbent.Client) service.SupportTicketMessageRepository {
	return &supportTicketMessageRepository{client: client}
}

func (r *supportTicketRepository) CreateWithMessage(ctx context.Context, ticket *service.SupportTicket, message *service.SupportTicketMessage) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	created, err := client.SupportTicket.Create().
		SetUserID(ticket.UserID).
		SetTitle(ticket.Title).
		SetCategory(ticket.Category).
		SetStatus(ticket.Status).
		SetPriority(ticket.Priority).
		SetNillableLastMessageAt(ticket.LastMessageAt).
		SetNillableLastUserMessageAt(ticket.LastUserMessageAt).
		SetNillableLastAdminMessageAt(ticket.LastAdminMessageAt).
		SetNillableUserLastReadAt(ticket.UserLastReadAt).
		SetNillableAdminLastReadAt(ticket.AdminLastReadAt).
		SetNillableAssignedAdminID(ticket.AssignedAdminID).
		SetNillableClosedAt(ticket.ClosedAt).
		SetNillableClosedBy(ticket.ClosedBy).
		Save(txCtx)
	if err != nil {
		return err
	}
	applySupportTicketEntityToService(ticket, created)

	message.TicketID = created.ID
	msg, err := client.SupportTicketMessage.Create().
		SetTicketID(created.ID).
		SetSenderID(message.SenderID).
		SetSenderRole(message.SenderRole).
		SetContent(message.Content).
		SetCreatedAt(message.CreatedAt).
		Save(txCtx)
	if err != nil {
		return err
	}
	applySupportMessageEntityToService(message, msg)

	return tx.Commit()
}

func (r *supportTicketRepository) GetByID(ctx context.Context, id int64) (*service.SupportTicket, error) {
	m, err := r.client.SupportTicket.Query().
		Where(supportticket.IDEQ(id)).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	return supportTicketEntityToService(m), nil
}

func (r *supportTicketRepository) GetByIDForUser(ctx context.Context, userID, id int64) (*service.SupportTicket, error) {
	m, err := r.client.SupportTicket.Query().
		Where(
			supportticket.IDEQ(id),
			supportticket.UserIDEQ(userID),
		).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	return supportTicketEntityToService(m), nil
}

func (r *supportTicketRepository) List(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.SupportTicketListFilters,
) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	q := r.client.SupportTicket.Query()

	if filters.UserID > 0 {
		q = q.Where(supportticket.UserIDEQ(filters.UserID))
	}
	if filters.Status != "" {
		q = q.Where(supportticket.StatusEQ(filters.Status))
	}
	if filters.Category != "" {
		q = q.Where(supportticket.CategoryEQ(filters.Category))
	}
	if filters.Priority != "" {
		q = q.Where(supportticket.PriorityEQ(filters.Priority))
	}
	if filters.Search != "" {
		search := strings.TrimSpace(filters.Search)
		if id, err := strconv.ParseInt(search, 10, 64); err == nil && id > 0 {
			q = q.Where(supportticket.Or(
				supportticket.IDEQ(id),
				supportticket.UserIDEQ(id),
				supportticket.TitleContainsFold(search),
				supportticket.HasUserWith(
					dbuser.Or(
						dbuser.EmailContainsFold(search),
						dbuser.UsernameContainsFold(search),
					),
				),
			))
		} else {
			q = q.Where(supportticket.Or(
				supportticket.TitleContainsFold(search),
				supportticket.HasUserWith(
					dbuser.Or(
						dbuser.EmailContainsFold(search),
						dbuser.UsernameContainsFold(search),
					),
				),
			))
		}
	}
	if filters.UnreadOnly {
		if filters.UserID > 0 {
			q = q.Where(userSupportUnreadPredicate())
		} else {
			q = q.Where(adminSupportUnreadPredicate())
		}
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	itemsQuery := q.
		WithUser().
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range supportTicketListOrders(params) {
		itemsQuery = itemsQuery.Order(order)
	}

	items, err := itemsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return supportTicketEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

func (r *supportTicketRepository) Update(ctx context.Context, ticket *service.SupportTicket) error {
	client := clientFromContext(ctx, r.client)
	builder := client.SupportTicket.UpdateOneID(ticket.ID).
		SetTitle(ticket.Title).
		SetCategory(ticket.Category).
		SetStatus(ticket.Status).
		SetPriority(ticket.Priority)

	setTimeOrClear(builder.SetLastMessageAt, builder.ClearLastMessageAt, ticket.LastMessageAt)
	setTimeOrClear(builder.SetLastUserMessageAt, builder.ClearLastUserMessageAt, ticket.LastUserMessageAt)
	setTimeOrClear(builder.SetLastAdminMessageAt, builder.ClearLastAdminMessageAt, ticket.LastAdminMessageAt)
	setTimeOrClear(builder.SetUserLastReadAt, builder.ClearUserLastReadAt, ticket.UserLastReadAt)
	setTimeOrClear(builder.SetAdminLastReadAt, builder.ClearAdminLastReadAt, ticket.AdminLastReadAt)
	setTimeOrClear(builder.SetClosedAt, builder.ClearClosedAt, ticket.ClosedAt)
	setInt64OrClear(builder.SetAssignedAdminID, builder.ClearAssignedAdminID, ticket.AssignedAdminID)
	setInt64OrClear(builder.SetClosedBy, builder.ClearClosedBy, ticket.ClosedBy)

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	ticket.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *supportTicketRepository) UpdateReadAt(ctx context.Context, ticketID int64, role string, readAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	builder := client.SupportTicket.UpdateOneID(ticketID)
	switch role {
	case service.SupportMessageSenderAdmin:
		builder.SetAdminLastReadAt(readAt)
	case service.SupportMessageSenderUser:
		builder.SetUserLastReadAt(readAt)
	default:
		return service.ErrSupportSenderRoleInvalid
	}
	_, err := builder.Save(ctx)
	return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
}

func (r *supportTicketRepository) CountCreatedSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	return r.client.SupportTicket.Query().
		Where(
			supportticket.UserIDEQ(userID),
			supportticket.CreatedAtGTE(since),
		).
		Count(ctx)
}

func (r *supportTicketRepository) CountMessagesSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	return r.client.SupportTicketMessage.Query().
		Where(
			supportticketmessage.SenderIDEQ(userID),
			supportticketmessage.SenderRoleEQ(service.SupportMessageSenderUser),
			supportticketmessage.CreatedAtGTE(since),
		).
		Count(ctx)
}

func (r *supportTicketMessageRepository) Create(ctx context.Context, ticket *service.SupportTicket, message *service.SupportTicketMessage) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.createWithClient(ctx, tx.Client(), ticket, message)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := r.createWithClient(txCtx, tx.Client(), ticket, message); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *supportTicketMessageRepository) createWithClient(ctx context.Context, client *dbent.Client, ticket *service.SupportTicket, message *service.SupportTicketMessage) error {
	msg, err := client.SupportTicketMessage.Create().
		SetTicketID(ticket.ID).
		SetSenderID(message.SenderID).
		SetSenderRole(message.SenderRole).
		SetContent(message.Content).
		SetCreatedAt(message.CreatedAt).
		Save(ctx)
	if err != nil {
		return err
	}
	applySupportMessageEntityToService(message, msg)

	updated, err := client.SupportTicket.UpdateOneID(ticket.ID).
		SetTitle(ticket.Title).
		SetCategory(ticket.Category).
		SetStatus(ticket.Status).
		SetPriority(ticket.Priority).
		SetNillableLastMessageAt(ticket.LastMessageAt).
		SetNillableLastUserMessageAt(ticket.LastUserMessageAt).
		SetNillableLastAdminMessageAt(ticket.LastAdminMessageAt).
		SetNillableUserLastReadAt(ticket.UserLastReadAt).
		SetNillableAdminLastReadAt(ticket.AdminLastReadAt).
		SetNillableAssignedAdminID(ticket.AssignedAdminID).
		SetNillableClosedAt(ticket.ClosedAt).
		SetNillableClosedBy(ticket.ClosedBy).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	ticket.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *supportTicketMessageRepository) ListByTicketID(ctx context.Context, ticketID int64, filters service.SupportMessageListFilters) ([]service.SupportTicketMessage, error) {
	limit := filters.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := r.client.SupportTicketMessage.Query().
		Where(supportticketmessage.TicketIDEQ(ticketID)).
		Order(dbent.Desc(supportticketmessage.FieldID)).
		Limit(limit)
	if filters.BeforeID > 0 {
		q = q.Where(supportticketmessage.IDLT(filters.BeforeID))
	}

	items, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := supportMessageEntitiesToService(items)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func supportTicketListOrders(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)
	field := supportticket.FieldLastMessageAt
	switch sortBy {
	case "id":
		field = supportticket.FieldID
	case "created_at":
		field = supportticket.FieldCreatedAt
	case "updated_at":
		field = supportticket.FieldUpdatedAt
	case "priority":
		field = supportticket.FieldPriority
	case "status":
		field = supportticket.FieldStatus
	case "category":
		field = supportticket.FieldCategory
	case "", "last_message_at":
		field = supportticket.FieldLastMessageAt
	default:
		field = supportticket.FieldLastMessageAt
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(supportticket.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(supportticket.FieldID)}
}

func userSupportUnreadPredicate() func(*entsql.Selector) {
	return func(s *entsql.Selector) {
		lastAdmin := s.C(supportticket.FieldLastAdminMessageAt)
		userRead := s.C(supportticket.FieldUserLastReadAt)
		s.Where(entsql.And(
			entsql.NotNull(lastAdmin),
			entsql.Or(
				entsql.IsNull(userRead),
				entsql.ExprP(fmt.Sprintf("%s > %s", lastAdmin, userRead)),
			),
		))
	}
}

func adminSupportUnreadPredicate() func(*entsql.Selector) {
	return func(s *entsql.Selector) {
		lastUser := s.C(supportticket.FieldLastUserMessageAt)
		adminRead := s.C(supportticket.FieldAdminLastReadAt)
		s.Where(entsql.And(
			entsql.NotNull(lastUser),
			entsql.Or(
				entsql.IsNull(adminRead),
				entsql.ExprP(fmt.Sprintf("%s > %s", lastUser, adminRead)),
			),
		))
	}
}

func setTimeOrClear(set func(time.Time) *dbent.SupportTicketUpdateOne, clear func() *dbent.SupportTicketUpdateOne, value *time.Time) {
	if value != nil {
		set(*value)
	} else {
		clear()
	}
}

func setInt64OrClear(set func(int64) *dbent.SupportTicketUpdateOne, clear func() *dbent.SupportTicketUpdateOne, value *int64) {
	if value != nil {
		set(*value)
	} else {
		clear()
	}
}

func applySupportTicketEntityToService(dst *service.SupportTicket, src *dbent.SupportTicket) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func supportTicketEntityToService(m *dbent.SupportTicket) *service.SupportTicket {
	if m == nil {
		return nil
	}
	out := &service.SupportTicket{
		ID:                 m.ID,
		UserID:             m.UserID,
		Title:              m.Title,
		Category:           m.Category,
		Status:             m.Status,
		Priority:           m.Priority,
		LastMessageAt:      m.LastMessageAt,
		LastUserMessageAt:  m.LastUserMessageAt,
		LastAdminMessageAt: m.LastAdminMessageAt,
		UserLastReadAt:     m.UserLastReadAt,
		AdminLastReadAt:    m.AdminLastReadAt,
		AssignedAdminID:    m.AssignedAdminID,
		ClosedAt:           m.ClosedAt,
		ClosedBy:           m.ClosedBy,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
	if m.Edges.User != nil {
		out.User = &service.SupportTicketUser{
			ID:       m.Edges.User.ID,
			Email:    m.Edges.User.Email,
			Username: m.Edges.User.Username,
		}
	}
	return out
}

func supportTicketEntitiesToService(models []*dbent.SupportTicket) []service.SupportTicket {
	out := make([]service.SupportTicket, 0, len(models))
	for i := range models {
		if s := supportTicketEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}

func applySupportMessageEntityToService(dst *service.SupportTicketMessage, src *dbent.SupportTicketMessage) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.TicketID = src.TicketID
	dst.CreatedAt = src.CreatedAt
}

func supportMessageEntityToService(m *dbent.SupportTicketMessage) *service.SupportTicketMessage {
	if m == nil {
		return nil
	}
	return &service.SupportTicketMessage{
		ID:         m.ID,
		TicketID:   m.TicketID,
		SenderID:   m.SenderID,
		SenderRole: m.SenderRole,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
	}
}

func supportMessageEntitiesToService(models []*dbent.SupportTicketMessage) []service.SupportTicketMessage {
	out := make([]service.SupportTicketMessage, 0, len(models))
	for i := range models {
		if s := supportMessageEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}
