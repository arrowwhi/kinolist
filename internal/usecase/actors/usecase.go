// Package actors — раздел «Актёры»: подписки, профиль человека и все его
// роли из каталога поверх коллекции пользователя. Устроен как раздел
// режиссёров; отдельно — догрузка актёрского состава старых работ.
package actors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

//go:generate mockgen -source=usecase.go -destination=mocks/mocks.go -package=mocks

type (
	actorRepository interface {
		UpsertPerson(ctx context.Context, p movie.Person) (int64, error)
		ReplaceWorks(ctx context.Context, actorID int64, works []movie.ActorWork) error
		TouchSynced(ctx context.Context, actorID int64) error
		SyncState(ctx context.Context, actorID int64) (movie.ActorSyncState, error)
		SyncStateByTMDB(ctx context.Context, tmdbID int32) (movie.ActorSyncState, error)
		StaleActors(ctx context.Context, olderThan time.Time, limit int) ([]movie.ActorSyncState, error)
		Follow(ctx context.Context, userID, actorID int64) error
		Unfollow(ctx context.Context, userID, actorID int64) error
		KnownByTMDB(ctx context.Context, userID int64, tmdbIDs []int32) (map[int32]movie.PersonSearchResult, error)
		MyActors(ctx context.Context, f movie.MyActorsFilter) ([]movie.ActorOverview, int, movie.ActorScopeCounts, error)
		Overview(ctx context.Context, userID, actorID int64) (movie.ActorOverview, error)
		Profile(ctx context.Context, userID, actorID int64) (movie.ActorProfile, error)
	}

	// castRepository — работы коллекций без загруженного состава.
	castRepository interface {
		MoviesWithoutCast(ctx context.Context, limit int) ([]movie.Movie, error)
		ReplaceCast(ctx context.Context, movieID int64, cast []movie.CastMember) error
	}

	// peopleCatalog — TMDB: поиск людей и их роли.
	peopleCatalog interface {
		SearchPeopleByDepartment(ctx context.Context, query, department string) ([]movie.Person, error)
		PersonDetails(ctx context.Context, tmdbID int32) (movie.PersonDetails, error)
	}

	// TitleCatalog — каталог работ (TMDB или Кинопоиск) для догрузки
	// состава. Экспортирован, чтобы app мог собрать список каталогов.
	TitleCatalog interface {
		Provider() movie.Provider
		Details(ctx context.Context, externalID int32, mediaTypes ...movie.MediaType) (movie.Candidate, error)
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
	Actors  actorRepository
	Movies  castRepository
	Catalog peopleCatalog
	// Titles — каталоги работ для догрузки состава; Кинопоиск опционален.
	Titles []TitleCatalog
	// Names опционален: без ключа Кинопоиска имена берутся из TMDB.
	Names nameResolver
	Tx    txManager
	// WorksTTL — как долго снимок ролей считается свежим.
	WorksTTL time.Duration
	// SyncTimeout ограничивает синхронизацию внутри HTTP-запроса профиля.
	SyncTimeout time.Duration
	Now         func() time.Time
}

type UseCase struct {
	repo        actorRepository
	movies      castRepository
	catalog     peopleCatalog
	titles      []TitleCatalog
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
		repo: d.Actors, movies: d.Movies, catalog: d.Catalog, titles: d.Titles, names: d.Names, tx: d.Tx,
		worksTTL: d.WorksTTL, syncTimeout: d.SyncTimeout, now: d.Now,
	}
}

type Page struct {
	Items  []movie.ActorOverview
	Total  int
	Counts movie.ActorScopeCounts
	Limit  int
	Offset int
}

func (uc *UseCase) MyActors(ctx context.Context, f movie.MyActorsFilter) (Page, error) {
	f = f.Normalize()
	if err := f.Validate(); err != nil {
		return Page{}, movie.ErrInvalidActorFilter
	}
	items, total, counts, err := uc.repo.MyActors(ctx, f)
	if err != nil {
		return Page{}, fmt.Errorf("list my actors: %w", err)
	}
	return Page{Items: items, Total: total, Counts: counts, Limit: f.Limit, Offset: f.Offset}, nil
}

