package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

type MovieRepository struct {
	tx *transactions.Manager
}

func NewMovieRepository(tx *transactions.Manager) *MovieRepository {
	return &MovieRepository{tx: tx}
}

// Upsert сохраняет карточку фильма в общий каталог и переписывает её жанры.
// Каталог общий для всех пользователей, поэтому повторное добавление того же
// фильма обновляет данные, а не создаёт дубль.
func (r *MovieRepository) Upsert(ctx context.Context, m movie.Movie) (movie.Movie, error) {
	q := r.tx.Querier(ctx)

	const upsertTMDB = `
		INSERT INTO movies (tmdb_id, kinopoisk_id, title, original_title, release_year,
		                    overview, poster_path, runtime_min, rating)
		VALUES ($1, nullif($2, 0), $3, $4, nullif($5, 0)::smallint, $6, $7, nullif($8, 0)::smallint, nullif($9, 0))
		ON CONFLICT (tmdb_id) DO UPDATE
		SET title          = EXCLUDED.title,
		    original_title = EXCLUDED.original_title,
		    release_year   = EXCLUDED.release_year,
		    overview       = EXCLUDED.overview,
		    poster_path    = EXCLUDED.poster_path,
		    runtime_min    = coalesce(EXCLUDED.runtime_min, movies.runtime_min),
		    rating         = coalesce(EXCLUDED.rating, movies.rating),
		    kinopoisk_id   = coalesce(EXCLUDED.kinopoisk_id, movies.kinopoisk_id),
		    updated_at     = now()
		RETURNING id`

	const upsertKinopoisk = `
		INSERT INTO movies (tmdb_id, kinopoisk_id, title, original_title, release_year,
		                    overview, poster_path, runtime_min, rating)
		VALUES (nullif($1, 0), $2, $3, $4, nullif($5, 0)::smallint, $6, $7, nullif($8, 0)::smallint, nullif($9, 0))
		ON CONFLICT (kinopoisk_id) DO UPDATE
		SET title          = EXCLUDED.title,
		    original_title = EXCLUDED.original_title,
		    release_year   = EXCLUDED.release_year,
		    overview       = EXCLUDED.overview,
		    poster_path    = EXCLUDED.poster_path,
		    runtime_min    = coalesce(EXCLUDED.runtime_min, movies.runtime_min),
		    rating         = coalesce(EXCLUDED.rating, movies.rating),
		    tmdb_id        = coalesce(EXCLUDED.tmdb_id, movies.tmdb_id),
		    updated_at     = now()
		RETURNING id`

	stmt := upsertTMDB
	if m.TMDBID == 0 {
		if m.KinopoiskID == 0 {
			return movie.Movie{}, fmt.Errorf("upsert movie: %w", errNoExternalID)
		}
		stmt = upsertKinopoisk
	}

	var id int64
	err := q.QueryRow(ctx, stmt,
		m.TMDBID, m.KinopoiskID, m.Title, m.OriginalTitle, m.Year,
		m.Overview, m.PosterPath, m.RuntimeMin, m.Rating,
	).Scan(&id)
	if err != nil {
		return movie.Movie{}, fmt.Errorf("upsert movie: %w", err)
	}

	m.ID = id

	if err := r.replaceGenres(ctx, id, m.Genres); err != nil {
		return movie.Movie{}, err
	}

	return m, nil
}

func (r *MovieRepository) replaceGenres(ctx context.Context, movieID int64, genres []movie.Genre) error {
	q := r.tx.Querier(ctx)

	if _, err := q.Exec(ctx, `DELETE FROM movie_genres WHERE movie_id = $1`, movieID); err != nil {
		return fmt.Errorf("clear movie genres: %w", err)
	}
	if len(genres) == 0 {
		return nil
	}

	ins := builder.Insert("movie_genres").Columns("movie_id", "genre_id")
	for _, g := range genres {
		ins = ins.Values(movieID, g.ID)
	}
	sql, args, err := ins.Suffix("ON CONFLICT DO NOTHING").ToSql()
	if err != nil {
		return fmt.Errorf("build movie genres insert: %w", err)
	}
	if _, err := q.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("insert movie genres: %w", err)
	}

	return nil
}

