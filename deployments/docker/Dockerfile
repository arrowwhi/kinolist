# --- сборка фронтенда ---
FROM node:24-alpine AS web
WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# --- сборка бинаря ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# Зависимости отдельным слоем: они меняются реже исходников.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/web/dist ./web/dist

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
      -X 'github.com/arrowwhi/kinolist/internal/app.version=${VERSION}' \
      -X 'github.com/arrowwhi/kinolist/internal/app.commit=${COMMIT}' \
      -X 'github.com/arrowwhi/kinolist/internal/app.buildTime=${BUILD_TIME}'" \
    -o /out/app ./cmd/app

# --- финальный образ ---
FROM alpine:3.22

# ca-certificates нужны для HTTPS к Telegram, TMDB и скраперу.
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 -S app \
    && adduser -u 10001 -S -G app app

COPY --from=build /out/app /usr/local/bin/app

USER app:app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/app"]
