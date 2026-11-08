// Package dto описывает контракт HTTP API. Доменные структуры json-тегов
// не имеют, поэтому всё, что уходит наружу, объявлено здесь.
package dto

type Error struct {
	Code                string `json:"code"`
	Message             string `json:"message"`
	ExistingUserMovieID int64  `json:"existing_user_movie_id,omitempty"`
}

type Genre struct {
	ID     int16  `json:"id"`
	NameRU string `json:"name_ru"`
	NameEN string `json:"name_en"`
}

type Movie struct {
	MediaType       string     `json:"media_type"`
	SeasonCount     *int16     `json:"season_count"`
	ID              int64      `json:"id"`
	TMDBID          int32      `json:"tmdb_id,omitempty"`
	KinopoiskID     int32      `json:"kinopoisk_id,omitempty"`
	Title           string     `json:"title"`
	OriginalTitle   string     `json:"original_title,omitempty"`
	Year            int16      `json:"year,omitempty"`
	Overview        string     `json:"overview"`
	PosterURL       string     `json:"poster_url,omitempty"`
	RuntimeMin      int16      `json:"runtime_min,omitempty"`
	RatingProvider  string     `json:"rating_provider,omitempty"`
	RatingVoteCount *int32     `json:"rating_vote_count"`
	Rating          float32    `json:"rating,omitempty"`
	Genres          []Genre    `json:"genres"`
	Directors       []Director `json:"directors"`
}

type UserMovie struct {
	ID         int64  `json:"id"`
	Movie      Movie  `json:"movie"`
	Status     string `json:"status"`
	UserRating int16  `json:"user_rating,omitempty"`
	Review     string `json:"review"`
	Note       string `json:"note,omitempty"`
	Source     string `json:"source"`
	SourceURL  string `json:"source_url,omitempty"`
	AddedAt    string `json:"added_at"`
	WatchedAt  string `json:"watched_at,omitempty"`
}

type MovieList struct {
	Items   []UserMovie `json:"items"`
	Total   int         `json:"total"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
	Summary Summary     `json:"summary"`
}

type GenreList struct {
	Items []Genre `json:"items"`
}

type UpdateMovieRequest struct {
	Status     *string `json:"status,omitempty"`
	UserRating *int16  `json:"user_rating,omitempty"`
	Review     *string `json:"review,omitempty"`
	Note       *string `json:"note,omitempty"`
}

type CreateJobRequest struct {
	// Input принимает и ссылку на рилс, и название фильма —
	// сервис сам определяет, что это.
	Input string `json:"input"`
}

type Candidate struct {
	MediaType       string     `json:"media_type"`
	SeasonCount     *int16     `json:"season_count"`
	OriginalTitle   string     `json:"original_title,omitempty"`
	RuntimeMin      int16      `json:"runtime_min,omitempty"`
	RatingProvider  string     `json:"rating_provider,omitempty"`
	RatingVoteCount *int32     `json:"rating_vote_count"`
	Genres          []Genre    `json:"genres"`
	Directors       []Director `json:"directors"`
	Provider        string     `json:"provider"`
	ExternalID      int32      `json:"external_id"`
	Title           string     `json:"title"`
	Year            int16      `json:"year,omitempty"`
	Overview        string     `json:"overview"`
	PosterURL       string     `json:"poster_url,omitempty"`
	Rating          float32    `json:"rating,omitempty"`
}

type Job struct {
	ID         int64       `json:"id"`
	Kind       string      `json:"kind"`
	Status     string      `json:"status"`
	QueryTitle string      `json:"query_title,omitempty"`
	QueryYear  int16       `json:"query_year,omitempty"`
	SourceURL  string      `json:"source_url,omitempty"`
	Candidates []Candidate `json:"candidates"`
	Error      string      `json:"error,omitempty"`
	ErrorCode  string      `json:"error_code,omitempty"`
}

type ConfirmJobRequest struct {
	MediaType  string `json:"media_type,omitempty"`
	Status     string `json:"status,omitempty"`
	UserRating int16  `json:"user_rating,omitempty"`
	Note       string `json:"note,omitempty"`
	Review     string `json:"review,omitempty"`
	Provider   string `json:"provider"`
	ExternalID int32  `json:"external_id"`
}

type Me struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}

type Summary struct {
	MovieCount        int      `json:"movie_count"`
	RatedCount        int      `json:"rated_count"`
	AverageUserRating *float64 `json:"average_user_rating"`
}
type GenreStats struct {
	Genre
	WantToWatchCount int `json:"want_to_watch_count"`
	WatchedCount     int `json:"watched_count"`
}

type Statistics struct {
	TotalCount       int `json:"total_count"`
	WantToWatchCount int `json:"want_to_watch_count"`
	WatchedCount     int `json:"watched_count"`
}

// Preview identities have external IDs; an internal ID is assigned on confirmation.
type Director struct {
	ID           int64  `json:"id,omitempty"`
	TMDBID       int32  `json:"tmdb_id,omitempty"`
	KinopoiskID  int32  `json:"kinopoisk_id,omitempty"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
}
type DirectorStats struct {
	Director
	TotalCount        int      `json:"total_count"`
	WantToWatchCount  int      `json:"want_to_watch_count"`
	WatchedCount      int      `json:"watched_count"`
	RatedCount        int      `json:"rated_count"`
	AverageUserRating *float64 `json:"average_user_rating"`
}
type DirectorList struct {
	Items  []DirectorStats `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// DirectorOverview — карточка в разделе «Мои режиссёры».
// filmography_complete=false: фильмография из каталога ещё не загружена,
// work_count пока считает только работы из коллекции.
type DirectorOverview struct {
	Director
	ID                  int64    `json:"id"`
	PhotoURL            string   `json:"photo_url,omitempty"`
	BirthDate           string   `json:"birth_date,omitempty"`
	DeathDate           string   `json:"death_date,omitempty"`
	PlaceOfBirth        string   `json:"place_of_birth,omitempty"`
	KnownFor            []string `json:"known_for"`
	Followed            bool     `json:"followed"`
	FilmographyComplete bool     `json:"filmography_complete"`
	WorkCount           int      `json:"work_count"`
	InCollectionCount   int      `json:"in_collection_count"`
	WatchedCount        int      `json:"watched_count"`
	WantToWatchCount    int      `json:"want_to_watch_count"`
	RatedCount          int      `json:"rated_count"`
	AverageUserRating   *float64 `json:"average_user_rating"`
}

type DirectorScopeCounts struct {
	All        int `json:"all"`
	Followed   int `json:"followed"`
	Collection int `json:"collection"`
}

type DirectorOverviewList struct {
	Items  []DirectorOverview  `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
	Counts DirectorScopeCounts `json:"counts"`
}

