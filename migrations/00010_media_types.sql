-- +goose Up
ALTER TABLE movies
 ADD COLUMN media_type TEXT NOT NULL DEFAULT 'movie',
 ADD COLUMN season_count SMALLINT,
 ADD CONSTRAINT movies_media_type_check CHECK (media_type IN ('movie','tv')),
 ADD CONSTRAINT movies_season_count_check CHECK (season_count IS NULL OR (media_type='tv' AND season_count>0));
ALTER TABLE movies DROP CONSTRAINT movies_tmdb_id_key;
ALTER TABLE movies ADD CONSTRAINT movies_type_tmdb_id_key UNIQUE (media_type,tmdb_id);

-- TV-only TMDB genres join the same internal genre vocabulary.
INSERT INTO genres (tmdb_id,name_ru,name_en) VALUES
 (10759,'боевик и приключения','Action & Adventure'),
 (10762,'детский','Kids'), (10763,'новости','News'),
 (10764,'реалити','Reality'), (10765,'фантастика и фэнтези','Sci-Fi & Fantasy'),
 (10766,'мыльная опера','Soap'), (10767,'ток-шоу','Talk'),
 (10768,'война и политика','War & Politics')
 ON CONFLICT (tmdb_id) DO NOTHING;
DELETE FROM genre_external_map WHERE source='kinopoisk' AND external_name='сериал';

-- +goose Down
-- Fails safely if TV and movie IDs collide: never delete user records to downgrade.
ALTER TABLE movies DROP CONSTRAINT movies_type_tmdb_id_key;
ALTER TABLE movies ADD CONSTRAINT movies_tmdb_id_key UNIQUE (tmdb_id);
ALTER TABLE movies DROP CONSTRAINT movies_season_count_check;
ALTER TABLE movies DROP CONSTRAINT movies_media_type_check;
ALTER TABLE movies DROP COLUMN season_count, DROP COLUMN media_type;
INSERT INTO genre_external_map (genre_id,source,external_name)
 SELECT id,'kinopoisk','сериал' FROM genres WHERE tmdb_id=10770
 ON CONFLICT DO NOTHING;
-- Keep added genres: they may already be referenced by collection entries.
