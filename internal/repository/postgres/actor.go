package postgres

import (
	"context"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

// ActorRepository — справочник актёров, подписки и снимки ролей. Актёры
// попадают в справочник из состава работ (MovieRepository) и из профиля
// человека в каталоге. Устроен как справочник режиссёров, но отдельно от
// него: один человек может быть и там, и там.
type ActorRepository struct {
	store personStore
}

func NewActorRepository(tx *transactions.Manager) *ActorRepository {
	return &ActorRepository{store: personStore{tx: tx, dir: actorDirectory}}
}

// UpsertPerson сохраняет человека из TMDB и возвращает id актёра.
func (r *ActorRepository) UpsertPerson(ctx context.Context, p movie.Person) (int64, error) {
	return r.store.upsertPerson(ctx, p)
}

// ReplaceWorks заменяет снимок ролей и отмечает время синхронизации.
func (r *ActorRepository) ReplaceWorks(ctx context.Context, actorID int64, works []movie.ActorWork) error {
	return r.store.replaceWorks(ctx, actorID, works)
}

func (r *ActorRepository) TouchSynced(ctx context.Context, actorID int64) error {
	return r.store.touchSynced(ctx, actorID)
}

func (r *ActorRepository) SyncState(ctx context.Context, actorID int64) (movie.ActorSyncState, error) {
	return r.store.syncState(ctx, `id=$1`, actorID)
}

func (r *ActorRepository) SyncStateByTMDB(ctx context.Context, tmdbID int32) (movie.ActorSyncState, error) {
	return r.store.syncState(ctx, `tmdb_id=$1`, tmdbID)
}

// StaleActors — актёры с подпиской или из чьей-то коллекции (минимум
// movie.MinCollectionRoles работ), у которых снимок ролей устарел.
func (r *ActorRepository) StaleActors(ctx context.Context, olderThan time.Time, limit int) ([]movie.ActorSyncState, error) {
	return r.store.stale(ctx, olderThan, limit)
}

func (r *ActorRepository) Follow(ctx context.Context, userID, actorID int64) error {
	return r.store.follow(ctx, userID, actorID)
}

func (r *ActorRepository) Unfollow(ctx context.Context, userID, actorID int64) error {
	return r.store.unfollow(ctx, userID, actorID)
}

func (r *ActorRepository) KnownByTMDB(ctx context.Context, userID int64, tmdbIDs []int32) (map[int32]movie.PersonSearchResult, error) {
	return r.store.knownByTMDB(ctx, userID, tmdbIDs)
}

// MyActors — подписки пользователя и актёры минимум из двух работ его коллекции.
func (r *ActorRepository) MyActors(ctx context.Context, f movie.MyActorsFilter) ([]movie.ActorOverview, int, movie.ActorScopeCounts, error) {
	return r.store.mine(ctx, f)
}

func (r *ActorRepository) Overview(ctx context.Context, userID, actorID int64) (movie.ActorOverview, error) {
	p, err := r.store.overview(ctx, userID, actorID)
	return p.DirectorOverview, err
}

// Profile — карточка актёра с биографией и всеми ролями.
func (r *ActorRepository) Profile(ctx context.Context, userID, actorID int64) (movie.ActorProfile, error) {
	return r.store.profile(ctx, userID, actorID)
}
