-- +goose Up

ALTER TABLE plugin_command_settings
    ADD COLUMN allowed_origins JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX idx_plugin_cmd_settings_allowed_origins
    ON plugin_command_settings USING GIN (allowed_origins);

COMMENT ON COLUMN plugin_command_settings.allowed_origins IS 'Allowed frontend origins for credentialed HTTP trigger CORS and auth redirects';

-- +goose Down

DROP INDEX IF EXISTS idx_plugin_cmd_settings_allowed_origins;

ALTER TABLE plugin_command_settings
    DROP COLUMN IF EXISTS allowed_origins;
