.DEFAULT_GOAL := help

MODULE      := github.com/arrowwhi/kinolist
BIN         := bin/app
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X '$(MODULE)/internal/app.version=$(VERSION)' \
	-X '$(MODULE)/internal/app.commit=$(COMMIT)' \
	-X '$(MODULE)/internal/app.buildTime=$(BUILD_TIME)'

GOOSE_DRIVER := postgres
MIGRATIONS   := ./migrations

-include .env
export

.PHONY: help
help: ## Список целей
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## Запустить сервис локально
	go run -ldflags "$(LDFLAGS)" ./cmd/app

.PHONY: dev
dev: ## Запустить с hot reload (air)
	air -c .air.toml

.PHONY: build
build: web-build ## Собрать бинарь (со встроенным фронтендом)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/app

.PHONY: lint
lint: ## Линтеры
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Форматирование
	gofumpt -l -w .

.PHONY: test
test: ## Unit-тесты
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Интеграционные тесты (testcontainers, нужен Docker)
	go test -race -count=1 -tags=integration ./internal/repository/...

.PHONY: generate
generate: ## Вся кодогенерация
	go generate ./...

.PHONY: mocks
mocks: ## Перегенерировать моки (gomock)
	go generate ./internal/usecase/...

.PHONY: migrate-up
migrate-up: ## Накатить миграции
	goose -dir $(MIGRATIONS) $(GOOSE_DRIVER) "$(POSTGRES_DSN)" up

.PHONY: migrate-down
migrate-down: ## Откатить последнюю миграцию
	goose -dir $(MIGRATIONS) $(GOOSE_DRIVER) "$(POSTGRES_DSN)" down

.PHONY: migrate-status
migrate-status: ## Статус миграций
	goose -dir $(MIGRATIONS) $(GOOSE_DRIVER) "$(POSTGRES_DSN)" status

.PHONY: migrate-create
migrate-create: ## Создать миграцию: make migrate-create name=add_something
	goose -dir $(MIGRATIONS) create $(name) sql

.PHONY: web-install
web-install: ## Установить зависимости фронтенда
	cd web && npm install

.PHONY: web-build
web-build: ## Собрать Mini App
	cd web && npm ci && npm run build

.PHONY: web-dev
web-dev: ## Vite dev-сервер
	cd web && npm run dev

.PHONY: docker-build
docker-build: ## Собрать прод-образ
	docker build -f deployments/docker/Dockerfile -t kinolist:$(VERSION) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_TIME=$(BUILD_TIME) .

.PHONY: compose-up
compose-up: ## Поднять локальное окружение целиком
	docker compose up -d --build

.PHONY: compose-infra
compose-infra: ## Поднять только инфраструктуру (для работы через air)
	docker compose --profile infra up -d

.PHONY: compose-down
compose-down: ## Остановить окружение
	docker compose down

.PHONY: compose-logs
compose-logs: ## Логи приложения
	docker compose logs -f app

.PHONY: tools
tools: ## Установить инструменты разработки
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install go.uber.org/mock/mockgen@latest
	go install mvdan.cc/gofumpt@latest
	go install github.com/air-verse/air@latest