func (r *MovieRepository) GetByTMDBID(ctx context.Context, tmdbID int32) (movie.Movie, error) {
	const q = `
		SELECT ` + movieColumns + `
		FROM movies m
		WHERE m.tmdb_id = $1`

	m, err := scanMovie(r.tx.Querier(ctx).QueryRow(ctx, q, tmdbID))
	if err != nil {
		if isNoRows(err) {
			return movie.Movie{}, movie.ErrNotFound
		}
		return movie.Movie{}, fmt.Errorf("get movie by tmdb id: %w", err)
	}

	genres, err := r.genresForMovies(ctx, []int64{m.ID})
	if err != nil {
		return movie.Movie{}, err
	}
	m.Genres = genres[m.ID]

	return m, nil
}

// Add кладёт фильм в коллекцию пользователя. Повторное добавление —
// не ошибка инфраструктуры, а доменное состояние ErrAlreadyInCollection.
func (r *MovieRepository) Add(ctx context.Context, um movie.UserMovie) (movie.UserMovie, error) {
	// ON CONFLICT DO NOTHING вместо перехвата 23505: нарушение уникальности
	// внутри транзакции переводит её в aborted, и все последующие запросы
	// падают уже с ошибкой драйвера, затирая доменную ErrAlreadyInCollection.
	const q = `
		INSERT INTO user_movies (user_id, movie_id, status, source, source_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, movie_id) DO NOTHING
		RETURNING id, added_at`

	status := um.Status
	if status == "" {
		status = movie.StatusWantToWatch
	}
	source := um.Source
	if source == "" {
		source = movie.SourceManual
	}

	err := r.tx.Querier(ctx).
		QueryRow(ctx, q, um.UserID, um.Movie.ID, status, source, um.SourceURL).
		Scan(&um.ID, &um.AddedAt)
	if err != nil {
		if isNoRows(err) || isUniqueViolation(err) {
			return movie.UserMovie{}, movie.ErrAlreadyInCollection
		}
		return movie.UserMovie{}, fmt.Errorf("add movie to collection: %w", err)
	}

	um.Status = status
	um.Source = source

	return um, nil
}

func (r *MovieRepository) Get(ctx context.Context, userID, userMovieID int64) (movie.UserMovie, error) {
	sql, args, err := r.selectUserMovies().
		Where(squirrel.Eq{"um.user_id": userID, "um.id": userMovieID}).
		ToSql()
	if err != nil {
		return movie.UserMovie{}, fmt.Errorf("build get user movie: %w", err)
	}

	um, err := scanUserMovie(r.tx.Querier(ctx).QueryRow(ctx, sql, args...))
	if err != nil {
		if isNoRows(err) {
			return movie.UserMovie{}, movie.ErrNotFound
		}
		return movie.UserMovie{}, fmt.Errorf("get user movie: %w", err)
	}

	genres, err := r.genresForMovies(ctx, []int64{um.Movie.ID})
	if err != nil {
		return movie.UserMovie{}, err
	}
	um.Movie.Genres = genres[um.Movie.ID]

	return um, nil
}

func (r *MovieRepository) List(ctx context.Context, f movie.Filter) ([]movie.UserMovie, error) {
	f = f.Normalize()

	q := r.applyFilter(r.selectUserMovies(), f).
		OrderBy(orderClause(f)).
		Limit(uint64(f.Limit)).  //nolint:gosec // Limit/Offset нормализованы Filter.Normalize и неотрицательны
		Offset(uint64(f.Offset)) //nolint:gosec // Limit/Offset нормализованы Filter.Normalize и неотрицательны

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list user movies: %w", err)
	}

	rows, err := r.tx.Querier(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list user movies: %w", err)
	}
	defer rows.Close()

	items := make([]movie.UserMovie, 0, f.Limit)
	movieIDs := make([]int64, 0, f.Limit)
	for rows.Next() {
		um, err := scanUserMovie(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user movie: %w", err)
		}
		items = append(items, um)
		movieIDs = append(movieIDs, um.Movie.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user movies: %w", err)
	}

	// Жанры добираются одним запросом на страницу, а не по фильму:
	// иначе список из 50 карточек давал бы 51 обращение к базе.
	genres, err := r.genresForMovies(ctx, movieIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Movie.Genres = genres[items[i].Movie.ID]
	}

	return items, nil
}

func (r *MovieRepository) Count(ctx context.Context, f movie.Filter) (int, error) {
	base := builder.Select("count(*)").
		From("user_movies um").
		Join("movies m ON m.id = um.movie_id")

	sql, args, err := r.applyFilter(base, f).ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count user movies: %w", err)
	}

	var total int
	if err := r.tx.Querier(ctx).QueryRow(ctx, sql, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count user movies: %w", err)
	}

	return total, nil
}

