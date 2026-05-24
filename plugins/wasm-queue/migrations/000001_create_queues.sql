-- +goose Up
CREATE TABLE IF NOT EXISTS queues (
    id         SERIAL PRIMARY KEY,
    chat_id    TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    status     TEXT    NOT NULL DEFAULT 'open',
    created_by BIGINT  NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS queue_places (
    id        SERIAL PRIMARY KEY,
    queue_id  INTEGER NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    user_id   BIGINT  NOT NULL,
    position  INTEGER NOT NULL,
    status    TEXT    NOT NULL DEFAULT 'waiting',
    joined_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_queues_chat_name
    ON queues(chat_id, name)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_queues_chat_status
    ON queues(chat_id, status);

CREATE INDEX IF NOT EXISTS idx_queue_places_queue_status
    ON queue_places(queue_id, status, position);

CREATE UNIQUE INDEX IF NOT EXISTS idx_queue_places_user_active
    ON queue_places(queue_id, user_id)
    WHERE status = 'waiting';

-- +goose Down
DROP TABLE IF EXISTS queue_places;
DROP TABLE IF EXISTS queues;
