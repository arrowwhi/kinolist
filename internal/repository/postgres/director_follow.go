package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

// DirectorRepository — подписки на режиссёров и снимки их фильмографий.
// Справочник directors общий с MovieRepository: там режиссёры появляются
// из карточек фильмов, здесь — из профиля человека в каталоге.
type DirectorRepository struct {
	tx *transactions.Manager
}

func NewDirectorRepository(tx *transactions.Manager) *DirectorRepository {
	return &DirectorRepository{tx: tx}
}

// nameRankSQL повторяет movie.NameSource.Rank: имя из более надёжного
// источника не затирается менее надёжным.
func nameRankSQL(column string) string {
	return `(CASE ` + column + ` WHEN 'kinopoisk' THEN 3 WHEN 'tmdb_translation' THEN 2 ELSE 1 END)`
}

// UpsertPerson сохраняет человека из TMDB и возвращает id режиссёра.
// Если Кинопоиск опознал того же человека, а в справочнике уже есть
// строка только с kinopoisk_id, она сливается в TMDB-строку.
func (r *DirectorRepository) UpsertPerson(ctx context.Context, p movie.Person) (id int64, err error) {
	ctx = queryName(ctx, "director.upsert_person")

	if p.TMDBID <= 0 || p.Name == "" {
		return 0, movie.ErrInvalidPersonID
	}
	ctx, err = r.tx.BeginTransaction(ctx)
	if err != nil {
		return 0, err
	}
	defer r.tx.FinishTransaction(ctx, &err)

	q := r.tx.Querier(ctx)
	source := p.NameSource
	if source == "" {
		source = movie.NameSourceTMDB
	}
	sql := `INSERT INTO directors (tmdb_id, name, original_name, name_source, profile_path,
  birth_date, death_date, place_of_birth, biography, biography_lang)
 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
 ON CONFLICT (tmdb_id) DO UPDATE SET
  name = CASE WHEN ` + nameRankSQL("EXCLUDED.name_source") + ` >= ` + nameRankSQL("directors.name_source") + `
   THEN EXCLUDED.name ELSE directors.name END,
  name_source = CASE WHEN ` + nameRankSQL("EXCLUDED.name_source") + ` >= ` + nameRankSQL("directors.name_source") + `
   THEN EXCLUDED.name_source ELSE directors.name_source END,
  original_name = coalesce(nullif(EXCLUDED.original_name, ''), directors.original_name),
  profile_path = coalesce(nullif(EXCLUDED.profile_path, ''), directors.profile_path),
  birth_date = coalesce(EXCLUDED.birth_date, directors.birth_date),
  death_date = coalesce(EXCLUDED.death_date, directors.death_date),
  place_of_birth = coalesce(nullif(EXCLUDED.place_of_birth, ''), directors.place_of_birth),
  biography = CASE WHEN EXCLUDED.biography <> '' THEN EXCLUDED.biography ELSE directors.biography END,
  biography_lang = CASE WHEN EXCLUDED.biography <> '' THEN EXCLUDED.biography_lang ELSE directors.biography_lang END,
  updated_at = now()
 RETURNING id`
	if err = q.QueryRow(ctx, sql, p.TMDBID, p.Name, p.OriginalName, source, p.ProfilePath,
		p.BirthDate, p.DeathDate, p.PlaceOfBirth, p.Biography, p.BiographyLang).Scan(&id); err != nil {
		return 0, fmt.Errorf("upsert person: %w", err)
	}
	if p.KinopoiskID > 0 {
		if err = r.attachKinopoisk(ctx, id, p.KinopoiskID); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *DirectorRepository) attachKinopoisk(ctx context.Context, id int64, kinopoiskID int32) error {
	ctx = queryName(ctx, "director.attach_kinopoisk")

	q := r.tx.Querier(ctx)
	var other int64
	var otherHasTMDB bool
	err := q.QueryRow(ctx, `SELECT id, tmdb_id IS NOT NULL FROM directors WHERE kinopoisk_id=$1 AND id<>$2`, kinopoiskID, id).
		Scan(&other, &otherHasTMDB)
	switch {
	case isNoRows(err):
	case err != nil:
		return fmt.Errorf("find kinopoisk director: %w", err)
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
			{`INSERT INTO movie_directors (movie_id, director_id, position)
			 SELECT movie_id, $2, position FROM movie_directors WHERE director_id=$1 ON CONFLICT DO NOTHING`, []any{other, id}},
			{`INSERT INTO user_directors (user_id, director_id, followed_at)
			 SELECT user_id, $2, followed_at FROM user_directors WHERE director_id=$1 ON CONFLICT DO NOTHING`, []any{other, id}},
			{`DELETE FROM movie_directors WHERE director_id=$1`, []any{other}},
			{`DELETE FROM directors WHERE id=$1`, []any{other}},
		} {
			if _, err := q.Exec(ctx, stmt.sql, stmt.args...); err != nil {
				return fmt.Errorf("merge kinopoisk director: %w", err)
			}
		}
	}
	if _, err := q.Exec(ctx, `UPDATE directors SET kinopoisk_id=coalesce(kinopoisk_id, $2) WHERE id=$1`, id, kinopoiskID); err != nil {
		return fmt.Errorf("attach kinopoisk id: %w", err)
	}
	return nil
}

