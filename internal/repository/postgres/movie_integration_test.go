//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func seedUser(t *testing.T, telegramID int64) user.User {
	t.Helper()
	repo := postgres.NewUserRepository(newManager(t))
	u, err := repo.EnsureExists(context.Background(), user.User{TelegramID: telegramID, Username: "tester"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func genresByTMDB(t *testing.T, repo *postgres.MovieRepository, ids ...int32) []movie.Genre {
	t.Helper()
	g, err := repo.GenresByTMDBIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("genres by tmdb ids: %v", err)
	}
	return g
}

func TestUserRepositoryEnsureExistsIsIdempotent(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewUserRepository(newManager(t))

	first, err := repo.EnsureExists(ctx, user.User{TelegramID: 555, Username: "old", FirstName: "Иван"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := repo.EnsureExists(ctx, user.User{TelegramID: 555, Username: "new", FirstName: "Иван"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("id changed: %d -> %d", first.ID, second.ID)
	}
	if second.Username != "new" {
		t.Fatalf("username = %q, want updated to %q", second.Username, "new")
	}

	if _, err := repo.GetByTelegramID(ctx, 999); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMovieUpsertReplacesGenres(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))

	drama := genresByTMDB(t, repo, 18)
	if len(drama) != 1 {
		t.Fatalf("expected the seeded drama genre, got %d", len(drama))
	}

	m, err := repo.Upsert(ctx, movie.Movie{
		TMDBID: 438631, Title: "Дюна", OriginalTitle: "Dune", Year: 2021,
		Overview: "Пустыня", Rating: 7.8, Genres: drama,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Повторный upsert того же фильма обновляет карточку, а не плодит дубли.
	scifi := genresByTMDB(t, repo, 878, 12)
	again, err := repo.Upsert(ctx, movie.Movie{
		TMDBID: 438631, Title: "Дюна", OriginalTitle: "Dune", Year: 2021,
		Overview: "Обновлённое описание", Rating: 8.0, Genres: scifi,
	})
	if err != nil {
		t.Fatalf("upsert again: %v", err)
	}
	if again.ID != m.ID {
		t.Fatalf("movie duplicated: %d != %d", again.ID, m.ID)
	}

	stored, err := repo.GetByTMDBID(ctx, 438631)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Overview != "Обновлённое описание" {
		t.Fatalf("overview not updated: %q", stored.Overview)
	}
	if len(stored.Genres) != 2 {
		t.Fatalf("genres = %d, want 2 (old ones must be replaced)", len(stored.Genres))
	}
}

func TestCollectionFiltersAndSorting(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	u := seedUser(t, 100)
	other := seedUser(t, 200)

	drama := genresByTMDB(t, repo, 18)
	comedy := genresByTMDB(t, repo, 35)

	add := func(tmdbID int32, title string, year int16, genres []movie.Genre, status movie.Status) movie.UserMovie {
		t.Helper()
		m, err := repo.Upsert(ctx, movie.Movie{TMDBID: tmdbID, Title: title, OriginalTitle: title, Year: year, Genres: genres})
		if err != nil {
			t.Fatalf("upsert %s: %v", title, err)
		}
		um, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m, Status: status, Source: movie.SourceManual})
		if err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
		return um
	}

	dune := add(1, "Дюна", 2021, drama, movie.StatusWantToWatch)
	add(2, "Брат", 1997, drama, movie.StatusWatched)
	add(3, "Бриджет Джонс", 2001, comedy, movie.StatusWantToWatch)

	// Чужая коллекция не должна протекать в выдачу.
	foreign, _ := repo.Upsert(ctx, movie.Movie{TMDBID: 4, Title: "Чужое", Genres: drama})
	if _, err := repo.Add(ctx, movie.UserMovie{UserID: other.ID, Movie: foreign}); err != nil {
		t.Fatalf("add foreign: %v", err)
	}

	tests := []struct {
		name      string
		filter    movie.Filter
		wantCount int
		wantFirst string
	}{
		{
			name:      "all movies of the user, newest first",
			filter:    movie.Filter{UserID: u.ID, SortBy: movie.SortByAddedAt, Desc: true},
			wantCount: 3, wantFirst: "Бриджет Джонс",
		},
		{
			name:      "filter by status",
			filter:    movie.Filter{UserID: u.ID, Status: movie.StatusWatched},
			wantCount: 1, wantFirst: "Брат",
		},
		{
			name:      "filter by genre",
			filter:    movie.Filter{UserID: u.ID, GenreIDs: []int16{comedy[0].ID}},
			wantCount: 1, wantFirst: "Бриджет Джонс",
		},
		{
			name:      "search by title substring, case insensitive",
			filter:    movie.Filter{UserID: u.ID, Query: "дюн"},
			wantCount: 1, wantFirst: "Дюна",
		},
		{
			name:      "sort by year ascending",
			filter:    movie.Filter{UserID: u.ID, SortBy: movie.SortByYear},
			wantCount: 3, wantFirst: "Брат",
		},
		{
			name:      "other user's collection is isolated",
			filter:    movie.Filter{UserID: other.ID},
			wantCount: 1, wantFirst: "Чужое",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(items) != tt.wantCount {
				t.Fatalf("items = %d, want %d", len(items), tt.wantCount)
			}
			if items[0].Movie.Title != tt.wantFirst {
				t.Fatalf("first = %q, want %q", items[0].Movie.Title, tt.wantFirst)
			}

			total, err := repo.Count(ctx, tt.filter)
			if err != nil {
				t.Fatalf("count: %v", err)
			}
			if total != tt.wantCount {
				t.Fatalf("count = %d, want %d", total, tt.wantCount)
			}
		})
	}

	t.Run("genres are attached to every listed movie", func(t *testing.T) {
		items, err := repo.List(ctx, movie.Filter{UserID: u.ID})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, it := range items {
			if len(it.Movie.Genres) == 0 {
				t.Fatalf("%q came back without genres", it.Movie.Title)
			}
		}
	})

	t.Run("pagination", func(t *testing.T) {
		page, err := repo.List(ctx, movie.Filter{UserID: u.ID, Limit: 2, Offset: 2, SortBy: movie.SortByAddedAt})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(page) != 1 {
			t.Fatalf("page = %d, want 1", len(page))
		}
	})

	t.Run("duplicate is reported as a domain error", func(t *testing.T) {
		_, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: dune.Movie})
		if !errors.Is(err, movie.ErrAlreadyInCollection) {
			t.Fatalf("err = %v, want ErrAlreadyInCollection", err)
		}
	})
}

