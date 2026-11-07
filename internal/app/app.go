// Package app собирает зависимости вручную и запускает их на инфраструктуре
// github.com/arrowwhi/infrastructure: пакет telegram инфраструктуры отвечает за логи, метрики,
// трейсинг, служебный порт и порядок остановки, здесь — только wiring kinolist.
package app

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/arrowwhi/infrastructure/observability/logger"
	"github.com/arrowwhi/infrastructure/observability/metrics"
	"github.com/arrowwhi/infrastructure/observability/tracing"
	tg "github.com/arrowwhi/infrastructure/telegram"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	kllog "github.com/arrowwhi/kinolist/internal/pkg/logger"
	kmetrics "github.com/arrowwhi/kinolist/internal/pkg/metrics"
	"github.com/arrowwhi/kinolist/internal/repository/instagram"
	"github.com/arrowwhi/kinolist/internal/repository/kinopoisk"
	"github.com/arrowwhi/kinolist/internal/repository/llm"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
	"github.com/arrowwhi/kinolist/internal/repository/tmdb"
	transporthttp "github.com/arrowwhi/kinolist/internal/transport/http"
	"github.com/arrowwhi/kinolist/internal/transport/http/handler"
	httpmiddleware "github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	"github.com/arrowwhi/kinolist/internal/transport/telegram"
	"github.com/arrowwhi/kinolist/internal/transport/worker"
	collectionuc "github.com/arrowwhi/kinolist/internal/usecase/collection"
	directorsuc "github.com/arrowwhi/kinolist/internal/usecase/directors"
	enrichmentuc "github.com/arrowwhi/kinolist/internal/usecase/enrichment"
	"github.com/arrowwhi/kinolist/pkg/transactions"
	"github.com/arrowwhi/kinolist/web"
)

const serviceName = "kinolist"

// Значения прошиваются линкером через -ldflags -X.
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func Version() string {
	return fmt.Sprintf("%s (%s, built %s)", version, commit, buildTime)
}

// Run поднимает сервис и блокируется до сигнала остановки.
func Run(ctx context.Context, envFile string) error {
	cfg, err := config.Load(envFile)
	if err != nil {
		return err
	}

	a, err := tg.NewApp(ctx, infraConfig(cfg))
	if err != nil {
		return err
	}

	kllog.Init()

	logger.Info(ctx).
		Str("commit", commit).
		Str("telegram_mode", cfg.Telegram.Mode).
		Bool("tracing", cfg.Tracing.Enabled).
		Msg("starting kinolist")

	// --- инфраструктура ---
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		_ = a.Shutdown(ctx)
		return err
	}
	a.OnShutdown(func(context.Context) error {
		pool.Close()
		return nil
	})
	a.Health().Register("postgres", pool.Healthcheck)

	m, err := kmetrics.New()
	if err != nil {
		_ = a.Shutdown(ctx)
		return err
	}

	txManager := transactions.NewManager(pool)

	// --- репозитории ---
	users := postgres.NewUserRepository(txManager)
	movies := postgres.NewMovieRepository(txManager)
	jobs := postgres.NewJobRepository(txManager)
	cache := postgres.NewCacheRepository(txManager)

	primaryCatalog := tmdb.New(cfg.TMDB, cache)
	fetcher := instagram.New(cfg.Instagram)
	extractor := llm.New(cfg.LLM)

	enrichDeps := enrichmentuc.Deps{
		Jobs: jobs, Movies: movies, Fetcher: fetcher, Extractor: extractor,
		Primary: primaryCatalog, Tx: txManager,
		MaxAttempts: cfg.Worker.MaxAttempts, RetryDelay: 30 * time.Second,
	}
	directorDeps := directorsuc.Deps{
		Directors: postgres.NewDirectorRepository(txManager), Catalog: primaryCatalog, Tx: txManager,
		WorksTTL: cfg.Directors.WorksTTL,
	}
	if cfg.Kinopoisk.Enabled() {
		kinopoiskClient := kinopoisk.New(cfg.Kinopoisk)
		enrichDeps.Fallback = kinopoiskClient
		directorDeps.Names = kinopoiskClient
	} else {
		logger.Warn(ctx).Msg("kinopoisk fallback is disabled: KINOPOISK_API_KEY is not set")
	}
	if !fetcher.Enabled() {
		logger.Warn(ctx).Msg("instagram scraper is disabled: reel links will be rejected, only titles work")
	}
	if !extractor.Enabled() {
		logger.Warn(ctx).Msg("title extractor is disabled: reel links will be rejected, only titles work")
	}

	// --- usecase ---
	collectionUC := collectionuc.New(movies, users)
	enrichmentUC := enrichmentuc.New(enrichDeps)
	directorsUC := directorsuc.New(directorDeps)

	// --- telegram ---
	bot, err := tg.New(botConfig(cfg),
		tg.WithErrorHandler(tg.ReplyOnError("⚠️ Что-то пошло не так. Попробуйте ещё раз.")))
	if err != nil {
		_ = a.Shutdown(ctx)
		return err
	}

	handlers := telegram.NewBot(enrichmentUC, collectionUC, webAppURL(cfg))
	handlers.Register(bot)

	if err := handlers.SetupMenuButton(ctx, bot.API()); err != nil {
		// Кнопка меню — украшение; без неё бот полностью работоспособен.
		logger.Warn(ctx).Err(err).Msg("set chat menu button failed")
	}

	notifier := telegram.NewNotifier(bot.API())

	// --- http ---
	health := handler.NewHealth(a.Health(), version)

	assets, err := web.Assets()
	if err != nil {
		logger.Warn(ctx).Err(err).Msg("mini app assets are not embedded, serving api only")
		assets = nil
	}

	routerDeps := transporthttp.RouterDeps{
		Config:      cfg,
		ServiceName: serviceName,
		Movies:      handler.NewMovie(collectionUC),
		Jobs:        handler.NewEnrichment(enrichmentUC),
		Directors:   handler.NewDirectors(directorsUC),
		Health:      health,
		WebAssets:   assets,
		ResolveUser: resolveUserFunc(collectionUC),
	}
	if cfg.Telegram.IsWebhook() {
		// Обновления приходят на публичный HTTP-сервер (за Caddy), бот только
		// регистрирует webhook и обрабатывает очередь.
		routerDeps.TelegramWebhook = bot.WebhookHandler()
	}

	server := transporthttp.NewServer(cfg.HTTP, transporthttp.NewRouter(routerDeps))

	// --- запуск ---
	return a.Run(ctx,
		httpComponent(server, health, cfg),
		worker.New(enrichmentUC, notifier, m, cfg.Worker),
		worker.NewDirectorSync(directorsUC, cfg.Directors.SyncInterval, cfg.Directors.SyncBatch),
		bot,
	)
}

