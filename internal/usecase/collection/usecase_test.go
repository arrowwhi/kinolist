package collection_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/usecase/collection"
	"github.com/arrowwhi/kinolist/internal/usecase/collection/mocks"
)

func newUseCase(t *testing.T) (*collection.UseCase, *mocks.MockmovieRepository, *mocks.MockuserRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	movies := mocks.NewMockmovieRepository(ctrl)
	users := mocks.NewMockuserRepository(ctrl)
	return collection.New(movies, users), movies, users
}

func TestListNormalizesFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         movie.Filter
		wantSortBy movie.SortBy
		wantLimit  int
		wantOffset int
	}{
		{
			name: "defaults are applied", in: movie.Filter{UserID: 1},
			wantSortBy: movie.SortByAddedAt, wantLimit: movie.DefaultLimit,
		},
		{
			name:       "unknown sort falls back to added_at",
			in:         movie.Filter{UserID: 1, SortBy: "; DROP TABLE movies", Limit: 10},
			wantSortBy: movie.SortByAddedAt, wantLimit: 10,
		},
		{
			name: "limit is capped", in: movie.Filter{UserID: 1, Limit: 100000},
			wantSortBy: movie.SortByAddedAt, wantLimit: movie.MaxLimit,
		},
		{
			name: "negative offset is clamped", in: movie.Filter{UserID: 1, Offset: -5},
			wantSortBy: movie.SortByAddedAt, wantLimit: movie.DefaultLimit, wantOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, movies, _ := newUseCase(t)

			check := func(_ context.Context, f movie.Filter) {
				if f.SortBy != tt.wantSortBy {
					t.Fatalf("sort by = %q, want %q", f.SortBy, tt.wantSortBy)
				}
				if f.Limit != tt.wantLimit {
					t.Fatalf("limit = %d, want %d", f.Limit, tt.wantLimit)
				}
				if f.Offset != tt.wantOffset {
					t.Fatalf("offset = %d, want %d", f.Offset, tt.wantOffset)
				}
			}

			movies.EXPECT().List(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f movie.Filter) ([]movie.UserMovie, error) {
					check(ctx, f)
					return []movie.UserMovie{{ID: 1}}, nil
				})
			movies.EXPECT().Count(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f movie.Filter) (int, error) {
					check(ctx, f)
					return 1, nil
				})

			page, err := uc.List(context.Background(), tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page.Total != 1 || len(page.Items) != 1 {
				t.Fatalf("unexpected page: %+v", page)
			}
		})
	}
}

func TestListRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUseCase(t)

	_, err := uc.List(context.Background(), movie.Filter{UserID: 1, Status: "seen_maybe"})
	if !errors.Is(err, movie.ErrInvalidStatus) {
		t.Fatalf("err = %v, want ErrInvalidStatus", err)
	}
}

func TestUpdateValidation(t *testing.T) {
	t.Parallel()

	status := func(s movie.Status) *movie.Status { return &s }
	rating := func(r int16) *int16 { return &r }
	note := func(n string) *string { return &n }

	tests := []struct {
		name    string
		upd     movie.Update
		wantErr error
		allowed bool
	}{
		{name: "empty update is rejected", upd: movie.Update{}},
		{name: "invalid status", upd: movie.Update{Status: status("burned")}, wantErr: movie.ErrInvalidStatus},
		{name: "rating too high", upd: movie.Update{UserRating: rating(11)}, wantErr: movie.ErrInvalidRating},
		{name: "rating too low", upd: movie.Update{UserRating: rating(-1)}, wantErr: movie.ErrInvalidRating},
		{name: "valid status", upd: movie.Update{Status: status(movie.StatusWatched)}, allowed: true},
		{name: "valid rating", upd: movie.Update{UserRating: rating(9)}, allowed: true},
		{name: "zero rating clears it", upd: movie.Update{UserRating: rating(0)}, allowed: true},
		{name: "note only", upd: movie.Update{Note: note("пересмотреть")}, allowed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, movies, _ := newUseCase(t)
			if tt.allowed {
				movies.EXPECT().Update(gomock.Any(), int64(1), int64(2), tt.upd).
					Return(movie.UserMovie{ID: 2}, nil)
			}

			_, err := uc.Update(context.Background(), 1, 2, tt.upd)

			switch {
			case tt.allowed:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			default:
				if err == nil {
					t.Fatal("expected an error")
				}
			}
		})
	}
}

func TestResolveUser(t *testing.T) {
	t.Parallel()

	t.Run("creates the user on first contact", func(t *testing.T) {
		t.Parallel()

		uc, _, users := newUseCase(t)
		users.EXPECT().EnsureExists(gomock.Any(), gomock.Any()).
			Return(user.User{ID: 42, TelegramID: 777}, nil)

		got, err := uc.ResolveUser(context.Background(), user.User{TelegramID: 777})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != 42 {
			t.Fatalf("id = %d, want 42", got.ID)
		}
	})

	t.Run("rejects a missing telegram id", func(t *testing.T) {
		t.Parallel()

		uc, _, _ := newUseCase(t)
		_, err := uc.ResolveUser(context.Background(), user.User{})
		if !errors.Is(err, user.ErrInvalidTelegram) {
			t.Fatalf("err = %v, want ErrInvalidTelegram", err)
		}
	})
}

func TestDelete(t *testing.T) {
	t.Parallel()

	uc, movies, _ := newUseCase(t)
	movies.EXPECT().Delete(gomock.Any(), int64(1), int64(2)).Return(movie.ErrNotFound)

	err := uc.Delete(context.Background(), 1, 2)
	if !errors.Is(err, movie.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (wrapping must preserve the sentinel)", err)
	}
}