// ReplaceWorks заменяет снимок фильмографии и отмечает время синхронизации.
func (r *DirectorRepository) ReplaceWorks(ctx context.Context, directorID int64, works []movie.DirectorWork) (err error) {
	ctx = queryName(ctx, "director.replace_works")

	ctx, err = r.tx.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	defer r.tx.FinishTransaction(ctx, &err)

	q := r.tx.Querier(ctx)
	if _, err = q.Exec(ctx, `DELETE FROM director_works WHERE director_id=$1`, directorID); err != nil {
		return fmt.Errorf("clear director works: %w", err)
	}
	if len(works) > 0 {
		insert := builder.Insert("director_works").Columns("director_id", "media_type", "tmdb_id", "title",
			"original_title", "release_date", "release_year", "poster_path", "rating", "vote_count", "episode_count")
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
			insert = insert.Values(directorID, w.MediaType, w.TMDBID, w.Title, w.OriginalTitle,
				w.ReleaseDate, year, w.PosterPath, w.Rating, w.VoteCount, w.EpisodeCount)
		}
		if len(seen) > 0 {
			sql, args, buildErr := insert.ToSql()
			if buildErr != nil {
				return fmt.Errorf("build director works insert: %w", buildErr)
			}
			if _, err = q.Exec(ctx, sql, args...); err != nil {
				return fmt.Errorf("insert director works: %w", err)
			}
		}
	}
	return r.TouchSynced(ctx, directorID)
}

// TouchSynced отмечает попытку синхронизации, не меняя снимок: человек
// пропал из каталога, и повторять запрос каждые полчаса незачем.
func (r *DirectorRepository) TouchSynced(ctx context.Context, directorID int64) error {
	ctx = queryName(ctx, "director.touch_synced")

	if _, err := r.tx.Querier(ctx).Exec(ctx, `UPDATE directors SET works_synced_at=now() WHERE id=$1`, directorID); err != nil {
		return fmt.Errorf("mark director synced: %w", err)
	}
	return nil
}

func (r *DirectorRepository) SyncState(ctx context.Context, directorID int64) (movie.DirectorSyncState, error) {
	ctx = queryName(ctx, "director.sync_state")

	return r.syncState(ctx, `id=$1`, directorID)
}

func (r *DirectorRepository) SyncStateByTMDB(ctx context.Context, tmdbID int32) (movie.DirectorSyncState, error) {
	ctx = queryName(ctx, "director.sync_state_by_tmdb")

	return r.syncState(ctx, `tmdb_id=$1`, tmdbID)
}

func (r *DirectorRepository) syncState(ctx context.Context, where string, arg any) (movie.DirectorSyncState, error) {
	ctx = queryName(ctx, "director.sync_state")

	var s movie.DirectorSyncState
	err := r.tx.Querier(ctx).QueryRow(ctx, `SELECT id, coalesce(tmdb_id, 0), name_source, works_synced_at
 FROM directors WHERE `+where, arg).Scan(&s.ID, &s.TMDBID, &s.NameSource, &s.SyncedAt)
	if isNoRows(err) {
		return s, movie.ErrDirectorNotFound
	}
	if err != nil {
		return s, fmt.Errorf("get director sync state: %w", err)
	}
	return s, nil
}

