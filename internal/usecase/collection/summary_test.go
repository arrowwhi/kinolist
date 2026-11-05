package collection_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestSearchDoesNotChangeSummaryFilter(t *testing.T) {
	uc, movies, _ := newUseCase(t)
	f := movie.Filter{UserID: 7, Status: movie.StatusWatched, GenreIDs: []int16{18}, Query: "Dune", Limit: 1}
	movies.EXPECT().List(gomock.Any(), gomock.Any()).Return([]movie.UserMovie{{ID: 1}}, nil)
	movies.EXPECT().Count(gomock.Any(), gomock.Any()).Return(1, nil)
	avg := 8.4
	movies.EXPECT().Summary(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got movie.Filter) (movie.Summary, error) {
		if got.Query != "" || got.Status != f.Status || got.UserID != 7 || len(got.GenreIDs) != 1 || got.GenreIDs[0] != 18 {
			t.Fatalf("summary filter: %+v", got)
		}
		return movie.Summary{MovieCount: 18, RatedCount: 15, AverageUserRating: &avg}, nil
	})
	page, err := uc.List(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Summary.MovieCount != 18 || *page.Summary.AverageUserRating != 8.4 {
		t.Fatalf("page: %+v", page)
	}
}

func TestReviewValidation(t *testing.T) {
	uc, movies, _ := newUseCase(t)
	tooLong := strings.Repeat("я", 5001)
	_, err := uc.Update(context.Background(), 7, 1, movie.Update{Review: &tooLong})
	if !errors.Is(err, movie.ErrReviewTooLong) {
		t.Fatal(err)
	}
	empty := ""
	movies.EXPECT().Update(gomock.Any(), int64(7), int64(1), movie.Update{Review: &empty}).Return(movie.UserMovie{ID: 1}, nil)
	if _, err := uc.Update(context.Background(), 7, 1, movie.Update{Review: &empty}); err != nil {
		t.Fatal(err)
	}
}
