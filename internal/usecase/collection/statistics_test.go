package collection_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestStatisticsUsesOnlyCurrentUser(t *testing.T) {
	uc, movies, _ := newUseCase(t)
	expected := movie.Statistics{TotalCount: 42, WantToWatchCount: 24, WatchedCount: 18}
	movies.EXPECT().Statistics(gomock.Any(), int64(7)).Return(expected, nil)
	actual, err := uc.Statistics(context.Background(), 7)
	if err != nil || actual != expected {
		t.Fatalf("statistics: %+v %v", actual, err)
	}
}

func TestStatisticsPreservesRepositoryError(t *testing.T) {
	uc, movies, _ := newUseCase(t)
	cause := errors.New("database unavailable")
	movies.EXPECT().Statistics(gomock.Any(), int64(7)).Return(movie.Statistics{}, cause)
	if _, err := uc.Statistics(context.Background(), 7); !errors.Is(err, cause) {
		t.Fatalf("error: %v", err)
	}
}
