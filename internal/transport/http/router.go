package http

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/pkg/metrics"
	"github.com/arrowwhi/kinolist/internal/transport/http/handler"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

// RouterDeps — всё, что нужно роутеру. Собирается в app.
type RouterDeps struct {
	Config    *config.Config
	Log       *zap.Logger
	Metrics   *metrics.Metrics
	Movies    *handler.Movie
	Jobs      *handler.Enrichment
	Health    *handler.Health
	WebAssets fs.FS
	// ResolveUser превращает подтверждённую Telegram личность во внутренний
	// id пользователя, заводя его при первом обращении.
	ResolveUser func(context.Context, middleware.Identity) (int64, error)
	// TelegramWebhook монтируется только в режиме webhook.
	TelegramWebhook http.HandlerFunc
}

func NewRouter(d RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery)
	r.Use(middleware.Logger(d.Log))
	r.Use(middleware.Metrics(d.Metrics))

	// Служебные эндпоинты живут вне таймаута и вне авторизации:
	// проба готовности не должна зависеть от нагрузки на API.
	r.Get("/healthz", d.Health.Live)
	r.Get("/readyz", d.Health.Ready)
	r.Handle("/metrics", promhttp.HandlerFor(d.Metrics.Registry(), promhttp.HandlerOpts{}))

	if d.TelegramWebhook != nil {
		r.Post(d.Config.Telegram.WebhookPath, d.TelegramWebhook)
	}

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.Timeout(d.Config.HTTP.HandlerTimeout))
		api.Use(middleware.TelegramAuth(d.Config.Telegram.BotToken, d.ResolveUser))

		api.Get("/me", d.Movies.Me)
		api.Get("/genres", d.Movies.Genres)

		api.Route("/movies", func(m chi.Router) {
			m.Get("/", d.Movies.List)
			m.Get("/{id}", d.Movies.Get)
			m.Patch("/{id}", d.Movies.Update)
			m.Delete("/{id}", d.Movies.Delete)
		})

		api.Route("/jobs", func(j chi.Router) {
			j.Post("/", d.Jobs.Create)
			j.Get("/{id}", d.Jobs.Get)
			j.Post("/{id}/confirm", d.Jobs.Confirm)
			j.Post("/{id}/reject", d.Jobs.Reject)
		})
	})

	if d.WebAssets != nil {
		r.Handle("/*", spaHandler(d.WebAssets))
	}

	return r
}

// spaHandler отдаёт собранный Mini App и заворачивает неизвестные пути на
// index.html — иначе клиентские маршруты ломались бы при перезагрузке.
func spaHandler(assets fs.FS) http.Handler {
	files := http.FileServer(http.FS(assets))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(assets, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		// Хэшированные ассеты кэшируются надолго, index.html — никогда,
		// иначе после деплоя пользователь останется на старой версии.
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		files.ServeHTTP(w, r)
	})
}
