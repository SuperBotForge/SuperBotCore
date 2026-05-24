package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// QueuePlaceCriterion holds the schema definition for the QueuePlaceCriterion entity.
type QueuePlaceCriterion struct {
	ent.Schema
}

func (QueuePlaceCriterion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		field.ID("queue_place_id", "criterion_id"),
	}
}

// Fields of the QueuePlaceCriterion.
func (QueuePlaceCriterion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("queue_place_id"),
		field.Int64("criterion_id"),
		field.Bool("passed"),
	}
}

// Edges of the QueuePlaceCriterion.
func (QueuePlaceCriterion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("queue_place", QueuePlace.Type).
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Field("queue_place_id"),
		edge.To("criterion", Criterion.Type).
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Field("criterion_id"),
	}
}
