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

func TestDetailsLoadsOnlyDirectorsAndSurvivesStaffFailure(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
		loaded bool
		count  int
	}{
		{`[{"staffId":7,"nameRu":"Дени","nameEn":"Denis","professionKey":"DIRECTOR"},{"staffId":7,"nameRu":"Дени","professionKey":"DIRECTOR"},{"staffId":8,"nameEn":"Other","professionKey":"DIRECTOR"},{"staffId":9,"nameRu":"Актёр","professionKey":"ACTOR"}]`, 200, true, 2},
		{`[]`, 200, true, 0},
		{`null`, 200, false, 0},
		{`{"error":"Unavailable"}`, 503, false, 0},
	} {
		t.Run(tc.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/staff" {
					if r.URL.Query().Get("filmId") != "10" {
						t.Errorf("filmId: %s", r.URL.RawQuery)
					}
					w.WriteHeader(tc.status)
					_, _ = fmt.Fprint(w, tc.body)
					return
				}
				_, _ = fmt.Fprint(w, `{"kinopoiskId":10,"nameRu":"Фильм","type":"FILM"}`)
			}))
			defer server.Close()
			client := New(config.Kinopoisk{BaseURL: server.URL, APIKey: "test", Timeout: time.Second})
			got, err := client.Details(context.Background(), 10, movie.MediaMovie)
			if err != nil || got.DirectorsLoaded != tc.loaded || len(got.Directors) != tc.count {
				t.Fatalf("directors: %+v %v", got, err)
			}
		})
	}
}
