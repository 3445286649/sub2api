package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SupportTicket holds the schema definition for support tickets.
type SupportTicket struct {
	ent.Schema
}

func (SupportTicket) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "support_tickets"},
	}
}

func (SupportTicket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").Comment("工单所属用户ID"),
		field.String("title").MaxLen(120).NotEmpty().Comment("工单标题"),
		field.String("category").MaxLen(30).Default(domain.SupportTicketCategoryGeneral).Comment("工单分类"),
		field.String("status").MaxLen(30).Default(domain.SupportTicketStatusPendingAdmin).Comment("工单状态"),
		field.String("priority").MaxLen(20).Default(domain.SupportTicketPriorityNormal).Comment("工单优先级"),
		field.Time("last_message_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_user_message_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_admin_message_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("user_last_read_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("admin_last_read_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("assigned_admin_id").Optional().Nillable().Comment("指派管理员ID"),
		field.Time("closed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("closed_by").Optional().Nillable().Comment("关闭人用户ID"),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("support_tickets").
			Field("user_id").
			Required().
			Unique(),
		edge.From("assigned_admin", User.Type).
			Ref("assigned_support_tickets").
			Field("assigned_admin_id").
			Unique(),
		edge.From("closed_by_user", User.Type).
			Ref("closed_support_tickets").
			Field("closed_by").
			Unique(),
		edge.To("messages", SupportTicketMessage.Type),
	}
}

func (SupportTicket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("category"),
		index.Fields("priority"),
		index.Fields("assigned_admin_id"),
		index.Fields("last_message_at"),
		index.Fields("last_user_message_at"),
		index.Fields("last_admin_message_at"),
	}
}
