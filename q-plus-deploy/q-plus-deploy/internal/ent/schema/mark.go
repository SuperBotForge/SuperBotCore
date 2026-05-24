package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"time"
)

// Mark holds the schema definition for the Mark entity.
type Mark struct {
	ent.Schema
}

func (Mark) Annotations() []schema.Annotation {
	return []schema.Annotation{
		//field.ID("queue_place_id", "criterion_id"),
		field.ID("user_id", "criterion_id"),
	}
}

// Fields of the Mark.
func (Mark) Fields() []ent.Field {
	return []ent.Field{
		//field.Int64("queue_place_id"),
		field.Int64("user_id"), // student
		field.Int64("criterion_id"),
		field.String("value"),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
		field.Int64("updated_by_exam_id").
			Optional(),
	}
}

// Edges of the Mark.
func (Mark) Edges() []ent.Edge {
	return []ent.Edge{
		//edge.To("queue_place", QueuePlace.Type).
		//	Unique().
		//	Required().
		//	Field("queue_place_id"),
		edge.To("student", User.Type).
			Unique().
			Required().
			Field("user_id"),
		edge.To("criterion", Criterion.Type).
			Unique().
			Required().
			Field("criterion_id"),
		edge.To("updated_by", User.Type).
			Unique().
			Field("updated_by_exam_id"),
	}
}
