-- +goose Up
CREATE TABLE directors (
 id BIGSERIAL PRIMARY KEY,
 tmdb_id INTEGER UNIQUE CHECK (tmdb_id > 0),
 kinopoisk_id INTEGER UNIQUE CHECK (kinopoisk_id > 0),
 name TEXT NOT NULL,
 original_name TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (tmdb_id IS NOT NULL OR kinopoisk_id IS NOT NULL)
);
CREATE TABLE movie_directors (
 movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
 director_id BIGINT NOT NULL REFERENCES directors(id),
 position INTEGER NOT NULL CHECK (position >= 0),
 PRIMARY KEY (movie_id, director_id)
);
CREATE INDEX movie_directors_director_movie_idx ON movie_directors(director_id, movie_id);
ALTER TABLE movies ADD COLUMN directors_fetched_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE movies DROP COLUMN directors_fetched_at;
DROP TABLE movie_directors;
DROP TABLE directors;
