package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arrowwhi/infrastructure/observability/health"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/transport/http/handler"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

func newTestRouter(checks *health.Registry) http.Handler {
	return NewRouter(RouterDeps{
		Config:      &config.Config{},
		ServiceName: "kinolist-test",
		Health:      handler.NewHealth(checks, "test"),
		ResolveUser: func(context.Context, middleware.Identity) (int64, error) {
			return 1, nil
		},
	})
}

// Публичный роутер не отдаёт метрики: они живут на служебном порту инфраструктуры.
func TestMetricsAreNotOnPublicRouter(t *testing.T) {
	w := httptest.NewRecorder()
	newTestRouter(health.New()).ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))

	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "# TYPE") {
		t.Fatalf("public router exposes metrics: %d", w.Code)
	}
}

func TestReadyzUsesHealthRegistry(t *testing.T) {
	checks := health.New()
	router := newTestRouter(checks)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("readyz without checks: %d", w.Code)
	}

	checks.Register("postgres", func(context.Context) error { return errors.New("connection refused") })

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "postgres") {
		t.Fatalf("readyz with failing check: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "connection refused") {
		t.Fatalf("readyz leaks error text: %s", w.Body.String())
	}
}
