-- +goose Up
CREATE TABLE IF NOT EXISTS bot_blacklist (
    user_id    BIGINT PRIMARY KEY,
    reason     TEXT NOT NULL DEFAULT '',
    blocked_by BIGINT,
    blocked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS bot_blacklist;
