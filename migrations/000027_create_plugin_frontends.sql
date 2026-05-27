-- +goose Up
CREATE TABLE IF NOT EXISTS plugin_frontends (
    plugin_id TEXT PRIMARY KEY REFERENCES wasm_plugins(id) ON DELETE CASCADE,
    entrypoint TEXT NOT NULL DEFAULT 'index.html',
    assets_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS plugin_frontends;
