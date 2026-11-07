package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRedactURL(t *testing.T) {
	u, _ := url.Parse("https://user:pass@api.example.com/3/search?query=dune&api_key=SECRET&AccessToken=T")

	got := RedactURL(u)
	for _, leak := range []string{"SECRET", "pass", "=T"} {
		if strings.Contains(got, leak) {
			t.Fatalf("secret %q leaked: %s", leak, got)
		}
	}

	if !strings.Contains(got, "query=dune") {
		t.Fatalf("regular params must stay: %s", got)
	}
}

func TestClientTracesWithoutSecrets(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))

	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	var traceparent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceparent = r.Header.Get("Traceparent")
	}))
	defer srv.Close()

	ctx, parent := tp.Tracer("test").Start(context.Background(), "parent")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/movie?api_key=SECRET", nil)

	resp, err := New("tmdb", time.Second).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	parent.End()

	if !strings.Contains(traceparent, parent.SpanContext().TraceID().String()) {
		t.Fatalf("trace context not propagated: %q", traceparent)
	}

	var found bool

	for _, s := range spans.Ended() {
		if s.Name() != "tmdb GET" {
			continue
		}

		found = true

		for _, a := range s.Attributes() {
			if strings.Contains(a.Value.String(), "SECRET") {
				t.Fatalf("attribute %s leaks api key: %s", a.Key, a.Value.String())
			}
		}
	}

	if !found {
		t.Fatal("client span not recorded")
	}
}
