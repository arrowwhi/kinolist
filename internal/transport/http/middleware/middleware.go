package middleware

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"github.com/arrowwhi/kinolist/internal/pkg/logger"
)

const requestIDHeader = "X-Request-Id"

type requestIDKey struct{}

// RequestID проставляет идентификатор запроса и кладёт его в поля логов контекста,
// чтобы все записи одного запроса можно было связать. trace_id в логи добавляет
// инфраструктура, если запрос трассируется.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(requestIDHeader, id)

		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		ctx = logger.With(ctx, "request_id", id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Logger пишет ровно одну запись на запрос: метод, маршрут, код, длительность.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		event := logger.Info(r.Context())
		if rw.status >= http.StatusInternalServerError {
			event = logger.Error(r.Context())
		}

		event = event.
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Dur("duration", time.Since(start))

		// Ни query, ни тело не логируются: там оказываются initData
		// и другие чувствительные значения.
		if uid, ok := UserID(r.Context()); ok {
			event = event.Int64("user_id", uid)
		}

		event.Msg("request")
	})
}

// Recovery не даёт панике в одном хендлере уронить процесс целиком.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { //nolint:contextcheck // r.Context() захвачен замыканием, отдельный параметр не нужен
			if rec := recover(); rec != nil {
				logger.Error(r.Context()).
					Interface("panic", rec).
					Str("path", r.URL.Path).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"internal","message":"Что-то пошло не так"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
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

// Unwrap нужен http.ResponseController и otelhttp.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
