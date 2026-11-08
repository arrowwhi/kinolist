package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

// Колонки внешних идентификаторов людей: из них выбирается колонка
// конфликта при upsert по составу работы.
const (
	columnTMDBID      = "tmdb_id"
	columnKinopoiskID = "kinopoisk_id"
)

// personDirectory описывает справочник людей одного раздела: режиссёров
// или актёров. Таблицы устроены одинаково, поэтому SQL общий и собирается
// из шаблона подстановкой имён — только из констант ниже, пользовательские
// строки в шаблон не попадают.
type personDirectory struct {
	// metric — префикс имени запроса в метриках: director, actor.
	metric string
	// people, links, works, follows — справочник, связи с работами в базе,
	// снимок фильмографии из каталога и подписки; column — колонка id
	// человека в трёх последних.
	people, links, works, follows, column string
	// character — есть ли у связей и снимка колонка персонажа.
	character bool
	// minCollection — сколько работ коллекции нужно, чтобы человек без
	// подписки попал в список раздела.
	minCollection int
	// knownFor — фильтр и порядок работ для known_for.
	knownForWhere, knownForOrder string
	notFound                     error
}

var (
	directorDirectory = personDirectory{
		metric: "director", people: "directors", links: "movie_directors", works: "director_works",
		follows: "user_directors", column: "director_id", minCollection: 1,
		knownForWhere: "rating IS NOT NULL", knownForOrder: "rating DESC NULLS LAST, vote_count DESC, title",
		notFound: movie.ErrDirectorNotFound,
	}
	// У актёров known_for — самые популярные роли: число голосов говорит о
	// известности работы лучше её оценки.
	actorDirectory = personDirectory{
		metric: "actor", people: "actors", links: "movie_actors", works: "actor_works",
		follows: "user_actors", column: "actor_id", character: true, minCollection: movie.MinCollectionRoles,
		knownForWhere: "vote_count > 0", knownForOrder: "vote_count DESC, rating DESC NULLS LAST, title",
		notFound: movie.ErrActorNotFound,
	}
)

// sql подставляет имена таблиц в шаблон.
func (d personDirectory) sql(template string) string {
	workChar, linkChar, charColumn := `''::text`, `''::text`, ``
	if d.character {
		workChar, linkChar, charColumn = `w0.character`, `l0.character`, `, character`
	}
	return strings.NewReplacer(
		"{people}", d.people, "{links}", d.links, "{works}", d.works, "{follows}", d.follows,
		"{col}", d.column, "{min}", strconv.Itoa(d.minCollection),
		"{work_char}", workChar, "{link_char}", linkChar, "{char_column}", charColumn,
		"{known_for_where}", d.knownForWhere, "{known_for_order}", d.knownForOrder,
	).Replace(template)
}

func (d personDirectory) name(method string) string { return d.metric + "." + method }

// personStore — общие запросы справочников людей.
type personStore struct {
	tx  *transactions.Manager
	dir personDirectory
}

// nameRankSQL повторяет movie.NameSource.Rank: имя из более надёжного
// источника не затирается менее надёжным.
func nameRankSQL(column string) string {
	return `(CASE ` + column + ` WHEN 'kinopoisk' THEN 3 WHEN 'tmdb_translation' THEN 2 ELSE 1 END)`
}