// StaleDirectors — режиссёры, которые кому-то нужны (подписка или работа
// в коллекции), а фильмография не загружена или устарела.
func (r *DirectorRepository) StaleDirectors(ctx context.Context, olderThan time.Time, limit int) ([]movie.DirectorSyncState, error) {
	ctx = queryName(ctx, "director.stale_directors")

	rows, err := r.tx.Querier(ctx).Query(ctx, `SELECT d.id, d.tmdb_id, d.name_source, d.works_synced_at FROM directors d
 WHERE d.tmdb_id IS NOT NULL AND (d.works_synced_at IS NULL OR d.works_synced_at < $1)
  AND (EXISTS (SELECT 1 FROM user_directors ud WHERE ud.director_id = d.id)
   OR EXISTS (SELECT 1 FROM movie_directors md JOIN user_movies um ON um.movie_id = md.movie_id WHERE md.director_id = d.id))
 ORDER BY d.works_synced_at NULLS FIRST, d.id LIMIT $2`, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale directors: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.DirectorSyncState, error) {
		var s movie.DirectorSyncState
		err := row.Scan(&s.ID, &s.TMDBID, &s.NameSource, &s.SyncedAt)
		return s, err
	})
}

func (r *DirectorRepository) Follow(ctx context.Context, userID, directorID int64) error {
	ctx = queryName(ctx, "director.follow")

	_, err := r.tx.Querier(ctx).Exec(ctx, `INSERT INTO user_directors (user_id, director_id) VALUES ($1, $2)
 ON CONFLICT DO NOTHING`, userID, directorID)
	if hasCode(err, codeForeignKeyViolation) {
		return movie.ErrDirectorNotFound
	}
	if err != nil {
		return fmt.Errorf("follow director: %w", err)
	}
	return nil
}

func (r *DirectorRepository) Unfollow(ctx context.Context, userID, directorID int64) error {
	ctx = queryName(ctx, "director.unfollow")

	if _, err := r.tx.Querier(ctx).Exec(ctx, `DELETE FROM user_directors WHERE user_id=$1 AND director_id=$2`, userID, directorID); err != nil {
		return fmt.Errorf("unfollow director: %w", err)
	}
	return nil
}

