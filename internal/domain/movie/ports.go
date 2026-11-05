package movie

import "context"

// Repository — доступ к каталогу фильмов и коллекциям пользователей.
type Repository interface {
	// Upsert сохраняет карточку фильма и её жанры, возвращая запись с ID.
	Upsert(ctx context.Context, m Movie) (Movie, error)
	GetByTMDBID(ctx context.Context, tmdbID int32, mediaTypes ...MediaType) (Movie, error)

	Add(ctx context.Context, um UserMovie) (UserMovie, error)
	Get(ctx context.Context, userID, userMovieID int64) (UserMovie, error)
	List(ctx context.Context, f Filter) ([]UserMovie, error)
	Count(ctx context.Context, f Filter) (int, error)
	Summary(ctx context.Context, f Filter) (Summary, error)
	ListDirectors(ctx context.Context, f DirectorFilter) ([]DirectorStats, int, error)
	GetDirector(ctx context.Context, userID, directorID int64, mediaType MediaType) (DirectorStats, error)
	Statistics(ctx context.Context, userID int64) (Statistics, error)
	CollectionGenres(ctx context.Context, userID int64, mediaTypes ...MediaType) ([]GenreStats, error)
	CollectionGenresFiltered(ctx context.Context, f Filter) ([]GenreStats, error)
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
	Details(ctx context.Context, externalID int32, mediaTypes ...MediaType) (Candidate, error)
}
