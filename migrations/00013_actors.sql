-- +goose Up
-- Справочник актёров отдельный от режиссёров: один человек может быть и
-- там, и там. Поля — как у directors после 00012.
CREATE TABLE actors (
  id              BIGSERIAL PRIMARY KEY,
  tmdb_id         INTEGER UNIQUE CHECK (tmdb_id > 0),
  kinopoisk_id    INTEGER UNIQUE CHECK (kinopoisk_id > 0),
  name            TEXT        NOT NULL,
  original_name   TEXT        NOT NULL DEFAULT '',
  profile_path    TEXT        NOT NULL DEFAULT '',
  birth_date      DATE,
  death_date      DATE,
  place_of_birth  TEXT        NOT NULL DEFAULT '',
  biography       TEXT        NOT NULL DEFAULT '',
  biography_lang  TEXT        NOT NULL DEFAULT '',
  name_source     TEXT        NOT NULL DEFAULT 'tmdb'
    CONSTRAINT actors_name_source_check CHECK (name_source IN ('kinopoisk', 'tmdb_translation', 'tmdb')),
  works_synced_at TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (tmdb_id IS NOT NULL OR kinopoisk_id IS NOT NULL)
);

-- Состав работы: первые актёры титров в их порядке (position).
CREATE TABLE movie_actors (
  movie_id  BIGINT  NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
  actor_id  BIGINT  NOT NULL REFERENCES actors(id),
  character TEXT    NOT NULL DEFAULT '',
  position  INTEGER NOT NULL CHECK (position >= 0),
  PRIMARY KEY (movie_id, actor_id)
);
CREATE INDEX movie_actors_actor_movie_idx ON movie_actors (actor_id, movie_id);

-- Снимок ролей из каталога, как director_works.
CREATE TABLE actor_works (
  actor_id       BIGINT   NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  media_type     TEXT     NOT NULL CHECK (media_type IN ('movie', 'tv')),
  tmdb_id        INTEGER  NOT NULL CHECK (tmdb_id > 0),
  title          TEXT     NOT NULL,
  original_title TEXT     NOT NULL DEFAULT '',
  release_date   DATE,
  release_year   SMALLINT,
  poster_path    TEXT     NOT NULL DEFAULT '',
  rating         NUMERIC(3, 1),
  vote_count     INTEGER  NOT NULL DEFAULT 0 CHECK (vote_count >= 0),
  episode_count  INTEGER  CHECK (episode_count IS NULL OR episode_count > 0),
  character      TEXT     NOT NULL DEFAULT '',
  PRIMARY KEY (actor_id, media_type, tmdb_id)
);
CREATE INDEX actor_works_title_idx ON actor_works (media_type, tmdb_id);

CREATE TABLE user_actors (
  user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_id    BIGINT      NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  followed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, actor_id)
);
CREATE INDEX user_actors_actor_idx ON user_actors (actor_id);

-- NULL — состав ещё не загружен: работы, добавленные до появления актёров,
-- догружает фоновый тикер.
ALTER TABLE movies ADD COLUMN cast_fetched_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE movies DROP COLUMN cast_fetched_at;
DROP TABLE user_actors;
DROP TABLE actor_works;
DROP TABLE movie_actors;
DROP TABLE actors;