func (r *MovieRepository) Update(ctx context.Context, userID, userMovieID int64, upd movie.Update) (movie.UserMovie, error) {
	q := builder.Update("user_movies").
		Where(squirrel.Eq{"user_id": userID, "id": userMovieID})

	changed := false
	if upd.Status != nil {
		q = q.Set("status", *upd.Status)
		// watched_at ставится вместе со статусом, чтобы «когда посмотрел»
		// нельзя было рассинхронизировать с самим фактом просмотра.
		if *upd.Status == movie.StatusWatched {
			q = q.Set("watched_at", squirrel.Expr("coalesce(watched_at, now())"))
		} else {
			q = q.Set("watched_at", nil)
		}
		changed = true
	}
	if upd.UserRating != nil {
		if *upd.UserRating == 0 {
			q = q.Set("user_rating", nil)
		} else {
			q = q.Set("user_rating", *upd.UserRating)
		}
		changed = true
	}
	if upd.Note != nil {
		q = q.Set("note", *upd.Note)
		changed = true
	}
	if !changed {
		return r.Get(ctx, userID, userMovieID)
	}

	sql, args, err := q.Suffix("RETURNING id").ToSql()
	if err != nil {
		return movie.UserMovie{}, fmt.Errorf("build update user movie: %w", err)
	}

	var id int64
	if err := r.tx.Querier(ctx).QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		if isNoRows(err) {
			return movie.UserMovie{}, movie.ErrNotFound
		}
		if hasCode(err, codeCheckViolation) {
			return movie.UserMovie{}, movie.ErrInvalidStatus
		}
		return movie.UserMovie{}, fmt.Errorf("update user movie: %w", err)
	}

	return r.Get(ctx, userID, userMovieID)
}

func (r *MovieRepository) Delete(ctx context.Context, userID, userMovieID int64) error {
	const q = `DELETE FROM user_movies WHERE user_id = $1 AND id = $2`

	tag, err := r.tx.Querier(ctx).Exec(ctx, q, userID, userMovieID)
	if err != nil {
		return fmt.Errorf("delete user movie: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return movie.ErrNotFound
	}

	return nil
}

func (r *MovieRepository) ListGenres(ctx context.Context) ([]movie.Genre, error) {
	const q = `SELECT id, tmdb_id, name_ru, name_en FROM genres ORDER BY name_ru`

	rows, err := r.tx.Querier(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	defer rows.Close()

	return collectGenres(rows)
}

func (r *MovieRepository) GenresByTMDBIDs(ctx context.Context, tmdbIDs []int32) ([]movie.Genre, error) {
	if len(tmdbIDs) == 0 {
		return nil, nil
	}

	const q = `SELECT id, tmdb_id, name_ru, name_en FROM genres WHERE tmdb_id = ANY($1)`

	rows, err := r.tx.Querier(ctx).Query(ctx, q, tmdbIDs)
	if err != nil {
		return nil, fmt.Errorf("genres by tmdb ids: %w", err)
	}
	defer rows.Close()

	return collectGenres(rows)
}

// GenresByExternalNames резолвит жанры стороннего источника через таблицу
// маппинга: без неё «фантастика» из Кинопоиска и Science Fiction из TMDB
// оказались бы разными жанрами.
func (r *MovieRepository) GenresByExternalNames(ctx context.Context, p movie.Provider, names []string) ([]movie.Genre, error) {
	if len(names) == 0 {
		return nil, nil
	}

	lowered := make([]string, 0, len(names))
	for _, n := range names {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(n)))
	}

	const q = `
		SELECT DISTINCT g.id, g.tmdb_id, g.name_ru, g.name_en
		FROM genres g
		JOIN genre_external_map gem ON gem.genre_id = g.id
		WHERE gem.source = $1 AND lower(gem.external_name) = ANY($2)`

	rows, err := r.tx.Querier(ctx).Query(ctx, q, string(p), lowered)
	if err != nil {
		return nil, fmt.Errorf("genres by external names: %w", err)
	}
	defer rows.Close()

	return collectGenres(rows)
}

