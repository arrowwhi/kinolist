package enrichment_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	domain "github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestCandidateDetailsScopedAndCachedForConfirm(t *testing.T) {
	u, d := newUseCase(t, false)
	chosen := candidate(123, "Dune", "Dune", 2021)
	job := domain.Job{ID: 1, UserID: 7, Status: domain.StatusConfirming, Candidates: []movie.Candidate{chosen}}
	d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(job, nil).Times(3)
	if _, err := u.CandidateDetails(context.Background(), 7, 1, movie.ProviderTMDB, 999); !errors.Is(err, domain.ErrCandidateNotInJob) {
		t.Fatal(err)
	}
	votes := int32(150)
	chosen.RuntimeMin = 166
	chosen.Rating = 8.5
	chosen.RatingProvider = movie.ProviderTMDB
	chosen.RatingVoteCount = &votes
	d.primary.EXPECT().Provider().Return(movie.ProviderTMDB)
	d.primary.EXPECT().Details(gomock.Any(), int32(123)).Return(chosen, nil)
	d.movies.EXPECT().GenresByTMDBIDs(gomock.Any(), []int32{18}).Return([]movie.Genre{{ID: 18}}, nil).Times(2)
	got, err := u.CandidateDetails(context.Background(), 7, 1, movie.ProviderTMDB, 123)
	if err != nil {
		t.Fatal(err)
	}
	if got.RuntimeMin != 166 || got.RatingVoteCount == nil || *got.RatingVoteCount != 150 {
		t.Fatalf("details: %+v", got)
	}
	d.tx.EXPECT().BeginTransaction(gomock.Any()).DoAndReturn(func(ctx context.Context) (context.Context, error) { return ctx, nil })
	d.tx.EXPECT().FinishTransaction(gomock.Any(), gomock.Any())
	d.movies.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m movie.Movie) (movie.Movie, error) {
		if m.RuntimeMin != 166 {
			t.Fatalf("runtime: %d", m.RuntimeMin)
		}
		m.ID = 55
		return m, nil
	})
	d.movies.EXPECT().Add(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, um movie.UserMovie) (movie.UserMovie, error) {
		if um.Status != movie.StatusWatched || um.UserRating != 9 || um.Review != "Great" || um.Note != "Personal" {
			t.Fatalf("entry: %+v", um)
		}
		um.ID = 100
		return um, nil
	})
	d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)
	_, err = u.ConfirmWithOptions(context.Background(), 7, 1, movie.ProviderTMDB, 123, movie.ConfirmOptions{Status: movie.StatusWatched, UserRating: 9, Review: "Great", Note: "Personal"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCandidateDetailsChecksJobOwnership(t *testing.T) {
	u, d := newUseCase(t, false)
	d.jobs.EXPECT().GetForUser(gomock.Any(), int64(8), int64(1)).Return(domain.Job{}, domain.ErrJobNotFound)
	_, err := u.CandidateDetails(context.Background(), 8, 1, movie.ProviderTMDB, 123)
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatal(err)
	}
}

func TestConfirmOptionsValidatedBeforeLookup(t *testing.T) {
	u, _ := newUseCase(t, false)
	for _, opts := range []movie.ConfirmOptions{{Status: "bad"}, {UserRating: 11}} {
		if _, err := u.ConfirmWithOptions(context.Background(), 7, 1, movie.ProviderTMDB, 123, opts); err == nil {
			t.Fatal("expected validation error")
		}
	}
}
