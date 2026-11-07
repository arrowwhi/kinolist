// Package worker выполняет фоновые задачи обогащения. Очередь живёт в
// Postgres, поэтому воркер — обычная горутина в том же процессе, а не
// отдельный сервис с брокером.
package worker

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/pkg/logger"
	"github.com/arrowwhi/kinolist/internal/pkg/metrics"
)

type (
	enrichmentUseCase interface {
		Claim(ctx context.Context, limit int) ([]enrichment.Job, error)
		Process(ctx context.Context, job enrichment.Job) (enrichment.Outcome, error)
	}

	notifier interface {
		NotifyOutcome(ctx context.Context, out enrichment.Outcome) error
	}
)

type Worker struct {
	uc       enrichmentUseCase
	notifier notifier
	metrics  *metrics.Metrics
	tracer   trace.Tracer
	cfg      config.Worker

	jobs chan enrichment.Job
	wg   sync.WaitGroup
}

func New(uc enrichmentUseCase, n notifier, m *metrics.Metrics, cfg config.Worker) *Worker {
	return &Worker{
		uc: uc, notifier: n, metrics: m, cfg: cfg,
		tracer: otel.Tracer("github.com/arrowwhi/kinolist/internal/transport/worker"),
		jobs:   make(chan enrichment.Job, cfg.Concurrency),
	}
}

// Run блокируется до отмены контекста, после чего дожидается задач,
// которые уже начали выполняться.
func (w *Worker) Run(ctx context.Context) error {
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go w.consume(ctx)
	}

	logger.Info(ctx).Int("concurrency", w.cfg.Concurrency).Msg("enrichment worker started")

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(w.jobs)
			w.wg.Wait()
			logger.Info(ctx).Msg("enrichment worker stopped")
			return nil
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	// Берём ровно столько, сколько влезет в буфер: иначе задачи ушли бы в
	// processing и застряли там до перезапуска.
	limit := w.cfg.Concurrency - len(w.jobs)
	if limit <= 0 {
		return
	}

	jobs, err := w.uc.Claim(ctx, limit)
	if err != nil {
		if ctx.Err() == nil {
			logger.Warn(ctx).Err(err).Msg("claim jobs failed")
		}
		return
	}

	for _, j := range jobs {
		select {
		case w.jobs <- j:
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) consume(ctx context.Context) {
	defer w.wg.Done()

	for job := range w.jobs {
		w.execute(ctx, job)
	}
}

func (w *Worker) execute(parent context.Context, job enrichment.Job) {
	// Задача не отменяется вместе с shutdown: начатую работу нужно
	// доводить до конца, иначе она останется в processing навсегда.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), w.cfg.JobTimeout)
	defer cancel()

	ctx = logger.With(ctx, "job_id", job.ID)
	ctx = logger.With(ctx, "user_id", job.UserID)
	ctx = logger.With(ctx, "job_kind", string(job.Kind))

	// Отдельный корневой спан на задачу: запросы к БД, TMDB и Telegram внутри
	// обработки складываются в один трейс.
	ctx, span := w.tracer.Start(ctx, "worker.enrichment_job", trace.WithAttributes(
		attribute.Int64("job.id", job.ID),
		attribute.String("job.kind", string(job.Kind)),
	))
	defer span.End()

	defer func() {
		if rec := recover(); rec != nil {
			span.SetStatus(codes.Error, "panic")
			logger.Error(ctx).Interface("panic", rec).Msg("panic while processing job")
		}
	}()

	start := time.Now()
	w.metrics.JobStarted(ctx)
	defer w.metrics.JobFinished(ctx)

	out, err := w.uc.Process(ctx, job)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.Error(ctx).Err(err).Msg("process job failed")
		w.metrics.ObserveJob(ctx, string(job.Kind), "error", time.Since(start).Seconds())
		return
	}

	outcome := outcomeLabel(out)
	span.SetAttributes(attribute.String("job.outcome", outcome))
	w.metrics.ObserveJob(ctx, string(job.Kind), outcome, time.Since(start).Seconds())

	// Пустой Outcome означает запланированный повтор — пользователя не трогаем.
	if out.Job.ID == 0 {
		return
	}

	if err := w.notifier.NotifyOutcome(ctx, out); err != nil {
		logger.Warn(ctx).Err(err).Msg("notify outcome failed")
	}
}

func outcomeLabel(out enrichment.Outcome) string {
	switch {
	case out.Job.ID == 0:
		return "retry"
	case out.Err != nil:
		return "failed"
	default:
		return string(out.Kind)
	}
}
