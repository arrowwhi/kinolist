//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func TestGlobalStatisticsCountsEntriesAndTracksChanges(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	u := seedUser(t, 901)
	other := seedUser(t, 902)
	check := func(want movie.Statistics) {
		t.Helper()
		got, err := repo.Statistics(ctx, u.ID)
		if err != nil || got != want || got.TotalCount != got.WantToWatchCount+got.WatchedCount {
			t.Fatalf("statistics: %+v, want %+v, error %v", got, want, err)
		}
	}
	check(movie.Statistics{})
	film, err := repo.Upsert(ctx, movie.Movie{TMDBID: 7, Title: "Film"})
	if err != nil {
		t.Fatal(err)
	}
	seasons := int16(12)
	series, err := repo.Upsert(ctx, movie.Movie{TMDBID: 7, Title: "Series", MediaType: movie.MediaTV, SeasonCount: &seasons})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: film})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: series, Status: movie.StatusWatched}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Add(ctx, movie.UserMovie{UserID: other.ID, Movie: film}); err != nil {
		t.Fatal(err)
	}
	check(movie.Statistics{TotalCount: 2, WantToWatchCount: 1, WatchedCount: 1})
	watched := movie.StatusWatched
	if _, err := repo.Update(ctx, u.ID, entry.ID, movie.Update{Status: &watched}); err != nil {
		t.Fatal(err)
	}
	check(movie.Statistics{TotalCount: 2, WatchedCount: 2})
	if err := repo.Delete(ctx, u.ID, entry.ID); err != nil {
		t.Fatal(err)
	}
	check(movie.Statistics{TotalCount: 1, WatchedCount: 1})
}
