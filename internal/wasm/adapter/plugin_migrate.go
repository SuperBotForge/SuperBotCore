package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"testing/fstest"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

var unsafeCharsRe = regexp.MustCompile(`[^a-z0-9_]`)
var unsafeIdentifierCharsRe = regexp.MustCompile(`[^a-z0-9_]`)

func sanitizeDescription(desc string) string {
	s := strings.ToLower(strings.TrimSpace(desc))
	s = strings.ReplaceAll(s, " ", "_")
	s = unsafeCharsRe.ReplaceAllString(s, "")
	if s == "" {
		s = "migration"
	}
	return s
}

func pluginMigrationTableName(pluginID string) string {
	s := strings.ToLower(strings.TrimSpace(pluginID))
	s = unsafeIdentifierCharsRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "plugin"
	}
	return "_goose_plugin_" + s
}

func pluginSchemaName(pluginID string) string {
	s := strings.ToLower(strings.TrimSpace(pluginID))
	s = unsafeIdentifierCharsRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "plugin"
	}
	return "plugin_" + s
}

// runPluginMigrations runs goose SQL migrations declared in plugin metadata.
// Each plugin gets its own goose version table (_goose_plugin_{pluginID}).
func runPluginMigrations(ctx context.Context, pluginID, dsn string, migrations []wasmrt.MigrationDef) error {
	if len(migrations) == 0 {
		return nil
	}

	// Build in-memory FS with goose-formatted SQL files.
	fsys := make(fstest.MapFS, len(migrations))
	for _, m := range migrations {
		filename := fmt.Sprintf("%06d_%s.sql", m.Version, sanitizeDescription(m.Description))

		content := "-- +goose Up\n" + m.Up + "\n"
		if m.Down != "" {
			content += "\n-- +goose Down\n" + m.Down + "\n"
		}

		fsys[filename] = &fstest.MapFile{Data: []byte(content)}
	}

	// Open a dedicated connection for goose (separate from the plugin's pgxpool).
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	schema := pgx.Identifier{pluginSchemaName(pluginID)}.Sanitize()
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		return fmt.Errorf("create plugin schema: %w", err)
	}

	// Per-plugin goose tracking table.
	tableName := pluginMigrationTableName(pluginID)
	store, err := database.NewStore(database.DialectPostgres, tableName)
	if err != nil {
		return fmt.Errorf("create goose store: %w", err)
	}

	provider, err := goose.NewProvider("", db, fsys, goose.WithStore(store))
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	for _, r := range results {
		slog.Info("wasm: plugin migration applied",
			"plugin", pluginID,
			"version", r.Source.Version,
			"duration_ms", r.Duration.Milliseconds(),
		)
	}

	if len(results) == 0 {
		slog.Debug("wasm: no pending migrations", "plugin", pluginID)
	}

	return nil
}

// dropPluginMigrations rolls back every goose migration declared by the plugin
// and drops the per-plugin goose tracking table. It is the inverse of
// runPluginMigrations and is called when a plugin is being uninstalled so that
// its schema artifacts don't linger in the shared database.
func dropPluginMigrations(ctx context.Context, pluginID, dsn string, migrations []wasmrt.MigrationDef) error {
	if len(migrations) == 0 {
		return nil
	}

	fsys := make(fstest.MapFS, len(migrations))
	for _, m := range migrations {
		filename := fmt.Sprintf("%06d_%s.sql", m.Version, sanitizeDescription(m.Description))
		content := "-- +goose Up\n" + m.Up + "\n"
		if m.Down != "" {
			content += "\n-- +goose Down\n" + m.Down + "\n"
		}
		fsys[filename] = &fstest.MapFile{Data: []byte(content)}
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	tableName := pluginMigrationTableName(pluginID)
	store, err := database.NewStore(database.DialectPostgres, tableName)
	if err != nil {
		return fmt.Errorf("create goose store: %w", err)
	}

	provider, err := goose.NewProvider("", db, fsys, goose.WithStore(store))
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	results, err := provider.DownTo(ctx, 0)
	if err != nil {
		return fmt.Errorf("run down migrations: %w", err)
	}
	for _, r := range results {
		slog.Info("wasm: plugin migration reverted",
			"plugin", pluginID,
			"version", r.Source.Version,
			"duration_ms", r.Duration.Milliseconds(),
		)
	}

	quoted := pgx.Identifier{tableName}.Sanitize()
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+quoted); err != nil {
		return fmt.Errorf("drop goose tracking table: %w", err)
	}

	schemaQuoted := pgx.Identifier{pluginSchemaName(pluginID)}.Sanitize()
	if _, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schemaQuoted+" CASCADE"); err != nil {
		return fmt.Errorf("drop plugin schema: %w", err)
	}

	return nil
}
