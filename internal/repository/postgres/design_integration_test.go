//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func TestDesignCollectionMetadataAndStats(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	u := seedUser(t, 100)
	other := seedUser(t, 200)
	drama := genresByTMDB(t, repo, 18)
	comedy := genresByTMDB(t, repo, 35)
	votes := int32(150)
	m, err := repo.Upsert(ctx, movie.Movie{TMDBID: 1, Title: "Dune", RuntimeMin: 166, Rating: 8.5, RatingProvider: movie.ProviderTMDB, RatingVoteCount: &votes, Genres: drama})
	if err != nil {
		t.Fatal(err)
	}
	watched, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m, Status: movie.StatusWatched, UserRating: 9, Review: "Great", Note: "Personal"})
	if err != nil {
		t.Fatal(err)
	}
	if watched.WatchedAt == nil {
		t.Fatal("missing automatic watch date")
	}
	got, err := repo.Get(ctx, u.ID, watched.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Review != "Great" || got.Note != "Personal" || got.Movie.RuntimeMin != 166 || got.Movie.Rating != 8.5 || got.Movie.RatingProvider != movie.ProviderTMDB || got.Movie.RatingVoteCount == nil || *got.Movie.RatingVoteCount != votes {
		t.Fatalf("metadata: %+v", got)
	}
	// A degraded search result must not destroy cached details and rating provenance.
	updated, err := repo.Upsert(ctx, movie.Movie{TMDBID: 1, Title: "Dune", Genres: drama})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RuntimeMin != 166 || updated.RatingVoteCount == nil || *updated.RatingVoteCount != 150 {
		t.Fatalf("upsert lost details: %+v", updated)
	}
	m2, err := repo.Upsert(ctx, movie.Movie{TMDBID: 2, Title: "Other", Genres: comedy})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m2, Status: movie.StatusWatched}); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Add(ctx, movie.UserMovie{UserID: other.ID, Movie: m2, Status: movie.StatusWatched, UserRating: 1}); err != nil {
		t.Fatal(err)
	}
	f := movie.Filter{UserID: u.ID, Status: movie.StatusWatched, Query: "no matches", Limit: 1}
	total, err := repo.Count(ctx, f)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := repo.Summary(ctx, f)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || summary.MovieCount != 2 || summary.RatedCount != 1 || summary.AverageUserRating == nil || *summary.AverageUserRating != 9 {
		t.Fatalf("total=%d summary=%+v", total, summary)
	}
	f.GenreIDs = []int16{comedy[0].ID}
	summary, err = repo.Summary(ctx, f)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MovieCount != 1 || summary.RatedCount != 0 || summary.AverageUserRating != nil {
		t.Fatalf("unrated summary: %+v", summary)
	}
	stats, err := repo.CollectionGenres(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("genres=%+v", stats)
	}
	for _, g := range stats {
		if g.WatchedCount != 1 || g.WantToWatchCount != 0 {
			t.Fatalf("genre stats: %+v", g)
		}
	}
	for _, desc := range []bool{true, false} {
		items, err := repo.List(ctx, movie.Filter{UserID: u.ID, SortBy: movie.SortByUserRating, Desc: desc})
		if err != nil {
			t.Fatal(err)
		}
		if items[0].ID != watched.ID {
			t.Fatal("unrated must sort last")
		}
	}
	note := ""
	review := "Updated"
	originalDate := *got.WatchedAt
	got, err = repo.Update(ctx, u.ID, watched.ID, movie.Update{Note: &note, Review: &review})
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != "" || got.Review != "Updated" || !got.WatchedAt.Equal(originalDate) {
		t.Fatalf("review edit changed date: %+v", got)
	}
	want := movie.StatusWantToWatch
	got, err = repo.Update(ctx, u.ID, watched.ID, movie.Update{Status: &want})
	if err != nil {
		t.Fatal(err)
	}
	if got.WatchedAt != nil || got.Review != "Updated" || got.UserRating != 9 {
		t.Fatalf("return lost personal data: %+v", got)
	}
	summary, err = repo.Summary(ctx, movie.Filter{UserID: u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RatedCount != 0 || summary.AverageUserRating != nil {
		t.Fatal("want-to-watch rating counted")
	}
	_, err = repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m})
	var duplicate *movie.AlreadyInCollectionError
	if !errors.As(err, &duplicate) || duplicate.UserMovieID != watched.ID {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestRatingProviderChangeClearsOldVotes(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	votes := int32(400)
	_, err := repo.Upsert(ctx, movie.Movie{TMDBID: 1, Title: "A", Rating: 8, RatingProvider: movie.ProviderTMDB, RatingVoteCount: &votes})
	if err != nil {
		t.Fatal(err)
	}
	m, err := repo.Upsert(ctx, movie.Movie{TMDBID: 1, Title: "A", Rating: 7, RatingProvider: movie.ProviderKinopoisk})
	if err != nil {
		t.Fatal(err)
	}
	if m.RatingProvider != movie.ProviderKinopoisk || m.RatingVoteCount != nil {
		t.Fatalf("mixed rating sources: %+v", m)
	}
}
