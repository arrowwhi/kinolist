package postgres

import (
	"context"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

// DirectorRepository — подписки на режиссёров и снимки их фильмографий.
// Справочник directors общий с MovieRepository: там режиссёры появляются
// из карточек фильмов, здесь — из профиля человека в каталоге. Запросы
// общие со справочником актёров (personStore).
type DirectorRepository struct {
	store personStore
}

func NewDirectorRepository(tx *transactions.Manager) *DirectorRepository {
	return &DirectorRepository{store: personStore{tx: tx, dir: directorDirectory}}
}

// UpsertPerson сохраняет человека из TMDB и возвращает id режиссёра.
// Если Кинопоиск опознал того же человека, а в справочнике уже есть
// строка только с kinopoisk_id, она сливается в TMDB-строку.
func (r *DirectorRepository) UpsertPerson(ctx context.Context, p movie.Person) (int64, error) {
	return r.store.upsertPerson(ctx, p)
}

// ReplaceWorks заменяет снимок фильмографии и отмечает время синхронизации.
func (r *DirectorRepository) ReplaceWorks(ctx context.Context, directorID int64, works []movie.DirectorWork) error {
	return r.store.replaceWorks(ctx, directorID, works)
}

// TouchSynced отмечает попытку синхронизации, не меняя снимок.
func (r *DirectorRepository) TouchSynced(ctx context.Context, directorID int64) error {
	return r.store.touchSynced(ctx, directorID)
}

func (r *DirectorRepository) SyncState(ctx context.Context, directorID int64) (movie.DirectorSyncState, error) {
	return r.store.syncState(ctx, `id=$1`, directorID)
}

func (r *DirectorRepository) SyncStateByTMDB(ctx context.Context, tmdbID int32) (movie.DirectorSyncState, error) {
	return r.store.syncState(ctx, `tmdb_id=$1`, tmdbID)
}

// StaleDirectors — режиссёры, которые кому-то нужны (подписка или работа
// в коллекции), а фильмография не загружена или устарела.
func (r *DirectorRepository) StaleDirectors(ctx context.Context, olderThan time.Time, limit int) ([]movie.DirectorSyncState, error) {
	return r.store.stale(ctx, olderThan, limit)
}

func (r *DirectorRepository) Follow(ctx context.Context, userID, directorID int64) error {
	return r.store.follow(ctx, userID, directorID)
}

func (r *DirectorRepository) Unfollow(ctx context.Context, userID, directorID int64) error {
	return r.store.unfollow(ctx, userID, directorID)
}

// KnownByTMDB сопоставляет людей из поиска с уже заведёнными режиссёрами.
func (r *DirectorRepository) KnownByTMDB(ctx context.Context, userID int64, tmdbIDs []int32) (map[int32]movie.PersonSearchResult, error) {
	return r.store.knownByTMDB(ctx, userID, tmdbIDs)
}

// MyDirectors — подписки пользователя и режиссёры его коллекции.
func (r *DirectorRepository) MyDirectors(ctx context.Context, f movie.MyDirectorsFilter) ([]movie.DirectorOverview, int, movie.DirectorScopeCounts, error) {
	return r.store.mine(ctx, f)
}

func (r *DirectorRepository) Overview(ctx context.Context, userID, directorID int64) (movie.DirectorOverview, error) {
	p, err := r.store.overview(ctx, userID, directorID)
	return p.DirectorOverview, err
}

// Profile — карточка режиссёра с биографией и полной фильмографией.
func (r *DirectorRepository) Profile(ctx context.Context, userID, directorID int64) (movie.DirectorProfile, error) {
	return r.store.profile(ctx, userID, directorID)
}
