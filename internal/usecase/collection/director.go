package collection

import (
	"context"
	"fmt"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

type DirectorPage struct {
	Items  []movie.DirectorStats
	Total  int
	Limit  int
	Offset int
}

func (uc *UseCase) ListDirectors(ctx context.Context, f movie.DirectorFilter) (DirectorPage, error) {
	if f.MediaType != "" && !f.MediaType.Valid() {
		return DirectorPage{}, movie.ErrInvalidMediaType
	}
	f = f.Normalize()
	switch f.Sort {
	case "name", "work_count", "average_user_rating":
	default:
		return DirectorPage{}, movie.ErrInvalidDirectorFilter
	}
	items, total, err := uc.movies.ListDirectors(ctx, f)
	if err != nil {
		return DirectorPage{}, fmt.Errorf("list collection directors: %w", err)
	}
	return DirectorPage{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (uc *UseCase) GetDirector(ctx context.Context, userID, id int64, mt movie.MediaType) (movie.DirectorStats, error) {
	if mt != "" && !mt.Valid() {
		return movie.DirectorStats{}, movie.ErrInvalidMediaType
	}
	d, err := uc.movies.GetDirector(ctx, userID, id, mt)
	if err != nil {
		return d, fmt.Errorf("get collection director: %w", err)
	}
	return d, nil
}

func (uc *UseCase) CollectionGenresFiltered(ctx context.Context, f movie.Filter) ([]movie.GenreStats, error) {
	if f.MediaType != "" && !f.MediaType.Valid() {
		return nil, movie.ErrInvalidMediaType
	}
	return uc.movies.CollectionGenresFiltered(ctx, f)
}
