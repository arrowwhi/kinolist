package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/pkg/metrics"
	"github.com/arrowwhi/kinolist/internal/transport/http/handler"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

// Публичный роутер не отдаёт метрики: они живут на отдельном внутреннем порту.
func TestMetricsAreNotOnPublicRouter(t *testing.T) {
	m := metrics.New()
	router := NewRouter(RouterDeps{
		Config: &config.Config{}, Log: zap.NewNop(), Metrics: m,
		Health: handler.NewHealth(nil, "test"),
		ResolveUser: func(context.Context, middleware.Identity) (int64, error) {
			return 1, nil
		},
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	if strings.Contains(w.Body.String(), "kinolist_") {
		t.Fatalf("public router exposes metrics: %d", w.Code)
	}

	w = httptest.NewRecorder()
	NewMetricsHandler(m).ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "kinolist_worker_jobs_in_flight") {
		t.Fatalf("metrics handler: %d %s", w.Code, w.Body.String())
	}
}
