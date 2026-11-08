package actors_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/usecase/actors"
	"github.com/arrowwhi/kinolist/internal/usecase/actors/mocks"
)

type fakeTx struct{}

func (fakeTx) BeginTransaction(ctx context.Context) (context.Context, error) { return ctx, nil }
func (fakeTx) FinishTransaction(context.Context, *error)                     {}

var now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

type env struct {
	uc        *actors.UseCase
	repo      *mocks.MockactorRepository
	movies    *mocks.MockcastRepository
	catalog   *mocks.MockpeopleCatalog
	tmdb      *mocks.MockTitleCatalog
	kinopoisk *mocks.MockTitleCatalog
	names     *mocks.MocknameResolver
}

func newEnv(t *testing.T, withNames bool) env {
	t.Helper()
	ctrl := gomock.NewController(t)
	e := env{
		repo:      mocks.NewMockactorRepository(ctrl),
		movies:    mocks.NewMockcastRepository(ctrl),
		catalog:   mocks.NewMockpeopleCatalog(ctrl),
		tmdb:      mocks.NewMockTitleCatalog(ctrl),
		kinopoisk: mocks.NewMockTitleCatalog(ctrl),
		names:     mocks.NewMocknameResolver(ctrl),
	}
	e.tmdb.EXPECT().Provider().Return(movie.ProviderTMDB).AnyTimes()
	e.kinopoisk.EXPECT().Provider().Return(movie.ProviderKinopoisk).AnyTimes()
	deps := actors.Deps{
		Actors: e.repo, Movies: e.movies, Catalog: e.catalog, Tx: fakeTx{},
		Titles:   []actors.TitleCatalog{e.tmdb, e.kinopoisk},
		WorksTTL: 7 * 24 * time.Hour, Now: func() time.Time { return now },
	}
	if withNames {
		deps.Names = e.names
	}
	e.uc = actors.New(deps)
	return e
}

func chalamet() movie.PersonDetails {
	birth := time.Date(1995, 12, 27, 0, 0, 0, 0, time.UTC)
	return movie.PersonDetails{
		Person: movie.Person{TMDBID: 1190668, Name: "Timothée Chalamet", OriginalName: "Timothée Chalamet", NameSource: movie.NameSourceTMDB, BirthDate: &birth},
		Works:  []movie.DirectorWork{{MediaType: movie.MediaMovie, TMDBID: 1, Title: "Режиссёрский дебют"}},
		Roles:  []movie.ActorWork{{MediaType: movie.MediaMovie, TMDBID: 438631, Title: "Дюна", Character: "Paul Atreides"}},
	}
}

