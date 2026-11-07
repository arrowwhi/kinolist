package directors_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/usecase/directors"
	"github.com/arrowwhi/kinolist/internal/usecase/directors/mocks"
)

type fakeTx struct{}

func (fakeTx) BeginTransaction(ctx context.Context) (context.Context, error) { return ctx, nil }
func (fakeTx) FinishTransaction(context.Context, *error)                     {}

var now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

type env struct {
	uc      *directors.UseCase
	repo    *mocks.MockdirectorRepository
	catalog *mocks.MockpeopleCatalog
	names   *mocks.MocknameResolver
}

func newEnv(t *testing.T, withNames bool) env {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := env{
		repo:    mocks.NewMockdirectorRepository(ctrl),
		catalog: mocks.NewMockpeopleCatalog(ctrl),
		names:   mocks.NewMocknameResolver(ctrl),
	}
	deps := directors.Deps{
		Directors: e.repo, Catalog: e.catalog, Tx: fakeTx{},
		WorksTTL: 7 * 24 * time.Hour, Now: func() time.Time { return now },
	}
	if withNames {
		deps.Names = e.names
	}
	e.uc = directors.New(deps)
	return e
}

func villeneuve() movie.PersonDetails {
	birth := time.Date(1967, 10, 3, 0, 0, 0, 0, time.UTC)
	return movie.PersonDetails{
		Person: movie.Person{TMDBID: 137427, Name: "Denis Villeneuve", OriginalName: "Denis Villeneuve", NameSource: movie.NameSourceTMDB, BirthDate: &birth},
		Works:  []movie.DirectorWork{{MediaType: movie.MediaMovie, TMDBID: 438631, Title: "Дюна"}},
	}
}

func TestFollowByTMDBImportsWithKinopoiskName(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	e.repo.EXPECT().SyncStateByTMDB(ctx, int32(137427)).Return(movie.DirectorSyncState{}, movie.ErrDirectorNotFound)
	e.catalog.EXPECT().PersonDetails(gomock.Any(), int32(137427)).Return(villeneuve(), nil)
	e.names.EXPECT().ResolveRussianName(gomock.Any(), "Denis Villeneuve", 1967).Return("Дени Вильнёв", int32(11), true, nil)
	e.repo.EXPECT().UpsertPerson(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p movie.Person) (int64, error) {
		if p.Name != "Дени Вильнёв" || p.NameSource != movie.NameSourceKinopoisk || p.KinopoiskID != 11 || p.OriginalName != "Denis Villeneuve" {
			t.Fatalf("person: %+v", p)
		}
		return 5, nil
	})
	e.repo.EXPECT().ReplaceWorks(gomock.Any(), int64(5), villeneuve().Works).Return(nil)
	e.repo.EXPECT().Follow(ctx, int64(1), int64(5)).Return(nil)
	e.repo.EXPECT().Overview(ctx, int64(1), int64(5)).Return(movie.DirectorOverview{Director: movie.Director{ID: 5}, Followed: true}, nil)

	got, err := e.uc.FollowByTMDB(ctx, 1, 137427)
	if err != nil || got.ID != 5 || !got.Followed {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestKinopoiskFailureDoesNotBlockSync(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	e.repo.EXPECT().SyncStateByTMDB(ctx, int32(137427)).Return(movie.DirectorSyncState{}, movie.ErrDirectorNotFound)
	e.catalog.EXPECT().PersonDetails(gomock.Any(), int32(137427)).Return(villeneuve(), nil)
	e.names.EXPECT().ResolveRussianName(gomock.Any(), gomock.Any(), gomock.Any()).Return("", int32(0), false, movie.ErrProviderUnavailable)
	e.repo.EXPECT().UpsertPerson(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p movie.Person) (int64, error) {
		if p.NameSource != movie.NameSourceTMDB {
			t.Fatalf("name source: %s", p.NameSource)
		}
		return 5, nil
	})
	e.repo.EXPECT().ReplaceWorks(gomock.Any(), int64(5), gomock.Any()).Return(nil)
	e.repo.EXPECT().Overview(ctx, int64(1), int64(5)).Return(movie.DirectorOverview{Director: movie.Director{ID: 5}}, nil)
	if _, err := e.uc.Import(ctx, 1, 137427); err != nil {
		t.Fatal(err)
	}
}

func TestImportSkipsFreshDirectorAndKnownRussianName(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	fresh := now.Add(-time.Hour)
	e.repo.EXPECT().SyncStateByTMDB(ctx, int32(137427)).Return(movie.DirectorSyncState{ID: 5, TMDBID: 137427, SyncedAt: &fresh}, nil)
	e.repo.EXPECT().Overview(ctx, int64(1), int64(5)).Return(movie.DirectorOverview{Director: movie.Director{ID: 5}}, nil)
	if _, err := e.uc.Import(ctx, 1, 137427); err != nil {
		t.Fatal(err)
	}
	if _, err := e.uc.Import(ctx, 1, 0); !errors.Is(err, movie.ErrInvalidPersonID) {
		t.Fatalf("invalid id err = %v", err)
	}
}

