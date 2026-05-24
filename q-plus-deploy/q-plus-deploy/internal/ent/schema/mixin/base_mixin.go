package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"q+/internal/ent/rule"
	"q+/internal/generated/ent/privacy"
	"time"
)

type BaseMixin struct {
	mixin.Schema
}

func (BaseMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			Annotations(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (BaseMixin) Policy() ent.Policy {
	return privacy.Policy{
		Mutation: privacy.MutationPolicy{
			rule.AllowIfDiscordCommand(),
			rule.AllowIfCronTask(),
			//privacy.AlwaysDenyRule(),
		},
	}
}
