//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func TestJobLifecycle(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewJobRepository(newManager(t))
	u := seedUser(t, 400)

	created, err := repo.Create(ctx, enrichment.Job{
		UserID: u.ID, Kind: enrichment.KindReel, Status: enrichment.StatusPending,
		SourceURL: "https://instagram.com/reel/A/", ChatID: 777, RunAfter: time.Now(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 || created.ChatID != 777 {
		t.Fatalf("unexpected created job: %+v", created)
	}

	created.Status = enrichment.StatusConfirming
	created.QueryTitle = "Дюна"
	created.QueryYear = 2021
	created.Candidates = []movie.Candidate{{
		Provider: movie.ProviderTMDB, ExternalID: 438631,
		Title: "Дюна", OriginalTitle: "Dune", Year: 2021, GenreTMDBIDs: []int32{878},
	}}
	if err := repo.Update(ctx, created); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetForUser(ctx, u.ID, created.ID)
	if err != nil {
		t.Fatalf("get for user: %v", err)
	}
	if got.QueryYear != 2021 || len(got.Candidates) != 1 {
		t.Fatalf("job round-trip lost data: %+v", got)
	}
	if got.Candidates[0].ExternalID != 438631 || len(got.Candidates[0].GenreTMDBIDs) != 1 {
		t.Fatalf("candidates round-trip lost data: %+v", got.Candidates[0])
	}

	// Чужую задачу по угаданному id достать нельзя.
	stranger := seedUser(t, 401)
	if _, err := repo.GetForUser(ctx, stranger.ID, created.ID); !errors.Is(err, enrichment.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound for another user's job", err)
	}
}

func TestClaimSkipsFutureAndNonPendingJobs(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewJobRepository(newManager(t))
	u := seedUser(t, 500)

	mustCreate := func(status enrichment.Status, runAfter time.Time) enrichment.Job {
		t.Helper()
		j, err := repo.Create(ctx, enrichment.Job{
			UserID: u.ID, Kind: enrichment.KindTitle, Status: status,
			QueryTitle: "Дюна", RunAfter: runAfter,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return j
	}

	ready := mustCreate(enrichment.StatusPending, time.Now().Add(-time.Minute))
	mustCreate(enrichment.StatusPending, time.Now().Add(time.Hour)) // отложена ретраем
	mustCreate(enrichment.StatusConfirming, time.Now().Add(-time.Hour))

	claimed, err := repo.Claim(ctx, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != ready.ID {
		t.Fatalf("claimed = %+v, want only the ready job %d", claimed, ready.ID)
	}
	if claimed[0].Status != enrichment.StatusProcessing {
		t.Fatalf("status = %q, want processing", claimed[0].Status)
	}

	// Повторный вызов не должен выдать ту же задачу второй раз.
	again, err := repo.Claim(ctx, 10)
	if err != nil {
		t.Fatalf("claim again: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("claimed %d jobs on the second call, want 0", len(again))
	}
}

// TestClaimIsExclusiveUnderConcurrency — главный тест очереди: несколько
// воркеров разбирают её параллельно, и ни одна задача не должна достаться
// двоим сразу.
func TestClaimIsExclusiveUnderConcurrency(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewJobRepository(newManager(t))
	u := seedUser(t, 600)

	const jobCount = 20
	for i := 0; i < jobCount; i++ {
		if _, err := repo.Create(ctx, enrichment.Job{
			UserID: u.ID, Kind: enrichment.KindTitle, Status: enrichment.StatusPending,
			QueryTitle: "Фильм", RunAfter: time.Now().Add(-time.Second),
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	const workers = 4

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = make(map[int64]int)
		errs []error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				jobs, err := repo.Claim(ctx, 3)
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
					return
				}
				if len(jobs) == 0 {
					return
				}
				mu.Lock()
				for _, j := range jobs {
					seen[j.ID]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("claim errors: %v", errs)
	}
	if len(seen) != jobCount {
		t.Fatalf("claimed %d distinct jobs, want %d", len(seen), jobCount)
	}
	for id, times := range seen {
		if times != 1 {
			t.Fatalf("job %d was claimed %d times, want exactly 1", id, times)
		}
	}
}

func TestSessionRoundTrip(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewJobRepository(newManager(t))
	u := seedUser(t, 700)

	// Пустая сессия — не ошибка, а состояние idle.
	empty, err := repo.GetSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("get empty session: %v", err)
	}
	if empty.State != enrichment.SessionIdle {
		t.Fatalf("state = %q, want idle", empty.State)
	}

	job, err := repo.Create(ctx, enrichment.Job{
		UserID: u.ID, Kind: enrichment.KindTitle, Status: enrichment.StatusConfirming,
		QueryTitle: "Дюна", RunAfter: time.Now(),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := repo.SaveSession(ctx, enrichment.Session{
		UserID: u.ID, State: enrichment.SessionAwaitingPick, JobID: job.ID, ChatID: 888, MessageID: 42,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}

	got, err := repo.GetSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.JobID != job.ID || got.MessageID != 42 || got.State != enrichment.SessionAwaitingPick {
		t.Fatalf("session round-trip lost data: %+v", got)
	}

	if err := repo.ClearSession(ctx, u.ID); err != nil {
		t.Fatalf("clear session: %v", err)
	}
	cleared, err := repo.GetSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("get cleared session: %v", err)
	}
	if cleared.State != enrichment.SessionIdle {
		t.Fatalf("state = %q, want idle after clear", cleared.State)
	}
}
