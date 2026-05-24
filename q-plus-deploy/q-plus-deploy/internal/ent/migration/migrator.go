package migration

import (
	"context"
	"github.com/samber/do/v2"
	"q+/internal/generated/ent"
)

type Migrator struct {
	Client *ent.Client
}

func NewMigrator(i do.Injector) (*Migrator, error) {
	return &Migrator{Client: do.MustInvoke[*ent.Client](i)}, nil
}

func (m *Migrator) RunAutomaticMigration(ctx context.Context) error {
	return m.Client.Schema.Create(ctx)
}
