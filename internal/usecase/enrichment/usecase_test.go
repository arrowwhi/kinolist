package enrichment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	domain "github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	uc "github.com/arrowwhi/kinolist/internal/usecase/enrichment"
	"github.com/arrowwhi/kinolist/internal/usecase/enrichment/mocks"
)

type deps struct {
	jobs      *mocks.MockjobRepository
	movies    *mocks.MockmovieRepository
	fetcher   *mocks.MockreelFetcher
	extractor *mocks.MocktitleExtractor
	primary   *mocks.Mockcatalog
	fallback  *mocks.Mockcatalog
	tx        *mocks.MocktxManager
}

func newUseCase(t *testing.T, withFallback bool) (*uc.UseCase, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)

	d := deps{
		jobs:      mocks.NewMockjobRepository(ctrl),
		movies:    mocks.NewMockmovieRepository(ctrl),
		fetcher:   mocks.NewMockreelFetcher(ctrl),
		extractor: mocks.NewMocktitleExtractor(ctrl),
		primary:   mocks.NewMockcatalog(ctrl),
		fallback:  mocks.NewMockcatalog(ctrl),
		tx:        mocks.NewMocktxManager(ctrl),
	}

	cfg := uc.Deps{
		Jobs: d.jobs, Movies: d.movies, Fetcher: d.fetcher, Extractor: d.extractor,
		Primary: d.primary, Tx: d.tx, MaxAttempts: 3, RetryDelay: time.Second,
	}
	if withFallback {
		cfg.Fallback = d.fallback
	}

	return uc.New(cfg), d
}

func candidate(id int32, title, original string, year int16) movie.Candidate {
	return movie.Candidate{
		Provider: movie.ProviderTMDB, ExternalID: id,
		Title: title, OriginalTitle: original, Year: year,
		GenreTMDBIDs: []int32{18},
	}
}

// --- Enqueue ---

func TestEnqueue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		fetcherOn bool
		setup     func(d deps)
		wantKind  domain.Kind
		wantURL   string
		wantTitle string
		wantErr   error
	}{
		{
			name: "reel url is queued as reel job", input: "https://instagram.com/reel/Abc123/", fetcherOn: true,
			setup: func(d deps) {
				d.jobs.EXPECT().Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, j domain.Job) (domain.Job, error) {
						j.ID = 1
						return j, nil
					})
			},
			wantKind: domain.KindReel, wantURL: "https://instagram.com/reel/Abc123/",
		},
		{
			name: "plain title is queued as title job", input: "Дюна",
			setup: func(d deps) {
				d.jobs.EXPECT().Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, j domain.Job) (domain.Job, error) {
						j.ID = 2
						return j, nil
					})
			},
			wantKind: domain.KindTitle, wantTitle: "Дюна",
		},
		{
			name:  "reel rejected when fetcher is not configured",
			input: "https://instagram.com/reel/Abc123/", fetcherOn: false,
			setup:   func(d deps) {},
			wantErr: domain.ErrFetcherDisabled,
		},
		{
			name: "foreign link is rejected", input: "https://tiktok.com/video/1",
			setup: func(d deps) {}, wantErr: domain.ErrInvalidReelURL,
		},
		{
			name: "empty input is rejected", input: "  ",
			setup: func(d deps) {}, wantErr: domain.ErrEmptyInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, d := newUseCase(t, false)
			d.fetcher.EXPECT().Enabled().Return(tt.fetcherOn).AnyTimes()
			tt.setup(d)

			job, err := u.Enqueue(context.Background(), 10, 20, tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if job.Kind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", job.Kind, tt.wantKind)
			}
			if job.SourceURL != tt.wantURL {
				t.Fatalf("source url = %q, want %q", job.SourceURL, tt.wantURL)
			}
			if job.QueryTitle != tt.wantTitle {
				t.Fatalf("query title = %q, want %q", job.QueryTitle, tt.wantTitle)
			}
			if job.Status != domain.StatusPending {
				t.Fatalf("status = %q, want pending", job.Status)
			}
		})
	}
}

// --- Process ---

