package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SupportTicketMessage holds the schema definition for support ticket messages.
type SupportTicketMessage struct {
	ent.Schema
}

func (SupportTicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "support_ticket_messages"},
	}
}

func (SupportTicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id").Comment("工单ID"),
		field.Int64("sender_id").Comment("发送人用户ID"),
		field.String("sender_role").MaxLen(20).Comment("发送人角色: user/admin/system"),
		field.String("content").SchemaType(map[string]string{dialect.Postgres: "text"}).NotEmpty().Comment("纯文本消息"),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).
			Ref("messages").
			Field("ticket_id").
			Required().
			Unique(),
		edge.From("sender", User.Type).
			Ref("support_ticket_messages").
			Field("sender_id").
			Required().
			Unique(),
	}
}

func (SupportTicketMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id"),
		index.Fields("ticket_id", "id"),
		index.Fields("ticket_id", "created_at"),
		index.Fields("sender_id"),
	}
}