func TestUpdateAndDelete(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))
	u := seedUser(t, 300)

	m, err := repo.Upsert(ctx, movie.Movie{TMDBID: 10, Title: "Дюна", Year: 2021})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	um, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	watched := movie.StatusWatched
	rating := int16(9)
	note := "лучшая часть"

	updated, err := repo.Update(ctx, u.ID, um.ID, movie.Update{Status: &watched, UserRating: &rating, Note: &note})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != movie.StatusWatched || updated.UserRating != 9 || updated.Note != note {
		t.Fatalf("update did not apply: %+v", updated)
	}
	if updated.WatchedAt == nil {
		t.Fatal("watched_at must be set together with the watched status")
	}

	// Возврат в "хочу посмотреть" обязан снять и отметку о дате просмотра.
	want := movie.StatusWantToWatch
	back, err := repo.Update(ctx, u.ID, um.ID, movie.Update{Status: &want})
	if err != nil {
		t.Fatalf("update back: %v", err)
	}
	if back.WatchedAt != nil {
		t.Fatal("watched_at must be cleared when the movie returns to the watchlist")
	}

	if _, err := repo.Update(ctx, u.ID+999, um.ID, movie.Update{Status: &want}); !errors.Is(err, movie.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for another user's row", err)
	}

	if err := repo.Delete(ctx, u.ID, um.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, u.ID, um.ID); !errors.Is(err, movie.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound on repeated delete", err)
	}
}

func TestGenresByExternalNames(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	repo := postgres.NewMovieRepository(newManager(t))

	// Кинопоиск отдаёт «фантастика», TMDB — Science Fiction; маппинг обязан
	// свести их в один жанр справочника.
	genres, err := repo.GenresByExternalNames(ctx, movie.ProviderKinopoisk, []string{"Фантастика", "драма"})
	if err != nil {
		t.Fatalf("genres by external names: %v", err)
	}
	if len(genres) != 2 {
		t.Fatalf("genres = %d, want 2", len(genres))
	}

	unknown, err := repo.GenresByExternalNames(ctx, movie.ProviderKinopoisk, []string{"такого жанра нет"})
	if err != nil {
		t.Fatalf("unknown genre: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("unknown genre resolved to %d rows", len(unknown))
	}
}

// TestAddDuplicateKeepsTransactionUsable закрывает найденный на дымовом
// прогоне баг: нарушение уникальности переводило транзакцию в aborted,
// и доменная ошибка подменялась ошибкой драйвера на следующем запросе.
func TestAddDuplicateKeepsTransactionUsable(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	manager := newManager(t)
	repo := postgres.NewMovieRepository(manager)
	jobs := postgres.NewJobRepository(manager)
	u := seedUser(t, 800)

	m, err := repo.Upsert(ctx, movie.Movie{TMDBID: 42, Title: "Дюна", Year: 2021})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := repo.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m}); err != nil {
		t.Fatalf("first add: %v", err)
	}

	txCtx, err := manager.BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	var txErr error
	defer manager.FinishTransaction(txCtx, &txErr)

	if _, err := repo.Add(txCtx, movie.UserMovie{UserID: u.ID, Movie: m}); !errors.Is(err, movie.ErrAlreadyInCollection) {
		t.Fatalf("err = %v, want ErrAlreadyInCollection", err)
	}

	// Транзакция обязана остаться работоспособной: usecase после дубля
	// ещё закрывает задачу и чистит сессию бота.
	if err := jobs.ClearSession(txCtx, u.ID); err != nil {
		t.Fatalf("transaction is unusable after a duplicate insert: %v", err)
	}
}
