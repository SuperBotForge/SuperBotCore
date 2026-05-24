package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserAccount holds the schema definition for the UserAccount entity.
type UserAccount struct {
	ent.Schema
}

// Fields of the UserAccount.
func (UserAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Enum("type").Values("discord", "gmail"),
		field.String("account_identifier"), // discord id or google email
	}
}

func (UserAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_identifier", "type").
			Unique(),
	}
}

// Edges of the UserAccount.
func (UserAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_accounts").
			Unique().
			Required().
			Field("user_id"),
	}
}
