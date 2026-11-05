//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func TestTypedIdentitySeasonPersistenceAndSummary(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	u := seedUser(t, 901)
	drama := genresByTMDB(t, repo, 18)
	seasons := int16(2)
	film, err := repo.Upsert(ctx, movie.Movie{TMDBID: 7, Title: "Shared", MediaType: movie.MediaMovie, Genres: drama})
	if err != nil {
		t.Fatal(err)
	}
	series, err := repo.Upsert(ctx, movie.Movie{TMDBID: 7, Title: "Shared", MediaType: movie.MediaTV, SeasonCount: &seasons, Genres: drama})
	if err != nil {
		t.Fatal(err)
	}
	if film.ID == series.ID {
		t.Fatal("film merged into series")
	}
	for _, entry := range []struct {
		m      movie.Movie
		rating int16
	}{{film, 4}, {series, 10}} {
		if _, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: entry.m, Status: movie.StatusWatched, UserRating: entry.rating}); err != nil {
			t.Fatal(err)
		}
	}
	updated, err := repo.Upsert(ctx, movie.Movie{TMDBID: 7, Title: "Shared", MediaType: movie.MediaTV, Genres: drama})
	if err != nil || updated.SeasonCount == nil || *updated.SeasonCount != 2 {
		t.Fatalf("lost seasons: %+v %v", updated, err)
	}
	stored, err := repo.GetByTMDBID(ctx, 7, movie.MediaTV)
	if err != nil || stored.ID != series.ID || stored.SeasonCount == nil {
		t.Fatalf("typed lookup: %+v %v", stored, err)
	}
	f := movie.Filter{UserID: u.ID, MediaType: movie.MediaTV, Status: movie.StatusWatched, GenreIDs: []int16{drama[0].ID}}
	list, err := repo.List(ctx, f)
	if err != nil || len(list) != 1 || list[0].Movie.MediaType != movie.MediaTV || list[0].Movie.SeasonCount == nil {
		t.Fatalf("list: %+v %v", list, err)
	}
	f.Query = "absent"
	count, err := repo.Count(ctx, f)
	if err != nil || count != 0 {
		t.Fatalf("count: %d %v", count, err)
	}
	summary, err := repo.Summary(ctx, f)
	if err != nil || summary.MovieCount != 1 || summary.AverageUserRating == nil || *summary.AverageUserRating != 10 {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	stats, err := repo.CollectionGenres(ctx, u.ID, movie.MediaTV)
	if err != nil || len(stats) != 1 || stats[0].WatchedCount != 1 {
		t.Fatalf("genres: %+v %v", stats, err)
	}
	var badMapping int
	if err := testPool.QueryRow(ctx, "SELECT count(*) FROM genre_external_map WHERE source='kinopoisk' AND external_name='сериал'").Scan(&badMapping); err != nil || badMapping != 0 {
		t.Fatalf("series is still a genre: %d %v", badMapping, err)
	}
}
