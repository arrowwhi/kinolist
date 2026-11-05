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

func TestDetailsUsesMovieAndAggregateTVDirectors(t *testing.T) {
	for _, tc := range []struct {
		mt      movie.MediaType
		append  string
		payload string
		loaded  bool
		count   int
	}{
		{movie.MediaMovie, "credits", `{"id":7,"credits":{"crew":[{"id":1,"name":"Дени","original_name":"Denis","job":"Director"},{"id":1,"name":"Дени","job":"Director"},{"id":2,"name":"Producer","job":"Producer"},{"id":3,"name":"Second","job":"Director"}]}}`, true, 2},
		{movie.MediaTV, "aggregate_credits", `{"id":7,"aggregate_credits":{"crew":[{"id":1,"name":"First","jobs":[{"job":"Director"},{"job":"Producer"}]},{"id":2,"name":"Creator","jobs":[{"job":"Creator"}]},{"id":3,"name":"Other season","jobs":[{"job":"Director"}]}]}}`, true, 2},
		{movie.MediaMovie, "credits", `{"id":7,"credits":{"crew":[]}}`, true, 0},
		{movie.MediaTV, "aggregate_credits", `{"id":7,"aggregate_credits":{"success":false,"status_code":34}}`, false, 0},
		{movie.MediaMovie, "credits", `{"id":7}`, false, 0},
	} {
		t.Run(string(tc.mt)+tc.payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("append_to_response") != tc.append {
					t.Errorf("wrong credits requested: %s", r.URL.RawQuery)
				}
				_, _ = fmt.Fprint(w, tc.payload)
			}))
			defer server.Close()
			client := New(config.TMDB{BaseURL: server.URL, APIKey: "test", Timeout: time.Second}, nil)
			got, err := client.Details(context.Background(), 7, tc.mt)
			if err != nil || got.DirectorsLoaded != tc.loaded || len(got.Directors) != tc.count {
				t.Fatalf("directors: %+v %v", got, err)
			}
			for _, d := range got.Directors {
				if d.TMDBID <= 0 || d.ID != 0 {
					t.Fatalf("preview identity: %+v", d)
				}
			}
		})
	}
}
