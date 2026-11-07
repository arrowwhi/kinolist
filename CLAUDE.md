# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

kinolist — a Telegram bot + Telegram Mini App for a personal movie/TV collection. The user sends the bot an Instagram reel link or a title; the service recognizes the title, finds it in TMDB (Kinopoisk as fallback), asks for confirmation and saves it. The collection (statuses, ratings, notes, reviews, genres, directors) is browsed in the Mini App.

One Go binary (HTTP API + Telegram bot + enrichment worker), PostgreSQL, and a React/TS Mini App in `web/` embedded into the binary via `embed.FS` (`web/embed.go`). Docs, code comments and user-facing strings are in Russian — keep that.

## Commands

```bash
make compose-infra        # postgres + migrations in Docker
make run                  # service on :8080 (reads .env; copy from .env.example)
make dev                  # hot reload via air (install tools: make tools)
make build                # builds web/ then the Go binary with version ldflags
make lint                 # golangci-lint (config includes the `integration` build tag)
make fmt                  # gofumpt
make test                 # unit tests: go test -race -count=1 ./...  (no network, no Docker)
make test-integration     # testcontainers Postgres, -tags=integration ./internal/repository/...
make mocks                # regenerate gomock mocks (go:generate in internal/usecase/*/usecase.go)
make migrate-create name=add_something
```

Single Go test: `go test -race -count=1 -run TestName ./internal/usecase/collection/`
Single integration test: `go test -count=1 -tags=integration -run TestName ./internal/repository/postgres/`
(if testcontainers' reaper conflicts with the Docker daemon: `TESTCONTAINERS_RYUK_DISABLED=true`).

Frontend (`cd web`): `npm run dev` (Vite, proxies `/api` to :8080), `npm run build`, `npm run typecheck`, `npm test` (node test runner over `tests/*.test.ts`), `npm run format` / `format:check` (prettier).

Catalog metadata backfill for old rows: `go run ./cmd/backfill [--apply] --limit=100 [--after-id=N]` (dry run without `--apply`).

## Architecture

Clean architecture, dependencies point inward: `domain` → `usecase` → `repository` / `transport` → `app`.

- `internal/domain/{user,movie,enrichment}` — entities, domain errors, and **ports (interfaces) declared on the consumer side**. Stdlib only, no struct tags. Title matching/normalization rules live in `domain/movie/matching.go` (see ADR-4).
- `internal/usecase/collection` — list/filter/group, card, status/rating/note/review updates, delete, statistics, directors.
- `internal/usecase/enrichment` — the add-movie pipeline: `Enqueue`, `ProcessJob`, `Confirm`, `Reject`. Both the bot and the Mini App call the same usecase.
- `internal/repository/` — `postgres` (infra `connections/postgres` pool + squirrel; every repo method names its queries via `queryName(ctx, "repo.method")` for the `db_query_name` metric label; `errors.go` maps pgx/23505 to domain errors), `tmdb`, `kinopoisk`, `instagram` (third-party scraper API), `llm` (title extraction from reel caption via Anthropic API). Outbound HTTP clients are built with `internal/pkg/httpclient.New` (otelhttp traces/metrics, secrets redacted from URLs).
- `internal/transport/` — `http` (chi router under `/api/v1`, Telegram `initData` HMAC auth middleware, DTO/converter layer, SPA handler), `telegram` (kinolist handlers registered on infra `telegram/bot`: polling or webhook via `TELEGRAM_MODE`, inline-keyboard confirmations, notifier on infra `telegram/api` client), `worker` (job pool).
- `internal/app/app.go` — manual wiring of all dependencies on top of `github.com/arrowwhi/infrastructure/telegram/app` (logger, metrics, tracing, admin port on `METRICS_ADDR`, LIFO `OnShutdown`). kinolist env names are kept and mapped into infra configs in `infraConfig`/`botConfig`. `cmd/app/main.go` stays tiny.
- `pkg/transactions` — usecases open a transaction and put it in `context`; the infra pool routes queries with that context into the tx, so repositories work identically inside/outside a tx. Nested `BeginTransaction` reuses the outer tx (owner commits).
- Logging: `internal/pkg/logger` wraps the infra zerolog logger; put request/job fields into the context with `logger.With(ctx, key, value)`, write with `logger.Info(ctx).…Msg(…)`. App metrics (`internal/pkg/metrics`) are OpenTelemetry instruments; infra covers HTTP, DB, Telegram and runtime metrics.

### Add-movie pipeline (spans transport → usecase → worker → repos)

1. Link/title → row in `enrichment_jobs` (`pending`); user gets an immediate reply / HTTP 202 with `job_id`.
2. Worker claims jobs with `FOR UPDATE SKIP LOCKED`, retries via `run_after` up to `WORKER_MAX_ATTEMPTS`. For reels: scraper → LLM extracts `{title, year, confidence}`.
3. Catalog search: TMDB, fallback Kinopoisk. Confident match → single candidate; otherwise top-5 stored in `candidates`, job → `confirming`.
4. `Confirm` writes movie, genres, directors, `user_movies` row and closes the job in **one transaction**; catalog details are fetched *before* the tx. Duplicate → `ErrAlreadyInCollection` (HTTP 409 with `existing_user_movie_id`).

Bot dialog state is persisted in `bot_sessions`, not memory, so restarts don't drop pending confirmations. External API responses are cached in Postgres (`catalog_cache`), no Redis.

### Data model notes

- `movies` is a shared catalog (one row per external title, keyed by TMDB id + `media_type` `movie|tv`); per-user data is in `user_movies` (`status` `want_to_watch|watched`, `user_rating` 1–10, separate `note` and `review`, `watched_at`).
- Genres are a fixed TMDB reference table; `genre_external_map` maps Kinopoisk names onto it (ADR-5).
- Migrations are goose SQL files in `migrations/`, embedded and applied by a separate binary `cmd/migrate` (the app never changes schema). Every migration needs a working `-- +goose Down`.
- API contract: `api/openapi/openapi.yaml`; the README's "API для обновлённого дизайна" section documents newer endpoint semantics (summary/statistics counting rules, PATCH field semantics, media types, directors).

### Frontend (`web/src`)

Feature-sliced layout: `app/` (root, hash-based router in `route.ts`/`navigation.ts` — must ignore Telegram `tgWebApp*` launch params in the hash), `pages/`, `widgets/`, `features/`, `entities/`, `shared/` (ui kit, hooks `useResource`/`useMutation`, design tokens in `shared/styles/tokens.css`). API client and types in `api/`. Uses Telegram WebApp SDK (theme params, BackButton, haptics, `showConfirm`).

## Config

All config comes from env (`internal/config/config.go`, `.env` via godotenv). Required: `POSTGRES_DSN`, `TELEGRAM_BOT_TOKEN`, `TMDB_API_KEY`; validation reports all missing vars at once. Without `INSTAGRAM_API_KEY`/`LLM_API_KEY` reel links are rejected but title search works. Mini App requires HTTPS `PUBLIC_URL`. Deployment: `deployments/compose.prod.yml` (Postgres + migrate container + app + Caddy); see `DEPLOY.md`. Architectural decisions: `docs/adr/`.
