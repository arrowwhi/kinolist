package movie

import "context"

// Repository — доступ к каталогу фильмов и коллекциям пользователей.
type Repository interface {
	// Upsert сохраняет карточку фильма и её жанры, возвращая запись с ID.
	Upsert(ctx context.Context, m Movie) (Movie, error)
	GetByTMDBID(ctx context.Context, tmdbID int32) (Movie, error)

	Add(ctx context.Context, um UserMovie) (UserMovie, error)
	Get(ctx context.Context, userID, userMovieID int64) (UserMovie, error)
	List(ctx context.Context, f Filter) ([]UserMovie, error)
	Count(ctx context.Context, f Filter) (int, error)
	Update(ctx context.Context, userID, userMovieID int64, upd Update) (UserMovie, error)
	Delete(ctx context.Context, userID, userMovieID int64) error

	ListGenres(ctx context.Context) ([]Genre, error)
	// GenresByExternalNames резолвит названия жанров стороннего источника
	// в собственный справочник через таблицу маппинга.
	GenresByExternalNames(ctx context.Context, provider Provider, names []string) ([]Genre, error)
	GenresByTMDBIDs(ctx context.Context, tmdbIDs []int32) ([]Genre, error)
}

// Catalog — внешний источник данных о фильмах (TMDB, Кинопоиск).
type Catalog interface {
	Provider() Provider
	Search(ctx context.Context, title string, year int16) ([]Candidate, error)
	Details(ctx context.Context, externalID int32) (Candidate, error)
}
