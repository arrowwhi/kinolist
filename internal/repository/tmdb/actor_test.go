package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestDetailsLoadsMovieCastAndTVAggregateCast(t *testing.T) {
	movieCast := `{"id":7,"credits":{"crew":[],"cast":[
  {"id":2,"name":"Zendaya","character":"Chani","order":1},
  {"id":1,"name":"Timothée Chalamet","original_name":"Timothée Chalamet","character":"Paul Atreides","order":0,"profile_path":"/t.jpg"},
  {"id":1,"name":"Timothée Chalamet","character":"Paul (voice)","order":30},
  {"id":0,"name":"broken","order":2},
  {"id":3,"name":"","original_name":"","order":3}`
	for i := 0; i < 12; i++ {
		movieCast += fmt.Sprintf(`,{"id":%d,"name":"Extra %d","order":%d}`, 100+i, i, 10+i)
	}
	movieCast += `]}}`
	for _, tc := range []struct {
		name      string
		mt        movie.MediaType
		payload   string
		loaded    bool
		count     int
		first     string
		character string
	}{
		{"movie", movie.MediaMovie, movieCast, true, 10, "Timothée Chalamet", "Paul Atreides / Paul (voice)"},
		{"tv", movie.MediaTV, `{"id":7,"aggregate_credits":{"crew":[],"cast":[
  {"id":5,"name":"Bryan Cranston","order":0,"roles":[{"character":"Walter White","episode_count":62},{"character":"Heisenberg","episode_count":3}]},
  {"id":6,"name":"Aaron Paul","order":1,"roles":[{"character":"","episode_count":62}]}]}}`, true, 2, "Bryan Cranston", "Walter White / Heisenberg"},
		{"empty", movie.MediaMovie, `{"id":7,"credits":{"crew":[],"cast":[]}}`, true, 0, "", ""},
		{"missing", movie.MediaMovie, `{"id":7,"credits":{"crew":[]}}`, false, 0, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, tc.payload)
			}))
			defer server.Close()
			client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
			got, err := client.Details(context.Background(), 7, tc.mt)
			if err != nil || got.CastLoaded != tc.loaded || len(got.Cast) != tc.count {
				t.Fatalf("cast: %+v %v", got.Cast, err)
			}
			if tc.count == 0 {
				return
			}
			first := got.Cast[0]
			if first.Name != tc.first || first.Character != tc.character || first.ID != 0 || first.OriginalName == "" {
				t.Fatalf("first: %+v", first)
			}
			if tc.name == "movie" && (got.Cast[1].TMDBID != 2 || first.ProfilePath != "/t.jpg" || got.Cast[9].Name != "Extra 7") {
				t.Fatalf("credit order: %+v", got.Cast)
			}
		})
	}
}

const actorPayload = `{
 "id": 1190668, "name": "Timothée Chalamet", "known_for_department": "Acting", "birthday": "1995-12-27",
 "combined_credits": {"cast": [
  {"id": 438631, "media_type": "movie", "character": "Paul Atreides", "title": "Дюна", "release_date": "2021-09-15", "vote_average": 7.8, "vote_count": 12000},
  {"id": 438631, "media_type": "movie", "character": "Paul (young)", "title": "Дюна", "release_date": "2021-09-15", "vote_count": 12000},
  {"id": 10, "media_type": "movie", "character": "Himself", "title": "Документалка", "release_date": "2020-01-01"},
  {"id": 11, "media_type": "tv", "character": "Self - Guest", "name": "Шоу", "first_air_date": "2019-01-01"},
  {"id": 12, "media_type": "tv", "character": "Guest", "name": "Вечернее шоу", "first_air_date": "2019-01-01", "genre_ids": [35, 10767]},
  {"id": 13, "media_type": "tv", "character": "Reporter", "name": "Новости", "first_air_date": "2019-01-01", "genre_ids": [10763]},
  {"id": 14, "media_type": "tv", "character": "Contestant", "name": "Реалити", "first_air_date": "2019-01-01", "genre_ids": [10764]},
  {"id": 15, "media_type": "movie", "character": "Kid", "title": "Клип", "release_date": "2010-01-01", "video": true},
  {"id": 77, "media_type": "tv", "character": "Finn Walden", "name": "Родина", "first_air_date": "2011-10-02", "episode_count": 8, "genre_ids": [18]},
  {"id": 77, "media_type": "tv", "character": "Finn", "name": "Родина", "first_air_date": "2011-10-02", "episode_count": 2},
  {"id": 78, "media_type": "tv", "character": "", "name": "Без персонажа", "first_air_date": "2015-01-01"}
 ],
 "crew": [{"id": 1, "media_type": "movie", "job": "Director", "title": "Дебют", "release_date": "2030-01-01"}]}
}`

func TestPersonDetailsFiltersActingRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, actorPayload)
	}))
	defer server.Close()
	client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
	got, err := client.PersonDetails(context.Background(), 1190668)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Works) != 1 || got.Works[0].Character != "" {
		t.Fatalf("directing works: %+v", got.Works)
	}
	byID := map[int32]movie.ActorWork{}
	for _, w := range got.Roles {
		byID[w.TMDBID] = w
	}
	if len(got.Roles) != 3 {
		t.Fatalf("roles = %d: %+v", len(got.Roles), got.Roles)
	}
	if dune := byID[438631]; dune.Character != "Paul Atreides / Paul (young)" || dune.Rating == nil || *dune.Rating != 7.8 {
		t.Fatalf("dune: %+v", dune)
	}
	if show := byID[77]; show.MediaType != movie.MediaTV || show.EpisodeCount == nil || *show.EpisodeCount != 8 || show.Character != "Finn Walden / Finn" {
		t.Fatalf("show collapsed wrong: %+v", show)
	}
	if blank, ok := byID[78]; !ok || blank.Character != "" {
		t.Fatalf("role without character must stay: %+v", blank)
	}
}

func TestPlaysSelf(t *testing.T) {
	for character, want := range map[string]bool{
		"Self": true, "Himself": true, "herself - host": true, "Himself (archive footage)": true,
		"(Self)": true, "играет самого себя": true, "Themselves": true,
		"Selfridge": false, "Paul Atreides": false, "": false, "Self-Made Man": false,
	} {
		if got := playsSelf(character); got != want {
			t.Errorf("playsSelf(%q) = %v", character, got)
		}
	}
}

func TestSearchPeopleByDepartmentPutsActorsFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"results":[
 {"id":525,"name":"Christopher Nolan","known_for_department":"Directing"},
 {"id":1,"name":"Actor One","known_for_department":"Acting"},
 {"id":2,"name":"Writer","known_for_department":"Writing"},
 {"id":3,"name":"Actor Two","known_for_department":"Acting"}]}`)
	}))
	defer server.Close()
	client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
	got, err := client.SearchPeopleByDepartment(context.Background(), "x", movie.ActingDepartment)
	if err != nil || len(got) != 4 || got[0].TMDBID != 1 || got[1].TMDBID != 3 || got[2].TMDBID != 525 || got[3].TMDBID != 2 {
		t.Fatalf("order: %+v %v", got, err)
	}
}
