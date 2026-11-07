-- +goose Up
-- Данные о человеке из каталога и снимок его фильмографии. Снимок нужен,
-- чтобы считать прогресс «посмотрено N из M» и сортировать список в SQL,
-- а не ходить в TMDB за каждым режиссёром при открытии экрана.
ALTER TABLE directors
  ADD COLUMN profile_path TEXT NOT NULL DEFAULT '',
  ADD COLUMN birth_date DATE,
  ADD COLUMN death_date DATE,
  ADD COLUMN place_of_birth TEXT NOT NULL DEFAULT '',
  ADD COLUMN biography TEXT NOT NULL DEFAULT '',
  ADD COLUMN biography_lang TEXT NOT NULL DEFAULT '',
  ADD COLUMN name_source TEXT NOT NULL DEFAULT 'tmdb',
  ADD COLUMN works_synced_at TIMESTAMPTZ,
  ADD CONSTRAINT directors_name_source_check
    CHECK (name_source IN ('kinopoisk', 'tmdb_translation', 'tmdb'));
-- Имена режиссёров, пришедших из Кинопоиска, уже русские.
UPDATE directors SET name_source = 'kinopoisk' WHERE tmdb_id IS NULL;

CREATE TABLE director_works (
  director_id    BIGINT   NOT NULL REFERENCES directors(id) ON DELETE CASCADE,
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
  PRIMARY KEY (director_id, media_type, tmdb_id)
);
CREATE INDEX director_works_title_idx ON director_works (media_type, tmdb_id);

CREATE TABLE user_directors (
  user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  director_id BIGINT      NOT NULL REFERENCES directors(id) ON DELETE CASCADE,
  followed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, director_id)
);
CREATE INDEX user_directors_director_idx ON user_directors (director_id);

-- +goose Down
DROP TABLE user_directors;
DROP TABLE director_works;
ALTER TABLE directors
  DROP CONSTRAINT directors_name_source_check,
  DROP COLUMN works_synced_at,
  DROP COLUMN name_source,
  DROP COLUMN biography_lang,
  DROP COLUMN biography,
  DROP COLUMN place_of_birth,
  DROP COLUMN death_date,
  DROP COLUMN birth_date,
  DROP COLUMN profile_path;
