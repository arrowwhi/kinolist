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

func TestDetailsLoadsCastFromStaff(t *testing.T) {
	staff := `[{"staffId":7,"nameRu":"Дени","professionKey":"DIRECTOR"},
 {"staffId":1,"nameRu":"Тимоти Шаламе","nameEn":"Timothée Chalamet","description":"Пол Атрейдес","posterUrl":"https://kp/1.jpg","professionKey":"ACTOR"},
 {"staffId":2,"nameRu":"","nameEn":"Zendaya","description":"Чани","professionKey":"ACTOR"},
 {"staffId":1,"nameRu":"Тимоти Шаламе","description":"Пол (в детстве)","professionKey":"ACTOR"},
 {"staffId":3,"nameRu":"","nameEn":"","professionKey":"ACTOR"}`
	for i := 0; i < 12; i++ {
		staff += fmt.Sprintf(`,{"staffId":%d,"nameRu":"Актёр %d","professionKey":"ACTOR"}`, 100+i, i)
	}
	staff += `]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/staff" {
			_, _ = fmt.Fprint(w, staff)
			return
		}
		_, _ = fmt.Fprint(w, `{"kinopoiskId":10,"nameRu":"Фильм","type":"FILM"}`)
	}))
	defer server.Close()
	client := New(config.Kinopoisk{BaseURL: server.URL, APIKey: "test", Timeout: time.Second})
	got, err := client.Details(context.Background(), 10, movie.MediaMovie)
	if err != nil || !got.CastLoaded || len(got.Cast) != movie.MaxCast || len(got.Directors) != 1 {
		t.Fatalf("cast: %+v %v", got.Cast, err)
	}
	first, second := got.Cast[0], got.Cast[1]
	if first.KinopoiskID != 1 || first.TMDBID != 0 || first.Name != "Тимоти Шаламе" || first.OriginalName != "Timothée Chalamet" ||
		first.Character != "Пол Атрейдес / Пол (в детстве)" || first.ProfilePath != "https://kp/1.jpg" {
		t.Fatalf("first: %+v", first)
	}
	if second.Name != "Zendaya" || second.OriginalName != "Zendaya" || second.Character != "Чани" {
		t.Fatalf("second: %+v", second)
	}
}
