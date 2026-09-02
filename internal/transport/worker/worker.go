// Package worker выполняет фоновые задачи обогащения. Очередь живёт в
// Postgres, поэтому воркер — обычная горутина в том же процессе, а не
// отдельный сервис с брокером.
package worker

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

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
	log      *zap.Logger
	metrics  *metrics.Metrics
	cfg      config.Worker

	jobs chan enrichment.Job
	wg   sync.WaitGroup
}

func New(uc enrichmentUseCase, n notifier, log *zap.Logger, m *metrics.Metrics, cfg config.Worker) *Worker {
	return &Worker{
		uc: uc, notifier: n, log: log, metrics: m, cfg: cfg,
		jobs: make(chan enrichment.Job, cfg.Concurrency),
	}
}

// Run блокируется до отмены контекста, после чего дожидается задач,
// которые уже начали выполняться.
func (w *Worker) Run(ctx context.Context) error {
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go w.consume(ctx)
	}

	w.log.Info("enrichment worker started", zap.Int("concurrency", w.cfg.Concurrency))

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(w.jobs)
			w.wg.Wait()
			w.log.Info("enrichment worker stopped")
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
			w.log.Warn("claim jobs failed", zap.Error(err))
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
	defer func() {
		if rec := recover(); rec != nil {
			w.log.Error("panic while processing job",
				zap.Any("panic", rec), zap.Int64("job_id", job.ID))
		}
	}()

	// Задача не отменяется вместе с shutdown: начатую работу нужно
	// доводить до конца, иначе она останется в processing навсегда.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), w.cfg.JobTimeout)
	defer cancel()

	ctx = logger.Into(ctx, w.log.With(
		zap.Int64("job_id", job.ID),
		zap.Int64("user_id", job.UserID),
		zap.String("job_kind", string(job.Kind)),
	))

	start := time.Now()
	w.metrics.JobStarted()
	defer w.metrics.JobFinished()

	out, err := w.uc.Process(ctx, job)
	if err != nil {
		logger.From(ctx).Error("process job failed", zap.Error(err))
		w.metrics.ObserveJob(string(job.Kind), "error", time.Since(start).Seconds())
		return
	}

	w.metrics.ObserveJob(string(job.Kind), outcomeLabel(out), time.Since(start).Seconds())

	// Пустой Outcome означает запланированный повтор — пользователя не трогаем.
	if out.Job.ID == 0 {
		return
	}

	if err := w.notifier.NotifyOutcome(ctx, out); err != nil {
		logger.From(ctx).Warn("notify outcome failed", zap.Error(err))
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
