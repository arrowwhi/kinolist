// Package httpclient собирает HTTP-клиенты для внешних API (TMDB, Кинопоиск,
// скрейпер Instagram, LLM) с трейсами и метриками OpenTelemetry.
//
// Каждый запрос — дочерний спан текущего трейса (заголовок traceparent уходит
// наружу), плюс метрики http.client.request.duration по хосту, методу и коду
// ответа. Секреты из query (например, api_key TMDB) в трейс не попадают.
package httpclient

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const redacted = "REDACTED"

// sensitiveParams — query-параметры, значения которых нельзя писать в трейсы.
var sensitiveParams = []string{"key", "token", "secret", "password", "signature", "auth"}

// New возвращает клиент с таймаутом. name — имя провайдера в имени спана ("tmdb GET").
func New(name string, timeout time.Duration) *http.Client {
	base := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // стандартный транспорт всегда *http.Transport

	return &http.Client{
		Timeout: timeout,
		Transport: otelhttp.NewTransport(redactingTransport{next: base},
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return name + " " + r.Method
			}),
		),
	}
}

// redactingTransport вызывается otelhttp уже внутри спана запроса и перезаписывает
// атрибут url.full версией без секретов (последнее значение атрибута побеждает).
type redactingTransport struct {
	next http.RoundTripper
}

func (t redactingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if span := trace.SpanFromContext(r.Context()); span.IsRecording() {
		span.SetAttributes(attribute.String("url.full", RedactURL(r.URL)))
	}

	return t.next.RoundTrip(r)
}

// RedactURL возвращает URL без userinfo и со скрытыми значениями чувствительных параметров.
func RedactURL(u *url.URL) string {
	clean := *u
	clean.User = nil

	q := clean.Query()
	changed := false

	for name := range q {
		lower := strings.ToLower(name)
		for _, s := range sensitiveParams {
			if strings.Contains(lower, s) {
				q.Set(name, redacted)
				changed = true

				break
			}
		}
	}

	if changed {
		clean.RawQuery = q.Encode()
	}

	return clean.String()
}
