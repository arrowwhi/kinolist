package kinopoisk

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/config"
)

func TestResolveRussianNameChecksBirthYear(t *testing.T) {
	staffCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/persons":
			if r.URL.Query().Get("name") != "Denis Villeneuve" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprint(w, `{"items":[
 {"kinopoiskId":10,"nameRu":"Дени Вильнёв (тёзка)","nameEn":"Denis Villeneuve"},
 {"kinopoiskId":11,"nameRu":"Дени Вильнёв","nameEn":"Dénis  Villeneuve"},
 {"kinopoiskId":12,"nameRu":"Другой","nameEn":"Someone Else"}]}`)
		case "/api/v1/staff/10":
			staffCalls++
			_, _ = fmt.Fprint(w, `{"nameRu":"Дени Вильнёв (тёзка)","birthday":"1980-01-01"}`)
		case "/api/v1/staff/11":
			staffCalls++
			_, _ = fmt.Fprint(w, `{"nameRu":"Дени Вильнёв","birthday":"1967-10-03"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := New(config.Kinopoisk{BaseURL: server.URL, APIKey: "k", Timeout: time.Second})

	name, id, ok, err := client.ResolveRussianName(context.Background(), "Denis Villeneuve", 1967)
	if err != nil || !ok || id != 11 || name != "Дени Вильнёв" || staffCalls != 2 {
		t.Fatalf("got %q %d %v %v (staff calls %d)", name, id, ok, err, staffCalls)
	}
	if _, _, ok, err := client.ResolveRussianName(context.Background(), "Denis Villeneuve", 1950); ok || err != nil {
		t.Fatalf("wrong year must not match: %v %v", ok, err)
	}
	// Без года два однофамильца неразличимы.
	if _, _, ok, _ := client.ResolveRussianName(context.Background(), "Denis Villeneuve", 0); ok {
		t.Fatal("ambiguous name without birth year must not match")
	}
}
