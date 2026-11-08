package http

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/infrastructure/observability/metrics"
	"github.com/arrowwhi/infrastructure/observability/tracing"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/transport/http/handler"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

// RouterDeps — всё, что нужно роутеру. Собирается в app.
type RouterDeps struct {
	Config *config.Config
	// ServiceName — имя сервиса в метриках и трейсах HTTP.
	ServiceName string
	Movies      *handler.Movie
	Jobs        *handler.Enrichment
	Directors   *handler.Directors
	Actors      *handler.Actors
	Health      *handler.Health
	WebAssets   fs.FS
	// ResolveUser превращает подтверждённую Telegram личность во внутренний
	// id пользователя, заводя его при первом обращении.
	ResolveUser func(context.Context, middleware.Identity) (int64, error)
	// TelegramWebhook монтируется только в режиме webhook.
	TelegramWebhook http.Handler
}

func NewRouter(d RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Трейсинг первым: спан запроса уже открыт, когда пишутся логи, поэтому
	// у записей есть trace_id. Метрики (http.server.*) размечены шаблоном маршрута chi.
	r.Use(tracing.Middleware(d.ServiceName))
	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery)
	r.Use(middleware.Logger)
	r.Use(metrics.Middleware(d.ServiceName))

	// Служебные эндпоинты живут вне таймаута и вне авторизации.
	// /metrics здесь нет: он на служебном порту инфраструктуры (METRICS_ADDR),
	// который не публикуется наружу.
	r.Get("/healthz", d.Health.Live)
	r.Get("/readyz", d.Health.Ready)

	if d.TelegramWebhook != nil {
		r.Method(http.MethodPost, d.Config.Telegram.WebhookPath, d.TelegramWebhook)
	}

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.Timeout(d.Config.HTTP.HandlerTimeout))
		api.Use(middleware.TelegramAuth(d.Config.Telegram.BotToken, d.ResolveUser))

		api.Get("/me", d.Movies.Me)
		api.Get("/statistics", d.Movies.Statistics)
		api.Get("/genres", d.Movies.Genres)
		api.Get("/directors", d.Movies.ListDirectors)
		api.Get("/directors/{id}", d.Movies.GetDirector)
		// Раздел «Мои режиссёры»: подписки и полная фильмография из каталога.
		api.Get("/my-directors", d.Directors.Mine)
		api.Get("/people/search", handler.PeopleSearch(d.Directors, d.Actors))
		api.Post("/directors/import", d.Directors.Import)
		api.Post("/directors/follow-by-tmdb", d.Directors.FollowByTMDB)
		api.Get("/directors/{id}/profile", d.Directors.Profile)
		api.Put("/directors/{id}/follow", d.Directors.Follow)
		api.Delete("/directors/{id}/follow", d.Directors.Unfollow)
		// Раздел «Актёры»: устроен так же, справочник отдельный.
		api.Get("/my-actors", d.Actors.Mine)
		api.Post("/actors/import", d.Actors.Import)
		api.Post("/actors/follow-by-tmdb", d.Actors.FollowByTMDB)
		api.Get("/actors/{id}/profile", d.Actors.Profile)
		api.Put("/actors/{id}/follow", d.Actors.Follow)
		api.Delete("/actors/{id}/follow", d.Actors.Unfollow)

		// Карточка работы прямо из каталога: фильмография режиссёра.
		api.Get("/catalog/{provider}/{media_type}/{external_id}", d.Jobs.CatalogTitle)

		api.Route("/movies", func(m chi.Router) {
			m.Get("/", d.Movies.List)
			m.Post("/", d.Jobs.AddFromCatalog)
			m.Get("/{id}", d.Movies.Get)
			m.Patch("/{id}", d.Movies.Update)
			m.Delete("/{id}", d.Movies.Delete)
		})

		api.Route("/jobs", func(j chi.Router) {
			j.Post("/", d.Jobs.Create)
			j.Get("/{id}", d.Jobs.Get)
			j.Get("/{id}/candidates/{provider}/{external_id}", d.Jobs.CandidateDetails) // legacy, rejects ambiguous IDs
			j.Get("/{id}/candidates/{provider}/{media_type}/{external_id}", d.Jobs.CandidateDetails)
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
