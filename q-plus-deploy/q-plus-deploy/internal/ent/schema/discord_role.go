package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/schema/mixin"
)

// DiscordRole holds the schema definition for the DiscordRole entity.
type DiscordRole struct {
	ent.Schema
}

// Fields of the DiscordRole.
func (DiscordRole) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("type").
			Values("student", "examiner"),
	}
}

// Edges of the DiscordRole.
func (DiscordRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("discord_server", DiscordServer.Type).
			Ref("discord_roles").
			Unique().
			Required(),
	}
}

func (DiscordRole) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.DiscordMixin{},
	}
}
