# Чек-лист перед деплоем

Сервис деплоится на VPS через `docker compose` (файл
[`deployments/compose.prod.yml`](deployments/compose.prod.yml)): Postgres +
контейнер миграций goose + приложение + Caddy с автоматическим Let's
Encrypt. Ниже — что нужно сделать по порядку, прежде чем поднимать прод.

## 1. Сервер

- [ ] VPS с публичным IP, Docker и Docker Compose v2 (`docker compose version`).
- [ ] Открыты порты **80** и **443** (Caddy сам ходит за сертификатом через
      HTTP-01, порт 80 должен быть доступен снаружи, иначе выпуск сертификата
      не пройдёт).
- [ ] Диск: Postgres данные лежат в volume `pgdata` — учитывайте рост объёма
      коллекции при выборе размера диска.

## 2. DNS

- [ ] A-запись домена (например, `kino.example.com`) указывает на IP VPS.
- [ ] Запись успела распространиться (`dig +short kino.example.com`) —
      без этого Caddy не сможет пройти ACME-challenge.

## 3. Telegram-бот

- [ ] Токен бота получен у [@BotFather](https://t.me/BotFather) (`/newbot`).
- [ ] Прод работает в режиме `TELEGRAM_MODE=webhook` — polling для этого не
      подходит (два режима существуют для локальной разработки и прода
      соответственно).
- [ ] Отдельный секрет для `TELEGRAM_WEBHOOK_SECRET` — случайная строка,
      которую сервис сверяет с заголовком `X-Telegram-Bot-Api-Secret-Token`
      на каждом входящем апдейте.
- [ ] Кнопка меню Mini App настраивается сервисом автоматически при старте
      (`Bot.Setup` → `SetChatMenuButton` на `PUBLIC_URL`) — регистрировать
      приложение через `@BotFather /newapp` для этого не обязательно, HTTPS
      на `PUBLIC_URL` уже достаточно.

## 4. Внешние API-ключи

| Ключ | Обязателен | Что будет без него |
|---|---|---|
| `TMDB_API_KEY` | да | сервис не стартует (падает на валидации конфига) |
| `KINOPOISK_API_KEY` | нет | фолбэк-поиск выключен, работает только TMDB |
| `INSTAGRAM_API_KEY` | нет | ссылки на рилсы отклоняются, доступно только добавление по названию |
| `LLM_API_KEY` | нет | извлечение названия из подписи рилса не работает |

- [ ] Ключ [TMDB](https://www.themoviedb.org/settings/api) получен — без
      него `internal/config.Validate` остановит запуск с понятной ошибкой.
- [ ] Решено, нужны ли на старте Кинопоиск / Instagram-скрапер / LLM — если
      нет, соответствующие `*_API_KEY` можно оставить пустыми осознанно.

## 5. Переменные окружения

Компоуз использует **два разных `.env`**, не перепутать:

- **`./.env`** (корень репозитория, скопировать из
  [`.env.example`](.env.example)) — конфиг самого приложения, монтируется в
  контейнер `app` через `env_file: ../.env`. Обязательно заполнить как
  минимум:
  - `APP_ENV=prod`
  - `PUBLIC_URL=https://kino.example.com`
  - `TELEGRAM_MODE=webhook`
  - `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`
  - `TMDB_API_KEY`
  - `POSTGRES_DSN=postgres://<user>:<password>@postgres:5432/<db>?sslmode=disable`
    (хост `postgres` — имя сервиса в docker-сети, не `localhost`)
- **`deployments/.env`** (создать вручную, файла-примера для него в
  репозитории нет) — переменные для подстановки в сам `compose.prod.yml`:
  ```
  POSTGRES_USER=kinolist
  POSTGRES_PASSWORD=<сгенерировать>
  POSTGRES_DB=kinolist
  DOMAIN=kino.example.com
  POSTGRES_DSN=postgres://kinolist:<тот же пароль>@postgres:5432/kinolist?sslmode=disable
  VERSION=<git tag или commit>
  COMMIT=<git rev-parse --short HEAD>
  BUILD_TIME=<date -u +%FT%TZ>
  ```
  `POSTGRES_DSN` здесь используется контейнером `migrate`, значение должно
  совпадать с тем, что указано в `./.env` для приложения.

- [ ] Оба `.env`-файла заполнены и **не закоммичены** (в `.gitignore` уже
      есть `.env`).
- [ ] `TELEGRAM_WEBHOOK_SECRET` и `POSTGRES_PASSWORD` — случайные значения
      (`openssl rand -hex 32`), не то, что было в примере.

## 6. Сборка и запуск

```bash
cd deployments
docker compose -f compose.prod.yml up -d --build
```

Порядок внутри compose: `postgres` (healthcheck) → `migrate` (накатывает
все миграции из `migrations/`, ждёт готовности Postgres) → `app` (стартует
после успешного завершения `migrate`) → `caddy` (получает сертификат и
проксирует на `app:8080`).

- [ ] `docker compose -f compose.prod.yml ps` — все сервисы `running`/`exited (0)`
      для `migrate`.
- [ ] `docker compose -f compose.prod.yml logs migrate` — миграции применились
      без ошибок.
- [ ] `docker compose -f compose.prod.yml logs caddy` — сертификат Let's
      Encrypt выпущен (без ошибок ACME).

## 7. Проверка после старта

- [ ] `curl https://kino.example.com/healthz` → `200 {"status":"ok",...}`.
- [ ] `curl https://kino.example.com/readyz` → `200` (после того как пул
      подключился к БД).
- [ ] `curl https://kino.example.com/metrics` → `404` (Caddy намеренно не
      отдаёт метрики наружу; для мониторинга snapshot'ить изнутри сети
      `docker compose`, например `docker compose exec app curl localhost:8080/metrics`).
- [ ] В логах `app` — `telegram webhook registered` без ошибок (`setWebhook`
      принят Telegram).
- [ ] Сертификат браузером принимается без предупреждений.

## 8. Смоук-тест сценария в Telegram

- [ ] `/start` — бот отвечает.
- [ ] Кнопка меню открывает Mini App, список (пустой на старте) загружается,
      запрос без `initData` внутри Mini App возвращает 401 (можно не
      проверять руками — покрыто тестами auth-middleware, но полезно один
      раз увидеть вживую).
- [ ] Отправка ссылки на реальный рилс — бот присылает кандидата с постером
      и кнопками `Да`/`Другой`/`Отмена`.
- [ ] Повторная отправка той же ссылки — «уже в коллекции».
- [ ] Отправка мусорной ссылки — внятный фолбэк-ответ, а не тишина/паника.
- [ ] Отправка названия текстом — тот же пайплайн подтверждения.
- [ ] Фильм появляется в Mini App, статус переключается, фильм можно
      удалить.

## 9. Эксплуатация

- [ ] Настроен бэкап volume `pgdata` (`docker run --rm -v kinolist_pgdata:/data ...`
      или снапшот диска на уровне провайдера) — это единственный источник
      правды коллекции.
- [ ] Логи `app` и `caddy` уходят куда-то за пределы контейнера
      (`docker compose logs` теряется при `docker compose down`), если нужна
      история дольше времени жизни контейнера.
- [ ] Обновление: `git pull`, затем повторно
      `docker compose -f compose.prod.yml up -d --build` — `migrate`
      перезапустится и применит только новые миграции (goose идемпотентен),
      простоя дольше времени пересборки образа не будет благодаря
      `restart: unless-stopped` и graceful shutdown в `app` (сначала
      `/readyz` уходит в 503, потом закрывается приём HTTP).
