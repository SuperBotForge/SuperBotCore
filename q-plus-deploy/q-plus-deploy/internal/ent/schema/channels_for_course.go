package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"q+/internal/ent/schema/mixin"
)

// ChannelsForCourse holds the schema definition for the ChannelsForCourse entity.
type ChannelsForCourse struct {
	ent.Schema
}

func (ChannelsForCourse) Indexes() []ent.Index {
	return []ent.Index{
		index.
			Fields("name").
			Edges("discord_server").
			Unique(),
	}
}

// Fields of the ChannelsForCourse.
func (ChannelsForCourse) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("teacher_channel_id").Unique(),
		field.String("student_channel_id").Unique(),
		field.String("queue_channel_id").Unique(),
	}
}

// Edges of the ChannelsForCourse.
func (ChannelsForCourse) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("discord_server", DiscordServer.Type).
			Unique().
			Required(),

		edge.To("course_instance", CourseInstance.Type).Unique(),
		//edge.To("queue_templates", QueueTemplate.Type),
		//edge.To("active_queue", Queue.Type).Unique(),
	}
}

func (ChannelsForCourse) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
