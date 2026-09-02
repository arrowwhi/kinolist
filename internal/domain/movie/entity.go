package movie

import "time"

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

type Genre struct {
	ID     int16
	TMDBID int32
	NameRU string
	NameEN string
}

// Movie — карточка фильма в общем каталоге, одна на всех пользователей.
type Movie struct {
	ID            int64
	TMDBID        int32
	KinopoiskID   int32
	Title         string
	OriginalTitle string
	Year          int16
	Overview      string
	PosterPath    string
	RuntimeMin    int16
	Rating        float32
	Genres        []Genre
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UserMovie — фильм в коллекции конкретного пользователя.
type UserMovie struct {
	ID         int64
	UserID     int64
	Movie      Movie
	Status     Status
	UserRating int16
	Note       string
	Source     Source
	SourceURL  string
	AddedAt    time.Time
	WatchedAt  *time.Time
}

// Candidate — результат поиска во внешнем каталоге, ещё не сохранённый.
type Candidate struct {
	Provider      Provider
	ExternalID    int32
	Title         string
	OriginalTitle string
	Year          int16
	Overview      string
	PosterPath    string
	Popularity    float32
	Rating        float32
	GenreTMDBIDs  []int32
	GenreNames    []string
}

// SortBy — порядок выдачи коллекции.
type SortBy string

const (
	SortByAddedAt SortBy = "added_at"
	SortByTitle   SortBy = "title"
	SortByYear    SortBy = "year"
	SortByRating  SortBy = "rating"
)

func (s SortBy) Valid() bool {
	switch s {
	case SortByAddedAt, SortByTitle, SortByYear, SortByRating:
		return true
	default:
		return false
	}
}

// Filter описывает выборку коллекции пользователя.
type Filter struct {
	UserID   int64
	Status   Status
	GenreIDs []int16
	Query    string
	SortBy   SortBy
	Desc     bool
	Limit    int
	Offset   int
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
	Note       *string
}

const (
	MinUserRating = 1
	MaxUserRating = 10
)
