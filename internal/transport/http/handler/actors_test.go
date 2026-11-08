package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/converter"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	actorsuc "github.com/arrowwhi/kinolist/internal/usecase/actors"
)

type actorsStub struct {
	filter   movie.MyActorsFilter
	tmdbID   int32
	imported int32
	followed []int64
	removed  []int64
	query    string
	err      error
}

func (s *actorsStub) MyActors(_ context.Context, f movie.MyActorsFilter) (actorsuc.Page, error) {
	s.filter = f
	return actorsuc.Page{
		Items: []movie.ActorOverview{{Director: movie.Director{ID: 5, Name: "Тимоти Шаламе"}, FilmographyLoaded: true, WorkCount: 30}},
		Total: 1, Counts: movie.ActorScopeCounts{All: 1, Collection: 1},
	}, s.err
}

func (s *actorsStub) Profile(_ context.Context, _, id int64) (movie.ActorProfile, error) {
	release := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	episodes := int32(8)
	return movie.ActorProfile{
		DirectorOverview: movie.ActorOverview{Director: movie.Director{ID: id, Name: "Тимоти"}},
		Biography:        "Актёр.",
		Filmography: []movie.FilmographyItem{
			{DirectorWork: movie.DirectorWork{MediaType: movie.MediaMovie, TMDBID: 1, Title: "Будущее", ReleaseDate: &release}},
			{DirectorWork: movie.DirectorWork{MediaType: movie.MediaMovie, TMDBID: 2, Title: "Дюна", Year: 2021, Character: "Paul Atreides"}, UserMovieID: 9, Status: movie.StatusWatched},
			{DirectorWork: movie.DirectorWork{MediaType: movie.MediaTV, TMDBID: 3, Title: "Родина", Year: 2012, EpisodeCount: &episodes, Character: "Finn / Finn Walden"}},
		},
	}, s.err
}

func (s *actorsStub) Search(_ context.Context, _ int64, query string) ([]movie.PersonSearchResult, error) {
	s.query = query
	return []movie.PersonSearchResult{{Person: movie.Person{TMDBID: 1190668, Name: "Шаламе", KnownForDepartment: movie.ActingDepartment}, PersonID: 5}}, s.err
}

func (s *actorsStub) Import(_ context.Context, _ int64, tmdbID int32) (movie.ActorOverview, error) {
	s.imported = tmdbID
	return movie.ActorOverview{Director: movie.Director{ID: 5, TMDBID: tmdbID}}, s.err
}

func (s *actorsStub) FollowByTMDB(_ context.Context, _ int64, tmdbID int32) (movie.ActorOverview, error) {
	s.tmdbID = tmdbID
	return movie.ActorOverview{Director: movie.Director{ID: 5}, Followed: true}, s.err
}

func (s *actorsStub) Follow(_ context.Context, _, id int64) error {
	s.followed = append(s.followed, id)
	return s.err
}

func (s *actorsStub) Unfollow(_ context.Context, _, id int64) error {
	s.removed = append(s.removed, id)
	return s.err
}

func actorsRouter(directors *directorsStub, actors *actorsStub) http.Handler {
	d, a := NewDirectors(directors), NewActors(actors)
	r := chi.NewRouter()
	r.Use(middleware.TelegramAuth("test-token", func(context.Context, middleware.Identity) (int64, error) { return 7, nil }))
	r.Get("/my-actors", a.Mine)
	r.Get("/people/search", PeopleSearch(d, a))
	r.Post("/actors/import", a.Import)
	r.Post("/actors/follow-by-tmdb", a.FollowByTMDB)
	r.Get("/actors/{id}/profile", a.Profile)
	r.Put("/actors/{id}/follow", a.Follow)
	r.Delete("/actors/{id}/follow", a.Unfollow)
	return r
}

