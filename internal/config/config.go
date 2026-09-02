package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       App
	HTTP      HTTP
	Postgres  Postgres
	Telegram  Telegram
	Worker    Worker
	TMDB      TMDB
	Kinopoisk Kinopoisk
	Instagram Instagram
	LLM       LLM
}

type App struct {
	Env             string
	LogLevel        string
	ShutdownTimeout time.Duration
}

func (a App) IsProd() bool { return a.Env == "prod" }

type HTTP struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	HandlerTimeout time.Duration
	PublicURL      string
}

type Postgres struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

type Telegram struct {
	BotToken      string
	Mode          string
	WebhookPath   string
	WebhookSecret string
	PollTimeout   time.Duration
}

func (t Telegram) IsWebhook() bool { return t.Mode == ModeWebhook }

const (
	ModePolling = "polling"
	ModeWebhook = "webhook"
)

type Worker struct {
	Concurrency  int
	PollInterval time.Duration
	JobTimeout   time.Duration
	MaxAttempts  int
}

type TMDB struct {
	APIKey   string
	BaseURL  string
	Language string
	Timeout  time.Duration
	CacheTTL time.Duration
}

type Kinopoisk struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

func (k Kinopoisk) Enabled() bool { return k.APIKey != "" }

type Instagram struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

func (i Instagram) Enabled() bool { return i.APIKey != "" && i.BaseURL != "" }

type LLM struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

func (l LLM) Enabled() bool { return l.APIKey != "" }

// Load читает .env (если он есть) и собирает конфиг из окружения.
// Отсутствие .env не ошибка: в контейнере переменные приходят снаружи.
func Load(path string) (*Config, error) {
	if path != "" {
		if err := godotenv.Load(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load env file %q: %w", path, err)
		}
	}

	p := &parser{}
	cfg := &Config{
		App: App{
			Env:             p.str("APP_ENV", "local"),
			LogLevel:        p.str("LOG_LEVEL", "info"),
			ShutdownTimeout: p.duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		HTTP: HTTP{
			Addr:           p.str("HTTP_ADDR", ":8080"),
			ReadTimeout:    p.duration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:   p.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			HandlerTimeout: p.duration("HTTP_HANDLER_TIMEOUT", 25*time.Second),
			PublicURL:      strings.TrimSuffix(p.str("PUBLIC_URL", ""), "/"),
		},
		Postgres: Postgres{
			DSN:               p.str("POSTGRES_DSN", ""),
			MaxConns:          int32(p.int("POSTGRES_MAX_CONNS", 10)), //nolint:gosec // размер пула соединений не выходит за диапазон int32
			MinConns:          int32(p.int("POSTGRES_MIN_CONNS", 2)),  //nolint:gosec // размер пула соединений не выходит за диапазон int32
			MaxConnLifetime:   p.duration("POSTGRES_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   p.duration("POSTGRES_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: p.duration("POSTGRES_HEALTH_CHECK_PERIOD", time.Minute),
		},
		Telegram: Telegram{
			BotToken:      p.str("TELEGRAM_BOT_TOKEN", ""),
			Mode:          p.str("TELEGRAM_MODE", ModePolling),
			WebhookPath:   p.str("TELEGRAM_WEBHOOK_PATH", "/telegram/webhook"),
			WebhookSecret: p.str("TELEGRAM_WEBHOOK_SECRET", ""),
			PollTimeout:   p.duration("TELEGRAM_POLL_TIMEOUT", 30*time.Second),
		},
		Worker: Worker{
			Concurrency:  p.int("WORKER_CONCURRENCY", 2),
			PollInterval: p.duration("WORKER_POLL_INTERVAL", 2*time.Second),
			JobTimeout:   p.duration("WORKER_JOB_TIMEOUT", 60*time.Second),
			MaxAttempts:  p.int("WORKER_MAX_ATTEMPTS", 3),
		},
		TMDB: TMDB{
			APIKey:   p.str("TMDB_API_KEY", ""),
			BaseURL:  p.str("TMDB_BASE_URL", "https://api.themoviedb.org/3"),
			Language: p.str("TMDB_LANGUAGE", "ru-RU"),
			Timeout:  p.duration("TMDB_TIMEOUT", 10*time.Second),
			CacheTTL: p.duration("TMDB_CACHE_TTL", 24*time.Hour),
		},
		Kinopoisk: Kinopoisk{
			APIKey:  p.str("KINOPOISK_API_KEY", ""),
			BaseURL: p.str("KINOPOISK_BASE_URL", "https://kinopoiskapiunofficial.tech"),
			Timeout: p.duration("KINOPOISK_TIMEOUT", 10*time.Second),
		},
		Instagram: Instagram{
			APIKey:  p.str("INSTAGRAM_API_KEY", ""),
			BaseURL: p.str("INSTAGRAM_BASE_URL", ""),
			Timeout: p.duration("INSTAGRAM_TIMEOUT", 30*time.Second),
		},
		LLM: LLM{
			APIKey:  p.str("LLM_API_KEY", ""),
			BaseURL: p.str("LLM_BASE_URL", "https://api.anthropic.com"),
			Model:   p.str("LLM_MODEL", "claude-haiku-4-5-20251001"),
			Timeout: p.duration("LLM_TIMEOUT", 30*time.Second),
		},
	}

	if err := p.err(); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var problems []string

	require := func(value, name string) {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, name+" is required")
		}
	}

	require(c.Postgres.DSN, "POSTGRES_DSN")
	require(c.Telegram.BotToken, "TELEGRAM_BOT_TOKEN")
	require(c.TMDB.APIKey, "TMDB_API_KEY")

	switch c.App.Env {
	case "local", "prod":
	default:
		problems = append(problems, "APP_ENV must be one of: local, prod")
	}

	switch c.Telegram.Mode {
	case ModePolling:
	case ModeWebhook:
		require(c.Telegram.WebhookSecret, "TELEGRAM_WEBHOOK_SECRET (required when TELEGRAM_MODE=webhook)")
		require(c.HTTP.PublicURL, "PUBLIC_URL (required when TELEGRAM_MODE=webhook)")
	default:
		problems = append(problems, "TELEGRAM_MODE must be one of: polling, webhook")
	}

	if c.Postgres.MaxConns < c.Postgres.MinConns {
		problems = append(problems, "POSTGRES_MAX_CONNS must be >= POSTGRES_MIN_CONNS")
	}
	if c.Worker.Concurrency < 1 {
		problems = append(problems, "WORKER_CONCURRENCY must be >= 1")
	}
	if c.Worker.MaxAttempts < 1 {
		problems = append(problems, "WORKER_MAX_ATTEMPTS must be >= 1")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}

	return nil
}

// parser копит ошибки разбора, чтобы показать пользователю все сразу,
// а не заставлять чинить переменные окружения по одной.
type parser struct {
	problems []string
}

func (p *parser) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func (p *parser) int(key string, def int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		p.problems = append(p.problems, fmt.Sprintf("%s: %q is not an integer", key, raw))
		return def
	}
	return v
}

func (p *parser) duration(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		p.problems = append(p.problems, fmt.Sprintf("%s: %q is not a duration (e.g. 10s, 1h)", key, raw))
		return def
	}
	return v
}

func (p *parser) err() error {
	if len(p.problems) == 0 {
		return nil
	}
	return errors.New("invalid configuration:\n  - " + strings.Join(p.problems, "\n  - "))
}
