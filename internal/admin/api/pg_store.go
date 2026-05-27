package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgPluginStore struct {
	pool *pgxpool.Pool
}

func NewPgPluginStore(pool *pgxpool.Pool) *PgPluginStore {
	return &PgPluginStore{pool: pool}
}

func (s *PgPluginStore) SavePlugin(ctx context.Context, record PluginRecord) error {
	configJSON := json.RawMessage("null")
	if len(record.ConfigJSON) > 0 {
		configJSON = record.ConfigJSON
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO wasm_plugins (id, wasm_key, config_json, enabled, wasm_hash, installed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			wasm_key    = EXCLUDED.wasm_key,
			config_json = EXCLUDED.config_json,
			enabled     = EXCLUDED.enabled,
			wasm_hash   = EXCLUDED.wasm_hash,
			updated_at  = EXCLUDED.updated_at
	`,
		record.ID,
		record.WasmKey,
		configJSON,
		record.Enabled,
		record.WasmHash,
		record.InstalledAt,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert plugin %q: %w", record.ID, err)
	}
	return nil
}

func (s *PgPluginStore) GetPlugin(ctx context.Context, id string) (PluginRecord, error) {
	var rec PluginRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, wasm_key, config_json, enabled, wasm_hash, installed_at, updated_at
		FROM wasm_plugins
		WHERE id = $1
	`, id).Scan(
		&rec.ID,
		&rec.WasmKey,
		&rec.ConfigJSON,
		&rec.Enabled,
		&rec.WasmHash,
		&rec.InstalledAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return PluginRecord{}, fmt.Errorf("get plugin %q: %w", id, err)
	}
	return rec, nil
}

func (s *PgPluginStore) ListPlugins(ctx context.Context) ([]PluginRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, wasm_key, config_json, enabled, wasm_hash, installed_at, updated_at
		FROM wasm_plugins
		ORDER BY installed_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()

	var records []PluginRecord
	for rows.Next() {
		var rec PluginRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.WasmKey,
			&rec.ConfigJSON,
			&rec.Enabled,
			&rec.WasmHash,
			&rec.InstalledAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plugin row: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin rows: %w", err)
	}
	return records, nil
}

func (s *PgPluginStore) DeletePlugin(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wasm_plugins WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete plugin %q: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("plugin %q not found", id)
	}
	return nil
}

func (s *PgPluginStore) SavePluginMetadata(ctx context.Context, record PluginMetadataRecord) error {
	metaJSON := json.RawMessage("null")
	if len(record.MetaJSON) > 0 {
		metaJSON = record.MetaJSON
	}
	configSchema := json.RawMessage("null")
	if len(record.ConfigSchema) > 0 {
		configSchema = record.ConfigSchema
	}
	requirements := json.RawMessage("null")
	if len(record.Requirements) > 0 {
		requirements = record.Requirements
	}
	triggers := json.RawMessage("null")
	if len(record.Triggers) > 0 {
		triggers = record.Triggers
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO wasm_plugin_metadata (
			plugin_id, name, version, sdk_version, meta_json, config_schema, requirements, triggers, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (plugin_id) DO UPDATE SET
			name          = EXCLUDED.name,
			version       = EXCLUDED.version,
			sdk_version   = EXCLUDED.sdk_version,
			meta_json     = EXCLUDED.meta_json,
			config_schema = EXCLUDED.config_schema,
			requirements  = EXCLUDED.requirements,
			triggers      = EXCLUDED.triggers,
			updated_at    = EXCLUDED.updated_at
	`,
		record.PluginID,
		record.Name,
		record.Version,
		record.SDKVersion,
		metaJSON,
		configSchema,
		requirements,
		triggers,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert plugin metadata %q: %w", record.PluginID, err)
	}
	return nil
}

func (s *PgPluginStore) GetPluginMetadata(ctx context.Context, id string) (PluginMetadataRecord, error) {
	var rec PluginMetadataRecord
	err := s.pool.QueryRow(ctx, `
		SELECT plugin_id, name, version, sdk_version, meta_json, config_schema, requirements, triggers, updated_at
		FROM wasm_plugin_metadata
		WHERE plugin_id = $1
	`, id).Scan(
		&rec.PluginID,
		&rec.Name,
		&rec.Version,
		&rec.SDKVersion,
		&rec.MetaJSON,
		&rec.ConfigSchema,
		&rec.Requirements,
		&rec.Triggers,
		&rec.UpdatedAt,
	)
	if err != nil {
		return PluginMetadataRecord{}, fmt.Errorf("get plugin metadata %q: %w", id, err)
	}
	return rec, nil
}

func (s *PgPluginStore) DeletePluginMetadata(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wasm_plugin_metadata WHERE plugin_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete plugin metadata %q: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("plugin metadata %q not found", id)
	}
	return nil
}

func (s *PgPluginStore) SavePluginFrontend(ctx context.Context, record PluginFrontendRecord) error {
	assetsJSON, err := json.Marshal(record.Assets)
	if err != nil {
		return fmt.Errorf("marshal plugin frontend assets %q: %w", record.PluginID, err)
	}
	entrypoint := record.Entrypoint
	if entrypoint == "" {
		entrypoint = "index.html"
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO plugin_frontends (plugin_id, entrypoint, assets_json, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (plugin_id) DO UPDATE SET
			entrypoint  = EXCLUDED.entrypoint,
			assets_json = EXCLUDED.assets_json,
			updated_at  = NOW()
	`,
		record.PluginID,
		entrypoint,
		assetsJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert plugin frontend %q: %w", record.PluginID, err)
	}
	return nil
}

func (s *PgPluginStore) GetPluginFrontend(ctx context.Context, pluginID string) (PluginFrontendRecord, error) {
	var rec PluginFrontendRecord
	var assetsJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT plugin_id, entrypoint, assets_json, created_at, updated_at
		FROM plugin_frontends
		WHERE plugin_id = $1
	`, pluginID).Scan(
		&rec.PluginID,
		&rec.Entrypoint,
		&assetsJSON,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return PluginFrontendRecord{}, fmt.Errorf("get plugin frontend %q: %w", pluginID, err)
	}
	if len(assetsJSON) > 0 {
		if err := json.Unmarshal(assetsJSON, &rec.Assets); err != nil {
			return PluginFrontendRecord{}, fmt.Errorf("decode plugin frontend assets %q: %w", pluginID, err)
		}
	}
	return rec, nil
}

func (s *PgPluginStore) DeletePluginFrontend(ctx context.Context, pluginID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM plugin_frontends WHERE plugin_id = $1`, pluginID)
	if err != nil {
		return fmt.Errorf("delete plugin frontend %q: %w", pluginID, err)
	}
	return nil
}

var _ PluginStore = (*PgPluginStore)(nil)
var _ PluginFrontendStore = (*PgPluginStore)(nil)
