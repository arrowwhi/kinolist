-- +goose Up
CREATE TABLE movies (
    id             BIGSERIAL PRIMARY KEY,
    tmdb_id        INTEGER UNIQUE,
    kinopoisk_id   INTEGER UNIQUE,
    title          TEXT        NOT NULL,
    original_title TEXT        NOT NULL DEFAULT '',
    release_year   SMALLINT,
    overview       TEXT        NOT NULL DEFAULT '',
    poster_path    TEXT        NOT NULL DEFAULT '',
    runtime_min    SMALLINT,
    rating         NUMERIC(3, 1),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT movies_external_id_present CHECK (tmdb_id IS NOT NULL OR kinopoisk_id IS NOT NULL)
);

CREATE TABLE movie_genres (
    movie_id BIGINT   NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    genre_id SMALLINT NOT NULL REFERENCES genres (id) ON DELETE RESTRICT,
    PRIMARY KEY (movie_id, genre_id)
);

CREATE INDEX movie_genres_genre_id_idx ON movie_genres (genre_id);

-- +goose Down
DROP TABLE movie_genres;
DROP TABLE movies;