func TestProcessTitleJob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		job        domain.Job
		setup      func(d deps)
		wantMatch  movie.MatchKind
		wantCands  int
		wantStatus domain.Status
		wantErrIs  error
		wantSilent bool
	}{
		{
			name: "confident match goes to confirmation",
			job:  domain.Job{ID: 1, UserID: 7, ChatID: 99, Kind: domain.KindTitle, QueryTitle: "Дюна", QueryYear: 2021},
			setup: func(d deps) {
				d.primary.EXPECT().Search(gomock.Any(), "Дюна", int16(2021)).
					Return([]movie.Candidate{candidate(438631, "Дюна", "Dune", 2021)}, nil)
				d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				d.jobs.EXPECT().SaveSession(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantMatch: movie.MatchExact, wantCands: 1, wantStatus: domain.StatusConfirming,
		},
		{
			name: "ambiguous match offers a list",
			job:  domain.Job{ID: 2, UserID: 7, ChatID: 99, Kind: domain.KindTitle, QueryTitle: "Дюна"},
			setup: func(d deps) {
				d.primary.EXPECT().Search(gomock.Any(), "Дюна", int16(0)).
					Return([]movie.Candidate{
						candidate(1, "Дюна", "Dune", 1984),
						candidate(2, "Дюна", "Dune", 2021),
					}, nil)
				d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				d.jobs.EXPECT().SaveSession(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantMatch: movie.MatchAmbiguous, wantCands: 2, wantStatus: domain.StatusConfirming,
		},
		{
			name: "empty catalog result fails the job without retry",
			job:  domain.Job{ID: 3, UserID: 7, ChatID: 99, Kind: domain.KindTitle, QueryTitle: "Абырвалг"},
			setup: func(d deps) {
				d.primary.EXPECT().Search(gomock.Any(), "Абырвалг", int16(0)).Return(nil, nil)
				d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)
			},
			wantMatch: movie.MatchNone, wantStatus: domain.StatusFailed, wantErrIs: movie.ErrNoCandidates,
		},
		{
			name: "provider failure is retried silently while attempts remain",
			job:  domain.Job{ID: 4, UserID: 7, ChatID: 99, Kind: domain.KindTitle, QueryTitle: "Дюна", Attempts: 0},
			setup: func(d deps) {
				d.primary.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, movie.ErrProviderUnavailable)
				d.primary.EXPECT().Provider().Return(movie.ProviderTMDB)
				d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, j domain.Job) error {
						if j.Status != domain.StatusPending {
							t.Fatalf("status = %q, want pending for retry", j.Status)
						}
						if j.Attempts != 1 {
							t.Fatalf("attempts = %d, want 1", j.Attempts)
						}
						if !j.RunAfter.After(time.Now()) {
							t.Fatal("RunAfter must be in the future for a retry")
						}
						return nil
					})
			},
			wantSilent: true,
		},
		{
			name: "provider failure on last attempt fails the job",
			job:  domain.Job{ID: 5, UserID: 7, ChatID: 99, Kind: domain.KindTitle, QueryTitle: "Дюна", Attempts: 2},
			setup: func(d deps) {
				d.primary.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, movie.ErrProviderUnavailable)
				d.primary.EXPECT().Provider().Return(movie.ProviderTMDB)
				d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)
			},
			wantMatch: movie.MatchNone, wantStatus: domain.StatusFailed, wantErrIs: movie.ErrProviderUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, d := newUseCase(t, false)
			tt.setup(d)

			out, err := u.Process(context.Background(), tt.job)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantSilent {
				if out.Job.ID != 0 || out.Err != nil {
					t.Fatalf("retry must not notify the user, got %+v", out)
				}
				return
			}

			if out.Kind != tt.wantMatch {
				t.Fatalf("match kind = %q, want %q", out.Kind, tt.wantMatch)
			}
			if len(out.Candidates) != tt.wantCands {
				t.Fatalf("candidates = %d, want %d", len(out.Candidates), tt.wantCands)
			}
			if out.Job.Status != tt.wantStatus {
				t.Fatalf("job status = %q, want %q", out.Job.Status, tt.wantStatus)
			}
			if tt.wantErrIs != nil && !errors.Is(out.Err, tt.wantErrIs) {
				t.Fatalf("outcome err = %v, want %v", out.Err, tt.wantErrIs)
			}
		})
	}
}