func TestProfileFallsBackToStaleSnapshot(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	old := now.Add(-30 * 24 * time.Hour)
	e.repo.EXPECT().SyncState(ctx, int64(5)).Return(movie.DirectorSyncState{ID: 5, TMDBID: 137427, SyncedAt: &old}, nil)
	e.catalog.EXPECT().PersonDetails(gomock.Any(), int32(137427)).Return(movie.PersonDetails{}, movie.ErrProviderUnavailable)
	e.repo.EXPECT().Profile(ctx, int64(1), int64(5)).Return(movie.DirectorProfile{DirectorOverview: movie.DirectorOverview{Director: movie.Director{ID: 5}}}, nil)
	p, err := e.uc.Profile(ctx, 1, 5)
	if err != nil || !p.Stale {
		t.Fatalf("profile: %+v %v", p, err)
	}
}

func TestProfileOfKinopoiskOnlyDirectorNeverSyncs(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.repo.EXPECT().SyncState(ctx, int64(9)).Return(movie.DirectorSyncState{ID: 9}, nil)
	e.repo.EXPECT().Profile(ctx, int64(1), int64(9)).Return(movie.DirectorProfile{}, nil)
	if p, err := e.uc.Profile(ctx, 1, 9); err != nil || p.Stale {
		t.Fatalf("profile: %+v %v", p, err)
	}
}

func TestSearchMergesKnownDirectors(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.catalog.EXPECT().SearchPeople(ctx, "nolan").Return([]movie.Person{
		{TMDBID: 525, Name: "Christopher Nolan", NameSource: movie.NameSourceTMDB},
		{TMDBID: 1, Name: "Other"},
	}, nil)
	e.repo.EXPECT().KnownByTMDB(ctx, int64(1), []int32{525, 1}).Return(map[int32]movie.PersonSearchResult{
		525: {DirectorID: 6, Followed: true, Person: movie.Person{TMDBID: 525, Name: "Кристофер Нолан", NameSource: movie.NameSourceKinopoisk}},
	}, nil)
	got, err := e.uc.Search(ctx, 1, "nolan")
	if err != nil || len(got) != 2 || got[0].DirectorID != 6 || !got[0].Followed || got[0].Name != "Кристофер Нолан" || got[1].DirectorID != 0 {
		t.Fatalf("search: %+v %v", got, err)
	}
}

func TestSearchWithoutQuerySuggestsCollectionDirectors(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.repo.EXPECT().MyDirectors(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, f movie.MyDirectorsFilter) ([]movie.DirectorOverview, int, movie.DirectorScopeCounts, error) {
		if f.Scope != movie.DirectorScopeCollection || f.UserID != 1 {
			t.Fatalf("filter: %+v", f)
		}
		return []movie.DirectorOverview{{Director: movie.Director{ID: 5, TMDBID: 93183, Name: "Алексей Балабанов"}}}, 1, movie.DirectorScopeCounts{}, nil
	})
	got, err := e.uc.Search(ctx, 1, "")
	if err != nil || len(got) != 1 || got[0].DirectorID != 5 || got[0].TMDBID != 93183 {
		t.Fatalf("suggestions: %+v %v", got, err)
	}
}

func TestMyDirectorsValidatesFilter(t *testing.T) {
	e := newEnv(t, false)
	for _, f := range []movie.MyDirectorsFilter{{Scope: "everyone"}, {Sort: "DROP TABLE"}} {
		if _, err := e.uc.MyDirectors(context.Background(), f); !errors.Is(err, movie.ErrInvalidDirectorFilter) {
			t.Fatalf("filter %+v: %v", f, err)
		}
	}
}

func TestSyncStaleContinuesAfterFailures(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.repo.EXPECT().StaleDirectors(ctx, now.Add(-7*24*time.Hour), 3).Return([]movie.DirectorSyncState{
		{ID: 1, TMDBID: 10}, {ID: 2, TMDBID: 20}, {ID: 3, TMDBID: 30},
	}, nil)
	e.catalog.EXPECT().PersonDetails(ctx, int32(10)).Return(movie.PersonDetails{}, movie.ErrPersonNotFound)
	e.repo.EXPECT().TouchSynced(ctx, int64(1)).Return(nil)
	e.catalog.EXPECT().PersonDetails(ctx, int32(20)).Return(movie.PersonDetails{}, movie.ErrProviderUnavailable)
	e.catalog.EXPECT().PersonDetails(ctx, int32(30)).Return(movie.PersonDetails{Person: movie.Person{TMDBID: 30, OriginalName: "X"}}, nil)
	e.repo.EXPECT().UpsertPerson(ctx, gomock.Any()).Return(int64(3), nil)
	e.repo.EXPECT().ReplaceWorks(ctx, int64(3), gomock.Any()).Return(nil)

	synced, err := e.uc.SyncStale(ctx, 3)
	if synced != 1 || !errors.Is(err, movie.ErrProviderUnavailable) {
		t.Fatalf("synced %d err %v", synced, err)
	}
}
