-- +goose Up
CREATE TABLE IF NOT EXISTS plugin_settings (
    plugin_id         TEXT        NOT NULL PRIMARY KEY,
    policy_expression TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS plugin_settings;