type FilmographyItem struct {
	TMDBID        int32    `json:"tmdb_id,omitempty"`
	MovieID       int64    `json:"movie_id,omitempty"`
	MediaType     string   `json:"media_type"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"original_title"`
	Year          int16    `json:"year,omitempty"`
	ReleaseDate   string   `json:"release_date,omitempty"`
	PosterURL     string   `json:"poster_url,omitempty"`
	Rating        *float64 `json:"rating"`
	VoteCount     int32    `json:"vote_count"`
	EpisodeCount  *int32   `json:"episode_count,omitempty"`
	Upcoming      bool     `json:"upcoming"`
	UserMovieID   int64    `json:"user_movie_id,omitempty"`
	Status        string   `json:"status,omitempty"`
	UserRating    int16    `json:"user_rating,omitempty"`
}

type DirectorProfile struct {
	DirectorOverview
	Biography     string            `json:"biography"`
	BiographyLang string            `json:"biography_lang,omitempty"`
	Stale         bool              `json:"stale"`
	Filmography   []FilmographyItem `json:"filmography"`
}

// PersonSearchResult: id есть, только если человек уже заведён как режиссёр.
type PersonSearchResult struct {
	ID                 *int64   `json:"id"`
	TMDBID             int32    `json:"tmdb_id"`
	Name               string   `json:"name"`
	OriginalName       string   `json:"original_name"`
	PhotoURL           string   `json:"photo_url,omitempty"`
	KnownForDepartment string   `json:"known_for_department,omitempty"`
	KnownFor           []string `json:"known_for"`
	Followed           bool     `json:"followed"`
}

type PersonSearchList struct {
	Items []PersonSearchResult `json:"items"`
}

type TMDBPersonRequest struct {
	TMDBID int32 `json:"tmdb_id"`
}

// AddMovieRequest — добавление работы из каталога (фильмография режиссёра,
// карточка каталога). Оценка и отзыв — для «Уже смотрел».
type AddMovieRequest struct {
	Provider   string `json:"provider"`
	ExternalID int32  `json:"external_id"`
	MediaType  string `json:"media_type"`
	Status     string `json:"status,omitempty"`
	UserRating int16  `json:"user_rating,omitempty"`
	Review     string `json:"review,omitempty"`
}

// CatalogMovie — карточка работы прямо из каталога. Поля Movie плюс дата
// выхода и пометки коллекции текущего пользователя; id равен 0, если работы
// нет в общем справочнике.
type CatalogMovie struct {
	Movie
	ReleaseDate string `json:"release_date,omitempty"`
	Upcoming    bool   `json:"upcoming"`
	UserMovieID int64  `json:"user_movie_id,omitempty"`
	Status      string `json:"status,omitempty"`
	UserRating  int16  `json:"user_rating,omitempty"`
}
