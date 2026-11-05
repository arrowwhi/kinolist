package handler

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestDuplicateErrorIncludesExistingEntry(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/jobs/1/confirm", nil)
	writeError(w, r, fmt.Errorf("confirm: %w", &movie.AlreadyInCollectionError{UserMovieID: 42}))
	var body struct {
		Code       string `json:"code"`
		ExistingID int64  `json:"existing_user_movie_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 409 || body.Code != "already_in_collection" || body.ExistingID != 42 {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
}

func TestNewSortFieldsAccepted(t *testing.T) {
	for _, sort := range []movie.SortBy{movie.SortByWatchedAt, movie.SortByUserRating} {
		r := httptest.NewRequest("GET", "/movies?sort="+string(sort)+"&order=asc&genre_ids=1,2", nil)
		f, err := parseFilter(r, 7)
		if err != nil {
			t.Fatal(err)
		}
		f = f.Normalize()
		if f.SortBy != sort || f.Desc || len(f.GenreIDs) != 2 {
			t.Fatalf("filter: %+v", f)
		}
	}
}

func TestMediaTypeFilterValidation(t *testing.T) {
	for _, kind := range []string{"movie", "tv", ""} {
		f, err := parseFilter(httptest.NewRequest("GET", "/movies?media_type="+kind, nil), 7)
		if err != nil || string(f.MediaType) != kind {
			t.Fatalf("filter: %+v %v", f, err)
		}
	}
	if _, err := parseFilter(httptest.NewRequest("GET", "/movies?media_type=person", nil), 7); err == nil {
		t.Fatal("invalid type accepted")
	}
}
