//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func TestDirectorsCollectionIsolationFiltersAndLiveRatings(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	u, other := seedUser(t, 9701), seedUser(t, 9702)
	a := movie.Director{TMDBID: 101, Name: "Дени Вильнёв", OriginalName: "Denis Villeneuve"}
	b := movie.Director{TMDBID: 102, Name: "Соавтор", OriginalName: "Co Director"}
	g := genresByTMDB(t, repo, 18)
	save := func(id int32, mt movie.MediaType, directors []movie.Director) movie.Movie {
		t.Helper()
		m, err := repo.Upsert(ctx, movie.Movie{TMDBID: id, Title: "Work", MediaType: mt, Genres: g, Directors: directors, DirectorsLoaded: true})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	add := func(userID int64, m movie.Movie, status movie.Status, rating int16) movie.UserMovie {
		t.Helper()
		entry, err := repo.Add(ctx, movie.UserMovie{UserID: userID, Movie: m, Status: status, UserRating: rating})
		if err != nil {
			t.Fatal(err)
		}
		return entry
	}
	film := save(1, movie.MediaMovie, []movie.Director{a, b, a})
	if len(film.Directors) != 2 {
		t.Fatalf("dedup: %+v", film.Directors)
	}
	series := save(1, movie.MediaTV, []movie.Director{a})
	unseen := save(2, movie.MediaMovie, []movie.Director{a})
	private := save(3, movie.MediaMovie, []movie.Director{{TMDBID: 103, Name: "Private"}})
	kp := movie.Director{KinopoiskID: 101, Name: a.Name, OriginalName: a.OriginalName}
	kpFilm := save(4, movie.MediaMovie, []movie.Director{kp})
	entry := add(u.ID, film, movie.StatusWatched, 8)
	add(u.ID, series, movie.StatusWatched, 10)
	add(u.ID, unseen, movie.StatusWantToWatch, 3) // ratings on unwatched entries never count
	add(u.ID, kpFilm, movie.StatusWatched, 0)
	add(other.ID, private, movie.StatusWatched, 1)
	add(other.ID, film, movie.StatusWatched, 2)
	id, secondID := film.Directors[0].ID, film.Directors[1].ID
	list, total, err := repo.ListDirectors(ctx, movie.DirectorFilter{UserID: u.ID, Query: "VILLENEUVE"})
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("search/provider identities: %+v %d %v", list, total, err)
	}
	d, err := repo.GetDirector(ctx, u.ID, id, "")
	if err != nil || d.TotalCount != 3 || d.WatchedCount != 2 || d.WantToWatchCount != 1 || d.RatedCount != 2 || d.AverageUserRating == nil || *d.AverageUserRating != 9 {
		t.Fatalf("stats: %+v %v", d, err)
	}
	typed, err := repo.GetDirector(ctx, u.ID, id, movie.MediaMovie)
	if err != nil || typed.TotalCount != 2 || typed.RatedCount != 1 || *typed.AverageUserRating != 8 {
		t.Fatalf("type stats: %+v %v", typed, err)
	}
	zero, err := repo.GetDirector(ctx, u.ID, secondID, movie.MediaTV)
	if err != nil || zero.TotalCount != 0 || zero.AverageUserRating != nil {
		t.Fatalf("empty type: %+v %v", zero, err)
	}
	if _, err := repo.GetDirector(ctx, u.ID, private.Directors[0].ID, ""); !errors.Is(err, movie.ErrDirectorNotFound) {
		t.Fatalf("private profile: %v", err)
	}
	for _, sortBy := range []string{"name", "work_count", "average_user_rating"} {
		sorted, n, sortErr := repo.ListDirectors(ctx, movie.DirectorFilter{UserID: u.ID, Sort: sortBy, Desc: true})
		if sortErr != nil || n != 3 || len(sorted) != 3 {
			t.Fatalf("sort %s: %+v %d %v", sortBy, sorted, n, sortErr)
		}
		if sortBy == "average_user_rating" && (sorted[0].ID != id || sorted[2].AverageUserRating != nil) {
			t.Fatalf("ratings ordering: %+v", sorted)
		}
	}
	typedList, n, err := repo.ListDirectors(ctx, movie.DirectorFilter{UserID: u.ID, MediaType: movie.MediaTV})
	if err != nil || n != 1 || len(typedList) != 1 || typedList[0].TotalCount != 1 || *typedList[0].AverageUserRating != 10 {
		t.Fatalf("typed list: %+v %d %v", typedList, n, err)
	}
	f := movie.Filter{UserID: u.ID, DirectorIDs: []int64{id, secondID}, GenreIDs: []int16{g[0].ID}}
	items, err := repo.List(ctx, f)
	if err != nil || len(items) != 3 || len(items[0].Movie.Directors) == 0 {
		t.Fatalf("list: %+v %v", items, err)
	}
	count, err := repo.Count(ctx, f)
	if err != nil || count != 3 {
		t.Fatalf("OR must not duplicate works: %d %v", count, err)
	}
	f.Query = "no matching movie"
	summary, err := repo.Summary(ctx, f)
	if err != nil || summary.MovieCount != 3 || *summary.AverageUserRating != 9 {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	genres, err := repo.CollectionGenresFiltered(ctx, movie.Filter{UserID: u.ID, DirectorIDs: []int64{secondID}})
	if err != nil || len(genres) != 1 || genres[0].WatchedCount != 1 || genres[0].WantToWatchCount != 0 {
		t.Fatalf("genre counts: %+v %v", genres, err)
	}
	page, total, err := repo.ListDirectors(ctx, movie.DirectorFilter{UserID: u.ID, Offset: 999})
	if err != nil || len(page) != 0 || total != 3 {
		t.Fatalf("empty page must retain total: %+v %d %v", page, total, err)
	}
	rating := int16(4)
	if _, err := repo.Update(ctx, u.ID, entry.ID, movie.Update{UserRating: &rating}); err != nil {
		t.Fatal(err)
	}
	d, err = repo.GetDirector(ctx, u.ID, id, "")
	if err != nil || *d.AverageUserRating != 7 {
		t.Fatalf("live rating: %+v %v", d, err)
	}
	if err := repo.Delete(ctx, u.ID, entry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetDirector(ctx, u.ID, secondID, ""); !errors.Is(err, movie.ErrDirectorNotFound) {
		t.Fatalf("removed last work: %v", err)
	}
}

func TestDirectorReplacementOnlyOnSuccessfulLoadAndAtomicRollback(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	m := movie.Movie{TMDBID: 70, Title: "Film", DirectorsLoaded: true, Directors: []movie.Director{{TMDBID: 7, Name: "Known"}}}
	saved, err := repo.Upsert(ctx, m)
	if err != nil || !saved.DirectorsLoaded || len(saved.Directors) != 1 {
		t.Fatal(err)
	}
	m.DirectorsLoaded = false
	m.Directors = nil
	saved, err = repo.Upsert(ctx, m)
	if err != nil || len(saved.Directors) != 1 {
		t.Fatalf("failure erased data: %+v %v", saved, err)
	}
	m.DirectorsLoaded = true
	m.Title = "Must roll back"
	m.Directors = []movie.Director{{Name: "Broken identity"}}
	if _, err := repo.Upsert(ctx, m); err == nil {
		t.Fatal("expected invalid director to fail")
	}
	saved, err = repo.GetByTMDBID(ctx, 70)
	if err != nil || saved.Title != "Film" || len(saved.Directors) != 1 {
		t.Fatalf("partial write: %+v %v", saved, err)
	}
	m.Title = "Film"
	m.Directors = []movie.Director{}
	saved, err = repo.Upsert(ctx, m)
	if err != nil || len(saved.Directors) != 0 {
		t.Fatalf("successful empty list: %+v %v", saved, err)
	}
	var fetched bool
	if err := testPool.QueryRow(ctx, `SELECT directors_fetched_at IS NOT NULL FROM movies WHERE id=$1`, saved.ID).Scan(&fetched); err != nil || !fetched {
		t.Fatalf("load marker: %v %v", fetched, err)
	}
}
