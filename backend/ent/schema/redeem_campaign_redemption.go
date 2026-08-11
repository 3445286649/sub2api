package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RedeemCampaignRedemption struct {
	ent.Schema
}

func (RedeemCampaignRedemption) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_campaign_redemptions"},
	}
}

func (RedeemCampaignRedemption) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("campaign_id"),
		field.Int64("user_id"),
		field.Int64("redeem_code_id").
			Optional().
			Nillable(),
		field.Time("redeemed_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RedeemCampaignRedemption) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("campaign_id", "user_id").Unique(),
		index.Fields("redeem_code_id").Unique(),
		index.Fields("user_id"),
	}
}