func TestProcessReelJob(t *testing.T) {
	t.Parallel()

	t.Run("caption yields a title and a confident match", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)
		content := domain.ReelContent{Caption: "топ драм: Дюна (2021)"}

		d.fetcher.EXPECT().Enabled().Return(true)
		d.extractor.EXPECT().Enabled().Return(true)
		d.fetcher.EXPECT().Fetch(gomock.Any(), "https://instagram.com/reel/A/").Return(content, nil)
		d.extractor.EXPECT().Extract(gomock.Any(), content).
			Return(domain.ExtractedTitle{Title: "Дюна", Year: 2021, Confidence: 0.9}, nil)
		d.primary.EXPECT().Search(gomock.Any(), "Дюна", int16(2021)).
			Return([]movie.Candidate{candidate(438631, "Дюна", "Dune", 2021)}, nil)
		d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
		d.jobs.EXPECT().SaveSession(gomock.Any(), gomock.Any()).Return(nil)

		out, err := u.Process(context.Background(), domain.Job{
			ID: 1, UserID: 7, ChatID: 99, Kind: domain.KindReel,
			SourceURL: "https://instagram.com/reel/A/",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Kind != movie.MatchExact {
			t.Fatalf("match kind = %q, want exact", out.Kind)
		}
		if out.Job.QueryTitle != "Дюна" || out.Job.QueryYear != 2021 {
			t.Fatalf("extracted query not persisted on job: %+v", out.Job)
		}
	})

	t.Run("unrecognized caption fails without retry", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)

		d.fetcher.EXPECT().Enabled().Return(true)
		d.extractor.EXPECT().Enabled().Return(true)
		d.fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).
			Return(domain.ReelContent{Caption: "подпишись 🔥"}, nil)
		d.extractor.EXPECT().Extract(gomock.Any(), gomock.Any()).
			Return(domain.ExtractedTitle{}, nil)
		d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, j domain.Job) error {
				if j.Status != domain.StatusFailed {
					t.Fatalf("status = %q, want failed (unrecognized title is not retryable)", j.Status)
				}
				return nil
			})
		d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)

		out, err := u.Process(context.Background(), domain.Job{
			ID: 1, UserID: 7, ChatID: 99, Kind: domain.KindReel, SourceURL: "https://instagram.com/reel/A/",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !errors.Is(out.Err, domain.ErrTitleNotRecognized) {
			t.Fatalf("outcome err = %v, want ErrTitleNotRecognized", out.Err)
		}
	})
}

