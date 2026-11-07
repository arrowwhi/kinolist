package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	"github.com/arrowwhi/kinolist/internal/usecase/collection"
)

type directorStub struct {
	collectionUseCase
	filter movie.DirectorFilter
	userID int64
	id     int64
	mt     movie.MediaType
	calls  int
	err    error
}

func (s *directorStub) ListDirectors(_ context.Context, f movie.DirectorFilter) (collection.DirectorPage, error) {
	s.filter = f
	s.calls++
	return collection.DirectorPage{Items: []movie.DirectorStats{{Director: movie.Director{ID: 17, Name: "Дени", OriginalName: "Denis"}, TotalCount: 2}}, Total: 1, Limit: 50}, s.err
}

func (s *directorStub) GetDirector(_ context.Context, userID, id int64, mt movie.MediaType) (movie.DirectorStats, error) {
	s.userID, s.id, s.mt = userID, id, mt
	s.calls++
	return movie.DirectorStats{Director: movie.Director{ID: id, Name: "Дени"}}, s.err
}

func directorRouter(stub *directorStub) http.Handler {
	h := NewMovie(stub)
	r := chi.NewRouter()
	r.Use(middleware.TelegramAuth("test-token", func(context.Context, middleware.Identity) (int64, error) { return 7, nil }))
	r.Get("/directors", h.ListDirectors)
	r.Get("/directors/{id}", h.GetDirector)
	return r
}

func TestDirectorHTTPIdentityAndContract(t *testing.T) {
	stub := &directorStub{}
	r := directorRouter(stub)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/directors?q=Denis&media_type=tv&sort=work_count&order=desc&limit=10&offset=5&user_id=999", nil)
	req.Header.Set("X-Telegram-Init-Data", statisticsInitData())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var page dto.DirectorList
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || stub.filter.UserID != 7 || stub.filter.MediaType != movie.MediaTV || stub.filter.Query != "Denis" || !stub.filter.Desc || stub.filter.Limit != 10 || stub.filter.Offset != 5 || len(page.Items) != 1 || page.Items[0].AverageUserRating != nil || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response: %d %s, filter %+v", w.Code, w.Body.String(), stub.filter)
	}
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/directors/17?media_type=movie&user_id=999", nil)
	req.Header.Set("X-Telegram-Init-Data", statisticsInitData())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || stub.userID != 7 || stub.id != 17 || stub.mt != movie.MediaMovie {
		t.Fatalf("profile scope: %d %+v", w.Code, stub)
	}
	stub.err = movie.ErrDirectorNotFound
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("missing profile: %d %s", w.Code, w.Body.String())
	}
}

func TestDirectorHTTPRejectsAnonymousAndInvalidPagination(t *testing.T) {
	for _, path := range []string{"/directors?limit=bad", "/directors?limit=0", "/directors?offset=-1", "/directors?order=invalid", "/directors/0"} {
		stub := &directorStub{}
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		req.Header.Set("X-Telegram-Init-Data", statisticsInitData())
		w := httptest.NewRecorder()
		directorRouter(stub).ServeHTTP(w, req)
		if (w.Code != 400 && w.Code != 404) || stub.calls != 0 {
			t.Fatalf("%s: %d calls=%d", path, w.Code, stub.calls)
		}
	}
	stub := &directorStub{}
	w := httptest.NewRecorder()
	directorRouter(stub).ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/directors", nil))
	if w.Code != 401 || stub.calls != 0 {
		t.Fatalf("anonymous: %d", w.Code)
	}
}

func TestDirectorIDsFilterParsing(t *testing.T) {
	for _, raw := range []string{"0", "-1", "1,", "abc", "9223372036854775808"} {
		if _, err := parseFilter(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/movies?director_ids="+raw, nil), 7); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	f, err := parseFilter(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/movies?director_ids=17,25,17&media_type=tv&genre_ids=18", nil), 7)
	if err != nil || len(f.DirectorIDs) != 2 || f.DirectorIDs[0] != 17 || f.DirectorIDs[1] != 25 || f.MediaType != movie.MediaTV || len(f.GenreIDs) != 1 {
		t.Fatalf("filter: %+v %v", f, err)
	}
}
