-- +goose Up
CREATE TABLE user_movies (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    movie_id    BIGINT      NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    status      TEXT        NOT NULL DEFAULT 'want_to_watch',
    user_rating SMALLINT,
    note        TEXT        NOT NULL DEFAULT '',
    source      TEXT        NOT NULL DEFAULT 'manual',
    source_url  TEXT        NOT NULL DEFAULT '',
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    watched_at  TIMESTAMPTZ,
    CONSTRAINT user_movies_unique UNIQUE (user_id, movie_id),
    CONSTRAINT user_movies_status_valid CHECK (status IN ('want_to_watch', 'watched')),
    CONSTRAINT user_movies_rating_range CHECK (user_rating IS NULL OR user_rating BETWEEN 1 AND 10)
);

CREATE INDEX user_movies_user_added_idx  ON user_movies (user_id, added_at DESC);
CREATE INDEX user_movies_user_status_idx ON user_movies (user_id, status);

-- +goose Down
DROP TABLE user_movies;
