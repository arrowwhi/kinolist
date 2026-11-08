package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	directorsuc "github.com/arrowwhi/kinolist/internal/usecase/directors"
)

type directorsStub struct {
	directorsUseCase
	filter   movie.MyDirectorsFilter
	tmdbID   int32
	followed []int64
	query    string
	err      error
}

func (s *directorsStub) MyDirectors(_ context.Context, f movie.MyDirectorsFilter) (directorsuc.Page, error) {
	s.filter = f
	return directorsuc.Page{Items: []movie.DirectorOverview{{Director: movie.Director{ID: 5, Name: "Дени Вильнёв"}, FilmographyLoaded: true, WorkCount: 12}}, Total: 1, Counts: movie.DirectorScopeCounts{All: 1, Followed: 1}}, s.err
}

func (s *directorsStub) Profile(_ context.Context, _, id int64) (movie.DirectorProfile, error) {
	release := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	return movie.DirectorProfile{
		DirectorOverview: movie.DirectorOverview{Director: movie.Director{ID: id, Name: "Дени"}},
		Filmography: []movie.FilmographyItem{
			{DirectorWork: movie.DirectorWork{MediaType: movie.MediaMovie, TMDBID: 1, Title: "Будущее", ReleaseDate: &release}},
			{DirectorWork: movie.DirectorWork{MediaType: movie.MediaMovie, TMDBID: 2, Title: "Дюна", Year: 2021}, UserMovieID: 9, Status: movie.StatusWatched, UserRating: 9},
		},
	}, s.err
}

func (s *directorsStub) Search(_ context.Context, _ int64, query string) ([]movie.PersonSearchResult, error) {
	s.query = query
	return []movie.PersonSearchResult{{Person: movie.Person{TMDBID: 525, Name: "Нолан"}}, {Person: movie.Person{TMDBID: 1, Name: "Вильнёв"}, PersonID: 5, Followed: true}}, s.err
}

func (s *directorsStub) FollowByTMDB(_ context.Context, _ int64, tmdbID int32) (movie.DirectorOverview, error) {
	s.tmdbID = tmdbID
	return movie.DirectorOverview{Director: movie.Director{ID: 5}, Followed: true}, s.err
}

func (s *directorsStub) Follow(_ context.Context, _, id int64) error {
	s.followed = append(s.followed, id)
	return s.err
}

func directorsRouter(stub *directorsStub) http.Handler {
	h := NewDirectors(stub)
	r := chi.NewRouter()
	r.Use(middleware.TelegramAuth("test-token", func(context.Context, middleware.Identity) (int64, error) { return 7, nil }))
	r.Get("/my-directors", h.Mine)
	r.Get("/people/search", h.Search)
	r.Post("/directors/follow-by-tmdb", h.FollowByTMDB)
	r.Get("/directors/{id}/profile", h.Profile)
	r.Put("/directors/{id}/follow", h.Follow)
	return r
}

func serveDirectors(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("X-Telegram-Init-Data", statisticsInitData())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMyDirectorsHTTPDefaultsAndValidation(t *testing.T) {
	stub := &directorsStub{}
	r := directorsRouter(stub)
	w := serveDirectors(t, r, http.MethodGet, "/my-directors?scope=followed&sort=progress&query=Ден&limit=10", "")
	var page dto.DirectorOverviewList
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	// Прогресс по умолчанию — от большего к меньшему.
	if w.Code != 200 || stub.filter.UserID != 7 || stub.filter.Scope != movie.DirectorScopeFollowed || !stub.filter.Desc ||
		stub.filter.Query != "Ден" || stub.filter.Limit != 10 || page.Counts.Followed != 1 || page.Items[0].ID != 5 ||
		!page.Items[0].FilmographyComplete || page.Items[0].KnownFor == nil {
		t.Fatalf("response %d %s filter %+v", w.Code, w.Body.String(), stub.filter)
	}
	serveDirectors(t, r, http.MethodGet, "/my-directors?sort=name", "")
	if stub.filter.Desc {
		t.Fatal("names must default to ascending order")
	}
	for _, path := range []string{"/my-directors?order=sideways", "/my-directors?limit=0", "/my-directors?offset=-1"} {
		if w := serveDirectors(t, r, http.MethodGet, path, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

func TestDirectorProfileHTTPMarksUpcomingOnlyOutsideCollection(t *testing.T) {
	w := serveDirectors(t, directorsRouter(&directorsStub{}), http.MethodGet, "/directors/5/profile", "")
	var p dto.DirectorProfile
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(p.Filmography) != 2 || !p.Filmography[0].Upcoming || p.Filmography[1].Upcoming ||
		p.Filmography[1].UserMovieID != 9 || p.Filmography[1].Rating != nil {
		t.Fatalf("profile %d %s", w.Code, w.Body.String())
	}
	if w := serveDirectors(t, directorsRouter(&directorsStub{}), http.MethodGet, "/directors/abc/profile", ""); w.Code != 404 {
		t.Fatalf("bad id: %d", w.Code)
	}
}

func TestPeopleSearchAndFollowHTTP(t *testing.T) {
	stub := &directorsStub{}
	r := directorsRouter(stub)
	w := serveDirectors(t, r, http.MethodGet, "/people/search?query=%20nolan%20", "")
	if w.Code != 200 || stub.query != "nolan" ||
		!strings.Contains(w.Body.String(), `"id":null`) || !strings.Contains(w.Body.String(), `"id":5`) {
		t.Fatalf("search %d %s (query %q)", w.Code, w.Body.String(), stub.query)
	}
	if w := serveDirectors(t, r, http.MethodPost, "/directors/follow-by-tmdb", `{"tmdb_id":137427}`); w.Code != 200 || stub.tmdbID != 137427 {
		t.Fatalf("follow by tmdb %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"tmdb_id":0}`, `{"tmdb_id":"x"}`, `{"tmdb_id":1,"extra":true}`} {
		if w := serveDirectors(t, r, http.MethodPost, "/directors/follow-by-tmdb", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
	if w := serveDirectors(t, r, http.MethodPut, "/directors/5/follow", ""); w.Code != http.StatusNoContent || len(stub.followed) != 1 {
		t.Fatalf("follow %d", w.Code)
	}
	stub.err = movie.ErrDirectorNotFound
	if w := serveDirectors(t, r, http.MethodPut, "/directors/6/follow", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown director %d", w.Code)
	}
}
