package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// replaceCast переписывает актёрский состав работы и отмечает, что он
// загружен. Актёры заводятся в справочнике по внешнему id, имя никогда не
// сливает разных людей.
func (r *MovieRepository) replaceCast(ctx context.Context, movieID int64, cast []movie.CastMember) error {
	ctx = queryName(ctx, "movie.replace_cast")

	q := r.tx.Querier(ctx)
	if _, err := q.Exec(ctx, `DELETE FROM movie_actors WHERE movie_id=$1`, movieID); err != nil {
		return fmt.Errorf("clear cast: %w", err)
	}
	if len(cast) > movie.MaxCast {
		cast = cast[:movie.MaxCast]
	}
	// Порядок вставки — по внешнему id, как у режиссёров: параллельные
	// сохранения разных фильмов берут блокировки строк в одном порядке.
	positions := make([]int, len(cast))
	for i := range cast {
		positions[i] = i
	}
	sort.SliceStable(positions, func(i, j int) bool {
		a, b := cast[positions[i]], cast[positions[j]]
		if (a.TMDBID > 0) != (b.TMDBID > 0) {
			return a.TMDBID > 0
		}
		if a.TMDBID > 0 {
			return a.TMDBID < b.TMDBID
		}
		return a.KinopoiskID < b.KinopoiskID
	})
	better := nameRankSQL("EXCLUDED.name_source") + ` >= ` + nameRankSQL("actors.name_source")
	for _, position := range positions {
		c := cast[position]
		column, externalID := columnTMDBID, c.TMDBID
		if externalID == 0 {
			column, externalID = columnKinopoiskID, c.KinopoiskID
		}
		name := strings.TrimSpace(c.Name)
		if externalID <= 0 || name == "" {
			return fmt.Errorf("save actor: invalid external identity or name")
		}
		// Кинопоиск отдаёт русские имена, TMDB в credits — обычно латиницу.
		source := movie.NameSourceTMDB
		switch {
		case column == columnKinopoiskID:
			source = movie.NameSourceKinopoisk
		case movie.HasCyrillic(name):
			source = movie.NameSourceTMDBTranslation
		}
		// Колонка конфликта — только из констант выше.
		sql := `INSERT INTO actors (` + column + `, name, original_name, name_source, profile_path) VALUES ($1, $2, $3, $4, $5)
 ON CONFLICT (` + column + `) DO UPDATE SET
  name = CASE WHEN ` + better + ` THEN EXCLUDED.name ELSE actors.name END,
  name_source = CASE WHEN ` + better + ` THEN EXCLUDED.name_source ELSE actors.name_source END,
  original_name = coalesce(nullif(EXCLUDED.original_name, ''), actors.original_name),
  profile_path = CASE WHEN actors.profile_path = '' THEN EXCLUDED.profile_path ELSE actors.profile_path END,
  updated_at = now()
 RETURNING id`
		var id int64
		if err := q.QueryRow(ctx, sql, externalID, name, strings.TrimSpace(c.OriginalName), source, c.ProfilePath).Scan(&id); err != nil {
			return fmt.Errorf("upsert actor: %w", err)
		}
		if _, err := q.Exec(ctx, `INSERT INTO movie_actors (movie_id, actor_id, character, position) VALUES ($1, $2, $3, $4)
 ON CONFLICT DO NOTHING`, movieID, id, c.Character, position); err != nil {
			return fmt.Errorf("link actor: %w", err)
		}
	}
	if _, err := q.Exec(ctx, `UPDATE movies SET cast_fetched_at = now() WHERE id = $1`, movieID); err != nil {
		return fmt.Errorf("mark cast loaded: %w", err)
	}
	return nil
}

// ReplaceCast сохраняет состав работы, догруженный после её добавления.
func (r *MovieRepository) ReplaceCast(ctx context.Context, movieID int64, cast []movie.CastMember) (err error) {
	ctx = queryName(ctx, "movie.replace_cast")

	ctx, err = r.tx.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	defer r.tx.FinishTransaction(ctx, &err)
	return r.replaceCast(ctx, movieID, cast)
}

// MoviesWithoutCast — работы из чьих-то коллекций, состав которых ещё не
// загружен: добавленные до появления актёров. Старые — первыми.
func (r *MovieRepository) MoviesWithoutCast(ctx context.Context, limit int) ([]movie.Movie, error) {
	ctx = queryName(ctx, "movie.without_cast")

	rows, err := r.tx.Querier(ctx).Query(ctx, `SELECT m.id, coalesce(m.tmdb_id, 0), coalesce(m.kinopoisk_id, 0), m.media_type, m.title
 FROM movies m
 WHERE m.cast_fetched_at IS NULL AND EXISTS (SELECT 1 FROM user_movies um WHERE um.movie_id = m.id)
 ORDER BY m.id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list movies without cast: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (movie.Movie, error) {
		var m movie.Movie
		err := row.Scan(&m.ID, &m.TMDBID, &m.KinopoiskID, &m.MediaType, &m.Title)
		return m, err
	})
}

func (r *MovieRepository) castForMovies(ctx context.Context, ids []int64) (map[int64][]movie.CastMember, error) {
	ctx = queryName(ctx, "movie.cast_for_movies")

	out := make(map[int64][]movie.CastMember, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.tx.Querier(ctx).Query(ctx, `SELECT ma.movie_id, a.id, coalesce(a.tmdb_id, 0), coalesce(a.kinopoisk_id, 0),
  a.name, a.original_name, ma.character, a.profile_path
 FROM movie_actors ma JOIN actors a ON a.id = ma.actor_id WHERE ma.movie_id = ANY($1)
 ORDER BY ma.movie_id, ma.position, a.id`, ids)
	if err != nil {
		return nil, fmt.Errorf("load cast: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var c movie.CastMember
		if err := rows.Scan(&id, &c.ID, &c.TMDBID, &c.KinopoiskID, &c.Name, &c.OriginalName, &c.Character, &c.ProfilePath); err != nil {
			return nil, fmt.Errorf("scan cast: %w", err)
		}
		out[id] = append(out[id], c)
	}
	return out, rows.Err()
}
