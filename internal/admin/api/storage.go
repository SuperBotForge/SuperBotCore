package api

import (
	"context"
	"encoding/json"
	"time"
)

type PluginRecord struct {
	ID          string          `json:"id"`
	WasmKey     string          `json:"wasm_key"`
	ConfigJSON  json.RawMessage `json:"config_json,omitempty"`
	Enabled     bool            `json:"enabled"`
	WasmHash    string          `json:"wasm_hash"`
	InstalledAt time.Time       `json:"installed_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type PluginMetadataRecord struct {
	PluginID     string          `json:"plugin_id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	SDKVersion   int             `json:"sdk_version"`
	MetaJSON     json.RawMessage `json:"meta_json,omitempty"`
	ConfigSchema json.RawMessage `json:"config_schema,omitempty"`
	Requirements json.RawMessage `json:"requirements,omitempty"`
	Triggers     json.RawMessage `json:"triggers,omitempty"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type PluginFrontendAsset struct {
	Path        string `json:"path"`
	Key         string `json:"key"`
	ContentType string `json:"content_type,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Size        int64  `json:"size"`
}

type PluginFrontendRecord struct {
	PluginID   string                `json:"plugin_id"`
	Entrypoint string                `json:"entrypoint"`
	Assets     []PluginFrontendAsset `json:"assets"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

type PluginStore interface {
	SavePlugin(ctx context.Context, record PluginRecord) error
	GetPlugin(ctx context.Context, id string) (PluginRecord, error)
	ListPlugins(ctx context.Context) ([]PluginRecord, error)
	DeletePlugin(ctx context.Context, id string) error
	SavePluginMetadata(ctx context.Context, record PluginMetadataRecord) error
	GetPluginMetadata(ctx context.Context, id string) (PluginMetadataRecord, error)
	DeletePluginMetadata(ctx context.Context, id string) error
}

type PluginFrontendStore interface {
	SavePluginFrontend(ctx context.Context, record PluginFrontendRecord) error
	GetPluginFrontend(ctx context.Context, pluginID string) (PluginFrontendRecord, error)
	DeletePluginFrontend(ctx context.Context, pluginID string) error
}

type VersionRecord struct {
	ID         int64           `json:"id"`
	PluginID   string          `json:"plugin_id"`
	Version    string          `json:"version"`
	WasmKey    string          `json:"wasm_key"`
	WasmHash   string          `json:"wasm_hash"`
	ConfigJSON json.RawMessage `json:"config_json,omitempty"`
	Changelog  string          `json:"changelog"`
	CreatedAt  time.Time       `json:"created_at"`
}

type VersionStore interface {
	SaveVersion(ctx context.Context, rec VersionRecord) (int64, error)
	ListVersions(ctx context.Context, pluginID string) ([]VersionRecord, error)
	GetVersion(ctx context.Context, id int64) (VersionRecord, error)
	DeleteVersion(ctx context.Context, id int64) error
	DeleteVersionsByPlugin(ctx context.Context, pluginID string) error
}
