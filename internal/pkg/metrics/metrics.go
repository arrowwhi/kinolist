// Package metrics собирает RED-метрики HTTP, метрики воркера, внешних
// провайдеров и пула БД в одном реестре.
package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	namespace         = "kinolist"
	subsystemProvider = "provider"
)

type Metrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpErrors   *prometheus.CounterVec

	jobsProcessed *prometheus.CounterVec
	jobDuration   *prometheus.HistogramVec
	jobsInFlight  prometheus.Gauge

	providerRequests *prometheus.CounterVec
	providerDuration *prometheus.HistogramVec
}

func New() *Metrics {
	r := prometheus.NewRegistry()
	r.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		registry: r,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "http", Name: "requests_total",
			Help: "Total HTTP requests.",
		}, []string{"method", "route", "code"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "http", Name: "request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		httpErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "http", Name: "errors_total",
			Help: "HTTP responses with status >= 500.",
		}, []string{"method", "route"}),

		jobsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "worker", Name: "jobs_processed_total",
			Help: "Enrichment jobs processed by outcome.",
		}, []string{"kind", "outcome"}),
		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "worker", Name: "job_duration_seconds",
			Help:    "Enrichment job duration.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"kind"}),
		jobsInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "worker", Name: "jobs_in_flight",
			Help: "Enrichment jobs being processed right now.",
		}),

		providerRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystemProvider, Name: "requests_total",
			Help: "Calls to external providers by outcome.",
		}, []string{subsystemProvider, "outcome"}),
		providerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: subsystemProvider, Name: "request_duration_seconds",
			Help:    "External provider latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{subsystemProvider}),
	}

	r.MustRegister(
		m.httpRequests, m.httpDuration, m.httpErrors,
		m.jobsProcessed, m.jobDuration, m.jobsInFlight,
		m.providerRequests, m.providerDuration,
	)

	return m
}

func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

func (m *Metrics) ObserveHTTP(method, route string, code int, seconds float64) {
	m.httpRequests.WithLabelValues(method, route, statusLabel(code)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(seconds)
	if code >= 500 {
		m.httpErrors.WithLabelValues(method, route).Inc()
	}
}

func (m *Metrics) ObserveJob(kind, outcome string, seconds float64) {
	m.jobsProcessed.WithLabelValues(kind, outcome).Inc()
	m.jobDuration.WithLabelValues(kind).Observe(seconds)
}

func (m *Metrics) JobStarted()  { m.jobsInFlight.Inc() }
func (m *Metrics) JobFinished() { m.jobsInFlight.Dec() }

func (m *Metrics) ObserveProvider(provider, outcome string, seconds float64) {
	m.providerRequests.WithLabelValues(provider, outcome).Inc()
	m.providerDuration.WithLabelValues(provider).Observe(seconds)
}

// RegisterPool публикует состояние пула pgx: без него исчерпание соединений
// выглядит просто как рост latency без причины.
func (m *Metrics) RegisterPool(pool *pgxpool.Pool) {
	m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: "db", Name: "pool_acquired_conns",
		Help: "Connections currently acquired from the pool.",
	}, func() float64 { return float64(pool.Stat().AcquiredConns()) }))

	m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: "db", Name: "pool_idle_conns",
		Help: "Idle connections in the pool.",
	}, func() float64 { return float64(pool.Stat().IdleConns()) }))

	m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: "db", Name: "pool_total_conns",
		Help: "Total connections in the pool.",
	}, func() float64 { return float64(pool.Stat().TotalConns()) }))

	m.registry.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "db", Name: "pool_canceled_acquires_total",
		Help: "Acquires cancelled while waiting for a free connection.",
	}, func() float64 { return float64(pool.Stat().CanceledAcquireCount()) }))
}

// statusLabel схлопывает коды в классы: иначе кардинальность метрики
// растёт вместе с числом статусов, а пользы от этого нет.
func statusLabel(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}