func TestProcessFallsBackToSecondaryCatalog(t *testing.T) {
	t.Parallel()

	u, d := newUseCase(t, true)

	d.primary.EXPECT().Search(gomock.Any(), "Брат", int16(1997)).Return(nil, nil)
	d.fallback.EXPECT().Search(gomock.Any(), "Брат", int16(1997)).
		Return([]movie.Candidate{{
			Provider: movie.ProviderKinopoisk, ExternalID: 41519,
			Title: "Брат", OriginalTitle: "Брат", Year: 1997,
		}}, nil)
	d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	d.jobs.EXPECT().SaveSession(gomock.Any(), gomock.Any()).Return(nil)

	out, err := u.Process(context.Background(), domain.Job{
		ID: 1, UserID: 7, ChatID: 99, Kind: domain.KindTitle, QueryTitle: "Брат", QueryYear: 1997,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != movie.MatchExact {
		t.Fatalf("match kind = %q, want exact", out.Kind)
	}
	if out.Candidates[0].Provider != movie.ProviderKinopoisk {
		t.Fatalf("provider = %q, want kinopoisk", out.Candidates[0].Provider)
	}
}

// --- Confirm ---

func TestConfirm(t *testing.T) {
	t.Parallel()

	confirming := domain.Job{
		ID: 1, UserID: 7, Kind: domain.KindReel, Status: domain.StatusConfirming,
		SourceURL:  "https://instagram.com/reel/A/",
		Candidates: []movie.Candidate{candidate(438631, "Дюна", "Dune", 2021)},
	}

	t.Run("stores the movie and closes the job", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)

		d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(confirming, nil)
		d.tx.EXPECT().BeginTransaction(gomock.Any()).DoAndReturn(
			func(ctx context.Context) (context.Context, error) { return ctx, nil })
		d.tx.EXPECT().FinishTransaction(gomock.Any(), gomock.Any())
		d.movies.EXPECT().GenresByTMDBIDs(gomock.Any(), []int32{18}).
			Return([]movie.Genre{{ID: 7, TMDBID: 18, NameRU: "драма"}}, nil)
		d.movies.EXPECT().Upsert(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, m movie.Movie) (movie.Movie, error) {
				if m.TMDBID != 438631 {
					t.Fatalf("tmdb id = %d, want 438631", m.TMDBID)
				}
				m.ID = 55
				return m, nil
			})
		d.movies.EXPECT().Add(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, um movie.UserMovie) (movie.UserMovie, error) {
				if um.Source != movie.SourceReel {
					t.Fatalf("source = %q, want reel", um.Source)
				}
				if um.Status != movie.StatusWantToWatch {
					t.Fatalf("status = %q, want want_to_watch", um.Status)
				}
				um.ID = 100
				return um, nil
			})
		d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, j domain.Job) error {
				if j.Status != domain.StatusDone {
					t.Fatalf("job status = %q, want done", j.Status)
				}
				return nil
			})
		d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)

		got, err := u.Confirm(context.Background(), 7, 1, movie.ProviderTMDB, 438631)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != 100 {
			t.Fatalf("user movie id = %d, want 100", got.ID)
		}
	})

	t.Run("duplicate closes the job and reports it", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)

		d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(confirming, nil)
		d.tx.EXPECT().BeginTransaction(gomock.Any()).DoAndReturn(
			func(ctx context.Context) (context.Context, error) { return ctx, nil })
		d.tx.EXPECT().FinishTransaction(gomock.Any(), gomock.Any())
		d.movies.EXPECT().GenresByTMDBIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
		d.movies.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(movie.Movie{ID: 55}, nil)
		d.movies.EXPECT().Add(gomock.Any(), gomock.Any()).Return(movie.UserMovie{}, movie.ErrAlreadyInCollection)
		d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
		d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)

		_, err := u.Confirm(context.Background(), 7, 1, movie.ProviderTMDB, 438631)
		if !errors.Is(err, movie.ErrAlreadyInCollection) {
			t.Fatalf("err = %v, want ErrAlreadyInCollection", err)
		}
	})

	// Дубль обязан коммитить транзакцию: иначе закрытие задачи откатывается
	// и пользователь остаётся с висящим подтверждением.
	t.Run("duplicate commits the transaction instead of rolling it back", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)

		d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(confirming, nil)
		d.tx.EXPECT().BeginTransaction(gomock.Any()).DoAndReturn(
			func(ctx context.Context) (context.Context, error) { return ctx, nil })
		d.tx.EXPECT().FinishTransaction(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, errPtr *error) {
				if *errPtr != nil {
					t.Fatalf("transaction would roll back on a duplicate: %v", *errPtr)
				}
			})
		d.movies.EXPECT().GenresByTMDBIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
		d.movies.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(movie.Movie{ID: 55}, nil)
		d.movies.EXPECT().Add(gomock.Any(), gomock.Any()).Return(movie.UserMovie{}, movie.ErrAlreadyInCollection)
		d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, j domain.Job) error {
				if j.Status != domain.StatusDone {
					t.Fatalf("job status = %q, want done", j.Status)
				}
				return nil
			})
		d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)

		_, err := u.Confirm(context.Background(), 7, 1, movie.ProviderTMDB, 438631)
		if !errors.Is(err, movie.ErrAlreadyInCollection) {
			t.Fatalf("err = %v, want ErrAlreadyInCollection", err)
		}
	})

	t.Run("rejects a candidate that is not in the job", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)
		d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(confirming, nil)

		_, err := u.Confirm(context.Background(), 7, 1, movie.ProviderTMDB, 999999)
		if !errors.Is(err, domain.ErrCandidateNotInJob) {
			t.Fatalf("err = %v, want ErrCandidateNotInJob", err)
		}
	})

	t.Run("rejects a job that is not awaiting confirmation", func(t *testing.T) {
		t.Parallel()

		u, d := newUseCase(t, false)
		done := confirming
		done.Status = domain.StatusDone
		d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).Return(done, nil)

		_, err := u.Confirm(context.Background(), 7, 1, movie.ProviderTMDB, 438631)
		if !errors.Is(err, domain.ErrJobNotConfirmable) {
			t.Fatalf("err = %v, want ErrJobNotConfirmable", err)
		}
	})
}

func TestReject(t *testing.T) {
	t.Parallel()

	u, d := newUseCase(t, false)

	d.jobs.EXPECT().GetForUser(gomock.Any(), int64(7), int64(1)).
		Return(domain.Job{ID: 1, UserID: 7, Status: domain.StatusConfirming}, nil)
	d.jobs.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, j domain.Job) error {
			if j.Status != domain.StatusCancelled {
				t.Fatalf("status = %q, want cancelled", j.Status)
			}
			return nil
		})
	d.jobs.EXPECT().ClearSession(gomock.Any(), int64(7)).Return(nil)

	if err := u.Reject(context.Background(), 7, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
