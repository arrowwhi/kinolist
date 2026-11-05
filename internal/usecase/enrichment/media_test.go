package enrichment_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	domain "github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestCandidateTypeDisambiguatesIDAndCache(t *testing.T) {
	u, d := newUseCase(t, false)
	film := candidate(7, "Shared", "Shared", 2022)
	film.MediaType = movie.MediaMovie
	series := film
	series.MediaType = movie.MediaTV
	job := domain.Job{ID: 1, UserID: 7, Status: domain.StatusConfirming, Candidates: []movie.Candidate{film, series}}
	d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(job, nil).Times(4)
	if _, err := u.CandidateDetails(context.Background(), 7, 1, movie.ProviderTMDB, 7); !errors.Is(err, domain.ErrCandidateNotInJob) {
		t.Fatalf("untyped ID: %v", err)
	}
	if _, err := u.ConfirmWithOptions(context.Background(), 7, 1, movie.ProviderTMDB, 7, movie.ConfirmOptions{}); !errors.Is(err, domain.ErrCandidateNotInJob) {
		t.Fatalf("untyped confirmation: %v", err)
	}
	seasons := int16(2)
	series.SeasonCount = &seasons
	d.primary.EXPECT().Provider().Return(movie.ProviderTMDB).Times(2)
	d.primary.EXPECT().Details(gomock.Any(), int32(7), movie.MediaMovie).Return(film, nil)
	d.primary.EXPECT().Details(gomock.Any(), int32(7), movie.MediaTV).Return(series, nil)
	d.movies.EXPECT().GenresByTMDBIDs(gomock.Any(), []int32{18}).Return([]movie.Genre{}, nil).Times(2)
	f, err := u.CandidateDetails(context.Background(), 7, 1, movie.ProviderTMDB, 7, movie.MediaMovie)
	if err != nil || f.MediaType != movie.MediaMovie {
		t.Fatalf("film: %+v %v", f, err)
	}
	s, err := u.CandidateDetails(context.Background(), 7, 1, movie.ProviderTMDB, 7, movie.MediaTV)
	if err != nil || s.MediaType != movie.MediaTV || s.SeasonCount == nil || *s.SeasonCount != 2 {
		t.Fatalf("series: %+v %v", s, err)
	}
}