func TestFollowByTMDBImportsRolesWithKinopoiskName(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	e.repo.EXPECT().SyncStateByTMDB(ctx, int32(1190668)).Return(movie.ActorSyncState{}, movie.ErrActorNotFound)
	e.catalog.EXPECT().PersonDetails(gomock.Any(), int32(1190668)).Return(chalamet(), nil)
	e.names.EXPECT().ResolveRussianName(gomock.Any(), "Timothée Chalamet", 1995).Return("Тимоти Шаламе", int32(11), true, nil)
	e.repo.EXPECT().UpsertPerson(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p movie.Person) (int64, error) {
		if p.Name != "Тимоти Шаламе" || p.NameSource != movie.NameSourceKinopoisk || p.KinopoiskID != 11 {
			t.Fatalf("person: %+v", p)
		}
		return 5, nil
	})
	// Сохраняются роли, а не режиссёрские работы.
	e.repo.EXPECT().ReplaceWorks(gomock.Any(), int64(5), chalamet().Roles).Return(nil)
	e.repo.EXPECT().Follow(ctx, int64(1), int64(5)).Return(nil)
	e.repo.EXPECT().Overview(ctx, int64(1), int64(5)).Return(movie.ActorOverview{Director: movie.Director{ID: 5}, Followed: true}, nil)

	got, err := e.uc.FollowByTMDB(ctx, 1, 1190668)
	if err != nil || got.ID != 5 || !got.Followed {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestImportSkipsFreshActorAndRejectsBadID(t *testing.T) {
	e := newEnv(t, true)
	ctx := context.Background()
	fresh := now.Add(-time.Hour)
	e.repo.EXPECT().SyncStateByTMDB(ctx, int32(1190668)).Return(movie.ActorSyncState{ID: 5, TMDBID: 1190668, SyncedAt: &fresh}, nil)
	e.repo.EXPECT().Overview(ctx, int64(1), int64(5)).Return(movie.ActorOverview{Director: movie.Director{ID: 5}}, nil)
	if _, err := e.uc.Import(ctx, 1, 1190668); err != nil {
		t.Fatal(err)
	}
	if _, err := e.uc.Import(ctx, 1, 0); !errors.Is(err, movie.ErrInvalidPersonID) {
		t.Fatalf("invalid id err = %v", err)
	}
}

func TestImportOfUnknownPersonReturnsNotFound(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.repo.EXPECT().SyncStateByTMDB(ctx, int32(42)).Return(movie.ActorSyncState{}, movie.ErrActorNotFound)
	e.catalog.EXPECT().PersonDetails(ctx, int32(42)).Return(movie.PersonDetails{}, movie.ErrPersonNotFound)
	if _, err := e.uc.Import(ctx, 1, 42); !errors.Is(err, movie.ErrPersonNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestActorProfileFallsBackToStaleSnapshot(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	old := now.Add(-30 * 24 * time.Hour)
	e.repo.EXPECT().SyncState(ctx, int64(5)).Return(movie.ActorSyncState{ID: 5, TMDBID: 1190668, SyncedAt: &old}, nil)
	e.catalog.EXPECT().PersonDetails(gomock.Any(), int32(1190668)).Return(movie.PersonDetails{}, movie.ErrProviderUnavailable)
	e.repo.EXPECT().Profile(ctx, int64(1), int64(5)).Return(movie.ActorProfile{DirectorOverview: movie.ActorOverview{Director: movie.Director{ID: 5}}}, nil)
	p, err := e.uc.Profile(ctx, 1, 5)
	if err != nil || !p.Stale {
		t.Fatalf("profile: %+v %v", p, err)
	}
}

func TestActorSearchPrefersActingAndMergesKnown(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.catalog.EXPECT().SearchPeopleByDepartment(ctx, "chalamet", movie.ActingDepartment).Return([]movie.Person{
		{TMDBID: 1190668, Name: "Timothée Chalamet", NameSource: movie.NameSourceTMDB},
		{TMDBID: 2, Name: "Other"},
	}, nil)
	e.repo.EXPECT().KnownByTMDB(ctx, int64(1), []int32{1190668, 2}).Return(map[int32]movie.PersonSearchResult{
		1190668: {PersonID: 6, Followed: true, Person: movie.Person{TMDBID: 1190668, Name: "Тимоти Шаламе", NameSource: movie.NameSourceKinopoisk}},
	}, nil)
	got, err := e.uc.Search(ctx, 1, "chalamet")
	if err != nil || len(got) != 2 || got[0].PersonID != 6 || !got[0].Followed || got[0].Name != "Тимоти Шаламе" || got[1].PersonID != 0 {
		t.Fatalf("search: %+v %v", got, err)
	}
}

func TestActorSearchWithoutQuerySuggestsCollectionActors(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.repo.EXPECT().MyActors(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, f movie.MyActorsFilter) ([]movie.ActorOverview, int, movie.ActorScopeCounts, error) {
		if f.Scope != movie.DirectorScopeCollection || f.UserID != 1 {
			t.Fatalf("filter: %+v", f)
		}
		return []movie.ActorOverview{{Director: movie.Director{ID: 5, TMDBID: 1190668, Name: "Тимоти Шаламе"}}}, 1, movie.ActorScopeCounts{}, nil
	})
	got, err := e.uc.Search(ctx, 1, "")
	if err != nil || len(got) != 1 || got[0].PersonID != 5 || got[0].KnownForDepartment != movie.ActingDepartment {
		t.Fatalf("suggestions: %+v %v", got, err)
	}
}

func TestMyActorsValidatesFilter(t *testing.T) {
	e := newEnv(t, false)
	for _, f := range []movie.MyActorsFilter{{Scope: "everyone"}, {Sort: "DROP TABLE"}} {
		if _, err := e.uc.MyActors(context.Background(), f); !errors.Is(err, movie.ErrInvalidActorFilter) {
			t.Fatalf("filter %+v: %v", f, err)
		}
	}
	ctx := context.Background()
	e.repo.EXPECT().MyActors(ctx, gomock.Any()).Return(nil, 3, movie.ActorScopeCounts{All: 3}, nil)
	page, err := e.uc.MyActors(ctx, movie.MyActorsFilter{UserID: 1})
	if err != nil || page.Total != 3 || page.Limit != movie.DefaultLimit {
		t.Fatalf("page: %+v %v", page, err)
	}
}

func TestBackfillCastUsesProviderOfTheWork(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.movies.EXPECT().MoviesWithoutCast(ctx, 5).Return([]movie.Movie{
		{ID: 1, TMDBID: 10, MediaType: movie.MediaTV},
		{ID: 2, KinopoiskID: 20, MediaType: movie.MediaMovie},
		{ID: 3, TMDBID: 30, MediaType: movie.MediaMovie},
		{ID: 4, TMDBID: 40, MediaType: movie.MediaMovie},
	}, nil)
	cast := []movie.CastMember{{TMDBID: 7, Name: "Actor", Character: "Hero"}}
	e.tmdb.EXPECT().Details(ctx, int32(10), movie.MediaTV).Return(movie.Candidate{Cast: cast, CastLoaded: true}, nil)
	e.movies.EXPECT().ReplaceCast(ctx, int64(1), cast).Return(nil)
	e.kinopoisk.EXPECT().Details(ctx, int32(20), movie.MediaMovie).Return(movie.Candidate{Cast: []movie.CastMember{}, CastLoaded: true}, nil)
	e.movies.EXPECT().ReplaceCast(ctx, int64(2), []movie.CastMember{}).Return(nil)
	// Работы нет в каталоге — пустой состав, чтобы не спрашивать снова.
	e.tmdb.EXPECT().Details(ctx, int32(30), movie.MediaMovie).Return(movie.Candidate{}, movie.ErrNotFound)
	e.movies.EXPECT().ReplaceCast(ctx, int64(3), nil).Return(nil)
	// Каталог недоступен — проход прерывается.
	e.tmdb.EXPECT().Details(ctx, int32(40), movie.MediaMovie).Return(movie.Candidate{}, movie.ErrProviderUnavailable)

	filled, err := e.uc.BackfillCast(ctx, 5)
	if filled != 3 || !errors.Is(err, movie.ErrProviderUnavailable) {
		t.Fatalf("filled %d err %v", filled, err)
	}
}

func TestSyncStaleBackfillsCastThenSyncsActors(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.movies.EXPECT().MoviesWithoutCast(ctx, 3).Return(nil, nil)
	e.repo.EXPECT().StaleActors(ctx, now.Add(-7*24*time.Hour), 3).Return([]movie.ActorSyncState{
		{ID: 1, TMDBID: 10}, {ID: 2, TMDBID: 20}, {ID: 3, TMDBID: 30},
	}, nil)
	e.catalog.EXPECT().PersonDetails(ctx, int32(10)).Return(movie.PersonDetails{}, movie.ErrPersonNotFound)
	e.repo.EXPECT().TouchSynced(ctx, int64(1)).Return(nil)
	e.catalog.EXPECT().PersonDetails(ctx, int32(20)).Return(movie.PersonDetails{}, movie.ErrProviderUnavailable)
	e.catalog.EXPECT().PersonDetails(ctx, int32(30)).Return(movie.PersonDetails{Person: movie.Person{TMDBID: 30, OriginalName: "X"}}, nil)
	e.repo.EXPECT().UpsertPerson(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, p movie.Person) (int64, error) {
		if p.Name != "X" {
			t.Fatalf("latin name fallback: %+v", p)
		}
		return 3, nil
	})
	e.repo.EXPECT().ReplaceWorks(ctx, int64(3), gomock.Any()).Return(nil)

	synced, err := e.uc.SyncStale(ctx, 3)
	if synced != 1 || !errors.Is(err, movie.ErrProviderUnavailable) {
		t.Fatalf("synced %d err %v", synced, err)
	}
}

func TestFollowAndUnfollowWrapRepositoryErrors(t *testing.T) {
	e := newEnv(t, false)
	ctx := context.Background()
	e.repo.EXPECT().Follow(ctx, int64(1), int64(9)).Return(movie.ErrActorNotFound)
	if err := e.uc.Follow(ctx, 1, 9); !errors.Is(err, movie.ErrActorNotFound) {
		t.Fatalf("follow: %v", err)
	}
	e.repo.EXPECT().Unfollow(ctx, int64(1), int64(9)).Return(nil)
	if err := e.uc.Unfollow(ctx, 1, 9); err != nil {
		t.Fatalf("unfollow: %v", err)
	}
}
