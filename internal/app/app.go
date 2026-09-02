// Package app собирает зависимости вручную, запускает компоненты и
// останавливает их в правильном порядке.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/logger"
	"github.com/arrowwhi/kinolist/internal/pkg/metrics"
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
	enrichmentuc "github.com/arrowwhi/kinolist/internal/usecase/enrichment"
	"github.com/arrowwhi/kinolist/pkg/transactions"
	"github.com/arrowwhi/kinolist/web"
)

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

	log, err := logger.New(cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()
	zap.ReplaceGlobals(log)

	log.Info("starting kinolist",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("env", cfg.App.Env),
		zap.String("telegram_mode", cfg.Telegram.Mode),
	)

	closer := NewCloser()
	defer func() { //nolint:contextcheck // отдельный таймаут: закрытие ресурсов не должно обрываться вместе с отменённым основным контекстом
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
		defer cancel()

		if err := closer.Close(shutdownCtx); err != nil {
			log.Error("shutdown completed with errors", zap.Error(err))
		}
	}()

	// --- инфраструктура ---
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	closer.Add("postgres pool", func(context.Context) error {
		pool.Close()
		return nil
	})

	m := metrics.New()
	m.RegisterPool(pool)

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
	if cfg.Kinopoisk.Enabled() {
		enrichDeps.Fallback = kinopoisk.New(cfg.Kinopoisk)
	} else {
		log.Warn("kinopoisk fallback is disabled: KINOPOISK_API_KEY is not set")
	}
	if !fetcher.Enabled() {
		log.Warn("instagram scraper is disabled: reel links will be rejected, only titles work")
	}
	if !extractor.Enabled() {
		log.Warn("title extractor is disabled: reel links will be rejected, only titles work")
	}

	// --- usecase ---
	collectionUC := collectionuc.New(movies, users)
	enrichmentUC := enrichmentuc.New(enrichDeps)

	// --- транспорт ---
	botAPI := telegram.NewAPI(cfg.Telegram.BotToken, cfg.Telegram.PollTimeout)
	bot := telegram.NewBot(botAPI, enrichmentUC, collectionUC, log, webAppURL(cfg))
	notifier := telegram.NewNotifier(botAPI, log)

	health := handler.NewHealth(pool, version)

	assets, err := web.Assets()
	if err != nil {
		log.Warn("mini app assets are not embedded, serving api only", zap.Error(err))
		assets = nil
	}

	routerDeps := transporthttp.RouterDeps{
		Config: cfg, Log: log, Metrics: m,
		Movies:      handler.NewMovie(collectionUC),
		Jobs:        handler.NewEnrichment(enrichmentUC),
		Health:      health,
		WebAssets:   assets,
		ResolveUser: resolveUserFunc(collectionUC),
	}
	if cfg.Telegram.IsWebhook() {
		routerDeps.TelegramWebhook = telegram.WebhookHandler(bot, cfg.Telegram.WebhookSecret, log)
	}

	server := transporthttp.NewServer(cfg.HTTP, transporthttp.NewRouter(routerDeps))
	enrichmentWorker := worker.New(enrichmentUC, notifier, log, m, cfg.Worker)

	if err := bot.Setup(ctx); err != nil {
		// Меню и команды — украшение; без них бот полностью работоспособен.
		log.Warn("bot setup failed", zap.Error(err))
	}

	// --- запуск ---
	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	group, groupCtx := errgroup.WithContext(runCtx)

	group.Go(func() error {
		log.Info("http server listening", zap.String("addr", cfg.HTTP.Addr))
		return server.Run()
	})

	group.Go(func() error {
		return enrichmentWorker.Run(groupCtx)
	})

	if cfg.Telegram.IsWebhook() {
		url := cfg.HTTP.PublicURL + cfg.Telegram.WebhookPath
		if err := botAPI.SetWebhook(ctx, url, cfg.Telegram.WebhookSecret); err != nil {
			return fmt.Errorf("set telegram webhook: %w", err)
		}
		log.Info("telegram webhook registered", zap.String("url", url))
	} else {
		poller := telegram.NewPoller(botAPI, bot, log, cfg.Telegram.PollTimeout)
		group.Go(func() error {
			return poller.Run(groupCtx)
		})
	}

	// Порядок остановки: /readyz начинает отвечать ошибкой, балансировщик
	// уводит трафик, и только потом закрывается приём запросов.
	group.Go(func() error {
		<-groupCtx.Done()

		log.Info("shutdown signal received")
		health.SetReady(false)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
		defer cancel()

		// Пауза даёт балансировщику увидеть новый статус readiness
		// прежде, чем сервер перестанет принимать соединения.
		select {
		case <-time.After(readinessDrainDelay):
		case <-shutdownCtx.Done():
		}

		if err := server.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck // отдельный таймаут: shutdown не должен обрываться вместе с отменённым groupCtx
			return err
		}

		log.Info("http server stopped")
		return nil
	})

	if err := group.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	log.Info("kinolist stopped")
	return nil
}

// readinessDrainDelay — сколько ждать между переводом /readyz в ошибку и
// закрытием сервера.
const readinessDrainDelay = 2 * time.Second

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
