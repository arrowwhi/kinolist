//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
)

func TestDirectorFollowsFilmographyAndNameMerge(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	tx := newManager(t)
	movies := postgres.NewMovieRepository(tx)
	repo := postgres.NewDirectorRepository(tx)
	u, other := seedUser(t, 9801), seedUser(t, 9802)

	latin := movie.Director{TMDBID: 137427, Name: "Denis Villeneuve", OriginalName: "Denis Villeneuve"}
	kpOnly := movie.Director{KinopoiskID: 777, Name: "Дени Вильнёв", OriginalName: "Denis Villeneuve"}
	dune, err := movies.Upsert(ctx, movie.Movie{TMDBID: 438631, Title: "Дюна", Year: 2021, MediaType: movie.MediaMovie, Directors: []movie.Director{latin}, DirectorsLoaded: true})
	if err != nil {
		t.Fatal(err)
	}
	polytech, err := movies.Upsert(ctx, movie.Movie{KinopoiskID: 500, Title: "Политех", Year: 2009, MediaType: movie.MediaMovie, Directors: []movie.Director{kpOnly}, DirectorsLoaded: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := movies.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: dune, Status: movie.StatusWatched, UserRating: 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := movies.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: polytech, Status: movie.StatusWantToWatch}); err != nil {
		t.Fatal(err)
	}
	directorID := dune.Directors[0].ID

	// Кинопоиск опознал человека: русское имя и слияние KP-строки в TMDB-строку.
	birth := time.Date(1967, 10, 3, 0, 0, 0, 0, time.UTC)
	id, err := repo.UpsertPerson(ctx, movie.Person{
		TMDBID: 137427, KinopoiskID: 777, Name: "Дени Вильнёв", OriginalName: "Denis Villeneuve",
		NameSource: movie.NameSourceKinopoisk, BirthDate: &birth, Biography: "Режиссёр.", BiographyLang: "ru",
		ProfilePath: "/p.jpg", PlaceOfBirth: "Канада",
	})
	if err != nil || id != directorID {
		t.Fatalf("upsert person: %d %v (want %d)", id, err, directorID)
	}
	// Латиница из credits следующего фильма не затирает русское имя.
	if _, err := movies.Upsert(ctx, movie.Movie{TMDBID: 1, Title: "Прибытие", MediaType: movie.MediaMovie, Directors: []movie.Director{latin}, DirectorsLoaded: true}); err != nil {
		t.Fatal(err)
	}
	state, err := repo.SyncStateByTMDB(ctx, 137427)
	if err != nil || state.ID != id || state.NameSource != movie.NameSourceKinopoisk || state.SyncedAt != nil {
		t.Fatalf("sync state: %+v %v", state, err)
	}

	rating := 7.8
	episodes := int32(6)
	released := time.Date(2021, 9, 15, 0, 0, 0, 0, time.UTC)
	works := []movie.DirectorWork{
		{MediaType: movie.MediaMovie, TMDBID: 438631, Title: "Дюна", OriginalTitle: "Dune", ReleaseDate: &released, Year: 2021, Rating: &rating, VoteCount: 12000},
		{MediaType: movie.MediaMovie, TMDBID: 2, Title: "Враг", Year: 2013, VoteCount: 5},
		{MediaType: movie.MediaTV, TMDBID: 77, Title: "Сериал", Year: 2018, EpisodeCount: &episodes},
		{MediaType: movie.MediaTV, TMDBID: 77, Title: "Дубль"},
	}
	if err := repo.ReplaceWorks(ctx, id, works); err != nil {
		t.Fatal(err)
	}

	items, total, counts, err := repo.MyDirectors(ctx, movie.MyDirectorsFilter{UserID: u.ID}.Normalize())
	if err != nil || total != 1 || counts != (movie.DirectorScopeCounts{All: 1, Collection: 1}) {
		t.Fatalf("my directors: %+v %d %+v %v", items, total, counts, err)
	}
	d := items[0]
	// 3 работы из каталога + фильм только из Кинопоиска, которого в снимке нет.
	if d.Name != "Дени Вильнёв" || d.KinopoiskID != 777 || !d.FilmographyLoaded || d.WorkCount != 4 ||
		d.InCollectionCount != 2 || d.WatchedCount != 1 || d.WantToWatchCount != 1 ||
		d.AverageUserRating == nil || *d.AverageUserRating != 9 || d.Followed || len(d.KnownFor) != 1 {
		t.Fatalf("overview: %+v", d)
	}

	if err := repo.Follow(ctx, other.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := repo.Follow(ctx, other.ID, id); err != nil {
		t.Fatalf("follow must be idempotent: %v", err)
	}
	if err := repo.Follow(ctx, other.ID, 999999); !errors.Is(err, movie.ErrDirectorNotFound) {
		t.Fatalf("unknown director: %v", err)
	}
	theirs, total, counts, err := repo.MyDirectors(ctx, movie.MyDirectorsFilter{UserID: other.ID, Scope: movie.DirectorScopeFollowed}.Normalize())
	if err != nil || total != 1 || counts.Followed != 1 || theirs[0].WorkCount != 3 || theirs[0].InCollectionCount != 0 || !theirs[0].Followed {
		t.Fatalf("follower without collection: %+v %+v %v", theirs, counts, err)
	}
	if _, total, _, _ := repo.MyDirectors(ctx, movie.MyDirectorsFilter{UserID: other.ID, Query: "nolan"}.Normalize()); total != 0 {
		t.Fatalf("search must filter, total = %d", total)
	}

	p, err := repo.Profile(ctx, u.ID, id)
	if err != nil || p.Biography != "Режиссёр." || len(p.Filmography) != 4 {
		t.Fatalf("profile: %+v %v", p, err)
	}
	byTitle := map[string]movie.FilmographyItem{}
	for _, it := range p.Filmography {
		byTitle[it.Title] = it
	}
	if f := byTitle["Дюна"]; f.UserMovieID == 0 || f.Status != movie.StatusWatched || f.UserRating != 9 || f.Rating == nil || *f.Rating != 7.8 {
		t.Fatalf("dune: %+v", f)
	}
	if f := byTitle["Политех"]; f.TMDBID != 0 || f.MovieID != polytech.ID || f.Status != movie.StatusWantToWatch {
		t.Fatalf("kinopoisk-only work: %+v", f)
	}
	if f := byTitle["Сериал"]; f.MediaType != movie.MediaTV || f.EpisodeCount == nil || *f.EpisodeCount != 6 || f.UserMovieID != 0 {
		t.Fatalf("series: %+v", f)
	}
	if p.Filmography[0].Title != "Дюна" {
		t.Fatalf("filmography must start with the newest work: %+v", p.Filmography)
	}

	stale, err := repo.StaleDirectors(ctx, time.Now().Add(-time.Hour), 10)
	if err != nil || len(stale) != 0 {
		t.Fatalf("fresh director reported stale: %+v %v", stale, err)
	}
	stale, err = repo.StaleDirectors(ctx, time.Now().Add(time.Hour), 10)
	if err != nil || len(stale) != 1 || stale[0].TMDBID != 137427 {
		t.Fatalf("stale: %+v %v", stale, err)
	}

	known, err := repo.KnownByTMDB(ctx, other.ID, []int32{137427, 525})
	if err != nil || len(known) != 1 || known[137427].DirectorID != id || !known[137427].Followed {
		t.Fatalf("known: %+v %v", known, err)
	}

	if err := repo.Unfollow(ctx, other.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, total, _, _ := repo.MyDirectors(ctx, movie.MyDirectorsFilter{UserID: other.ID}.Normalize()); total != 0 {
		t.Fatalf("unfollowed director without collection must disappear, total = %d", total)
	}
	if _, err := repo.Profile(ctx, u.ID, 999999); !errors.Is(err, movie.ErrDirectorNotFound) {
		t.Fatalf("missing profile: %v", err)
	}
}

func TestMyDirectorsSortsByProgress(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	tx := newManager(t)
	movies := postgres.NewMovieRepository(tx)
	repo := postgres.NewDirectorRepository(tx)
	u := seedUser(t, 9811)
	ids := map[string]int64{}
	for i, name := range []string{"Альфа", "Бета"} {
		dir := movie.Director{TMDBID: int32(500 + i), Name: name}
		m, err := movies.Upsert(ctx, movie.Movie{TMDBID: int32(600 + i), Title: name, MediaType: movie.MediaMovie, Directors: []movie.Director{dir}, DirectorsLoaded: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := movies.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: m, Status: movie.StatusWatched}); err != nil {
			t.Fatal(err)
		}
		ids[name] = m.Directors[0].ID
	}
	// У Альфы 1 из 4 просмотрено, у Беты 1 из 1.
	if err := repo.ReplaceWorks(ctx, ids["Альфа"], []movie.DirectorWork{
		{MediaType: movie.MediaMovie, TMDBID: 600, Title: "Альфа"},
		{MediaType: movie.MediaMovie, TMDBID: 601001, Title: "x"},
		{MediaType: movie.MediaMovie, TMDBID: 601002, Title: "y"},
		{MediaType: movie.MediaMovie, TMDBID: 601003, Title: "z"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceWorks(ctx, ids["Бета"], []movie.DirectorWork{{MediaType: movie.MediaMovie, TMDBID: 601, Title: "Бета"}}); err != nil {
		t.Fatal(err)
	}
	items, _, _, err := repo.MyDirectors(ctx, movie.MyDirectorsFilter{UserID: u.ID, Sort: movie.MyDirectorsSortProgress, Desc: true}.Normalize())
	if err != nil || len(items) != 2 || items[0].Name != "Бета" {
		t.Fatalf("progress order: %+v %v", items, err)
	}
}
