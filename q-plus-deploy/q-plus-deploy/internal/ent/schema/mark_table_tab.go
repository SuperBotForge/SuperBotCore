package schema

import (
	"entgo.io/contrib/entoas"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/rule"
	"q+/internal/ent/schema/mixin"
	"q+/internal/generated/ent/privacy"
)

// MarkTableTab holds the schema definition for the MarkTableTab entity.
type MarkTableTab struct {
	ent.Schema
}

// Annotations of the MarkTableTab.
func (MarkTableTab) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entoas.CreateOperation(entoas.OperationPolicy(entoas.PolicyExpose)),
	}
}

// Fields of the MarkTableTab.
func (MarkTableTab) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Int64("sheet_id"),
	}
}

// Edges of the MarkTableTab.
func (MarkTableTab) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("mark_table", MarkTable.Type).
			Unique().
			Required(),
		edge.From("queue_templates", QueueTemplate.Type).
			Ref("mark_table_tab"),
		edge.From("queues", Queue.Type).
			Ref("mark_table_tab"),
	}
}

func (MarkTableTab) Policy() ent.Policy {
	return privacy.Policy{
		Mutation: privacy.MutationPolicy{
			privacy.OnMutationOperation(
				rule.MarkTableTabAllowOnlyCertainCourseId(),
				ent.OpCreate,
			),
		},
	}
}

func (MarkTableTab) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
