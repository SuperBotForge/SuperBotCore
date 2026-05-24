package client

import (
	"context"
	"database/sql"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"q+/internal/generated/ent"
)
import _ "q+/internal/generated/ent/runtime"

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Debug    bool
}

// EntClientWrapper only needs to implement the do.Shutdowner interface
type EntClientWrapper struct {
	*ent.Client
}

func NewEntClient(i do.Injector) (*ent.Client, error) {
	client := do.MustInvoke[*EntClientWrapper](i)
	return client.Client, nil
}

func NewEntClientWrapper(i do.Injector) (*EntClientWrapper, error) {
	config := do.MustInvoke[DBConfig](i)

	db, err := sql.Open("pgx", fmt.Sprintf("postgres://%s:%s@%s:%s/%s", config.User, config.Password, config.Host, config.Port, config.Database))
	if err != nil {
		return nil, err
	}

	drv := entsql.OpenDB(dialect.Postgres, db)
	var client *ent.Client

	if config.Debug {
		client = ent.NewClient(ent.Driver(drv), ent.Debug())
	} else {
		client = ent.NewClient(ent.Driver(drv))
	}

	return &EntClientWrapper{client}, nil
}

func (c *EntClientWrapper) Shutdown(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	log.Info().
		Str("event", "closing_db").
		Msg("Closing DB connection...")
	return c.Close()
}
