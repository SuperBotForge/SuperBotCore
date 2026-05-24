package schema

import (
	"entgo.io/contrib/entoas"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"q+/internal/ent/rule"
	"q+/internal/ent/schema/mixin"
	"q+/internal/generated/ent/privacy"
	"time"
)

// QueueTemplate holds the schema definition for the QueueTemplate entity.
type QueueTemplate struct {
	ent.Schema
}

// Annotations of the QueueTemplate.
func (QueueTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entoas.CreateOperation(entoas.OperationPolicy(entoas.PolicyExpose)),
		entoas.ReadOperation(entoas.OperationPolicy(entoas.PolicyExpose)),
	}
}

// Indexes of the QueueTemplate.
func (QueueTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.
			Fields("name").
			Edges("course_instance").
			Unique(),
	}

}

// Fields of the QueueTemplate.
func (QueueTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Int64("sign_up_lead_time").
			GoType(time.Duration(0)),
	}
}

// Edges of the QueueTemplate.
func (QueueTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("course_instance", CourseInstance.Type).
			Unique().
			Required(),
		edge.From("queues", Queue.Type).
			Ref("queue_template").
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.To("criteria", Criterion.Type).
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.To("mark_table_tab", MarkTableTab.Type).
			Unique().
			Required().
			Annotations(entoas.ReadOperation(entoas.OperationPolicy(entoas.PolicyExpose))),

		//edge.From("channels_for_course", ChannelsForCourse.Type).
		//	Ref("queue_templates").
		//	Unique().
		//	Required(),
	}
}

func (QueueTemplate) Policy() ent.Policy {
	return privacy.Policy{
		Mutation: privacy.MutationPolicy{
			privacy.OnMutationOperation(
				rule.QueueTemplateAllowOnlyCertainCourseId(),
				ent.OpCreate,
			),
		},
	}
}

func (QueueTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
