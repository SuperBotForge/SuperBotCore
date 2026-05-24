package schema

import (
	"entgo.io/contrib/entoas"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"q+/internal/ent/schema/mixin"
)

// CourseInstance holds the schema definition for the CourseInstance entity.
type CourseInstance struct {
	ent.Schema
}

// Annotations of the CourseInstance.
func (CourseInstance) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose)),
		entoas.ReadOperation(entoas.OperationPolicy(entoas.PolicyExpose)),
	}
}

// Indexes of the CourseInstance.
func (CourseInstance) Indexes() []ent.Index {
	return []ent.Index{
		index.
			Fields("name").
			Edges("discord_server").
			Unique(),
	}
}

// Fields of the CourseInstance.
func (CourseInstance) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),

		field.String("queues_spreadsheet_id"),
	}
}

// Edges of the CourseInstance.
func (CourseInstance) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("criteria", Criterion.Type).
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.From("queue_templates", QueueTemplate.Type).
			Ref("course_instance").
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.From("discord_server", DiscordServer.Type).
			Ref("course_instances").
			Unique().
			Required(),
		edge.From("mark_table", MarkTable.Type).
			Ref("course_instance").
			Unique().
			Annotations(entoas.ReadOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.From("channels_for_course", ChannelsForCourse.Type).
			Ref("course_instance").
			Unique().
			Required(),
	}
}

func (CourseInstance) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}

// TODO cascade delete where appropriate
