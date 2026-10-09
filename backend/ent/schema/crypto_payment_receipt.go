package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"time"
)

// CryptoPaymentReceipt survives order refunds and deletion to prevent reuse.
type CryptoPaymentReceipt struct{ ent.Schema }

func (CryptoPaymentReceipt) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "crypto_payment_receipts"}}
}

func (CryptoPaymentReceipt) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.String("chain_id").MaxLen(32),
		field.String("tx_hash").MaxLen(66),
		field.Int64("order_id"),
		field.String("token_contract").MaxLen(128).Default(""),
		field.Int64("event_index").Optional().Nillable(),
		field.String("amount_atomic").MaxLen(78).Default(""),
		field.String("source").MaxLen(32),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (CryptoPaymentReceipt) Indexes() []ent.Index {
	return []ent.Index{index.Fields("chain_id", "tx_hash").Unique(), index.Fields("order_id")}
}
