package tmdb

import (
	"io"
	"strconv"
	"strings"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// maxResponseBytes страхует от неожиданно огромного ответа внешнего сервиса.
const maxResponseBytes = 4 << 20

func readLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseBytes))
}

type searchResponse struct {
	Results []result `json:"results"`
}

type result struct {
	ID            int32   `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	ReleaseDate   string  `json:"release_date"`
	Popularity    float32 `json:"popularity"`
	VoteAverage   float32 `json:"vote_average"`
	GenreIDs      []int32 `json:"genre_ids"`
}

func (r result) toCandidate() movie.Candidate {
	return movie.Candidate{
		Provider:      movie.ProviderTMDB,
		ExternalID:    r.ID,
		Title:         r.Title,
		OriginalTitle: r.OriginalTitle,
		Year:          parseYear(r.ReleaseDate),
		Overview:      r.Overview,
		PosterPath:    r.PosterPath,
		Popularity:    r.Popularity,
		Rating:        r.VoteAverage,
		GenreTMDBIDs:  r.GenreIDs,
	}
}

type detailsResponse struct {
	result
	Runtime int16 `json:"runtime"`
	Genres  []struct {
		ID   int32  `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

func (d detailsResponse) toCandidate() movie.Candidate {
	c := d.result.toCandidate()

	ids := make([]int32, 0, len(d.Genres))
	names := make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		ids = append(ids, g.ID)
		names = append(names, g.Name)
	}
	if len(ids) > 0 {
		c.GenreTMDBIDs = ids
		c.GenreNames = names
	}

	return c
}

func parseYear(releaseDate string) int16 {
	if len(releaseDate) < 4 {
		return 0
	}
	y, err := strconv.Atoi(strings.TrimSpace(releaseDate[:4]))
	if err != nil {
		return 0
	}
	return int16(y) //nolint:gosec // год фильма из внешнего API не выходит за диапазон int16
}
