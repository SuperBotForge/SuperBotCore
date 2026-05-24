package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"q+/internal/ent/schema/mixin"
)

// Criterion holds the schema definition for the Criterion entity.
type Criterion struct {
	ent.Schema
}

// Indexes of the Criterion.
func (Criterion) Indexes() []ent.Index {
	return []ent.Index{
		index.
			Fields("name").
			Edges("course_instance").
			Unique(),
	}
}

// Fields of the Criterion.
func (Criterion) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
	}
}

// Edges of the Criterion.
func (Criterion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("queue_places", QueuePlace.Type).
			Ref("criteria").
			Through("queue_place_criteria", QueuePlaceCriterion.Type),
		edge.From("course_instance", CourseInstance.Type).
			Ref("criteria").
			Unique().
			Required(),
		edge.From("examiners", Examiner.Type).
			Ref("criteria"),
		//edge.From("marked_queue_places", QueuePlace.Type).
		//	Ref("marked_criterion").
		//	Through("marks", Mark.Type),
		edge.From("marked_students", User.Type).
			Ref("marked_criteria").
			Through("marks", Mark.Type),
		edge.From("queue_templates", QueueTemplate.Type).
			Ref("criteria"),
		edge.From("queues", Queue.Type).
			Ref("criteria"),
	}
}

func (Criterion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
