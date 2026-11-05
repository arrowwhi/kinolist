package collection

import (
	"context"
	"errors"
	"fmt"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
)

//go:generate mockgen -source=usecase.go -destination=mocks/mocks.go -package=mocks

// Интерфейсы объявлены здесь, на стороне потребителя.
type (
	movieRepository interface {
		Get(ctx context.Context, userID, userMovieID int64) (movie.UserMovie, error)
		List(ctx context.Context, f movie.Filter) ([]movie.UserMovie, error)
		Count(ctx context.Context, f movie.Filter) (int, error)
		Summary(ctx context.Context, f movie.Filter) (movie.Summary, error)
		CollectionGenres(ctx context.Context, userID int64, mediaTypes ...movie.MediaType) ([]movie.GenreStats, error)
		Update(ctx context.Context, userID, userMovieID int64, upd movie.Update) (movie.UserMovie, error)
		Delete(ctx context.Context, userID, userMovieID int64) error
		ListGenres(ctx context.Context) ([]movie.Genre, error)
	}

	userRepository interface {
		EnsureExists(ctx context.Context, u user.User) (user.User, error)
		GetByTelegramID(ctx context.Context, telegramID int64) (user.User, error)
	}
)

type UseCase struct {
	movies movieRepository
	users  userRepository
}

func New(movies movieRepository, users userRepository) *UseCase {
	return &UseCase{movies: movies, users: users}
}

// Page — страница коллекции вместе с общим размером выборки.
type Page struct {
	Items   []movie.UserMovie
	Total   int
	Limit   int
	Offset  int
	Summary movie.Summary
}

func (uc *UseCase) List(ctx context.Context, f movie.Filter) (Page, error) {
	if f.MediaType != "" && !f.MediaType.Valid() {
		return Page{}, movie.ErrInvalidMediaType
	}
	f = f.Normalize()

	if f.Status != "" && !f.Status.Valid() {
		return Page{}, movie.ErrInvalidStatus
	}

	items, err := uc.movies.List(ctx, f)
	if err != nil {
		return Page{}, fmt.Errorf("list user movies: %w", err)
	}

	total, err := uc.movies.Count(ctx, f)
	if err != nil {
		return Page{}, fmt.Errorf("count user movies: %w", err)
	}

	summaryFilter := f
	summaryFilter.Query = ""
	summary, err := uc.movies.Summary(ctx, summaryFilter)
	if err != nil {
		return Page{}, fmt.Errorf("get summary: %w", err)
	}
	return Page{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset, Summary: summary}, nil
}

func (uc *UseCase) Get(ctx context.Context, userID, userMovieID int64) (movie.UserMovie, error) {
	um, err := uc.movies.Get(ctx, userID, userMovieID)
	if err != nil {
		return movie.UserMovie{}, fmt.Errorf("get user movie: %w", err)
	}
	return um, nil
}

func (uc *UseCase) Update(ctx context.Context, userID, userMovieID int64, upd movie.Update) (movie.UserMovie, error) {
	if err := validateUpdate(upd); err != nil {
		return movie.UserMovie{}, err
	}

	um, err := uc.movies.Update(ctx, userID, userMovieID, upd)
	if err != nil {
		return movie.UserMovie{}, fmt.Errorf("update user movie: %w", err)
	}

	return um, nil
}

func (uc *UseCase) Delete(ctx context.Context, userID, userMovieID int64) error {
	if err := uc.movies.Delete(ctx, userID, userMovieID); err != nil {
		return fmt.Errorf("delete user movie: %w", err)
	}
	return nil
}

func (uc *UseCase) Genres(ctx context.Context) ([]movie.Genre, error) {
	genres, err := uc.movies.ListGenres(ctx)
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	return genres, nil
}

// ResolveUser заводит пользователя при первом обращении: отдельного шага
// регистрации ни в боте, ни в Mini App нет.
func (uc *UseCase) ResolveUser(ctx context.Context, u user.User) (user.User, error) {
	if u.TelegramID == 0 {
		return user.User{}, user.ErrInvalidTelegram
	}

	resolved, err := uc.users.EnsureExists(ctx, u)
	if err != nil {
		return user.User{}, fmt.Errorf("ensure user exists: %w", err)
	}

	return resolved, nil
}

func validateUpdate(upd movie.Update) error {
	if upd.Review != nil {
		if err := movie.ValidateReview(*upd.Review); err != nil {
			return err
		}
	}
	if upd.Status == nil && upd.UserRating == nil && upd.Note == nil && upd.Review == nil {
		return errors.New("update: nothing to change")
	}
	if upd.Status != nil && !upd.Status.Valid() {
		return movie.ErrInvalidStatus
	}
	if upd.UserRating != nil && *upd.UserRating != 0 &&
		(*upd.UserRating < movie.MinUserRating || *upd.UserRating > movie.MaxUserRating) {
		return movie.ErrInvalidRating
	}
	return nil
}

func (uc *UseCase) CollectionGenres(ctx context.Context, userID int64, mediaTypes ...movie.MediaType) ([]movie.GenreStats, error) {
	if len(mediaTypes) > 0 && mediaTypes[0] != "" && !mediaTypes[0].Valid() {
		return nil, movie.ErrInvalidMediaType
	}
	return uc.movies.CollectionGenres(ctx, userID, mediaTypes...)
}
