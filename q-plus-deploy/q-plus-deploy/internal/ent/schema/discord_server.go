package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"q+/internal/ent/schema/mixin"
)

// DiscordServer holds the schema definition for the DiscordServer entity.
type DiscordServer struct {
	ent.Schema
}

// Fields of the DiscordServer.
func (DiscordServer) Fields() []ent.Field {
	return nil
}

// Edges of the DiscordServer.
func (DiscordServer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channels_for_courses", ChannelsForCourse.Type).
			Ref("discord_server"),
		edge.To("course_instances", CourseInstance.Type),
		edge.To("discord_channels", DiscordChannel.Type),
		edge.To("discord_roles", DiscordRole.Type),
	}
}

func (DiscordServer) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.DiscordMixin{},
	}
}
