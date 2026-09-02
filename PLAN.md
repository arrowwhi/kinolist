# kinolist — план реализации

## Context

Личная коллекция фильмов, которые попадаются в Instagram-рилсах. Пользователь кидает боту ссылку на рилс — сервис достаёт подпись, вытаскивает из неё название фильма, находит карточку в кинобазе, спрашивает подтверждение и сохраняет фильм с жанрами, описанием и постером. Просмотр коллекции — в Telegram Mini App с группировкой по жанру и дате добавления. Альтернативный вход — название текстом.

Проблема, которую решает проект: увиденный в рилсе фильм сейчас теряется — нет места, где он лежит с описанием и жанром, и нет способа отделить просмотренное от отложенного.

Результат: один Go-бинарь (HTTP API + Telegram-бот + воркер обогащения), Postgres, React-фронтенд Mini App, всё поднимается через docker compose за Caddy на VPS.

---

## Решения, зафиксированные на этапе проектирования

| Вопрос | Решение |
|---|---|
| Module path | `github.com/arrowwhi/kinolist` |
| Go | последняя стабильная, `toolchain` пришпилен в `go.mod` |
| Контент рилса | сторонний скрапер-API (Apify/RapidAPI) через порт `ReelFetcher`; при неудаче — фолбэк «пришли название текстом» |
| Извлечение названия | LLM по caption (порт `TitleExtractor` принимает `caption + []frameURL` — Vision добавляется позже без правок домена) |
| Кинобаза | TMDB основной, Кинопоиск (неофиц.) как фолбэк при пустом результате |
| Пользователи | многопользовательский с первого дня, авторизация по Telegram `initData` (HMAC-SHA256 от bot-token) |
| Подтверждение фильма | и inline-кнопками в чате, и в Mini App |
| Просмотр коллекции | только Mini App |
| Фронтенд | React + TS + Vite в `web/`, сборка embed'ится в бинарь через `embed.FS` |
| Telegram updates | оба режима, `TELEGRAM_MODE=polling\|webhook` |
| Деплой | VPS + docker compose + Caddy (Let's Encrypt автоматом) |
| Асинхронность | таблица `enrichment_jobs` + воркер-пул в том же бинаре, `FOR UPDATE SKIP LOCKED` |
| Состояние диалога бота | в Postgres (`bot_sessions`), не в памяти — рестарт не должен ронять висящие подтверждения |

### Что вырезано из исходного ТЗ и почему

- **gRPC, `.proto`, `buf`, конвертеры сгенерированных типов** — единственные клиенты это браузер Mini App и Telegram. Потребителя gRPC не существует.
- **Kafka (`transport/kafka`, consumer/producer)** — единственная асинхронная задача (обогащение фильма) закрывается таблицей задач в Postgres. Брокер добавил бы контейнер, лаг-метрики и коммит оффсетов в graceful shutdown ради одной очереди на десяток сообщений в день.
- **`deployments/k8s`** — один сервис и одна база на одном VPS.
- **Redis** — кэш TMDB-ответов ложится в Postgres, отдельного стораджа не требует.

Всё остальное из ТЗ (чистая архитектура, pgx/squirrel, goose отдельным контейнером, zap, RED-метрики, `/healthz` и `/readyz`, `Closer`, gomock, testcontainers, air, Makefile, ADR) сохранено.

### Что добавлено сверх ТЗ

Статус `watched|want_to_watch`, дедупликация по `(user_id, tmdb_id)`, удаление фильма, личная оценка и заметка.

---

## Схема БД (миграции goose)

- `00001_users.sql` — `users(id, telegram_id UNIQUE, username, language_code, created_at)`.
- `00002_genres.sql` — `genres(id, tmdb_id UNIQUE, name_ru, name_en)` + сид полного справочника жанров TMDB; `genre_external_map(genre_id, source, external_name)` — сюда ложится маппинг названий Кинопоиска, чтобы «Sci-Fi» и «фантастика» не стали двумя жанрами.
- `00003_movies.sql` — `movies(id, tmdb_id UNIQUE, kinopoisk_id, title, original_title, year, overview, poster_path, runtime, rating, created_at)` — общий каталог, не привязан к пользователю; `movie_genres(movie_id, genre_id)`.
- `00004_user_movies.sql` — `user_movies(id, user_id, movie_id, status, user_rating, note, source, source_url, added_at, watched_at)`, `UNIQUE(user_id, movie_id)`; индексы по `(user_id, added_at DESC)` и `(user_id, status)`.
- `00005_enrichment_jobs.sql` — `enrichment_jobs(id, user_id, kind, payload JSONB, status, attempts, last_error, candidates JSONB, run_after, created_at, updated_at)`; индекс `(status, run_after)` для выборки воркером.
- `00006_bot_sessions.sql` — `bot_sessions(user_id PK, state, job_id, chat_id, message_id, updated_at)`.
- `00007_tmdb_cache.sql` — `tmdb_cache(cache_key PK, payload JSONB, expires_at)` — экономит квоту и делает интеграционные тесты воспроизводимыми.

Каждая миграция с рабочим `-- +goose Down`.

---

## Пайплайн добавления фильма

1. Вход: ссылка на рилс (бот или Mini App) либо название текстом. Транспорт валидирует и вызывает `usecase/enrichment.Enqueue` → создаётся `enrichment_jobs` в статусе `pending`, пользователю сразу отдаётся `job_id` (HTTP 202 / сообщение «ищу…»).
2. Воркер забирает задачу (`FOR UPDATE SKIP LOCKED`), для рилса: `ReelFetcher.Fetch(ctx, url)` → caption + frameURLs → `TitleExtractor.Extract(ctx, caption, frames)` → `{title, year, confidence}`. Пустой результат → задача в `failed` с причиной `ErrTitleNotRecognized`, пользователю уходит «не распознал, пришли название текстом».
3. `MovieCatalog.Search(ctx, title, year)` — TMDB; пусто → Кинопоиск. Результаты нормализуются в `movie.Candidate`.
4. **Определение уверенного совпадения** (доменная функция, тестируется таблично): нормализованное название кандидата равно нормализованному запросу И год совпал (или год не был извлечён, а кандидат единственный с таким названием). Нормализация: lowercase, схлопывание пробелов, удаление пунктуации и артиклей, сравнение и по `title`, и по `original_title`. Совпало → задача переходит в `confirming` с единственным кандидатом; не совпало → берутся топ-5 кандидатов по популярности, кладутся в `candidates`.
5. Подтверждение: бот шлёт постер + описание + inline-кнопки (`Да` / `Другой` / `Отмена`), Mini App показывает те же кандидаты карточками. Оба транспорта вызывают один `usecase/enrichment.Confirm(ctx, jobID, tmdbID)`.
6. `Confirm` в одной транзакции: upsert фильма и его жанров в каталог, вставка `user_movies` (конфликт по `UNIQUE(user_id, movie_id)` → `ErrAlreadyInCollection` с датой первого добавления), перевод задачи в `done`, очистка `bot_sessions`.

Прямое добавление по названию идёт тем же путём, минуя шаги 2 и часть 3.

---

## Структура файлов

```
kinolist/
├── cmd/app/main.go                          # флаги версии + app.Run(), ≤30 строк
│
├── internal/
│   ├── app/
│   │   ├── app.go                           # сборка всех зависимостей вручную, старт HTTP/бота/воркера, ожидание сигнала
│   │   └── closer.go                        # LIFO-регистр функций закрытия с таймаутом
│   ├── config/config.go                     # единая Config из env через godotenv + Validate() с внятными ошибками
│   │
│   ├── domain/
│   │   ├── user/{entity,errors,ports}.go    # User, ErrUserNotFound, интерфейс Repository
│   │   ├── movie/
│   │   │   ├── entity.go                    # Movie, Genre, UserMovie, Status, Candidate — только stdlib, без тегов
│   │   │   ├── matching.go                  # нормализация названий и правило уверенного совпадения
│   │   │   ├── errors.go                    # ErrMovieNotFound, ErrAlreadyInCollection, ErrNoCandidates
│   │   │   └── ports.go                     # Repository, Catalog (TMDB/КП), интерфейсы объявлены здесь как у потребителя
│   │   └── enrichment/
│   │       ├── entity.go                    # Job, JobStatus, JobKind, ReelContent, ExtractedTitle
│   │       ├── errors.go                    # ErrJobNotFound, ErrTitleNotRecognized, ErrReelUnavailable
│   │       └── ports.go                     # JobRepository, ReelFetcher, TitleExtractor, Notifier
│   │
│   ├── usecase/
│   │   ├── collection/                      # список с фильтрами и группировкой, карточка, статус, оценка, заметка, удаление
│   │   │   ├── usecase.go
│   │   │   ├── usecase_test.go              # табличные тесты на моках портов
│   │   │   └── mocks/                       # gomock, go:generate
│   │   └── enrichment/                      # Enqueue, ProcessJob, Confirm, Reject — вся логика пайплайна
│   │       ├── usecase.go
│   │       ├── usecase_test.go
│   │       └── mocks/
│   │
│   ├── repository/
│   │   ├── postgres/
│   │   │   ├── pool.go                      # pgxpool с MaxConns/MaxConnLifetime/HealthCheckPeriod из конфига
│   │   │   ├── user.go
│   │   │   ├── movie.go                     # каталог + user_movies, squirrel, фильтры и группировки
│   │   │   ├── movie_integration_test.go    # //go:build integration, testcontainers — фильтры и upsert жанров
│   │   │   ├── job.go                       # выборка FOR UPDATE SKIP LOCKED, ретраи с run_after
│   │   │   ├── job_integration_test.go      # //go:build integration — конкурентный захват задач двумя воркерами
│   │   │   ├── session.go                   # bot_sessions
│   │   │   └── errors.go                    # pgx.ErrNoRows / 23505 → доменные ошибки
│   │   ├── tmdb/{client,converter}.go       # поиск и детали, конвертация в domain, кэш через tmdb_cache
│   │   ├── kinopoisk/{client,converter}.go  # фолбэк-источник, маппинг жанров через genre_external_map
│   │   ├── instagram/scraper.go             # ReelFetcher поверх скрапер-API, таймауты и ретраи
│   │   └── llm/extractor.go                 # TitleExtractor: caption → {title, year, confidence}, строгий JSON-ответ
│   │
│   ├── transport/
│   │   ├── http/
│   │   │   ├── server.go, router.go         # chi, монтирование API, статики Mini App и служебных эндпоинтов
│   │   │   ├── handler/                     # movies (список/карточка/статус/оценка/удаление), enrichment (создать/кандидаты/подтвердить), health
│   │   │   ├── dto/                         # request/response структуры с json-тегами, отдельно от домена
│   │   │   ├── converter/                   # domain ↔ dto
│   │   │   └── middleware/                  # request_id, logging, recovery, metrics, timeout, telegram_auth (проверка initData)
│   │   ├── telegram/
│   │   │   ├── bot.go                       # клиент, регистрация команд, кнопка запуска Mini App
│   │   │   ├── poller.go, webhook.go        # два источника апдейтов, один общий диспетчер
│   │   │   ├── handler/                     # /start, /help, ссылка на рилс, свободный текст, callback-кнопки подтверждения
│   │   │   ├── keyboard.go                  # inline-клавиатуры кандидатов
│   │   │   └── notifier.go                  # реализация domain/enrichment.Notifier — воркер сообщает результат в чат
│   │   └── worker/
│   │       ├── worker.go                    # пул горутин, поллинг enrichment_jobs, backoff, graceful stop
│   │       └── worker_test.go
│   │
│   └── pkg/{logger,metrics,errmap}/         # zap с request_id из контекста; RED-метрики + метрики пула БД; единая таблица домен→HTTP-код
│
├── pkg/transactions/                        # BeginTransaction/FinishTransaction, прокидывание tx через контекст
│
├── web/                                     # React + TS + Vite Mini App
│   ├── src/{api,components,pages,hooks,theme}/  # список с группировкой, карточка, добавление, экран подтверждения; тема из Telegram WebApp API
│   ├── index.html, vite.config.ts, package.json
│   └── embed.go                             # embed.FS собранного dist, отдаётся chi
│
├── api/openapi/openapi.yaml                 # контракт HTTP API — источник правды для фронта
├── migrations/*.sql + embed.go
├── deployments/
│   ├── docker/{Dockerfile,Dockerfile.dev}   # multi-stage: сборка web → сборка Go с ldflags → alpine, non-root
│   ├── compose.prod.yml                     # app + postgres + goose + caddy
│   └── caddy/Caddyfile
├── test/{integration,fixtures}/             # фикстуры: сохранённые ответы TMDB и скрапера, чтобы тесты не ходили в сеть
├── docs/adr/                                # ADR: отказ от gRPC/Kafka, очередь в Postgres, выбор кинобаз, стратегия матчинга
├── docker-compose.yml                       # dev, профиль infra для запуска только зависимостей под air
├── .air.toml .golangci.yml .dockerignore .env.example .gitignore
├── Makefile CLAUDE.md README.md go.mod
```

---

## Порядок реализации

Снизу вверх, после каждого слоя — сводка и что осталось.

1. **Каркас** — `go.mod`, `Makefile`, `.golangci.yml`, `.env.example`, `config`, `pkg/logger`, `pkg/transactions`, `internal/pkg/errmap`, миграции.
2. **`domain`** — три пакета сущностей, ошибок и портов. Только stdlib. Правила матчинга покрываются таблично сразу.
3. **`usecase`** — `collection` и `enrichment` с gomock-моками портов и табличными тестами: уверенное совпадение, неуверенное, пустой результат, дубль, нераспознанное название.
4. **`repository`** — postgres (+ интеграционные тесты на testcontainers для фильтров и конкурентного захвата задач), tmdb, kinopoisk, instagram, llm.
5. **`transport`** — http (+ middleware валидации `initData`), telegram (оба режима апдейтов), worker.
6. **`app`** — ручной DI, `Closer`, graceful shutdown в порядке: `/readyz` в ошибку → остановка приёма HTTP → остановка бота → дренаж воркеров → закрытие пула.
7. **`web/`** — Vite-приложение, тема и haptics из Telegram WebApp API, экраны: коллекция с группировкой, карточка, добавление, подтверждение.
8. **Инфраструктура и документация** — Dockerfile, compose (dev и prod), Caddyfile, README, ADR, `CLAUDE.md`.

По завершении план дублируется в `PLAN.md` в корне проекта, как просили в ТЗ.

---

## Верификация

- `make lint && make test` — линтеры и unit-тесты usecase зелёные, в сеть не ходят.
- `make test-integration` — testcontainers поднимает Postgres, прогоняются репозиторные тесты, включая захват задач двумя воркерами.
- `make compose-up` — сервис, база и goose стартуют; `/healthz` отвечает 200, `/readyz` — 200 после миграций и 503 при остановленной базе, `/metrics` отдаёт RED-метрики и метрики пула.
- Ручной сценарий в Telegram (polling локально): `/start` → отправка ссылки на рилс → бот присылает кандидата с постером → `Да` → фильм в базе; повторная отправка той же ссылки → «уже в коллекции»; отправка мусорной ссылки → внятный фолбэк; отправка названия текстом → тот же пайплайн.
- Mini App: открывается по кнопке, список группируется по жанру и дате, карточка показывает описание, статус переключается, фильм удаляется. Запрос к API без валидного `initData` получает 401.
- Прод: домен за Caddy отдаёт валидный сертификат, `TELEGRAM_MODE=webhook`, `setWebhook` принят Telegram.

## Риски, которые остаются открытыми

- **Скрапер Instagram** — внешний платный сервис, может менять формат ответа или упереться в лимиты. Смягчается изоляцией за портом `ReelFetcher` и фолбэком на ручной ввод: сценарий не тупикует никогда.
- **Покрытие caption-only извлечения ~40–60%** — в рилсах-подборках название часто вшито в кадр. `TitleExtractor` сразу принимает `frameURL`'ы, так что Vision-реализация добавляется без правок домена и usecase.
- **Неофициальный API Кинопоиска** — используется только как фолбэк, его отказ деградирует покрытие, но не ломает основной путь.