// KnownByTMDB сопоставляет людей из поиска с уже заведёнными режиссёрами.
func (r *DirectorRepository) KnownByTMDB(ctx context.Context, userID int64, tmdbIDs []int32) (map[int32]movie.PersonSearchResult, error) {
	ctx = queryName(ctx, "director.known_by_tmdb")

	out := make(map[int32]movie.PersonSearchResult, len(tmdbIDs))
	if len(tmdbIDs) == 0 {
		return out, nil
	}
	rows, err := r.tx.Querier(ctx).Query(ctx, `SELECT d.id, d.tmdb_id, d.name, d.name_source,
  EXISTS (SELECT 1 FROM user_directors ud WHERE ud.user_id=$1 AND ud.director_id=d.id)
 FROM directors d WHERE d.tmdb_id = ANY($2)`, userID, tmdbIDs)
	if err != nil {
		return nil, fmt.Errorf("find known directors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p movie.PersonSearchResult
		if err := rows.Scan(&p.DirectorID, &p.TMDBID, &p.Name, &p.NameSource, &p.Followed); err != nil {
			return nil, fmt.Errorf("scan known director: %w", err)
		}
		out[p.TMDBID] = p
	}
	return out, rows.Err()
}

// overviewSQL строит строки «Мои режиссёры» для набора режиссёров из CTE mine.
// Работы — объединение снимка из каталога с работами коллекции, которых в
// снимке нет (фильмы только из Кинопоиска, расхождения в credits TMDB).
// Параметр $1 — всегда id пользователя.
func overviewSQL(mine string) string {
	return `WITH mine AS (` + mine + `),
 works AS (
  SELECT dw.director_id, m.id AS movie_id FROM director_works dw
   LEFT JOIN movies m ON m.media_type = dw.media_type AND m.tmdb_id = dw.tmdb_id
   WHERE dw.director_id IN (SELECT director_id FROM mine)
  UNION ALL
  SELECT md.director_id, md.movie_id FROM movie_directors md
   JOIN movies m ON m.id = md.movie_id
   JOIN user_movies um ON um.movie_id = md.movie_id AND um.user_id = $1
   WHERE md.director_id IN (SELECT director_id FROM mine)
    AND NOT EXISTS (SELECT 1 FROM director_works dw
     WHERE dw.director_id = md.director_id AND dw.media_type = m.media_type AND dw.tmdb_id = m.tmdb_id)
 ),
 stats AS (
  SELECT w.director_id, count(*) AS work_count, count(um.id) AS in_collection,
   count(*) FILTER (WHERE um.status = 'watched') AS watched,
   count(*) FILTER (WHERE um.status = 'want_to_watch') AS want,
   count(um.user_rating) FILTER (WHERE um.status = 'watched') AS rated,
   round(avg(um.user_rating) FILTER (WHERE um.status = 'watched'), 1)::float8 AS avg_rating
  FROM works w LEFT JOIN user_movies um ON um.movie_id = w.movie_id AND um.user_id = $1
  GROUP BY w.director_id
 ),
 rows AS (
  SELECT d.id, coalesce(d.tmdb_id, 0) AS tmdb_id, coalesce(d.kinopoisk_id, 0) AS kinopoisk_id,
   d.name, d.original_name, d.profile_path, d.birth_date, d.death_date, d.place_of_birth,
   d.biography, d.biography_lang,
   EXISTS (SELECT 1 FROM user_directors ud WHERE ud.user_id = $1 AND ud.director_id = d.id) AS followed,
   (d.tmdb_id IS NULL OR d.works_synced_at IS NOT NULL) AS loaded,
   coalesce(s.work_count, 0) AS work_count, coalesce(s.in_collection, 0) AS in_collection,
   coalesce(s.watched, 0) AS watched, coalesce(s.want, 0) AS want, coalesce(s.rated, 0) AS rated,
   s.avg_rating
  FROM (SELECT DISTINCT director_id FROM mine) x
  JOIN directors d ON d.id = x.director_id
  LEFT JOIN stats s ON s.director_id = d.id
 )`
}

const myDirectorsMine = `SELECT director_id FROM user_directors WHERE user_id = $1
 UNION
 SELECT md.director_id FROM movie_directors md JOIN user_movies um ON um.movie_id = md.movie_id WHERE um.user_id = $1`

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

func (r *DirectorRepository) MyDirectors(ctx context.Context, f movie.MyDirectorsFilter) ([]movie.DirectorOverview, int, movie.DirectorScopeCounts, error) {
	ctx = queryName(ctx, "director.my_directors")

	f = f.Normalize()
	var counts movie.DirectorScopeCounts
	q := r.tx.Querier(ctx)
	base := overviewSQL(myDirectorsMine) + `, ` + searchFilterSQL
	if err := q.QueryRow(ctx, base+` SELECT count(*), count(*) FILTER (WHERE followed), count(*) FILTER (WHERE NOT followed) FROM filtered`,
		f.UserID, f.Query).Scan(&counts.All, &counts.Followed, &counts.Collection); err != nil {
		return nil, 0, counts, fmt.Errorf("count my directors: %w", err)
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
		return nil, 0, counts, fmt.Errorf("list my directors: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.DirectorOverview, error) {
		return scanOverview(row)
	})
	if err != nil {
		return nil, 0, counts, fmt.Errorf("scan my directors: %w", err)
	}
	if err := r.attachKnownFor(ctx, items); err != nil {
		return nil, 0, counts, err
	}
	return items, total, counts, nil
}

func (r *DirectorRepository) Overview(ctx context.Context, userID, directorID int64) (movie.DirectorOverview, error) {
	ctx = queryName(ctx, "director.overview")

	profile, err := r.overview(ctx, userID, directorID)
	return profile.DirectorOverview, err
}

func (r *DirectorRepository) overview(ctx context.Context, userID, directorID int64) (movie.DirectorProfile, error) {
	ctx = queryName(ctx, "director.overview")

	var p movie.DirectorProfile
	row := r.tx.Querier(ctx).QueryRow(ctx, overviewSQL(`SELECT $2::bigint AS director_id`)+
		` SELECT `+overviewColumns+`, biography, biography_lang FROM rows`, userID, directorID)
	overview, err := scanOverview(row, &p.Biography, &p.BiographyLang)
	if isNoRows(err) {
		return p, movie.ErrDirectorNotFound
	}
	if err != nil {
		return p, fmt.Errorf("get director overview: %w", err)
	}
	items := []movie.DirectorOverview{overview}
	if err := r.attachKnownFor(ctx, items); err != nil {
		return p, err
	}
	p.DirectorOverview = items[0]
	return p, nil
}

// Profile — карточка режиссёра с биографией и полной фильмографией.
func (r *DirectorRepository) Profile(ctx context.Context, userID, directorID int64) (movie.DirectorProfile, error) {
	ctx = queryName(ctx, "director.profile")

	p, err := r.overview(ctx, userID, directorID)
	if err != nil {
		return p, err
	}
	rows, err := r.tx.Querier(ctx).Query(ctx, `WITH works AS (
  SELECT dw.media_type, dw.tmdb_id, dw.title, dw.original_title, dw.release_date, dw.release_year,
   dw.poster_path, dw.rating::float8 AS rating, dw.vote_count, dw.episode_count, m.id AS movie_id
  FROM director_works dw
  LEFT JOIN movies m ON m.media_type = dw.media_type AND m.tmdb_id = dw.tmdb_id
  WHERE dw.director_id = $2
  UNION ALL
  SELECT m.media_type, coalesce(m.tmdb_id, 0), m.title, m.original_title, NULL::date, m.release_year,
   m.poster_path, m.rating::float8, coalesce(m.rating_vote_count, 0), NULL::integer, m.id
  FROM movie_directors md
  JOIN movies m ON m.id = md.movie_id
  JOIN user_movies um ON um.movie_id = m.id AND um.user_id = $1
  WHERE md.director_id = $2 AND NOT EXISTS (SELECT 1 FROM director_works dw
   WHERE dw.director_id = $2 AND dw.media_type = m.media_type AND dw.tmdb_id = m.tmdb_id)
 )
 SELECT w.media_type, w.tmdb_id, w.title, w.original_title, w.release_date, coalesce(w.release_year, 0),
  w.poster_path, w.rating, w.vote_count, w.episode_count, coalesce(w.movie_id, 0),
  coalesce(um.id, 0), coalesce(um.status, ''), coalesce(um.user_rating, 0)
 FROM works w LEFT JOIN user_movies um ON um.movie_id = w.movie_id AND um.user_id = $1
 ORDER BY coalesce(w.release_date, make_date(coalesce(w.release_year, 9999)::int, 12, 31)) DESC, w.title`, userID, directorID)
	if err != nil {
		return p, fmt.Errorf("list filmography: %w", err)
	}
	p.Filmography, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.FilmographyItem, error) {
		var it movie.FilmographyItem
		err := row.Scan(&it.MediaType, &it.TMDBID, &it.Title, &it.OriginalTitle, &it.ReleaseDate, &it.Year,
			&it.PosterPath, &it.Rating, &it.VoteCount, &it.EpisodeCount, &it.MovieID,
			&it.UserMovieID, &it.Status, &it.UserRating)
		return it, err
	})
	if err != nil {
		return p, fmt.Errorf("scan filmography: %w", err)
	}
	return p, nil
}

// attachKnownFor добавляет три самые высокооценённые работы каждого режиссёра.
func (r *DirectorRepository) attachKnownFor(ctx context.Context, items []movie.DirectorOverview) error {
	ctx = queryName(ctx, "director.attach_known_for")

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
	rows, err := r.tx.Querier(ctx).Query(ctx, `SELECT director_id, title FROM (
  SELECT director_id, title, row_number() OVER (PARTITION BY director_id
   ORDER BY rating DESC NULLS LAST, vote_count DESC, title) AS rn
  FROM director_works WHERE director_id = ANY($1) AND rating IS NOT NULL) t
 WHERE rn <= 3 ORDER BY director_id, rn`, ids)
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