func TestMyActorsHTTPDefaultsAndValidation(t *testing.T) {
	stub := &actorsStub{}
	r := actorsRouter(&directorsStub{}, stub)
	w := serveDirectors(t, r, http.MethodGet, "/my-actors?scope=collection&sort=user_rating&query=Шал&limit=10&offset=5", "")
	var page dto.ActorOverviewList
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || stub.filter.UserID != 7 || stub.filter.Scope != movie.DirectorScopeCollection || !stub.filter.Desc ||
		stub.filter.Query != "Шал" || stub.filter.Limit != 10 || stub.filter.Offset != 5 ||
		page.Counts.Collection != 1 || page.Items[0].ID != 5 || page.Items[0].WorkCount != 30 || page.Items[0].KnownFor == nil {
		t.Fatalf("response %d %s filter %+v", w.Code, w.Body.String(), stub.filter)
	}
	serveDirectors(t, r, http.MethodGet, "/my-actors?sort=name&order=desc", "")
	if !stub.filter.Desc {
		t.Fatal("explicit order must win")
	}
	for _, path := range []string{"/my-actors?order=sideways", "/my-actors?limit=0", "/my-actors?offset=-1"} {
		if w := serveDirectors(t, r, http.MethodGet, path, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	stub.err = movie.ErrInvalidActorFilter
	if w := serveDirectors(t, r, http.MethodGet, "/my-actors?scope=everyone", ""); w.Code != http.StatusBadRequest ||
		!strings.Contains(w.Body.String(), "invalid_actor_filter") {
		t.Fatalf("invalid filter: %d %s", w.Code, w.Body.String())
	}
}

func TestActorProfileHTTPHasCharacters(t *testing.T) {
	r := actorsRouter(&directorsStub{}, &actorsStub{})
	w := serveDirectors(t, r, http.MethodGet, "/actors/5/profile", "")
	var p dto.ActorProfile
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || p.ID != 5 || p.Biography != "Актёр." || len(p.Filmography) != 3 ||
		!p.Filmography[0].Upcoming || p.Filmography[1].Upcoming || p.Filmography[1].Character != "Paul Atreides" ||
		p.Filmography[2].EpisodeCount == nil || *p.Filmography[2].EpisodeCount != 8 || p.Filmography[2].Character != "Finn / Finn Walden" {
		t.Fatalf("profile %d %s", w.Code, w.Body.String())
	}
	// character обязателен даже пустым.
	if !strings.Contains(w.Body.String(), `"character":""`) {
		t.Fatalf("empty character must be present: %s", w.Body.String())
	}
	if w := serveDirectors(t, r, http.MethodGet, "/actors/abc/profile", ""); w.Code != http.StatusNotFound ||
		!strings.Contains(w.Body.String(), "actor_not_found") {
		t.Fatalf("bad id: %d %s", w.Code, w.Body.String())
	}
}

func TestPeopleSearchHTTPDispatchesByRole(t *testing.T) {
	directors, actors := &directorsStub{}, &actorsStub{}
	r := actorsRouter(directors, actors)
	if w := serveDirectors(t, r, http.MethodGet, "/people/search?query=nolan", ""); w.Code != 200 || directors.query != "nolan" || actors.query != "" {
		t.Fatalf("default role must search directors: %d", w.Code)
	}
	if w := serveDirectors(t, r, http.MethodGet, "/people/search?query=%20chalamet%20&role=actor", ""); w.Code != 200 ||
		actors.query != "chalamet" || !strings.Contains(w.Body.String(), `"id":5`) {
		t.Fatalf("actor search %d %s", w.Code, w.Body.String())
	}
	if w := serveDirectors(t, r, http.MethodGet, "/people/search?query=x&role=director", ""); w.Code != 200 || directors.query != "x" {
		t.Fatalf("explicit director role: %d", w.Code)
	}
	if w := serveDirectors(t, r, http.MethodGet, "/people/search?query=x&role=writer", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown role: %d", w.Code)
	}
}

func TestActorImportFollowHTTP(t *testing.T) {
	stub := &actorsStub{}
	r := actorsRouter(&directorsStub{}, stub)
	if w := serveDirectors(t, r, http.MethodPost, "/actors/import", `{"tmdb_id":1190668}`); w.Code != 200 || stub.imported != 1190668 ||
		!strings.Contains(w.Body.String(), `"tmdb_id":1190668`) {
		t.Fatalf("import %d %s", w.Code, w.Body.String())
	}
	if w := serveDirectors(t, r, http.MethodPost, "/actors/follow-by-tmdb", `{"tmdb_id":1190668}`); w.Code != 200 || stub.tmdbID != 1190668 ||
		!strings.Contains(w.Body.String(), `"followed":true`) {
		t.Fatalf("follow by tmdb %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"tmdb_id":0}`, `{"tmdb_id":"x"}`, `{"tmdb_id":1,"extra":true}`} {
		if w := serveDirectors(t, r, http.MethodPost, "/actors/import", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
	if w := serveDirectors(t, r, http.MethodPut, "/actors/5/follow", ""); w.Code != http.StatusNoContent || len(stub.followed) != 1 {
		t.Fatalf("follow %d", w.Code)
	}
	if w := serveDirectors(t, r, http.MethodDelete, "/actors/5/follow", ""); w.Code != http.StatusNoContent || len(stub.removed) != 1 {
		t.Fatalf("unfollow %d", w.Code)
	}
	stub.err = movie.ErrActorNotFound
	if w := serveDirectors(t, r, http.MethodPut, "/actors/6/follow", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown actor %d", w.Code)
	}
	stub.err = movie.ErrPersonNotFound
	if w := serveDirectors(t, r, http.MethodPost, "/actors/import", `{"tmdb_id":42}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown person %d", w.Code)
	}
}

func TestMovieAndCandidateAlwaysHaveCast(t *testing.T) {
	body, err := json.Marshal(converter.Movie(movie.Movie{ID: 1, Title: "x"}))
	if err != nil || !strings.Contains(string(body), `"cast":[]`) {
		t.Fatalf("empty cast must be an array: %s %v", body, err)
	}
	c := converter.Candidate(movie.Candidate{Cast: []movie.CastMember{
		{TMDBID: 1190668, Name: "Тимоти Шаламе", OriginalName: "Timothée Chalamet", Character: "Paul Atreides", ProfilePath: "/t.jpg"},
		{KinopoiskID: 9, Name: "Актёр", ProfilePath: "https://kinopoiskapiunofficial.tech/images/actor_posters/kp/9.jpg"},
	}})
	if len(c.Cast) != 2 || c.Cast[0].ID != 0 || c.Cast[0].PhotoURL != "https://image.tmdb.org/t/p/w500/t.jpg" ||
		c.Cast[1].PhotoURL != "https://kinopoiskapiunofficial.tech/images/actor_posters/kp/9.jpg" || c.Cast[1].Character != "" {
		t.Fatalf("cast: %+v", c.Cast)
	}
	body, _ = json.Marshal(c)
	if strings.Contains(string(body), `"id":0`) || !strings.Contains(string(body), `"character":""`) {
		t.Fatalf("preview cast json: %s", body)
	}
}
