package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"q+/internal/ent/schema/mixin"
)

// DiscordChannel holds the schema definition for the DiscordChannel entity.
type DiscordChannel struct {
	ent.Schema
}

// Fields of the DiscordChannel.
func (DiscordChannel) Fields() []ent.Field {
	return nil
}

// Edges of the DiscordChannel.
func (DiscordChannel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("discord_server", DiscordServer.Type).
			Ref("discord_channels").
			Unique().
			Required(),
	}
}

func (DiscordChannel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.DiscordMixin{},
	}
}