// readinessDrainDelay — сколько ждать между переводом /readyz в ошибку и
// закрытием сервера.
const readinessDrainDelay = 2 * time.Second

// httpComponent запускает публичный HTTP-сервер. Порядок остановки: /readyz
// начинает отвечать ошибкой, балансировщик уводит трафик, и только потом
// закрывается приём запросов.
func httpComponent(server *transporthttp.Server, health *handler.Health, cfg *config.Config) tg.ComponentFunc {
	return func(ctx context.Context) error {
		errCh := make(chan error, 1)

		go func() {
			logger.Info(ctx).Str("addr", cfg.HTTP.Addr).Msg("http server listening")
			errCh <- server.Run()
		}()

		select {
		case err := <-errCh:
			return err
		case <-ctx.Done():
		}

		health.SetReady(false)

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.App.ShutdownTimeout)
		defer cancel()

		// Пауза даёт балансировщику увидеть новый статус readiness
		// прежде, чем сервер перестанет принимать соединения.
		select {
		case <-time.After(readinessDrainDelay):
		case <-shutdownCtx.Done():
		}

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		logger.Info(ctx).Msg("http server stopped")

		return <-errCh
	}
}

// infraConfig переводит конфиг kinolist в конфиг инфраструктуры. Переменные
// окружения kinolist остаются прежними, поэтому деплой менять не нужно.
func infraConfig(cfg *config.Config) tg.AppConfig {
	ac := tg.AppConfig{
		ServiceName:     serviceName,
		Version:         version,
		Environment:     cfg.App.Env,
		ShutdownTimeout: cfg.App.ShutdownTimeout,
	}

	ac.Logger.Level = cfg.App.LogLevel
	ac.Logger.PrettyOutput = !cfg.App.IsProd()
	ac.Logger.Masking = true

	ac.Metrics = metrics.DefaultConfig() // Prometheus: /metrics на служебном порту

	ac.Tracing = tracing.DefaultConfig()
	ac.Tracing.Enabled = cfg.Tracing.Enabled
	ac.Tracing.Exporter = cfg.Tracing.Exporter
	ac.Tracing.OTLPEndpoint = cfg.Tracing.Endpoint
	ac.Tracing.OTLPInsecure = cfg.Tracing.Insecure
	ac.Tracing.SamplingRate = cfg.Tracing.SamplingRate

	// Служебный порт (/metrics, /healthz, /ready) — на METRICS_ADDR, который не
	// публикуется наружу. Пустое значение выключает его.
	if host, port, err := net.SplitHostPort(cfg.HTTP.MetricsAddr); err == nil && port != "" {
		ac.Admin = tg.AdminConfig{Host: host, Port: port, Enabled: true}
	}

	return ac
}

// botConfig собирает настройки бота инфраструктуры из конфига kinolist.
func botConfig(cfg *config.Config) tg.Config {
	bc := tg.DefaultConfig(cfg.Telegram.BotToken)
	bc.API.BaseURL = cfg.Telegram.APIURL
	bc.Mode = cfg.Telegram.Mode
	bc.PollTimeout = cfg.Telegram.PollTimeout
	bc.Workers = cfg.Telegram.Workers

	bc.Webhook.URL = cfg.HTTP.PublicURL + cfg.Telegram.WebhookPath
	bc.Webhook.Secret = cfg.Telegram.WebhookSecret
	bc.Webhook.Path = cfg.Telegram.WebhookPath
	// Встроенный webhook-сервер не нужен: обработчик смонтирован на публичный роутер.
	bc.Webhook.Port = ""
	// Апдейты, накопившиеся пока сервис лежал, чаще всего уже неактуальны.
	bc.Webhook.DropPendingUpdates = true

	return bc
}

func webAppURL(cfg *config.Config) string {
	if cfg.HTTP.PublicURL == "" {
		return ""
	}
	return cfg.HTTP.PublicURL + "/"
}

// resolveUserFunc связывает подтверждённую личность Telegram с внутренним
// пользователем: middleware не должен знать про usecase напрямую.
func resolveUserFunc(uc *collectionuc.UseCase) func(context.Context, httpmiddleware.Identity) (int64, error) {
	return func(ctx context.Context, id httpmiddleware.Identity) (int64, error) {
		u, err := uc.ResolveUser(ctx, user.User{
			TelegramID:   id.TelegramID,
			Username:     id.Username,
			FirstName:    id.FirstName,
			LanguageCode: id.LanguageCode,
		})
		if err != nil {
			return 0, err
		}
		return u.ID, nil
	}
}
