package kinopoisk

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

func TestSeriesSeasonsExcludeSpecials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.1/films/search-by-keyword":
			_, _ = fmt.Fprint(w, `{"films":[{"filmId":1,"nameRu":"Series","type":"TV_SERIES","year":"2022-2025"},{"filmId":2,"nameRu":"Film","type":"FILM","year":"2022"}]}`)
		case "/api/v2.2/films/1":
			_, _ = fmt.Fprint(w, `{"kinopoiskId":1,"nameRu":"Series","type":"TV_SERIES","year":2022,"filmLength":50}`)
		case "/api/v2.2/films/1/seasons":
			_, _ = fmt.Fprint(w, `{"items":[{"number":0},{"number":1},{"number":2},{"number":2}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := New(config.Kinopoisk{BaseURL: server.URL, APIKey: "test", Timeout: time.Second})
	got, err := c.Search(context.Background(), "Series", 0)
	if err != nil || len(got) != 2 || got[0].MediaType != movie.MediaTV || got[1].MediaType != movie.MediaMovie {
		t.Fatalf("search: %+v %v", got, err)
	}
	d, err := c.Details(context.Background(), 1, movie.MediaTV)
	if err != nil || d.SeasonCount == nil || *d.SeasonCount != 2 || d.RuntimeMin != 0 {
		t.Fatalf("details: %+v %v", d, err)
	}
}
