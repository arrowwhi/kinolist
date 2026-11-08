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

func TestMixedSearchAndTypedDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/multi":
			_, _ = fmt.Fprint(w, `{"results":[{"id":7,"media_type":"person","name":"Person"},{"id":7,"media_type":"movie","title":"Shared","release_date":"2022-01-01"},{"id":7,"media_type":"tv","name":"Shared","original_name":"Original series","first_air_date":"2022-02-01"}]}`)
		case "/movie/7":
			_, _ = fmt.Fprint(w, `{"id":7,"title":"Shared","runtime":120}`)
		case "/tv/7":
			_, _ = fmt.Fprint(w, `{"id":7,"name":"Shared","first_air_date":"2022-02-01","number_of_seasons":3,"seasons":[{"season_number":0},{"season_number":1},{"season_number":2}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
	ctx := context.Background()
	candidates, err := client.Search(ctx, "Shared", 2022)
	if err != nil || len(candidates) != 2 {
		t.Fatalf("search: %+v %v", candidates, err)
	}
	if candidates[0].MediaType != movie.MediaMovie || candidates[1].MediaType != movie.MediaTV || candidates[1].Title != "Shared" || candidates[1].Year != 2022 {
		t.Fatalf("normalization: %+v", candidates)
	}
	series, err := client.Details(ctx, 7, movie.MediaTV)
	if err != nil || series.SeasonCount == nil || *series.SeasonCount != 2 || series.RuntimeMin != 0 {
		t.Fatalf("series: %+v %v", series, err)
	}
	// У сериала датой выхода считается первый эфир.
	if series.ReleaseDate == nil || series.ReleaseDate.Format(time.DateOnly) != "2022-02-01" {
		t.Fatalf("series release date: %v", series.ReleaseDate)
	}
	film, err := client.Details(ctx, 7, movie.MediaMovie)
	if err != nil || film.RuntimeMin != 120 || film.SeasonCount != nil || film.MediaType != movie.MediaMovie {
		t.Fatalf("film: %+v %v", film, err)
	}
	if film.ReleaseDate != nil {
		t.Fatalf("film without release_date must stay nil: %v", film.ReleaseDate)
	}
}

func TestUnknownSeasonCountStaysNull(t *testing.T) {
	c := (detailsResponse{result: result{ID: 1, MediaType: movie.MediaTV}}).toCandidate()
	if c.SeasonCount != nil || c.RuntimeMin != 0 {
		t.Fatalf("unknown seasons: %+v", c)
	}
}
