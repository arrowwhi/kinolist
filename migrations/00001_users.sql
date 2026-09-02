-- +goose Up
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    telegram_id   BIGINT      NOT NULL UNIQUE,
    username      TEXT,
    first_name    TEXT,
    language_code TEXT        NOT NULL DEFAULT 'ru',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
