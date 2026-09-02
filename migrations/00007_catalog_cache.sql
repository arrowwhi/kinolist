-- +goose Up
CREATE TABLE catalog_cache (
    cache_key  TEXT PRIMARY KEY,
    payload    JSONB       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX catalog_cache_expires_idx ON catalog_cache (expires_at);

-- +goose Down
DROP TABLE catalog_cache;
