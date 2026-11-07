// Package directors — раздел «Мои режиссёры»: подписки, профиль человека
// и полная фильмография из каталога поверх коллекции пользователя.
package directors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

//go:generate mockgen -source=usecase.go -destination=mocks/mocks.go -package=mocks

type (
	directorRepository interface {
		UpsertPerson(ctx context.Context, p movie.Person) (int64, error)
		ReplaceWorks(ctx context.Context, directorID int64, works []movie.DirectorWork) error
		TouchSynced(ctx context.Context, directorID int64) error
		SyncState(ctx context.Context, directorID int64) (movie.DirectorSyncState, error)
		SyncStateByTMDB(ctx context.Context, tmdbID int32) (movie.DirectorSyncState, error)
		StaleDirectors(ctx context.Context, olderThan time.Time, limit int) ([]movie.DirectorSyncState, error)
		Follow(ctx context.Context, userID, directorID int64) error
		Unfollow(ctx context.Context, userID, directorID int64) error
		KnownByTMDB(ctx context.Context, userID int64, tmdbIDs []int32) (map[int32]movie.PersonSearchResult, error)
		MyDirectors(ctx context.Context, f movie.MyDirectorsFilter) ([]movie.DirectorOverview, int, movie.DirectorScopeCounts, error)
		Overview(ctx context.Context, userID, directorID int64) (movie.DirectorOverview, error)
		Profile(ctx context.Context, userID, directorID int64) (movie.DirectorProfile, error)
	}

	// peopleCatalog — TMDB: поиск людей и их фильмография.
	peopleCatalog interface {
		SearchPeople(ctx context.Context, query string) ([]movie.Person, error)
		PersonDetails(ctx context.Context, tmdbID int32) (movie.PersonDetails, error)
	}

	// nameResolver — Кинопоиск: русское имя по латинскому и году рождения.
	nameResolver interface {
		ResolveRussianName(ctx context.Context, name string, birthYear int) (string, int32, bool, error)
	}

	txManager interface {
		BeginTransaction(ctx context.Context) (context.Context, error)
		FinishTransaction(ctx context.Context, err *error)
	}
)

type Deps struct {
	Directors directorRepository
	Catalog   peopleCatalog
	// Names опционален: без ключа Кинопоиска имена берутся из TMDB.
	Names nameResolver
	Tx    txManager
	// WorksTTL — как долго снимок фильмографии считается свежим.
	WorksTTL time.Duration
	// SyncTimeout ограничивает синхронизацию внутри HTTP-запроса профиля.
	SyncTimeout time.Duration
	Now         func() time.Time
}

type UseCase struct {
	repo        directorRepository
	catalog     peopleCatalog
	names       nameResolver
	tx          txManager
	worksTTL    time.Duration
	syncTimeout time.Duration
	now         func() time.Time
}

