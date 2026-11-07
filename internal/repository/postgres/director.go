package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func (r *MovieRepository) replaceDirectors(ctx context.Context, movieID int64, directors []movie.Director) error {
	ctx = queryName(ctx, "movie.replace_directors")

	q := r.tx.Querier(ctx)
	if _, err := q.Exec(ctx, `DELETE FROM movie_directors WHERE movie_id=$1`, movieID); err != nil {
		return fmt.Errorf("clear directors: %w", err)
	}
	positions := make([]int, len(directors))
	for i := range directors {
		positions[i] = i
	}
	sort.SliceStable(positions, func(i, j int) bool {
		a, b := directors[positions[i]], directors[positions[j]]
		if (a.TMDBID > 0) != (b.TMDBID > 0) {
			return a.TMDBID > 0
		}
		if a.TMDBID > 0 {
			return a.TMDBID < b.TMDBID
		}
		return a.KinopoiskID < b.KinopoiskID
	})
	for _, position := range positions {
		d := directors[position]
		var id int64
		column, externalID := "tmdb_id", d.TMDBID
		if externalID == 0 {
			column, externalID = "kinopoisk_id", d.KinopoiskID
		}
		if externalID <= 0 || strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("save director: invalid external identity or name")
		}
		// Кинопоиск отдаёт русские имена, TMDB в credits — обычно латиницу.
		source := movie.NameSourceTMDB
		if column == "kinopoisk_id" {
			source = movie.NameSourceKinopoisk
		}
		// The conflict column comes only from the constants above. Names never merge identities.
		// Имя из менее надёжного источника не затирает уже найденное русское.
		better := nameRankSQL("EXCLUDED.name_source") + ` >= ` + nameRankSQL("directors.name_source")
		sql := `INSERT INTO directors (` + column + `,name,original_name,name_source) VALUES ($1,$2,$3,$4)
 ON CONFLICT (` + column + `) DO UPDATE SET name=CASE WHEN ` + better + ` THEN EXCLUDED.name ELSE directors.name END,
 name_source=CASE WHEN ` + better + ` THEN EXCLUDED.name_source ELSE directors.name_source END,
 original_name=coalesce(nullif(EXCLUDED.original_name,''),directors.original_name),updated_at=now() RETURNING id`
		if err := q.QueryRow(ctx, sql, externalID, d.Name, d.OriginalName, source).Scan(&id); err != nil {
			return fmt.Errorf("upsert director: %w", err)
		}
		if _, err := q.Exec(ctx, `INSERT INTO movie_directors(movie_id,director_id,position) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, movieID, id, position); err != nil {
			return fmt.Errorf("link director: %w", err)
		}
	}
	if _, err := q.Exec(ctx, `UPDATE movies SET directors_fetched_at=now() WHERE id=$1`, movieID); err != nil {
		return fmt.Errorf("mark directors loaded: %w", err)
	}
	return nil
}

func (r *MovieRepository) directorsForMovies(ctx context.Context, ids []int64) (map[int64][]movie.Director, error) {
	ctx = queryName(ctx, "movie.directors_for_movies")

	out := make(map[int64][]movie.Director, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.tx.Querier(ctx).Query(ctx, `SELECT md.movie_id,d.id,coalesce(d.tmdb_id,0),coalesce(d.kinopoisk_id,0),d.name,d.original_name
 FROM movie_directors md JOIN directors d ON d.id=md.director_id WHERE md.movie_id=ANY($1)
 ORDER BY md.movie_id,md.position,d.id`, ids)
	if err != nil {
		return nil, fmt.Errorf("load directors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var d movie.Director
		if err := rows.Scan(&id, &d.ID, &d.TMDBID, &d.KinopoiskID, &d.Name, &d.OriginalName); err != nil {
			return nil, fmt.Errorf("scan director: %w", err)
		}
		out[id] = append(out[id], d)
	}
	return out, rows.Err()
}

// Aggregate before searching or paginating: a name query never changes a person's counters.
const directorStatsSQL = `SELECT d.id,coalesce(d.tmdb_id,0) AS tmdb_id,coalesce(d.kinopoisk_id,0) AS kinopoisk_id,d.name,d.original_name,
 count(*) FILTER(WHERE $2='' OR m.media_type=$2) AS total_count,
 count(*) FILTER(WHERE ($2='' OR m.media_type=$2) AND um.status='want_to_watch') AS want_to_watch_count,
 count(*) FILTER(WHERE ($2='' OR m.media_type=$2) AND um.status='watched') AS watched_count,
 count(um.user_rating) FILTER(WHERE ($2='' OR m.media_type=$2) AND um.status='watched') AS rated_count,
 round(avg(um.user_rating) FILTER(WHERE ($2='' OR m.media_type=$2) AND um.status='watched'),1) AS average_user_rating
 FROM directors d JOIN movie_directors md ON md.director_id=d.id
 JOIN user_movies um ON um.movie_id=md.movie_id JOIN movies m ON m.id=md.movie_id
 WHERE um.user_id=$1 GROUP BY d.id`

func scanDirector(row scanner) (movie.DirectorStats, error) {
	var d movie.DirectorStats
	err := row.Scan(&d.ID, &d.TMDBID, &d.KinopoiskID, &d.Name, &d.OriginalName, &d.TotalCount, &d.WantToWatchCount, &d.WatchedCount, &d.RatedCount, &d.AverageUserRating)
	return d, err
}

func (r *MovieRepository) GetDirector(ctx context.Context, userID, directorID int64, mt movie.MediaType) (movie.DirectorStats, error) {
	ctx = queryName(ctx, "movie.get_director")

	d, err := scanDirector(r.tx.Querier(ctx).QueryRow(ctx, `SELECT * FROM (`+directorStatsSQL+`) s WHERE id=$3`, userID, mt, directorID))
	if isNoRows(err) {
		return d, movie.ErrDirectorNotFound
	}
	if err != nil {
		return d, fmt.Errorf("get director: %w", err)
	}
	return d, nil
}

func (r *MovieRepository) ListDirectors(ctx context.Context, f movie.DirectorFilter) ([]movie.DirectorStats, int, error) {
	ctx = queryName(ctx, "movie.list_directors")

	f = f.Normalize()
	order := "lower(name)"
	switch f.Sort {
	case "work_count":
		order = "total_count"
	case "average_user_rating":
		order = "average_user_rating"
	}
	direction := " ASC"
	if f.Desc {
		direction = " DESC"
	}
	// The count and page share one statement; the left join retains the count for an empty page.
	query := `WITH filtered AS (SELECT * FROM (` + directorStatsSQL + `) s WHERE total_count>0
 AND ($3='' OR strpos(lower(name),lower($3))>0 OR strpos(lower(original_name),lower($3))>0)),
 page AS (SELECT * FROM filtered ORDER BY ` + order + direction + ` NULLS LAST,id ASC LIMIT $4 OFFSET $5)
 SELECT (SELECT count(*) FROM filtered),p.id,p.tmdb_id,p.kinopoisk_id,p.name,p.original_name,
 p.total_count,p.want_to_watch_count,p.watched_count,p.rated_count,p.average_user_rating
 FROM (SELECT 1) anchor LEFT JOIN page p ON true ORDER BY ` + order + direction + ` NULLS LAST,p.id ASC`
	rows, err := r.tx.Querier(ctx).Query(ctx, query, f.UserID, f.MediaType, strings.TrimSpace(f.Query), f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list directors: %w", err)
	}
	defer rows.Close()
	out := make([]movie.DirectorStats, 0)
	total := 0
	for rows.Next() {
		var id *int64
		var tmdb, kp *int32
		var name, original *string
		var count, want, watched, rated *int
		var avg *float64
		if err := rows.Scan(&total, &id, &tmdb, &kp, &name, &original, &count, &want, &watched, &rated, &avg); err != nil {
			return nil, 0, fmt.Errorf("scan director page: %w", err)
		}
		if id != nil {
			out = append(out, movie.DirectorStats{Director: movie.Director{ID: *id, TMDBID: *tmdb, KinopoiskID: *kp, Name: *name, OriginalName: *original}, TotalCount: *count, WantToWatchCount: *want, WatchedCount: *watched, RatedCount: *rated, AverageUserRating: avg})
		}
	}
	return out, total, rows.Err()
}
