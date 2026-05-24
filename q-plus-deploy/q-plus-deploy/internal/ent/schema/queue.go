package schema

import (
	"entgo.io/contrib/entoas"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/schema/mixin"
	"time"
)

// Queue holds the schema definition for the Queue entity.
type Queue struct {
	ent.Schema
}

// Annotations of the Queue.
func (Queue) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose)),
	}
}

// Fields of the Queue.
func (Queue) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Int64("sign_up_lead_time").
			GoType(time.Duration(0)),
		field.Bool("sign_up_started").
			Default(false),
		field.Time("start_time").
			Optional().
			Nillable(),
		field.Bool("queue_started").
			Default(false),
		field.Time("end_time").
			Optional().
			Nillable(),
		field.Bool("queue_ended").
			Default(false),
		field.Bool("simple_queue").
			Default(false),

		field.Int64("sheet_id").
			Optional().
			Nillable(),
	}
}

// Edges of the Queue.
func (Queue) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("places", QueuePlace.Type).
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.To("queue_template", QueueTemplate.Type).
			Unique().
			Required(),
		edge.To("criteria", Criterion.Type).
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),
		edge.To("mark_table_tab", MarkTableTab.Type).
			Unique().
			Required(),
		edge.To("examiner_users", User.Type).
			Through("examiners", Examiner.Type).
			Annotations(entoas.ListOperation(entoas.OperationPolicy(entoas.PolicyExpose))),

		//edge.From("channels_for_course", ChannelsForCourse.Type).
		//	Ref("queues").
		//	Unique().
		//	Required(),
	}
}

func (Queue) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
