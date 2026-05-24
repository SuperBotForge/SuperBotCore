package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"q+/internal/ent/schema/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("surname"),
		field.String("name"),
		field.String("patronymic"),
		field.String("group"),
		field.String("discord_id").
			Unique(),

		// student
		field.Bool("is_busy").
			Default(false),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_accounts", UserAccount.Type),

		// teacher
		edge.From("examiner_queues", Queue.Type).
			Ref("examiner_users").
			Through("examiners", Examiner.Type),
		edge.From("queue_places", QueuePlace.Type).
			Ref("team"),

		// student
		edge.To("marked_criteria", Criterion.Type).
			Through("marks", Mark.Type),
	}
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
	}
}