// upsertPerson сохраняет человека из TMDB и возвращает его id в справочнике.
// Если Кинопоиск опознал того же человека, а в справочнике уже есть
// строка только с kinopoisk_id, она сливается в TMDB-строку.
func (s personStore) upsertPerson(ctx context.Context, p movie.Person) (id int64, err error) {
	ctx = queryName(ctx, s.dir.name("upsert_person"))

	if p.TMDBID <= 0 || p.Name == "" {
		return 0, movie.ErrInvalidPersonID
	}
	ctx, err = s.tx.BeginTransaction(ctx)
	if err != nil {
		return 0, err
	}
	defer s.tx.FinishTransaction(ctx, &err)

	q := s.tx.Querier(ctx)
	source := p.NameSource
	if source == "" {
		source = movie.NameSourceTMDB
	}
	better := nameRankSQL("EXCLUDED.name_source") + ` >= ` + nameRankSQL("{people}.name_source")
	sql := s.dir.sql(`INSERT INTO {people} (tmdb_id, name, original_name, name_source, profile_path,
  birth_date, death_date, place_of_birth, biography, biography_lang)
 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
 ON CONFLICT (tmdb_id) DO UPDATE SET
  name = CASE WHEN ` + better + ` THEN EXCLUDED.name ELSE {people}.name END,
  name_source = CASE WHEN ` + better + ` THEN EXCLUDED.name_source ELSE {people}.name_source END,
  original_name = coalesce(nullif(EXCLUDED.original_name, ''), {people}.original_name),
  profile_path = coalesce(nullif(EXCLUDED.profile_path, ''), {people}.profile_path),
  birth_date = coalesce(EXCLUDED.birth_date, {people}.birth_date),
  death_date = coalesce(EXCLUDED.death_date, {people}.death_date),
  place_of_birth = coalesce(nullif(EXCLUDED.place_of_birth, ''), {people}.place_of_birth),
  biography = CASE WHEN EXCLUDED.biography <> '' THEN EXCLUDED.biography ELSE {people}.biography END,
  biography_lang = CASE WHEN EXCLUDED.biography <> '' THEN EXCLUDED.biography_lang ELSE {people}.biography_lang END,
  updated_at = now()
 RETURNING id`)
	if err = q.QueryRow(ctx, sql, p.TMDBID, p.Name, p.OriginalName, source, p.ProfilePath,
		p.BirthDate, p.DeathDate, p.PlaceOfBirth, p.Biography, p.BiographyLang).Scan(&id); err != nil {
		return 0, fmt.Errorf("upsert person: %w", err)
	}
	if p.KinopoiskID > 0 {
		if err = s.attachKinopoisk(ctx, id, p.KinopoiskID); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s personStore) attachKinopoisk(ctx context.Context, id int64, kinopoiskID int32) error {
	ctx = queryName(ctx, s.dir.name("attach_kinopoisk"))

	q := s.tx.Querier(ctx)
	var other int64
	var otherHasTMDB bool
	err := q.QueryRow(ctx, s.dir.sql(`SELECT id, tmdb_id IS NOT NULL FROM {people} WHERE kinopoisk_id=$1 AND id<>$2`), kinopoiskID, id).
		Scan(&other, &otherHasTMDB)
	switch {
	case isNoRows(err):
	case err != nil:
		return fmt.Errorf("find kinopoisk person: %w", err)
	case otherHasTMDB:
		// Тот же kinopoisk_id у другого TMDB-человека: сопоставление сомнительно, не сливаем.
		return nil
	default:
		// Связи переносятся на TMDB-строку; подписки и работы старой строки
		// удаляются каскадом вместе с ней.
		for _, stmt := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO {links} (movie_id, {col}, position{char_column})
			 SELECT movie_id, $2, position{char_column} FROM {links} WHERE {col}=$1 ON CONFLICT DO NOTHING`, []any{other, id}},
			{`INSERT INTO {follows} (user_id, {col}, followed_at)
			 SELECT user_id, $2, followed_at FROM {follows} WHERE {col}=$1 ON CONFLICT DO NOTHING`, []any{other, id}},
			{`DELETE FROM {links} WHERE {col}=$1`, []any{other}},
			{`DELETE FROM {people} WHERE id=$1`, []any{other}},
		} {
			if _, err := q.Exec(ctx, s.dir.sql(stmt.sql), stmt.args...); err != nil {
				return fmt.Errorf("merge kinopoisk person: %w", err)
			}
		}
	}
	if _, err := q.Exec(ctx, s.dir.sql(`UPDATE {people} SET kinopoisk_id=coalesce(kinopoisk_id, $2) WHERE id=$1`), id, kinopoiskID); err != nil {
		return fmt.Errorf("attach kinopoisk id: %w", err)
	}
	return nil
}

// replaceWorks заменяет снимок фильмографии и отмечает время синхронизации.
func (s personStore) replaceWorks(ctx context.Context, personID int64, works []movie.DirectorWork) (err error) {
	ctx = queryName(ctx, s.dir.name("replace_works"))

	ctx, err = s.tx.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	defer s.tx.FinishTransaction(ctx, &err)

	q := s.tx.Querier(ctx)
	if _, err = q.Exec(ctx, s.dir.sql(`DELETE FROM {works} WHERE {col}=$1`), personID); err != nil {
		return fmt.Errorf("clear works: %w", err)
	}
	if len(works) > 0 {
		columns := []string{
			s.dir.column, "media_type", "tmdb_id", "title", "original_title", "release_date",
			"release_year", "poster_path", "rating", "vote_count", "episode_count",
		}
		if s.dir.character {
			columns = append(columns, "character")
		}
		insert := builder.Insert(s.dir.works).Columns(columns...)
		seen := map[string]bool{}
		for _, w := range works {
			key := fmt.Sprintf("%s:%d", w.MediaType, w.TMDBID)
			if seen[key] || !w.MediaType.Valid() || w.TMDBID <= 0 || w.Title == "" {
				continue
			}
			seen[key] = true
			var year *int16
			if w.Year > 0 {
				year = &w.Year
			}
			values := []any{
				personID, w.MediaType, w.TMDBID, w.Title, w.OriginalTitle,
				w.ReleaseDate, year, w.PosterPath, w.Rating, w.VoteCount, w.EpisodeCount,
			}
			if s.dir.character {
				values = append(values, w.Character)
			}
			insert = insert.Values(values...)
		}
		if len(seen) > 0 {
			sql, args, buildErr := insert.ToSql()
			if buildErr != nil {
				return fmt.Errorf("build works insert: %w", buildErr)
			}
			if _, err = q.Exec(ctx, sql, args...); err != nil {
				return fmt.Errorf("insert works: %w", err)
			}
		}
	}
	return s.touchSynced(ctx, personID)
}

// touchSynced отмечает попытку синхронизации, не меняя снимок: человек
// пропал из каталога, и повторять запрос каждые полчаса незачем.
func (s personStore) touchSynced(ctx context.Context, personID int64) error {
	ctx = queryName(ctx, s.dir.name("touch_synced"))

	if _, err := s.tx.Querier(ctx).Exec(ctx, s.dir.sql(`UPDATE {people} SET works_synced_at=now() WHERE id=$1`), personID); err != nil {
		return fmt.Errorf("mark person synced: %w", err)
	}
	return nil
}

func (s personStore) syncState(ctx context.Context, where string, arg any) (movie.DirectorSyncState, error) {
	ctx = queryName(ctx, s.dir.name("sync_state"))

	var st movie.DirectorSyncState
	err := s.tx.Querier(ctx).QueryRow(ctx, s.dir.sql(`SELECT id, coalesce(tmdb_id, 0), name_source, works_synced_at
 FROM {people} WHERE `+where), arg).Scan(&st.ID, &st.TMDBID, &st.NameSource, &st.SyncedAt)
	if isNoRows(err) {
		return st, s.dir.notFound
	}
	if err != nil {
		return st, fmt.Errorf("get sync state: %w", err)
	}
	return st, nil
}

// stale — люди, которые кому-то нужны (подписка или нужное число работ в
// чьей-то коллекции), а фильмография не загружена или устарела.
func (s personStore) stale(ctx context.Context, olderThan time.Time, limit int) ([]movie.DirectorSyncState, error) {
	ctx = queryName(ctx, s.dir.name("stale"))

	rows, err := s.tx.Querier(ctx).Query(ctx, s.dir.sql(`SELECT p.id, p.tmdb_id, p.name_source, p.works_synced_at FROM {people} p
 WHERE p.tmdb_id IS NOT NULL AND (p.works_synced_at IS NULL OR p.works_synced_at < $1)
  AND (EXISTS (SELECT 1 FROM {follows} f WHERE f.{col} = p.id)
   OR EXISTS (SELECT 1 FROM {links} l JOIN user_movies um ON um.movie_id = l.movie_id
    WHERE l.{col} = p.id GROUP BY um.user_id HAVING count(*) >= {min}))
 ORDER BY p.works_synced_at NULLS FIRST, p.id LIMIT $2`), olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale people: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.DirectorSyncState, error) {
		var st movie.DirectorSyncState
		err := row.Scan(&st.ID, &st.TMDBID, &st.NameSource, &st.SyncedAt)
		return st, err
	})
}

func (s personStore) follow(ctx context.Context, userID, personID int64) error {
	ctx = queryName(ctx, s.dir.name("follow"))

	_, err := s.tx.Querier(ctx).Exec(ctx, s.dir.sql(`INSERT INTO {follows} (user_id, {col}) VALUES ($1, $2)
 ON CONFLICT DO NOTHING`), userID, personID)
	if hasCode(err, codeForeignKeyViolation) {
		return s.dir.notFound
	}
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	return nil
}

func (s personStore) unfollow(ctx context.Context, userID, personID int64) error {
	ctx = queryName(ctx, s.dir.name("unfollow"))

	if _, err := s.tx.Querier(ctx).Exec(ctx, s.dir.sql(`DELETE FROM {follows} WHERE user_id=$1 AND {col}=$2`), userID, personID); err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}
	return nil
}

// knownByTMDB сопоставляет людей из поиска с уже заведёнными в справочнике.
func (s personStore) knownByTMDB(ctx context.Context, userID int64, tmdbIDs []int32) (map[int32]movie.PersonSearchResult, error) {
	ctx = queryName(ctx, s.dir.name("known_by_tmdb"))

	out := make(map[int32]movie.PersonSearchResult, len(tmdbIDs))
	if len(tmdbIDs) == 0 {
		return out, nil
	}
	rows, err := s.tx.Querier(ctx).Query(ctx, s.dir.sql(`SELECT p.id, p.tmdb_id, p.name, p.name_source,
  EXISTS (SELECT 1 FROM {follows} f WHERE f.user_id=$1 AND f.{col}=p.id)
 FROM {people} p WHERE p.tmdb_id = ANY($2)`), userID, tmdbIDs)
	if err != nil {
		return nil, fmt.Errorf("find known people: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p movie.PersonSearchResult
		if err := rows.Scan(&p.PersonID, &p.TMDBID, &p.Name, &p.NameSource, &p.Followed); err != nil {
			return nil, fmt.Errorf("scan known person: %w", err)
		}
		out[p.TMDBID] = p
	}
	return out, rows.Err()
}

// overviewSQL строит строки списка раздела для набора людей из CTE mine
// (колонка person_id). Работы — объединение снимка из каталога с работами
// коллекции, которых в снимке нет (фильмы только из Кинопоиска,
// расхождения в credits TMDB). Параметр $1 — всегда id пользователя.
func (s personStore) overviewSQL(mine string) string {
	return s.dir.sql(`WITH mine AS (` + mine + `),
 works AS (
  SELECT w0.{col} AS person_id, m.id AS movie_id FROM {works} w0
   LEFT JOIN movies m ON m.media_type = w0.media_type AND m.tmdb_id = w0.tmdb_id
   WHERE w0.{col} IN (SELECT person_id FROM mine)
  UNION ALL
  SELECT l0.{col}, l0.movie_id FROM {links} l0
   JOIN movies m ON m.id = l0.movie_id
   JOIN user_movies um ON um.movie_id = l0.movie_id AND um.user_id = $1
   WHERE l0.{col} IN (SELECT person_id FROM mine)
    AND NOT EXISTS (SELECT 1 FROM {works} w1
     WHERE w1.{col} = l0.{col} AND w1.media_type = m.media_type AND w1.tmdb_id = m.tmdb_id)
 ),
 stats AS (
  SELECT w.person_id, count(*) AS work_count, count(um.id) AS in_collection,
   count(*) FILTER (WHERE um.status = 'watched') AS watched,
   count(*) FILTER (WHERE um.status = 'want_to_watch') AS want,
   count(um.user_rating) FILTER (WHERE um.status = 'watched') AS rated,
   round(avg(um.user_rating) FILTER (WHERE um.status = 'watched'), 1)::float8 AS avg_rating
  FROM works w LEFT JOIN user_movies um ON um.movie_id = w.movie_id AND um.user_id = $1
  GROUP BY w.person_id
 ),
 rows AS (
  SELECT p.id, coalesce(p.tmdb_id, 0) AS tmdb_id, coalesce(p.kinopoisk_id, 0) AS kinopoisk_id,
   p.name, p.original_name, p.profile_path, p.birth_date, p.death_date, p.place_of_birth,
   p.biography, p.biography_lang,
   EXISTS (SELECT 1 FROM {follows} f WHERE f.user_id = $1 AND f.{col} = p.id) AS followed,
   (p.tmdb_id IS NULL OR p.works_synced_at IS NOT NULL) AS loaded,
   coalesce(st.work_count, 0) AS work_count, coalesce(st.in_collection, 0) AS in_collection,
   coalesce(st.watched, 0) AS watched, coalesce(st.want, 0) AS want, coalesce(st.rated, 0) AS rated,
   st.avg_rating
  FROM (SELECT DISTINCT person_id FROM mine) x
  JOIN {people} p ON p.id = x.person_id
  LEFT JOIN stats st ON st.person_id = p.id
 )`)
}

// mineSQL — подписки пользователя и люди из его коллекции: у режиссёров
// достаточно одной работы, у актёров — movie.MinCollectionRoles.
const mineSQL = `SELECT {col} AS person_id FROM {follows} WHERE user_id = $1
 UNION
 SELECT l.{col} FROM {links} l JOIN user_movies um ON um.movie_id = l.movie_id
  WHERE um.user_id = $1 GROUP BY l.{col} HAVING count(*) >= {min}`

const overviewColumns = `id, tmdb_id, kinopoisk_id, name, original_name, profile_path, birth_date, death_date,
 place_of_birth, followed, loaded, work_count, in_collection, watched, want, rated, avg_rating`

func scanOverview(row scanner, extra ...any) (movie.DirectorOverview, error) {
	var d movie.DirectorOverview
	dest := append([]any{
		&d.ID, &d.TMDBID, &d.KinopoiskID, &d.Name, &d.OriginalName, &d.ProfilePath,
		&d.BirthDate, &d.DeathDate, &d.PlaceOfBirth, &d.Followed, &d.FilmographyLoaded, &d.WorkCount,
		&d.InCollectionCount, &d.WatchedCount, &d.WantToWatchCount, &d.RatedCount, &d.AverageUserRating,
	}, extra...)
	err := row.Scan(dest...)
	return d, err
}

const searchFilterSQL = `filtered AS (SELECT * FROM rows
 WHERE $2 = '' OR strpos(lower(name), lower($2)) > 0 OR strpos(lower(original_name), lower($2)) > 0)`

func (s personStore) mine(ctx context.Context, f movie.MyDirectorsFilter) ([]movie.DirectorOverview, int, movie.DirectorScopeCounts, error) {
	ctx = queryName(ctx, s.dir.name("mine"))

	f = f.Normalize()
	var counts movie.DirectorScopeCounts
	q := s.tx.Querier(ctx)
	base := s.overviewSQL(s.dir.sql(mineSQL)) + `, ` + searchFilterSQL
	if err := q.QueryRow(ctx, base+` SELECT count(*), count(*) FILTER (WHERE followed), count(*) FILTER (WHERE NOT followed) FROM filtered`,
		f.UserID, f.Query).Scan(&counts.All, &counts.Followed, &counts.Collection); err != nil {
		return nil, 0, counts, fmt.Errorf("count people: %w", err)
	}
	total := counts.All
	switch f.Scope {
	case movie.DirectorScopeFollowed:
		total = counts.Followed
	case movie.DirectorScopeCollection:
		total = counts.Collection
	}

	order := "lower(name)"
	switch f.Sort {
	case movie.MyDirectorsSortProgress:
		order = "CASE WHEN loaded AND work_count > 0 THEN watched::float8 / work_count END"
	case movie.MyDirectorsSortUserRating:
		order = "avg_rating"
	}
	direction := " ASC"
	if f.Desc {
		direction = " DESC"
	}
	rows, err := q.Query(ctx, base+` SELECT `+overviewColumns+` FROM filtered
 WHERE $5 = 'all' OR ($5 = 'followed' AND followed) OR ($5 = 'collection' AND NOT followed)
 ORDER BY `+order+direction+` NULLS LAST, lower(name) ASC, id ASC LIMIT $3 OFFSET $4`,
		f.UserID, f.Query, f.Limit, f.Offset, string(f.Scope))
	if err != nil {
		return nil, 0, counts, fmt.Errorf("list people: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.DirectorOverview, error) {
		return scanOverview(row)
	})
	if err != nil {
		return nil, 0, counts, fmt.Errorf("scan people: %w", err)
	}
	if err := s.attachKnownFor(ctx, items); err != nil {
		return nil, 0, counts, err
	}
	return items, total, counts, nil
}

func (s personStore) overview(ctx context.Context, userID, personID int64) (movie.DirectorProfile, error) {
	ctx = queryName(ctx, s.dir.name("overview"))

	var p movie.DirectorProfile
	row := s.tx.Querier(ctx).QueryRow(ctx, s.overviewSQL(`SELECT $2::bigint AS person_id`)+
		` SELECT `+overviewColumns+`, biography, biography_lang FROM rows`, userID, personID)
	overview, err := scanOverview(row, &p.Biography, &p.BiographyLang)
	if isNoRows(err) {
		return p, s.dir.notFound
	}
	if err != nil {
		return p, fmt.Errorf("get person overview: %w", err)
	}
	items := []movie.DirectorOverview{overview}
	if err := s.attachKnownFor(ctx, items); err != nil {
		return p, err
	}
	p.DirectorOverview = items[0]
	return p, nil
}

// profile — карточка человека с биографией и полной фильмографией.
func (s personStore) profile(ctx context.Context, userID, personID int64) (movie.DirectorProfile, error) {
	ctx = queryName(ctx, s.dir.name("profile"))

	p, err := s.overview(ctx, userID, personID)
	if err != nil {
		return p, err
	}
	rows, err := s.tx.Querier(ctx).Query(ctx, s.dir.sql(`WITH works AS (
  SELECT w0.media_type, w0.tmdb_id, w0.title, w0.original_title, w0.release_date, w0.release_year,
   w0.poster_path, w0.rating::float8 AS rating, w0.vote_count, w0.episode_count, {work_char} AS character,
   m.id AS movie_id
  FROM {works} w0
  LEFT JOIN movies m ON m.media_type = w0.media_type AND m.tmdb_id = w0.tmdb_id
  WHERE w0.{col} = $2
  UNION ALL
  SELECT m.media_type, coalesce(m.tmdb_id, 0), m.title, m.original_title, NULL::date, m.release_year,
   m.poster_path, m.rating::float8, coalesce(m.rating_vote_count, 0), NULL::integer, {link_char}, m.id
  FROM {links} l0
  JOIN movies m ON m.id = l0.movie_id
  JOIN user_movies um ON um.movie_id = m.id AND um.user_id = $1
  WHERE l0.{col} = $2 AND NOT EXISTS (SELECT 1 FROM {works} w1
   WHERE w1.{col} = $2 AND w1.media_type = m.media_type AND w1.tmdb_id = m.tmdb_id)
 )
 SELECT w.media_type, w.tmdb_id, w.title, w.original_title, w.release_date, coalesce(w.release_year, 0),
  w.poster_path, w.rating, w.vote_count, w.episode_count, w.character, coalesce(w.movie_id, 0),
  coalesce(um.id, 0), coalesce(um.status, ''), coalesce(um.user_rating, 0)
 FROM works w LEFT JOIN user_movies um ON um.movie_id = w.movie_id AND um.user_id = $1
 ORDER BY coalesce(w.release_date, make_date(coalesce(w.release_year, 9999)::int, 12, 31)) DESC, w.title`), userID, personID)
	if err != nil {
		return p, fmt.Errorf("list filmography: %w", err)
	}
	p.Filmography, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.FilmographyItem, error) {
		var it movie.FilmographyItem
		err := row.Scan(&it.MediaType, &it.TMDBID, &it.Title, &it.OriginalTitle, &it.ReleaseDate, &it.Year,
			&it.PosterPath, &it.Rating, &it.VoteCount, &it.EpisodeCount, &it.Character, &it.MovieID,
			&it.UserMovieID, &it.Status, &it.UserRating)
		return it, err
	})
	if err != nil {
		return p, fmt.Errorf("scan filmography: %w", err)
	}
	return p, nil
}

// attachKnownFor добавляет каждому человеку три главные работы из снимка.
func (s personStore) attachKnownFor(ctx context.Context, items []movie.DirectorOverview) error {
	ctx = queryName(ctx, s.dir.name("attach_known_for"))

	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	index := make(map[int64]int, len(items))
	for i, d := range items {
		ids[i] = d.ID
		index[d.ID] = i
		items[i].KnownFor = []string{}
	}
	rows, err := s.tx.Querier(ctx).Query(ctx, s.dir.sql(`SELECT {col}, title FROM (
  SELECT {col}, title, row_number() OVER (PARTITION BY {col} ORDER BY {known_for_order}) AS rn
  FROM {works} WHERE {col} = ANY($1) AND {known_for_where}) t
 WHERE rn <= 3 ORDER BY {col}, rn`), ids)
	if err != nil {
		return fmt.Errorf("load known for: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return fmt.Errorf("scan known for: %w", err)
		}
		i := index[id]
		items[i].KnownFor = append(items[i].KnownFor, title)
	}
	return rows.Err()
}
