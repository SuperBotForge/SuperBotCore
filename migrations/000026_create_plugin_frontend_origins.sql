-- +goose Up

CREATE TABLE plugin_frontend_origins (
    plugin_id       VARCHAR(255) PRIMARY KEY,
    allowed_origins JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_plugin_frontend_origins_allowed_origins
    ON plugin_frontend_origins USING GIN (allowed_origins);

COMMENT ON TABLE plugin_frontend_origins IS 'Default frontend origins for plugin HTTP trigger CORS and auth redirects';
COMMENT ON COLUMN plugin_frontend_origins.allowed_origins IS 'Allowed frontend origins used by all plugin HTTP triggers unless a trigger has its own override';

-- +goose Down

DROP TABLE IF EXISTS plugin_frontend_origins;
