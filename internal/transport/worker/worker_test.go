package worker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/pkg/metrics"
	"github.com/arrowwhi/kinolist/internal/transport/worker"
)

type stubUseCase struct {
	mu         sync.Mutex
	toClaim    []enrichment.Job
	processed  []int64
	processErr error
	outcome    func(enrichment.Job) enrichment.Outcome
}

func (s *stubUseCase) Claim(_ context.Context, limit int) ([]enrichment.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.toClaim) == 0 {
		return nil, nil
	}
	if limit > len(s.toClaim) {
		limit = len(s.toClaim)
	}
	batch := s.toClaim[:limit]
	s.toClaim = s.toClaim[limit:]
	return batch, nil
}

func (s *stubUseCase) Process(_ context.Context, job enrichment.Job) (enrichment.Outcome, error) {
	s.mu.Lock()
	s.processed = append(s.processed, job.ID)
	s.mu.Unlock()

	if s.processErr != nil {
		return enrichment.Outcome{}, s.processErr
	}
	if s.outcome != nil {
		return s.outcome(job), nil
	}
	return enrichment.Outcome{Job: job, Kind: movie.MatchExact}, nil
}

func (s *stubUseCase) processedIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int64, len(s.processed))
	copy(out, s.processed)
	return out
}

type stubNotifier struct {
	mu       sync.Mutex
	notified []int64
	err      error
}

func (s *stubNotifier) NotifyOutcome(_ context.Context, out enrichment.Outcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notified = append(s.notified, out.Job.ID)
	return s.err
}

func (s *stubNotifier) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.notified)
}

func testConfig() config.Worker {
	return config.Worker{
		Concurrency: 2, PollInterval: 10 * time.Millisecond,
		JobTimeout: time.Second, MaxAttempts: 3,
	}
}

func runUntil(t *testing.T, w *worker.Worker, done func() bool) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- w.Run(ctx) }()

	deadline := time.After(3 * time.Second)
	for !done() {
		select {
		case <-deadline:
			cancel()
			t.Fatal("worker did not finish the expected work in time")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("worker returned an error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop on context cancellation")
	}
}

func TestWorkerProcessesAndNotifies(t *testing.T) {
	t.Parallel()

	uc := &stubUseCase{toClaim: []enrichment.Job{
		{ID: 1, UserID: 7, ChatID: 5, Kind: enrichment.KindTitle},
		{ID: 2, UserID: 7, ChatID: 5, Kind: enrichment.KindReel},
		{ID: 3, UserID: 7, ChatID: 5, Kind: enrichment.KindTitle},
	}}
	n := &stubNotifier{}

	w := worker.New(uc, n, zap.NewNop(), metrics.New(), testConfig())
	runUntil(t, w, func() bool { return n.count() == 3 })

	if got := len(uc.processedIDs()); got != 3 {
		t.Fatalf("processed %d jobs, want 3", got)
	}
}

func TestWorkerDoesNotNotifyOnScheduledRetry(t *testing.T) {
	t.Parallel()

	uc := &stubUseCase{
		toClaim: []enrichment.Job{{ID: 1, Kind: enrichment.KindReel}},
		// Пустой Outcome — договорённость usecase о запланированном повторе.
		outcome: func(enrichment.Job) enrichment.Outcome { return enrichment.Outcome{} },
	}
	n := &stubNotifier{}

	w := worker.New(uc, n, zap.NewNop(), metrics.New(), testConfig())
	runUntil(t, w, func() bool { return len(uc.processedIDs()) == 1 })

	if n.count() != 0 {
		t.Fatalf("notified %d times on a retry, want 0", n.count())
	}
}

func TestWorkerSurvivesProcessingErrors(t *testing.T) {
	t.Parallel()

	uc := &stubUseCase{
		toClaim:    []enrichment.Job{{ID: 1, Kind: enrichment.KindTitle}, {ID: 2, Kind: enrichment.KindTitle}},
		processErr: errors.New("database is on fire"),
	}
	n := &stubNotifier{}

	w := worker.New(uc, n, zap.NewNop(), metrics.New(), testConfig())
	runUntil(t, w, func() bool { return len(uc.processedIDs()) == 2 })

	if n.count() != 0 {
		t.Fatalf("notified %d times despite processing errors, want 0", n.count())
	}
}

func TestWorkerStopsCleanlyWithNoWork(t *testing.T) {
	t.Parallel()

	w := worker.New(&stubUseCase{}, &stubNotifier{}, zap.NewNop(), metrics.New(), testConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := w.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}
