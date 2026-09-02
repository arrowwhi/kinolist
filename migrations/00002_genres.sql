-- +goose Up
CREATE TABLE genres (
    id      SMALLSERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL UNIQUE,
    name_ru TEXT    NOT NULL,
    name_en TEXT    NOT NULL
);

-- Каждый внешний источник называет жанры по-своему; без этой таблицы
-- "Sci-Fi" из Кинопоиска и "Фантастика" из TMDB стали бы разными жанрами.
CREATE TABLE genre_external_map (
    genre_id      SMALLINT NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    source        TEXT     NOT NULL,
    external_name TEXT     NOT NULL,
    PRIMARY KEY (source, external_name)
);

INSERT INTO genres (tmdb_id, name_ru, name_en) VALUES
    (28,    'боевик',          'Action'),
    (12,    'приключения',     'Adventure'),
    (16,    'мультфильм',      'Animation'),
    (35,    'комедия',         'Comedy'),
    (80,    'криминал',        'Crime'),
    (99,    'документальный',  'Documentary'),
    (18,    'драма',           'Drama'),
    (10751, 'семейный',        'Family'),
    (14,    'фэнтези',         'Fantasy'),
    (36,    'история',         'History'),
    (27,    'ужасы',           'Horror'),
    (10402, 'музыка',          'Music'),
    (9648,  'детектив',        'Mystery'),
    (10749, 'мелодрама',       'Romance'),
    (878,   'фантастика',      'Science Fiction'),
    (10770, 'телефильм',       'TV Movie'),
    (53,    'триллер',         'Thriller'),
    (10752, 'военный',         'War'),
    (37,    'вестерн',         'Western');

INSERT INTO genre_external_map (genre_id, source, external_name)
SELECT id, 'kinopoisk', name_ru FROM genres;

INSERT INTO genre_external_map (genre_id, source, external_name)
SELECT g.id, 'kinopoisk', v.alias
FROM genres g
JOIN (VALUES
    (878,   'научная фантастика'),
    (10749, 'романтика'),
    (10749, 'romance'),
    (16,    'аниме'),
    (99,    'документальное'),
    (10752, 'военные'),
    (10770, 'сериал')
) AS v (tmdb_id, alias) ON v.tmdb_id = g.tmdb_id;

-- +goose Down
DROP TABLE genre_external_map;
DROP TABLE genres;
