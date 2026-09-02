// Package dto описывает контракт HTTP API. Доменные структуры json-тегов
// не имеют, поэтому всё, что уходит наружу, объявлено здесь.
package dto

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Genre struct {
	ID     int16  `json:"id"`
	NameRU string `json:"name_ru"`
	NameEN string `json:"name_en"`
}

type Movie struct {
	ID            int64   `json:"id"`
	TMDBID        int32   `json:"tmdb_id,omitempty"`
	KinopoiskID   int32   `json:"kinopoisk_id,omitempty"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title,omitempty"`
	Year          int16   `json:"year,omitempty"`
	Overview      string  `json:"overview"`
	PosterURL     string  `json:"poster_url,omitempty"`
	RuntimeMin    int16   `json:"runtime_min,omitempty"`
	Rating        float32 `json:"rating,omitempty"`
	Genres        []Genre `json:"genres"`
}

type UserMovie struct {
	ID         int64  `json:"id"`
	Movie      Movie  `json:"movie"`
	Status     string `json:"status"`
	UserRating int16  `json:"user_rating,omitempty"`
	Note       string `json:"note,omitempty"`
	Source     string `json:"source"`
	SourceURL  string `json:"source_url,omitempty"`
	AddedAt    string `json:"added_at"`
	WatchedAt  string `json:"watched_at,omitempty"`
}

type MovieList struct {
	Items  []UserMovie `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type GenreList struct {
	Items []Genre `json:"items"`
}

type UpdateMovieRequest struct {
	Status     *string `json:"status,omitempty"`
	UserRating *int16  `json:"user_rating,omitempty"`
	Note       *string `json:"note,omitempty"`
}

type CreateJobRequest struct {
	// Input принимает и ссылку на рилс, и название фильма —
	// сервис сам определяет, что это.
	Input string `json:"input"`
}

type Candidate struct {
	Provider   string  `json:"provider"`
	ExternalID int32   `json:"external_id"`
	Title      string  `json:"title"`
	Year       int16   `json:"year,omitempty"`
	Overview   string  `json:"overview"`
	PosterURL  string  `json:"poster_url,omitempty"`
	Rating     float32 `json:"rating,omitempty"`
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
}

type ConfirmJobRequest struct {
	Provider   string `json:"provider"`
	ExternalID int32  `json:"external_id"`
}

type Me struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}
