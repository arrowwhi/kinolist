package errmap_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
)

func TestMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", movie.ErrNotFound, http.StatusNotFound, "movie_not_found"},
		{"duplicate", movie.ErrAlreadyInCollection, http.StatusConflict, "already_in_collection"},
		{
			"wrapped sentinel is still mapped",
			fmt.Errorf("add movie to collection: %w", movie.ErrAlreadyInCollection),
			http.StatusConflict, "already_in_collection",
		},
		{
			"deeply wrapped",
			fmt.Errorf("usecase: %w", fmt.Errorf("repo: %w", enrichment.ErrJobNotFound)),
			http.StatusNotFound, "job_not_found",
		},
		{
			"unknown error never leaks details",
			errors.New("pq: duplicate key value violates unique constraint \"user_movies_unique\""),
			http.StatusInternalServerError, "internal",
		},
		{"nil is ok", nil, http.StatusOK, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := errmap.Map(tt.err)
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", got.Status, tt.wantStatus)
			}
			if got.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", got.Code, tt.wantCode)
			}
		})
	}
}

func TestMapNeverEchoesTheOriginalMessage(t *testing.T) {
	t.Parallel()

	secret := "password=hunter2 host=10.0.0.5"
	got := errmap.Map(errors.New("connect failed: " + secret))

	if got.Message == "" {
		t.Fatal("message must not be empty")
	}
	if got.Message == secret || len(got.Message) > 60 {
		t.Fatalf("internal details leaked into the client message: %q", got.Message)
	}
	if !errmap.IsInternal(errors.New("boom")) {
		t.Fatal("unknown errors must be classified as internal")
	}
}