func (r *MovieRepository) genresForMovies(ctx context.Context, movieIDs []int64) (map[int64][]movie.Genre, error) {
	out := make(map[int64][]movie.Genre, len(movieIDs))
	if len(movieIDs) == 0 {
		return out, nil
	}

	const q = `
		SELECT mg.movie_id, g.id, g.tmdb_id, g.name_ru, g.name_en
		FROM movie_genres mg
		JOIN genres g ON g.id = mg.genre_id
		WHERE mg.movie_id = ANY($1)
		ORDER BY g.name_ru`

	rows, err := r.tx.Querier(ctx).Query(ctx, q, movieIDs)
	if err != nil {
		return nil, fmt.Errorf("genres for movies: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var movieID int64
		var g movie.Genre
		if err := rows.Scan(&movieID, &g.ID, &g.TMDBID, &g.NameRU, &g.NameEN); err != nil {
			return nil, fmt.Errorf("scan movie genre: %w", err)
		}
		out[movieID] = append(out[movieID], g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate movie genres: %w", err)
	}

	return out, nil
}

func (r *MovieRepository) selectUserMovies() squirrel.SelectBuilder {
	return builder.Select(
		"um.id", "um.user_id", "um.status", "coalesce(um.user_rating, 0)", "um.note",
		"um.source", "um.source_url", "um.added_at", "um.watched_at",
		"m.id", "coalesce(m.tmdb_id, 0)", "coalesce(m.kinopoisk_id, 0)",
		"m.title", "m.original_title", "coalesce(m.release_year, 0)",
		"m.overview", "m.poster_path", "coalesce(m.runtime_min, 0)", "coalesce(m.rating, 0)",
		"m.created_at", "m.updated_at",
	).
		From("user_movies um").
		Join("movies m ON m.id = um.movie_id")
}

func (r *MovieRepository) applyFilter(q squirrel.SelectBuilder, f movie.Filter) squirrel.SelectBuilder {
	q = q.Where(squirrel.Eq{"um.user_id": f.UserID})

	if f.Status != "" {
		q = q.Where(squirrel.Eq{"um.status": f.Status})
	}
	if len(f.GenreIDs) > 0 {
		q = q.Where(
			"EXISTS (SELECT 1 FROM movie_genres mg WHERE mg.movie_id = m.id AND mg.genre_id = ANY(?))",
			f.GenreIDs,
		)
	}
	if q2 := strings.TrimSpace(f.Query); q2 != "" {
		pattern := "%" + strings.ToLower(q2) + "%"
		q = q.Where(squirrel.Or{
			squirrel.Expr("lower(m.title) LIKE ?", pattern),
			squirrel.Expr("lower(m.original_title) LIKE ?", pattern),
		})
	}

	return q
}

// orderClause собирается только из проверенных значений SortBy —
// пользовательская строка сюда попасть не может.
func orderClause(f movie.Filter) string {
	dir := "ASC"
	if f.Desc {
		dir = "DESC"
	}

	switch f.SortBy {
	case movie.SortByTitle:
		return "m.title " + dir + ", um.id DESC"
	case movie.SortByYear:
		return "m.release_year " + dir + " NULLS LAST, um.id DESC"
	case movie.SortByRating:
		return "m.rating " + dir + " NULLS LAST, um.id DESC"
	default:
		return "um.added_at " + dir + ", um.id DESC"
	}
}

func collectGenres(rows pgx.Rows) ([]movie.Genre, error) {
	var out []movie.Genre
	for rows.Next() {
		var g movie.Genre
		if err := rows.Scan(&g.ID, &g.TMDBID, &g.NameRU, &g.NameEN); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate genres: %w", err)
	}
	return out, nil
}

const movieColumns = `
	m.id, coalesce(m.tmdb_id, 0), coalesce(m.kinopoisk_id, 0),
	m.title, m.original_title, coalesce(m.release_year, 0),
	m.overview, m.poster_path, coalesce(m.runtime_min, 0), coalesce(m.rating, 0),
	m.created_at, m.updated_at`

func scanMovie(row scanner) (movie.Movie, error) {
	var m movie.Movie
	err := row.Scan(
		&m.ID, &m.TMDBID, &m.KinopoiskID, &m.Title, &m.OriginalTitle, &m.Year,
		&m.Overview, &m.PosterPath, &m.RuntimeMin, &m.Rating, &m.CreatedAt, &m.UpdatedAt,
	)
	return m, err
}

func scanUserMovie(row scanner) (movie.UserMovie, error) {
	var um movie.UserMovie
	err := row.Scan(
		&um.ID, &um.UserID, &um.Status, &um.UserRating, &um.Note,
		&um.Source, &um.SourceURL, &um.AddedAt, &um.WatchedAt,
		&um.Movie.ID, &um.Movie.TMDBID, &um.Movie.KinopoiskID,
		&um.Movie.Title, &um.Movie.OriginalTitle, &um.Movie.Year,
		&um.Movie.Overview, &um.Movie.PosterPath, &um.Movie.RuntimeMin, &um.Movie.Rating,
		&um.Movie.CreatedAt, &um.Movie.UpdatedAt,
	)
	return um, err
}