// Profile отдаёт профиль, при необходимости обновив роли. Если каталог не
// ответил, показываем то, что уже сохранено, с пометкой Stale.
func (uc *UseCase) Profile(ctx context.Context, userID, actorID int64) (movie.ActorProfile, error) {
	state, err := uc.repo.SyncState(ctx, actorID)
	if err != nil {
		return movie.ActorProfile{}, fmt.Errorf("get actor: %w", err)
	}
	stale := false
	if uc.needsSync(state) {
		syncCtx, cancel := context.WithTimeout(ctx, uc.syncTimeout)
		_, syncErr := uc.sync(syncCtx, state.TMDBID, state)
		cancel()
		if ctx.Err() != nil {
			return movie.ActorProfile{}, ctx.Err()
		}
		stale = syncErr != nil
	}
	p, err := uc.repo.Profile(ctx, userID, actorID)
	if err != nil {
		return p, fmt.Errorf("get actor profile: %w", err)
	}
	p.Stale = stale
	return p, nil
}

// Search ищет людей в TMDB, актёров — первыми. Пустой запрос возвращает
// подсказки — актёров из коллекции, на которых пользователь не подписан.
func (uc *UseCase) Search(ctx context.Context, userID int64, query string) ([]movie.PersonSearchResult, error) {
	if query == "" {
		items, _, _, err := uc.repo.MyActors(ctx, movie.MyActorsFilter{
			UserID: userID, Scope: movie.DirectorScopeCollection, Sort: movie.MyDirectorsSortName, Limit: 20,
		}.Normalize())
		if err != nil {
			return nil, fmt.Errorf("suggest actors: %w", err)
		}
		out := make([]movie.PersonSearchResult, 0, len(items))
		for _, a := range items {
			out = append(out, movie.PersonSearchResult{
				PersonID: a.ID, Followed: a.Followed,
				Person: movie.Person{
					TMDBID: a.TMDBID, KinopoiskID: a.KinopoiskID, Name: a.Name, OriginalName: a.OriginalName,
					ProfilePath: a.ProfilePath, BirthDate: a.BirthDate, DeathDate: a.DeathDate,
					KnownForDepartment: movie.ActingDepartment, KnownFor: a.KnownFor,
				},
			})
		}
		return out, nil
	}
	people, err := uc.catalog.SearchPeopleByDepartment(ctx, query, movie.ActingDepartment)
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
		return nil, fmt.Errorf("match known actors: %w", err)
	}
	out := make([]movie.PersonSearchResult, 0, len(people))
	for _, p := range people {
		r := movie.PersonSearchResult{Person: p}
		if k, ok := known[p.TMDBID]; ok {
			r.PersonID, r.Followed = k.PersonID, k.Followed
			// В справочнике имя могло быть уже уточнено Кинопоиском.
			if k.NameSource.Rank() > p.NameSource.Rank() {
				r.Name, r.NameSource = k.Name, k.NameSource
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// Import заводит человека из TMDB в справочник актёров. Повторный вызов
// ничего не меняет, пока снимок ролей свежий.
func (uc *UseCase) Import(ctx context.Context, userID int64, tmdbID int32) (movie.ActorOverview, error) {
	id, err := uc.ensure(ctx, tmdbID)
	if err != nil {
		return movie.ActorOverview{}, err
	}
	return uc.overview(ctx, userID, id)
}

func (uc *UseCase) FollowByTMDB(ctx context.Context, userID int64, tmdbID int32) (movie.ActorOverview, error) {
	id, err := uc.ensure(ctx, tmdbID)
	if err != nil {
		return movie.ActorOverview{}, err
	}
	if err := uc.repo.Follow(ctx, userID, id); err != nil {
		return movie.ActorOverview{}, fmt.Errorf("follow actor: %w", err)
	}
	return uc.overview(ctx, userID, id)
}

func (uc *UseCase) Follow(ctx context.Context, userID, actorID int64) error {
	if err := uc.repo.Follow(ctx, userID, actorID); err != nil {
		return fmt.Errorf("follow actor: %w", err)
	}
	return nil
}

func (uc *UseCase) Unfollow(ctx context.Context, userID, actorID int64) error {
	if err := uc.repo.Unfollow(ctx, userID, actorID); err != nil {
		return fmt.Errorf("unfollow actor: %w", err)
	}
	return nil
}

// SyncStale — проход фонового тикера: сначала догружает состав работ,
// добавленных до появления актёров (от него зависит, кто попадёт в «из
// коллекции»), затем обновляет устаревшие снимки ролей. Каждый шаг — до
// limit записей; ошибка одной записи не останавливает остальные.
func (uc *UseCase) SyncStale(ctx context.Context, limit int) (int, error) {
	filled, castErr := uc.BackfillCast(ctx, limit)
	if ctx.Err() != nil {
		return filled, ctx.Err()
	}
	states, err := uc.repo.StaleActors(ctx, uc.now().Add(-uc.worksTTL), limit)
	if err != nil {
		return filled, errors.Join(castErr, fmt.Errorf("list stale actors: %w", err))
	}
	synced := 0
	errs := []error{castErr}
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
			errs = append(errs, fmt.Errorf("sync actor %d: %w", s.ID, err))
			continue
		}
		synced++
	}
	return filled + synced, errors.Join(errs...)
}

// BackfillCast загружает актёрский состав работ, сохранённых без него.
// Работа, которой нет в каталоге или у которой каталог не отдал титры,
// получает пустой состав, чтобы не запрашивать её снова на каждом проходе.
func (uc *UseCase) BackfillCast(ctx context.Context, limit int) (int, error) {
	if uc.movies == nil || len(uc.titles) == 0 {
		return 0, nil
	}
	items, err := uc.movies.MoviesWithoutCast(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("list movies without cast: %w", err)
	}
	filled := 0
	var errs []error
	for _, m := range items {
		if ctx.Err() != nil {
			break
		}
		provider, externalID := movie.ProviderTMDB, m.TMDBID
		if externalID == 0 {
			provider, externalID = movie.ProviderKinopoisk, m.KinopoiskID
		}
		catalog := uc.titleCatalog(provider)
		if catalog == nil || externalID <= 0 {
			continue
		}
		details, err := catalog.Details(ctx, externalID, m.MediaType.Normalize())
		switch {
		case errors.Is(err, movie.ErrNotFound):
			details = movie.Candidate{CastLoaded: true}
		case errors.Is(err, movie.ErrProviderUnavailable):
			// Каталог лёг — остальные запросы тоже упадут, ждём следующего прохода.
			return filled, errors.Join(append(errs, fmt.Errorf("load cast of movie %d: %w", m.ID, err))...)
		case err != nil:
			errs = append(errs, fmt.Errorf("load cast of movie %d: %w", m.ID, err))
			continue
		}
		if err := uc.movies.ReplaceCast(ctx, m.ID, details.Cast); err != nil {
			errs = append(errs, fmt.Errorf("save cast of movie %d: %w", m.ID, err))
			continue
		}
		filled++
	}
	return filled, errors.Join(errs...)
}

func (uc *UseCase) titleCatalog(p movie.Provider) TitleCatalog {
	for _, c := range uc.titles {
		if c != nil && c.Provider() == p {
			return c
		}
	}
	return nil
}

func (uc *UseCase) ensure(ctx context.Context, tmdbID int32) (int64, error) {
	if tmdbID <= 0 {
		return 0, movie.ErrInvalidPersonID
	}
	state, err := uc.repo.SyncStateByTMDB(ctx, tmdbID)
	switch {
	case errors.Is(err, movie.ErrActorNotFound):
		return uc.sync(ctx, tmdbID, movie.ActorSyncState{})
	case err != nil:
		return 0, fmt.Errorf("get actor: %w", err)
	case uc.needsSync(state):
		if _, syncErr := uc.sync(ctx, tmdbID, state); syncErr != nil && !errors.Is(syncErr, movie.ErrProviderUnavailable) {
			return 0, syncErr
		}
	}
	return state.ID, nil
}

func (uc *UseCase) needsSync(s movie.ActorSyncState) bool {
	return s.TMDBID > 0 && (s.SyncedAt == nil || s.SyncedAt.Before(uc.now().Add(-uc.worksTTL)))
}

// sync загружает человека из каталога и сохраняет его вместе с ролями.
// Сетевые запросы идут до транзакции.
func (uc *UseCase) sync(ctx context.Context, tmdbID int32, known movie.ActorSyncState) (id int64, err error) {
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
	if err = uc.repo.ReplaceWorks(ctx, id, details.Roles); err != nil {
		return 0, fmt.Errorf("save roles: %w", err)
	}
	return id, nil
}

func (uc *UseCase) overview(ctx context.Context, userID, actorID int64) (movie.ActorOverview, error) {
	a, err := uc.repo.Overview(ctx, userID, actorID)
	if err != nil {
		return a, fmt.Errorf("get actor overview: %w", err)
	}
	return a, nil
}
