package movie

import (
	"strings"
	"time"
	"unicode"
)

// NameSource — откуда взято отображаемое имя режиссёра. Более надёжный
// источник никогда не перезаписывается менее надёжным.
type NameSource string

const (
	NameSourceKinopoisk       NameSource = "kinopoisk"
	NameSourceTMDBTranslation NameSource = "tmdb_translation"
	NameSourceTMDB            NameSource = "tmdb"
)

func (s NameSource) Rank() int {
	switch s {
	case NameSourceKinopoisk:
		return 3
	case NameSourceTMDBTranslation:
		return 2
	default:
		return 1
	}
}

// DirectingDepartment — значение known_for_department у режиссёров в TMDB.
const DirectingDepartment = "Directing"

// Person — человек из внешнего каталога вместе с биографией.
type Person struct {
	TMDBID             int32
	KinopoiskID        int32
	Name               string
	OriginalName       string
	NameSource         NameSource
	ProfilePath        string
	BirthDate          *time.Time
	DeathDate          *time.Time
	PlaceOfBirth       string
	Biography          string
	BiographyLang      string
	KnownForDepartment string
	KnownFor           []string
	Popularity         float32
}

func (p Person) BirthYear() int {
	if p.BirthDate == nil {
		return 0
	}
	return p.BirthDate.Year()
}

// DirectorWork — режиссёрская работа из фильмографии каталога.
type DirectorWork struct {
	MediaType     MediaType
	TMDBID        int32
	Title         string
	OriginalTitle string
	ReleaseDate   *time.Time
	Year          int16
	PosterPath    string
	Rating        *float64
	VoteCount     int32
	EpisodeCount  *int32
}

// Upcoming — работа ещё не вышла или дата выхода неизвестна.
func (w DirectorWork) Upcoming(now time.Time) bool {
	if w.ReleaseDate != nil {
		return w.ReleaseDate.After(now)
	}
	return w.Year == 0 || int(w.Year) > now.Year()
}

// MinVotesForRating — ниже этого числа голосов рейтинг каталога не показываем:
// у малоизвестных работ средняя из пары оценок ничего не говорит.
const MinVotesForRating = 20

// PersonDetails — человек и его режиссёрские работы.
type PersonDetails struct {
	Person
	Works []DirectorWork
}

// DirectorScope — какие режиссёры попадают в список «Мои режиссёры».
type DirectorScope string

const (
	DirectorScopeAll        DirectorScope = "all"
	DirectorScopeFollowed   DirectorScope = "followed"
	DirectorScopeCollection DirectorScope = "collection"
)

func (s DirectorScope) Valid() bool {
	return s == DirectorScopeAll || s == DirectorScopeFollowed || s == DirectorScopeCollection
}

const (
	MyDirectorsSortName       = "name"
	MyDirectorsSortProgress   = "progress"
	MyDirectorsSortUserRating = "user_rating"
)

type MyDirectorsFilter struct {
	UserID int64
	Scope  DirectorScope
	Query  string
	Sort   string
	Desc   bool
	Limit  int
	Offset int
}

func (f MyDirectorsFilter) Normalize() MyDirectorsFilter {
	page := (Filter{Limit: f.Limit, Offset: f.Offset}).Normalize()
	f.Limit, f.Offset = page.Limit, page.Offset
	if f.Scope == "" {
		f.Scope = DirectorScopeAll
	}
	if f.Sort == "" {
		f.Sort = MyDirectorsSortName
	}
	f.Query = strings.TrimSpace(f.Query)
	return f
}

func (f MyDirectorsFilter) Validate() error {
	if !f.Scope.Valid() {
		return ErrInvalidDirectorFilter
	}
	switch f.Sort {
	case MyDirectorsSortName, MyDirectorsSortProgress, MyDirectorsSortUserRating:
		return nil
	default:
		return ErrInvalidDirectorFilter
	}
}

// DirectorOverview — карточка режиссёра в списке «Мои режиссёры».
// Счётчики объединяют фильмографию из каталога с работами из коллекции.
type DirectorOverview struct {
	Director
	ProfilePath       string
	BirthDate         *time.Time
	DeathDate         *time.Time
	PlaceOfBirth      string
	KnownFor          []string
	Followed          bool
	FilmographyLoaded bool
	WorkCount         int
	InCollectionCount int
	WatchedCount      int
	WantToWatchCount  int
	RatedCount        int
	AverageUserRating *float64
}

type DirectorScopeCounts struct {
	All        int
	Followed   int
	Collection int
}

// FilmographyItem — работа режиссёра с пометками коллекции пользователя.
type FilmographyItem struct {
	DirectorWork
	MovieID     int64
	UserMovieID int64
	Status      Status
	UserRating  int16
}

type DirectorProfile struct {
	DirectorOverview
	Biography     string
	BiographyLang string
	// Stale — каталог не ответил, отдаём последний сохранённый снимок.
	Stale       bool
	Filmography []FilmographyItem
}

// DirectorSyncState — что известно о свежести фильмографии режиссёра.
type DirectorSyncState struct {
	ID         int64
	TMDBID     int32
	NameSource NameSource
	SyncedAt   *time.Time
}

// PersonSearchResult — человек из поиска; DirectorID равен 0, если его
// ещё нет в общем справочнике режиссёров.
type PersonSearchResult struct {
	Person
	DirectorID int64
	Followed   bool
}

// HasCyrillic отличает русское написание имени от латиницы.
func HasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}
