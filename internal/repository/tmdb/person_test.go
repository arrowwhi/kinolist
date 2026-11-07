package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

const personPayload = `{
 "id": 137427, "name": "Denis Villeneuve", "known_for_department": "Directing",
 "also_known_as": ["Дени Вильнёв", "Denis Villeneuve"],
 "biography": "", "birthday": "1967-10-03", "deathday": null,
 "place_of_birth": "Bécancour, Québec, Canada", "profile_path": "/p.jpg",
 "translations": {"translations": [
  {"iso_639_1": "en", "data": {"name": "", "biography": "Canadian director."}}
 ]},
 "combined_credits": {"crew": [
  {"id": 438631, "media_type": "movie", "job": "Director", "title": "Дюна", "original_title": "Dune", "release_date": "2021-09-15", "vote_average": 7.78, "vote_count": 12000},
  {"id": 438631, "media_type": "movie", "job": "Screenplay", "title": "Дюна", "release_date": "2021-09-15", "vote_count": 12000},
  {"id": 1, "media_type": "movie", "job": "Director", "title": "Короткий", "release_date": "1994-01-01", "vote_average": 9.5, "vote_count": 3},
  {"id": 2, "media_type": "movie", "job": "Director", "title": "Заброшенный", "release_date": "", "vote_count": 0},
  {"id": 3, "media_type": "movie", "job": "Director", "title": "Клип", "release_date": "2000-01-01", "video": true},
  {"id": 4, "media_type": "movie", "job": "Director", "title": "Дюна: Мессия", "release_date": "2099-12-18", "vote_count": 25},
  {"id": 77, "media_type": "tv", "job": "Director", "name": "Сериал", "original_name": "Show", "first_air_date": "2018-10-28", "episode_count": 2, "vote_average": 6.94, "vote_count": 60},
  {"id": 77, "media_type": "tv", "job": "Director", "name": "Сериал", "first_air_date": "2018-10-28", "episode_count": 6, "vote_count": 60},
  {"id": 78, "media_type": "tv", "job": "Creator", "name": "Чужой сериал", "first_air_date": "2020-01-01"}
 ]}
}`

func TestPersonDetailsFiltersDirectingCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/person/137427" || r.URL.Query().Get("append_to_response") != "combined_credits,translations" {
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = fmt.Fprint(w, personPayload)
	}))
	defer server.Close()
	client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)

	got, err := client.PersonDetails(context.Background(), 137427)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Дени Вильнёв" || got.NameSource != movie.NameSourceTMDBTranslation || got.OriginalName != "Denis Villeneuve" {
		t.Fatalf("name: %q %q %q", got.Name, got.NameSource, got.OriginalName)
	}
	if got.BirthYear() != 1967 || got.DeathDate != nil || got.Biography != "Canadian director." || got.BiographyLang != "en" {
		t.Fatalf("bio fields: %+v", got.Person)
	}
	byID := map[int32]movie.DirectorWork{}
	for _, w := range got.Works {
		byID[w.TMDBID] = w
	}
	if len(got.Works) != 4 {
		t.Fatalf("works = %d: %+v", len(got.Works), got.Works)
	}
	if dune := byID[438631]; dune.Rating == nil || *dune.Rating != 7.8 || dune.Year != 2021 || dune.OriginalTitle != "Dune" {
		t.Fatalf("dune: %+v", dune)
	}
	if short := byID[1]; short.Rating != nil {
		t.Fatalf("rating below vote threshold must be hidden: %+v", short)
	}
	if show := byID[77]; show.MediaType != movie.MediaTV || show.EpisodeCount == nil || *show.EpisodeCount != 6 || show.Title != "Сериал" {
		t.Fatalf("show collapsed wrong: %+v", show)
	}
	if !byID[4].Upcoming(time.Now()) {
		t.Fatal("future release must be upcoming")
	}
	if len(got.KnownFor) != 2 || got.KnownFor[0] != "Дюна" {
		t.Fatalf("known for: %v", got.KnownFor)
	}
}

func TestPersonDetailsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
	if _, err := client.PersonDetails(context.Background(), 5); !errors.Is(err, movie.ErrPersonNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchPeoplePutsDirectorsFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/person" || r.URL.Query().Get("query") != "nolan" {
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = fmt.Fprint(w, `{"results":[
 {"id":1,"name":"Actor Nolan","known_for_department":"Acting","known_for":[{"media_type":"movie","title":"X"}]},
 {"id":525,"name":"Christopher Nolan","known_for_department":"Directing","profile_path":"/n.jpg",
  "known_for":[{"media_type":"movie","title":"Начало"},{"media_type":"tv","name":"Сериал"}]},
 {"id":0,"name":"broken"}]}`)
	}))
	defer server.Close()
	client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
	got, err := client.SearchPeople(context.Background(), " nolan ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TMDBID != 525 || len(got[0].KnownFor) != 2 || got[0].KnownFor[1] != "Сериал" {
		t.Fatalf("search: %+v", got)
	}
	if _, err := client.SearchPeople(context.Background(), " "); !errors.Is(err, movie.ErrEmptyQuery) {
		t.Fatalf("empty query err = %v", err)
	}
}
