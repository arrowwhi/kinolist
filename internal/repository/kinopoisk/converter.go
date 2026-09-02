package kinopoisk

import (
	"strconv"
	"strings"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

type searchResponse struct {
	Films []film `json:"films"`
}

type film struct {
	FilmID      int32   `json:"filmId"`
	NameRU      string  `json:"nameRu"`
	NameEN      string  `json:"nameEn"`
	Year        string  `json:"year"`
	Description string  `json:"description"`
	PosterURL   string  `json:"posterUrl"`
	Rating      string  `json:"rating"`
	Genres      []genre `json:"genres"`
}

type genre struct {
	Genre string `json:"genre"`
}

func (f film) toCandidate() movie.Candidate {
	return movie.Candidate{
		Provider:      movie.ProviderKinopoisk,
		ExternalID:    f.FilmID,
		Title:         f.NameRU,
		OriginalTitle: f.NameEN,
		Year:          parseYear(f.Year),
		Overview:      f.Description,
		PosterPath:    f.PosterURL,
		Rating:        parseRating(f.Rating),
		GenreNames:    genreNames(f.Genres),
	}
}

type filmDetails struct {
	KinopoiskID  int32   `json:"kinopoiskId"`
	NameRU       string  `json:"nameRu"`
	NameOriginal string  `json:"nameOriginal"`
	Year         int16   `json:"year"`
	Description  string  `json:"description"`
	PosterURL    string  `json:"posterUrl"`
	RatingKP     float32 `json:"ratingKinopoisk"`
	FilmLength   int16   `json:"filmLength"`
	Genres       []genre `json:"genres"`
}

func (d filmDetails) toCandidate() movie.Candidate {
	return movie.Candidate{
		Provider:      movie.ProviderKinopoisk,
		ExternalID:    d.KinopoiskID,
		Title:         d.NameRU,
		OriginalTitle: d.NameOriginal,
		Year:          d.Year,
		Overview:      d.Description,
		PosterPath:    d.PosterURL,
		Rating:        d.RatingKP,
		GenreNames:    genreNames(d.Genres),
	}
}

func genreNames(genres []genre) []string {
	out := make([]string, 0, len(genres))
	for _, g := range genres {
		if name := strings.TrimSpace(g.Genre); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parseYear терпит и "1997", и диапазон "1997-2001" из сериалов.
func parseYear(s string) int16 {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return int16(y) //nolint:gosec // год фильма из внешнего API не выходит за диапазон int16
}

func parseRating(s string) float32 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil {
		return 0
	}
	return float32(v)
}
