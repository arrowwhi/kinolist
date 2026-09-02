# kinolist

Телеграм-бот и мини-приложение для коллекции фильмов, которые попадаются в
рилсах Instagram.

Кидаете боту ссылку на рилс — сервис достаёт подпись, распознаёт название,
находит карточку в кинобазе и спрашивает, тот ли это фильм. Можно и просто
написать название текстом. Вся коллекция с жанрами, описаниями, статусами и
оценками — в мини-приложении внутри Telegram.

## Что внутри

- Один Go-бинарь: HTTP API, Telegram-бот и фоновый воркер обогащения.
- PostgreSQL: коллекция, очередь задач, состояние диалога, кэш каталогов.
- React + TypeScript мини-приложение, встроенное в бинарь через `embed`.
- Чистая архитектура: `domain` → `usecase` → `repository` / `transport` → `app`.

Нетривиальные решения задокументированы в [`docs/adr/`](docs/adr/), контракт
API — в [`api/openapi/openapi.yaml`](api/openapi/openapi.yaml).

## Требования

- Go 1.26+
- Node.js 22+ (для сборки мини-приложения)
- Docker и Docker Compose
- Токен бота от [@BotFather](https://t.me/BotFather)
- Ключ [TMDB](https://www.themoviedb.org/settings/api) — бесплатный

## Быстрый старт

```bash
cp .env.example .env      # заполните TELEGRAM_BOT_TOKEN, TMDB_API_KEY, POSTGRES_DSN
make compose-infra        # postgres + миграции
make run                  # сервис на :8080
```

Для локальной разработки с горячей перезагрузкой: `make dev` (нужен `air`,
ставится через `make tools`). Фронтенд отдельно — `make web-dev`.

Без ключей `INSTAGRAM_API_KEY` и `LLM_API_KEY` сервис работает: ссылки на
рилсы отклоняются понятной ошибкой, добавление по названию доступно
полностью.

## Настройка бота

1. `@BotFather` → `/newbot`, получите токен.
2. Разверните сервис на домене с валидным HTTPS (см. ниже). Mini App без
   HTTPS не откроется.
3. `@BotFather` → `/newapp` → укажите тот же домен.
4. В `.env`: `PUBLIC_URL=https://ваш-домен`, `TELEGRAM_MODE=webhook`.

Локально удобнее `TELEGRAM_MODE=polling` — публичный адрес для самого бота
тогда не нужен (но нужен для Mini App, например через `cloudflared tunnel`).

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `APP_ENV` | `local` | `local` — текстовые логи, `prod` — JSON |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | `15s` | Предел graceful shutdown |
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `HTTP_READ_TIMEOUT` | `10s` | Таймаут чтения запроса |
| `HTTP_WRITE_TIMEOUT` | `30s` | Таймаут записи ответа |
| `HTTP_HANDLER_TIMEOUT` | `25s` | Таймаут обработки одного запроса |
| `PUBLIC_URL` | — | Публичный HTTPS-адрес; обязателен для webhook и Mini App |
| `POSTGRES_DSN` | — | **Обязательно.** Строка подключения |
| `POSTGRES_MAX_CONNS` | `10` | Максимум соединений в пуле |
| `POSTGRES_MIN_CONNS` | `2` | Минимум соединений в пуле |
| `POSTGRES_MAX_CONN_LIFETIME` | `1h` | Время жизни соединения |
| `POSTGRES_MAX_CONN_IDLE_TIME` | `30m` | Простой до закрытия |
| `POSTGRES_HEALTH_CHECK_PERIOD` | `1m` | Период проверки пула |
| `TELEGRAM_BOT_TOKEN` | — | **Обязательно.** Токен от BotFather |
| `TELEGRAM_MODE` | `polling` | `polling` или `webhook` |
| `TELEGRAM_WEBHOOK_PATH` | `/telegram/webhook` | Путь вебхука |
| `TELEGRAM_WEBHOOK_SECRET` | — | Обязателен при `TELEGRAM_MODE=webhook` |
| `TELEGRAM_POLL_TIMEOUT` | `30s` | Таймаут long-polling |
| `WORKER_CONCURRENCY` | `2` | Число параллельных задач обогащения |
| `WORKER_POLL_INTERVAL` | `2s` | Как часто воркер смотрит в очередь |
| `WORKER_JOB_TIMEOUT` | `60s` | Предел на одну задачу |
| `WORKER_MAX_ATTEMPTS` | `3` | Попыток до перевода задачи в `failed` |
| `TMDB_API_KEY` | — | **Обязательно.** Ключ TMDB (v3 или v4) |
| `TMDB_BASE_URL` | `https://api.themoviedb.org/3` | База API |
| `TMDB_LANGUAGE` | `ru-RU` | Язык описаний |
| `TMDB_TIMEOUT` | `10s` | Таймаут запроса |
| `TMDB_CACHE_TTL` | `24h` | Время жизни кэша ответов |
| `KINOPOISK_API_KEY` | — | Ключ неофициального API; пусто — фолбэк выключен |
| `KINOPOISK_BASE_URL` | `https://kinopoiskapiunofficial.tech` | База API |
| `KINOPOISK_TIMEOUT` | `10s` | Таймаут запроса |
| `INSTAGRAM_API_KEY` | — | Ключ скрапера; пусто — ссылки отклоняются |
| `INSTAGRAM_BASE_URL` | — | Эндпоинт скрапера |
| `INSTAGRAM_TIMEOUT` | `30s` | Таймаут запроса |
| `LLM_API_KEY` | — | Ключ модели; пусто — ссылки отклоняются |
| `LLM_BASE_URL` | `https://api.anthropic.com` | База API |
| `LLM_MODEL` | `claude-haiku-4-5-20251001` | Модель для разбора подписи |
| `LLM_TIMEOUT` | `30s` | Таймаут запроса |

Секреты значений по умолчанию не имеют: без них сервис падает на старте со
списком всех недостающих переменных сразу.

## Makefile

| Цель | Что делает |
|---|---|
| `make run` | Запустить сервис |
| `make dev` | Запустить с горячей перезагрузкой (air) |
| `make build` | Собрать фронтенд и бинарь с версией в `-ldflags` |
| `make lint` | golangci-lint |
| `make fmt` | gofumpt |
| `make test` | Unit-тесты (без сети и без Docker) |
| `make test-integration` | Интеграционные тесты на testcontainers |
| `make migrate-up` / `migrate-down` / `migrate-status` | Миграции goose |
| `make migrate-create name=...` | Новая миграция |
| `make mocks` | Перегенерировать моки gomock |
| `make web-install` / `web-build` / `web-dev` | Фронтенд |
| `make docker-build` | Прод-образ |
| `make compose-up` / `compose-infra` / `compose-down` | Локальное окружение |
| `make tools` | Поставить goose, mockgen, gofumpt, air |

## Тесты

```bash
make test              # быстрые, не ходят в сеть
make test-integration  # поднимает реальный Postgres в Docker
```

Интеграционные тесты собраны под тегом `integration` и требуют работающего
Docker. Если ваш демон конфликтует с reaper'ом testcontainers, помогает
`TESTCONTAINERS_RYUK_DISABLED=true`.

## Деплой

```bash
cd deployments
DOMAIN=kino.example.com docker compose -f compose.prod.yml up -d --build
```

Caddy сам получит и будет продлевать сертификат Let's Encrypt. Миграции
накатывает отдельный контейнер до старта приложения.

Служебные эндпоинты: `/healthz` (liveness), `/readyz` (readiness, проверяет
БД), `/metrics` (Prometheus). `/metrics` наружу не выставляется — Caddy
отдаёт на него 404.

## Как это работает

1. Ссылка или название попадают в `enrichment_jobs` со статусом `pending`,
   пользователю сразу уходит ответ.
2. Воркер забирает задачу через `FOR UPDATE SKIP LOCKED`, для ссылки —
   скрапер и распознавание названия по подписи.
3. Поиск в TMDB, при пустом результате — в Кинопоиске.
4. Если название и год совпали — предлагается один вариант, иначе до пяти
   на выбор. Правило описано в [ADR-4](docs/adr/0004-match-confidence.md).
5. Подтверждение одной транзакцией пишет карточку фильма, жанры, запись
   коллекции и закрывает задачу.

Состояние диалога с ботом хранится в БД, а не в памяти: перезапуск сервиса не
роняет висящие подтверждения.
