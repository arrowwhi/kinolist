package collection_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestDirectorValidationAndPagination(t *testing.T) {
	uc, repo, _ := newUseCase(t)
	for _, f := range []movie.DirectorFilter{{MediaType: "unknown"}, {Sort: "DROP TABLE directors"}} {
		if _, err := uc.ListDirectors(context.Background(), f); err == nil {
			t.Fatalf("invalid filter: %+v", f)
		}
	}
	if _, err := uc.GetDirector(context.Background(), 7, 17, "bad"); !errors.Is(err, movie.ErrInvalidMediaType) {
		t.Fatal(err)
	}
	repo.EXPECT().ListDirectors(gomock.Any(), movie.DirectorFilter{UserID: 7, Sort: "name", Limit: 200}).Return([]movie.DirectorStats{}, 3, nil)
	page, err := uc.ListDirectors(context.Background(), movie.DirectorFilter{UserID: 7, Limit: 999, Offset: -1})
	if err != nil || page.Total != 3 || page.Limit != 200 || page.Offset != 0 {
		t.Fatalf("page: %+v %v", page, err)
	}
	repo.EXPECT().GetDirector(gomock.Any(), int64(7), int64(17), movie.MediaTV).Return(movie.DirectorStats{}, movie.ErrDirectorNotFound)
	if _, err := uc.GetDirector(context.Background(), 7, 17, movie.MediaTV); !errors.Is(err, movie.ErrDirectorNotFound) {
		t.Fatal(err)
	}
}
