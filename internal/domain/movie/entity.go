package movie

import (
	"time"
	"unicode/utf8"
)

// Source — откуда фильм попал в коллекцию.
type Source string

const (
	SourceReel   Source = "reel"
	SourceManual Source = "manual"
)

// Status — состояние фильма в коллекции пользователя.
type Status string

const (
	StatusWantToWatch Status = "want_to_watch"
	StatusWatched     Status = "watched"
)

func (s Status) Valid() bool {
	return s == StatusWantToWatch || s == StatusWatched
}

// Provider — внешний каталог, из которого пришли данные фильма.
type Provider string

const (
	ProviderTMDB      Provider = "tmdb"
	ProviderKinopoisk Provider = "kinopoisk"
)

type MediaType string

const (
	MediaMovie MediaType = "movie"
	MediaTV    MediaType = "tv"
)

func (m MediaType) Valid() bool { return m == MediaMovie || m == MediaTV }

// Normalize supports candidates persisted before media types were introduced.
func (m MediaType) Normalize() MediaType {
	if m == "" {
		return MediaMovie
	}
	return m
}

func (m MediaType) Label() string {
	if m == MediaTV {
		return "Сериал"
	}
	return "Фильм"
}

type Genre struct {
	ID     int16
	TMDBID int32
	NameRU string
	NameEN string
}

// Movie — карточка фильма в общем каталоге, одна на всех пользователей.
type Movie struct {
	Directors       []Director
	DirectorsLoaded bool
	MediaType       MediaType
	SeasonCount     *int16
	ID              int64
	TMDBID          int32
	KinopoiskID     int32
	Title           string
	OriginalTitle   string
	Year            int16
	Overview        string
	PosterPath      string
	RuntimeMin      int16
	RatingProvider  Provider
	RatingVoteCount *int32
	Rating          float32
	Genres          []Genre
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserMovie — фильм в коллекции конкретного пользователя.
type UserMovie struct {
	ID         int64
	UserID     int64
	Movie      Movie
	Status     Status
	UserRating int16
	Review     string
	Note       string
	Source     Source
	SourceURL  string
	AddedAt    time.Time
	WatchedAt  *time.Time
}

// Candidate — результат поиска во внешнем каталоге, ещё не сохранённый.
type Candidate struct {
	Directors       []Director
	DirectorsLoaded bool
	MediaType       MediaType
	SeasonCount     *int16
	Provider        Provider
	ExternalID      int32
	Title           string
	OriginalTitle   string
	Year            int16
	Overview        string
	PosterPath      string
	RuntimeMin      int16
	Popularity      float32
	// ReleaseDate — дата премьеры (у сериала — первого эпизода); nil, если
	// каталог её не знает: Кинопоиск отдаёт только год.
	ReleaseDate     *time.Time
	RatingProvider  Provider
	RatingVoteCount *int32
	Rating          float32
	GenreTMDBIDs    []int32
	GenreNames      []string
}

// SortBy — порядок выдачи коллекции.
type SortBy string

const (
	SortByAddedAt    SortBy = "added_at"
	SortByTitle      SortBy = "title"
	SortByYear       SortBy = "year"
	SortByRating     SortBy = "rating"
	SortByUserRating SortBy = "user_rating"
	SortByWatchedAt  SortBy = "watched_at"
)

func (s SortBy) Valid() bool {
	switch s {
	case SortByAddedAt, SortByTitle, SortByYear, SortByRating, SortByUserRating, SortByWatchedAt:
		return true
	default:
		return false
	}
}

// Filter описывает выборку коллекции пользователя.
type Filter struct {
	DirectorIDs []int64
	MediaType   MediaType
	UserID      int64
	Status      Status
	GenreIDs    []int16
	Query       string
	SortBy      SortBy
	Desc        bool
	Limit       int
	Offset      int
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Normalize приводит фильтр к безопасным значениям, чтобы транспорту
// не приходилось дублировать эти проверки в каждом хендлере.
func (f Filter) Normalize() Filter {
	if !f.SortBy.Valid() {
		f.SortBy = SortByAddedAt
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return f
}

// Update — частичное изменение записи коллекции; nil-поля не трогаются.
type Update struct {
	Status     *Status
	UserRating *int16
	Review     *string
	Note       *string
}

const (
	MinUserRating = 1
	MaxUserRating = 10
)

// Summary ignores search and pagination; ratings only include watched films.
type Summary struct {
	MovieCount        int
	RatedCount        int
	AverageUserRating *float64
}
type GenreStats struct {
	Genre
	WantToWatchCount int
	WatchedCount     int
}
type ConfirmOptions struct {
	MediaType  MediaType
	Status     Status
	UserRating int16
	Note       string
	Review     string
}

const MaxReviewLength = 5000

func ValidateReview(review string) error {
	if utf8.RuneCountInString(review) > MaxReviewLength {
		return ErrReviewTooLong
	}
	return nil
}

func (o ConfirmOptions) Validate() error {
	if o.MediaType != "" && !o.MediaType.Valid() {
		return ErrInvalidMediaType
	}
	if o.Status != "" && !o.Status.Valid() {
		return ErrInvalidStatus
	}
	if o.UserRating != 0 && (o.UserRating < MinUserRating || o.UserRating > MaxUserRating) {
		return ErrInvalidRating
	}
	return ValidateReview(o.Review)
}

// CatalogMark — что база знает о работе из каталога: id в общем справочнике
// фильмов и запись в коллекции пользователя. Нули — работы там нет.
type CatalogMark struct {
	MovieID     int64
	UserMovieID int64
	Status      Status
	UserRating  int16
}

// CatalogTitle — карточка работы прямо из каталога, с пометками коллекции
// пользователя. Movie.ID равен 0, если работы нет в общем справочнике.
type CatalogTitle struct {
	Movie
	ReleaseDate *time.Time
	UserMovieID int64
	Status      Status
	UserRating  int16
}

// AlreadyInCollectionError preserves the domain sentinel and identifies the existing entry.
type AlreadyInCollectionError struct{ UserMovieID int64 }

func (e *AlreadyInCollectionError) Error() string { return ErrAlreadyInCollection.Error() }
func (e *AlreadyInCollectionError) Unwrap() error { return ErrAlreadyInCollection }

// Statistics describes the complete user collection, independent of list filters.
// A series counts as one entry regardless of its number of seasons.
type Statistics struct {
	TotalCount       int
	WantToWatchCount int
	WatchedCount     int
}
