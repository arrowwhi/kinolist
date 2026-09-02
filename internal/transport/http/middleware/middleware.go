package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/arrowwhi/kinolist/internal/pkg/logger"
	"github.com/arrowwhi/kinolist/internal/pkg/metrics"
)

const requestIDHeader = "X-Request-Id"

type requestIDKey struct{}

// RequestID проставляет идентификатор запроса и кладёт его в логгер контекста,
// чтобы все записи одного запроса можно было связать.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(requestIDHeader, id)

		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		ctx = logger.With(ctx, zap.String("request_id", id))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Logger пишет ровно одну запись на запрос: метод, маршрут, код, длительность.
func Logger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			ctx := logger.Into(r.Context(), log.With(zap.String("request_id", RequestIDFrom(r.Context()))))
			next.ServeHTTP(rw, r.WithContext(ctx))

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.status),
				zap.Duration("duration", time.Since(start)),
			}
			// Ни query, ни тело не логируются: там оказываются initData
			// и другие чувствительные значения.
			if uid, ok := UserID(r.Context()); ok {
				fields = append(fields, zap.Int64("user_id", uid))
			}

			switch {
			case rw.status >= http.StatusInternalServerError:
				logger.From(r.Context()).Error("request failed", fields...)
			default:
				logger.From(r.Context()).Info("request", fields...)
			}
		})
	}
}

// Recovery не даёт панике в одном хендлере уронить процесс целиком.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { //nolint:contextcheck // r.Context() захвачен замыканием, отдельный параметр не нужен
			if rec := recover(); rec != nil {
				logger.From(r.Context()).Error("panic recovered",
					zap.Any("panic", rec),
					zap.String("path", r.URL.Path),
					zap.Stack("stack"),
				)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"internal","message":"Что-то пошло не так"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func Metrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			// Шаблон маршрута, а не конкретный путь: иначе каждый id
			// создавал бы новую серию метрики.
			route := "unmatched"
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				route = rctx.RoutePattern()
			}

			m.ObserveHTTP(r.Method, route, rw.status, time.Since(start).Seconds())
		})
	}
}

func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
