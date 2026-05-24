package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/schema/mixin"
)

// Examiner holds the schema definition for the Examiner entity.
type Examiner struct {
	ent.Schema
}

// Fields of the Examiner.
func (Examiner) Fields() []ent.Field {
	return []ent.Field{
		field.String("note").Optional(),
		field.Int64("queue_id"),
		field.Int64("teacher_id"),
	}
}

// Edges of the Examiner.
func (Examiner) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("queue", Queue.Type).
			Unique().
			Required().
			Field("queue_id"),
		edge.To("teacher", User.Type).
			Unique().
			Required().
			Field("teacher_id"),
		edge.To("criteria", Criterion.Type),
		edge.To("current_queue_place", QueuePlace.Type).
			Unique().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Examiner) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