func New(d Deps) *UseCase {
	if d.WorksTTL <= 0 {
		d.WorksTTL = 7 * 24 * time.Hour
	}
	if d.SyncTimeout <= 0 {
		d.SyncTimeout = 5 * time.Second
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &UseCase{
		repo: d.Directors, catalog: d.Catalog, names: d.Names, tx: d.Tx,
		worksTTL: d.WorksTTL, syncTimeout: d.SyncTimeout, now: d.Now,
	}
}

type Page struct {
	Items  []movie.DirectorOverview
	Total  int
	Counts movie.DirectorScopeCounts
	Limit  int
	Offset int
}

func (uc *UseCase) MyDirectors(ctx context.Context, f movie.MyDirectorsFilter) (Page, error) {
	f = f.Normalize()
	if err := f.Validate(); err != nil {
		return Page{}, err
	}
	items, total, counts, err := uc.repo.MyDirectors(ctx, f)
	if err != nil {
		return Page{}, fmt.Errorf("list my directors: %w", err)
	}
	return Page{Items: items, Total: total, Counts: counts, Limit: f.Limit, Offset: f.Offset}, nil
}

// Profile отдаёт профиль, при необходимости обновив фильмографию. Если
// каталог не ответил, показываем то, что уже сохранено, с пометкой Stale.
func (uc *UseCase) Profile(ctx context.Context, userID, directorID int64) (movie.DirectorProfile, error) {
	state, err := uc.repo.SyncState(ctx, directorID)
	if err != nil {
		return movie.DirectorProfile{}, fmt.Errorf("get director: %w", err)
	}
	stale := false
	if uc.needsSync(state) {
		syncCtx, cancel := context.WithTimeout(ctx, uc.syncTimeout)
		_, syncErr := uc.sync(syncCtx, state.TMDBID, state)
		cancel()
		if ctx.Err() != nil {
			return movie.DirectorProfile{}, ctx.Err()
		}
		stale = syncErr != nil
	}
	p, err := uc.repo.Profile(ctx, userID, directorID)
	if err != nil {
		return p, fmt.Errorf("get director profile: %w", err)
	}
	p.Stale = stale
	return p, nil
}

// Search ищет людей в TMDB. Пустой запрос возвращает подсказки — режиссёров
// из коллекции, на которых пользователь ещё не подписан.
func (uc *UseCase) Search(ctx context.Context, userID int64, query string) ([]movie.PersonSearchResult, error) {
	if query == "" {
		items, _, _, err := uc.repo.MyDirectors(ctx, movie.MyDirectorsFilter{
			UserID: userID, Scope: movie.DirectorScopeCollection, Sort: movie.MyDirectorsSortName, Limit: 20,
		}.Normalize())
		if err != nil {
			return nil, fmt.Errorf("suggest directors: %w", err)
		}
		out := make([]movie.PersonSearchResult, 0, len(items))
		for _, d := range items {
			out = append(out, movie.PersonSearchResult{
				DirectorID: d.ID, Followed: d.Followed,
				Person: movie.Person{
					TMDBID: d.TMDBID, KinopoiskID: d.KinopoiskID, Name: d.Name, OriginalName: d.OriginalName,
					ProfilePath: d.ProfilePath, BirthDate: d.BirthDate, DeathDate: d.DeathDate,
					KnownForDepartment: movie.DirectingDepartment, KnownFor: d.KnownFor,
				},
			})
		}
		return out, nil
	}
	people, err := uc.catalog.SearchPeople(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	const maxResults = 20
	if len(people) > maxResults {
		people = people[:maxResults]
	}
	ids := make([]int32, 0, len(people))
	for _, p := range people {
		ids = append(ids, p.TMDBID)
	}
	known, err := uc.repo.KnownByTMDB(ctx, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("match known directors: %w", err)
	}
	out := make([]movie.PersonSearchResult, 0, len(people))
	for _, p := range people {
		r := movie.PersonSearchResult{Person: p}
		if k, ok := known[p.TMDBID]; ok {
			r.DirectorID, r.Followed = k.DirectorID, k.Followed
			// В справочнике имя могло быть уже уточнено Кинопоиском.
			if k.NameSource.Rank() > p.NameSource.Rank() {
				r.Name, r.NameSource = k.Name, k.NameSource
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// Import заводит человека из TMDB в справочник режиссёров. Повторный вызов
// ничего не меняет, пока фильмография свежая.
func (uc *UseCase) Import(ctx context.Context, userID int64, tmdbID int32) (movie.DirectorOverview, error) {
	id, err := uc.ensure(ctx, tmdbID)
	if err != nil {
		return movie.DirectorOverview{}, err
	}
	return uc.overview(ctx, userID, id)
}

func (uc *UseCase) FollowByTMDB(ctx context.Context, userID int64, tmdbID int32) (movie.DirectorOverview, error) {
	id, err := uc.ensure(ctx, tmdbID)
	if err != nil {
		return movie.DirectorOverview{}, err
	}
	if err := uc.repo.Follow(ctx, userID, id); err != nil {
		return movie.DirectorOverview{}, fmt.Errorf("follow director: %w", err)
	}
	return uc.overview(ctx, userID, id)
}

func (uc *UseCase) Follow(ctx context.Context, userID, directorID int64) error {
	if err := uc.repo.Follow(ctx, userID, directorID); err != nil {
		return fmt.Errorf("follow director: %w", err)
	}
	return nil
}

func (uc *UseCase) Unfollow(ctx context.Context, userID, directorID int64) error {
	if err := uc.repo.Unfollow(ctx, userID, directorID); err != nil {
		return fmt.Errorf("unfollow director: %w", err)
	}
	return nil
}

// SyncStale обновляет фильмографии пачкой; вызывается фоновым тикером.
// Ошибка одного режиссёра не останавливает остальных.
func (uc *UseCase) SyncStale(ctx context.Context, limit int) (int, error) {
	states, err := uc.repo.StaleDirectors(ctx, uc.now().Add(-uc.worksTTL), limit)
	if err != nil {
		return 0, fmt.Errorf("list stale directors: %w", err)
	}
	synced := 0
	var errs []error
	for _, s := range states {
		if ctx.Err() != nil {
			break
		}
		if _, err := uc.sync(ctx, s.TMDBID, s); err != nil {
			if errors.Is(err, movie.ErrPersonNotFound) {
				// Человек пропал из каталога: не повторяем до следующего TTL.
				if touchErr := uc.repo.TouchSynced(ctx, s.ID); touchErr != nil {
					errs = append(errs, touchErr)
				}
				continue
			}
			errs = append(errs, fmt.Errorf("sync director %d: %w", s.ID, err))
			continue
		}
		synced++
	}
	return synced, errors.Join(errs...)
}

func (uc *UseCase) ensure(ctx context.Context, tmdbID int32) (int64, error) {
	if tmdbID <= 0 {
		return 0, movie.ErrInvalidPersonID
	}
	state, err := uc.repo.SyncStateByTMDB(ctx, tmdbID)
	switch {
	case errors.Is(err, movie.ErrDirectorNotFound):
		return uc.sync(ctx, tmdbID, movie.DirectorSyncState{})
	case err != nil:
		return 0, fmt.Errorf("get director: %w", err)
	case uc.needsSync(state):
		if _, syncErr := uc.sync(ctx, tmdbID, state); syncErr != nil && !errors.Is(syncErr, movie.ErrProviderUnavailable) {
			return 0, syncErr
		}
	}
	return state.ID, nil
}

func (uc *UseCase) needsSync(s movie.DirectorSyncState) bool {
	return s.TMDBID > 0 && (s.SyncedAt == nil || s.SyncedAt.Before(uc.now().Add(-uc.worksTTL)))
}

// sync загружает человека из каталога и сохраняет его вместе с фильмографией.
// Сетевые запросы идут до транзакции.
func (uc *UseCase) sync(ctx context.Context, tmdbID int32, known movie.DirectorSyncState) (id int64, err error) {
	details, err := uc.catalog.PersonDetails(ctx, tmdbID)
	if err != nil {
		return 0, fmt.Errorf("load person: %w", err)
	}
	if uc.names != nil && known.NameSource != movie.NameSourceKinopoisk {
		// Кинопоиск — лишь уточнение имени: его сбой не мешает синхронизации.
		name, kinopoiskID, ok, resolveErr := uc.names.ResolveRussianName(ctx, details.OriginalName, details.BirthYear())
		if resolveErr == nil && ok {
			details.Name, details.NameSource, details.KinopoiskID = name, movie.NameSourceKinopoisk, kinopoiskID
		}
	}
	if details.Name == "" {
		details.Name = details.OriginalName
	}

	ctx, err = uc.tx.BeginTransaction(ctx)
	if err != nil {
		return 0, err
	}
	defer uc.tx.FinishTransaction(ctx, &err)
	if id, err = uc.repo.UpsertPerson(ctx, details.Person); err != nil {
		return 0, fmt.Errorf("save person: %w", err)
	}
	if err = uc.repo.ReplaceWorks(ctx, id, details.Works); err != nil {
		return 0, fmt.Errorf("save filmography: %w", err)
	}
	return id, nil
}

func (uc *UseCase) overview(ctx context.Context, userID, directorID int64) (movie.DirectorOverview, error) {
	d, err := uc.repo.Overview(ctx, userID, directorID)
	if err != nil {
		return d, fmt.Errorf("get director overview: %w", err)
	}
	return d, nil
}
