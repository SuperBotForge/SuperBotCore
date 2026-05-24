package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type DiscordMixin struct {
	mixin.Schema
}

func (DiscordMixin) Fields() []ent.Field {
	return []ent.Field{
		field.String("id"),
	}
}
