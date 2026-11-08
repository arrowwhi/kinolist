package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

type enrichmentStub struct {
	enrichmentUseCase
	title   movie.CatalogTitle
	err     error
	options movie.ConfirmOptions
	got     struct {
		provider   movie.Provider
		externalID int32
		mediaType  movie.MediaType
	}
}

func (s *enrichmentStub) CatalogTitle(_ context.Context, _ int64, p movie.Provider, id int32, mt movie.MediaType) (movie.CatalogTitle, error) {
	s.got.provider, s.got.externalID, s.got.mediaType = p, id, mt
	return s.title, s.err
}

func (s *enrichmentStub) AddFromCatalog(_ context.Context, _ int64, _ movie.Provider, _ int32, o movie.ConfirmOptions) (movie.UserMovie, error) {
	s.options = o
	return movie.UserMovie{ID: 100, Status: o.Status, UserRating: o.UserRating, Review: o.Review}, s.err
}

func catalogRouter(stub *enrichmentStub, now time.Time) http.Handler {
	h := NewEnrichment(stub)
	h.now = func() time.Time { return now }
	r := chi.NewRouter()
	r.Use(middleware.TelegramAuth("test-token", func(context.Context, middleware.Identity) (int64, error) { return 7, nil }))
	r.Get("/catalog/{provider}/{media_type}/{external_id}", h.CatalogTitle)
	r.Post("/movies", h.AddFromCatalog)
	return r
}

func TestCatalogTitleHTTP(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	release := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	stub := &enrichmentStub{title: movie.CatalogTitle{
		Movie:       movie.Movie{TMDBID: 42, MediaType: movie.MediaMovie, Title: "Будущее", Year: 2027},
		ReleaseDate: &release,
	}}
	w := serveDirectors(t, catalogRouter(stub, now), http.MethodGet, "/catalog/tmdb/movie/42", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["release_date"] != "2027-03-01" || body["upcoming"] != true || body["id"] != float64(0) ||
		body["tmdb_id"] != float64(42) || body["media_type"] != "movie" {
		t.Fatalf("body: %v", body)
	}
	if _, ok := body["user_movie_id"]; ok {
		t.Fatalf("user_movie_id must be omitted outside collection: %v", body)
	}
	if stub.got.provider != movie.ProviderTMDB || stub.got.externalID != 42 || stub.got.mediaType != movie.MediaMovie {
		t.Fatalf("args: %+v", stub.got)
	}

	// Работа из коллекции: пометки пользователя есть, «скоро» — нет.
	stub.title.UserMovieID, stub.title.Status, stub.title.UserRating = 9, movie.StatusWatched, 8
	w = serveDirectors(t, catalogRouter(stub, now), http.MethodGet, "/catalog/tmdb/movie/42", "")
	body = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["user_movie_id"] != float64(9) || body["status"] != "watched" || body["user_rating"] != float64(8) || body["upcoming"] != false {
		t.Fatalf("in collection: %v", body)
	}
}

func TestCatalogTitleHTTPErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		err  error
		want int
	}{
		{"/catalog/imdb/movie/42", nil, http.StatusBadRequest},
		{"/catalog/tmdb/anime/42", nil, http.StatusBadRequest},
		{"/catalog/tmdb/movie/0", nil, http.StatusBadRequest},
		{"/catalog/tmdb/movie/abc", nil, http.StatusBadRequest},
		{"/catalog/tmdb/movie/99999999999", nil, http.StatusBadRequest},
		{"/catalog/tmdb/movie/42", movie.ErrNotFound, http.StatusNotFound},
		{"/catalog/tmdb/tv/42", movie.ErrProviderUnavailable, http.StatusBadGateway},
	}
	for _, tt := range tests {
		stub := &enrichmentStub{err: tt.err}
		w := serveDirectors(t, catalogRouter(stub, time.Now()), http.MethodGet, tt.path, "")
		if w.Code != tt.want {
			t.Fatalf("%s: status = %d, want %d: %s", tt.path, w.Code, tt.want, w.Body)
		}
	}
}

func TestAddFromCatalogHTTPPassesRatingAndReview(t *testing.T) {
	t.Parallel()

	stub := &enrichmentStub{}
	w := serveDirectors(t, catalogRouter(stub, time.Now()), http.MethodPost, "/movies",
		`{"provider":"tmdb","external_id":42,"media_type":"movie","status":"watched","user_rating":8,"review":"Хорошо"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	if stub.options.Status != movie.StatusWatched || stub.options.UserRating != 8 || stub.options.Review != "Хорошо" || stub.options.MediaType != movie.MediaMovie {
		t.Fatalf("options: %+v", stub.options)
	}

	stub.err = &movie.AlreadyInCollectionError{UserMovieID: 9}
	w = serveDirectors(t, catalogRouter(stub, time.Now()), http.MethodPost, "/movies",
		`{"provider":"tmdb","external_id":42,"media_type":"movie"}`)
	if w.Code != http.StatusConflict || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("duplicate: %d %s", w.Code, w.Body)
	}
	var e map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e["existing_user_movie_id"] != float64(9) {
		t.Fatalf("duplicate body: %v", e)
	}
}
