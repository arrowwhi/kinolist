-- +goose Up
-- Состояние диалога живёт в БД, а не в памяти процесса: рестарт сервиса
-- не должен ронять висящие подтверждения фильмов.
CREATE TABLE bot_sessions (
    user_id    BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    state      TEXT        NOT NULL DEFAULT 'idle',
    job_id     BIGINT      REFERENCES enrichment_jobs (id) ON DELETE SET NULL,
    chat_id    BIGINT      NOT NULL,
    message_id INTEGER,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT bot_sessions_state_valid CHECK (state IN ('idle', 'awaiting_confirmation', 'awaiting_title'))
);

-- +goose Down
DROP TABLE bot_sessions;
