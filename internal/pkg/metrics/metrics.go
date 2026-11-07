// Package metrics — прикладные метрики kinolist поверх OpenTelemetry.
//
// Инфраструктурные метрики (HTTP, пул и запросы БД, Telegram Bot API и обновления
// бота, рантайм Go) снимают пакеты github.com/arrowwhi/infrastructure. Здесь — только
// то, что знает лишь kinolist: задачи обогащения.
package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "kinolist"

type Metrics struct {
	jobsProcessed metric.Int64Counter
	jobDuration   metric.Float64Histogram
	jobsInFlight  metric.Int64UpDownCounter
}

// New создаёт инструменты в глобальном MeterProvider (его настраивает telegram/app).
func New() (*Metrics, error) {
	meter := otel.Meter(meterName)

	jobsProcessed, err := meter.Int64Counter("kinolist.worker.jobs.processed",
		metric.WithDescription("Enrichment jobs processed by kind and outcome"),
		metric.WithUnit("{job}"))
	if err != nil {
		return nil, fmt.Errorf("jobs processed counter: %w", err)
	}

	jobDuration, err := meter.Float64Histogram("kinolist.worker.job.duration",
		metric.WithDescription("Enrichment job duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.25, .5, 1, 2.5, 5, 10, 30, 60))
	if err != nil {
		return nil, fmt.Errorf("job duration histogram: %w", err)
	}

	jobsInFlight, err := meter.Int64UpDownCounter("kinolist.worker.jobs.in_flight",
		metric.WithDescription("Enrichment jobs being processed right now"),
		metric.WithUnit("{job}"))
	if err != nil {
		return nil, fmt.Errorf("jobs in flight counter: %w", err)
	}

	return &Metrics{jobsProcessed: jobsProcessed, jobDuration: jobDuration, jobsInFlight: jobsInFlight}, nil
}

func (m *Metrics) ObserveJob(ctx context.Context, kind, outcome string, seconds float64) {
	m.jobsProcessed.Add(ctx, 1, metric.WithAttributes(
		attribute.String("kind", kind),
		attribute.String("outcome", outcome),
	))
	m.jobDuration.Record(ctx, seconds, metric.WithAttributes(attribute.String("kind", kind)))
}

func (m *Metrics) JobStarted(ctx context.Context)  { m.jobsInFlight.Add(ctx, 1) }
func (m *Metrics) JobFinished(ctx context.Context) { m.jobsInFlight.Add(ctx, -1) }
