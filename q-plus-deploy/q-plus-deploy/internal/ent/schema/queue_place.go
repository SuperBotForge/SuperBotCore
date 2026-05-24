package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/schema/mixin"
)

// QueuePlace holds the schema definition for the QueuePlace entity.
type QueuePlace struct {
	ent.Schema
}

// Fields of the QueuePlace.
func (QueuePlace) Fields() []ent.Field {
	return []ent.Field{
		field.Int("position"),
		field.String("note"),
	}
}

// Edges of the QueuePlace.
func (QueuePlace) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("queue", Queue.Type).
			Ref("places").
			Unique().
			Required(),
		edge.To("criteria", Criterion.Type).
			Through("queue_place_criteria", QueuePlaceCriterion.Type),
		//edge.To("marked_criterion", Criterion.Type).
		//	Through("marks", Mark.Type),
		edge.To("team", User.Type),
		edge.From("current_examiner", Examiner.Type).
			Ref("current_queue_place"),
	}
}

func (QueuePlace) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
