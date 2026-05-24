package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/schema/mixin"
)

// MarkTable holds the schema definition for the MarkTable entity.
type MarkTable struct {
	ent.Schema
}

// Fields of the MarkTable.
func (MarkTable) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("spreadsheet_id"),
	}
}

// Edges of the MarkTable.
func (MarkTable) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("course_instance", CourseInstance.Type).
			Unique().
			Required(),
		edge.From("mark_table_tabs", MarkTableTab.Type).
			Ref("mark_table"),
	}
}

func (MarkTable) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
