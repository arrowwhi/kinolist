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

func TestMovieCastIsSavedLoadedAndBackfilled(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	movies := postgres.NewMovieRepository(newManager(t))
	u := seedUser(t, 9901)

	cast := []movie.CastMember{
		{TMDBID: 1190668, Name: "Timothée Chalamet", OriginalName: "Timothée Chalamet", Character: "Paul Atreides", ProfilePath: "/t.jpg"},
		{TMDBID: 505710, Name: "Zendaya", OriginalName: "Zendaya", Character: "Chani"},
		{TMDBID: 7, Name: "Олег", OriginalName: "Oleg"},
	}
	dune, err := movies.Upsert(ctx, movie.Movie{TMDBID: 438631, Title: "Дюна", MediaType: movie.MediaMovie, Cast: cast, CastLoaded: true})
	if err != nil {
		t.Fatal(err)
	}
	if !dune.CastLoaded || len(dune.Cast) != 3 || dune.Cast[0].TMDBID != 1190668 || dune.Cast[0].ID == 0 ||
		dune.Cast[0].Character != "Paul Atreides" || dune.Cast[0].ProfilePath != "/t.jpg" || dune.Cast[2].Name != "Олег" {
		t.Fatalf("saved cast must keep credit order and get ids: %+v", dune.Cast)
	}
	// Ошибка титров при повторном сохранении не стирает известный состав.
	again, err := movies.Upsert(ctx, movie.Movie{TMDBID: 438631, Title: "Дюна", MediaType: movie.MediaMovie})
	if err != nil || len(again.Cast) != 3 || !again.CastLoaded {
		t.Fatalf("partial credits wiped cast: %+v %v", again.Cast, err)
	}
	um, err := movies.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: dune})
	if err != nil {
		t.Fatal(err)
	}
	got, err := movies.Get(ctx, u.ID, um.ID)
	if err != nil || len(got.Movie.Cast) != 3 || got.Movie.Cast[1].Character != "Chani" {
		t.Fatalf("get: %+v %v", got.Movie.Cast, err)
	}
	list, err := movies.List(ctx, movie.Filter{UserID: u.ID})
	if err != nil || len(list) != 1 || len(list[0].Movie.Cast) != 3 {
		t.Fatalf("list: %+v %v", list, err)
	}
	byTMDB, err := movies.GetByTMDBID(ctx, 438631)
	if err != nil || len(byTMDB.Cast) != 3 {
		t.Fatalf("by tmdb: %+v %v", byTMDB.Cast, err)
	}

	// Работа, добавленная до появления актёров, ждёт догрузки состава.
	old, err := movies.Upsert(ctx, movie.Movie{KinopoiskID: 500, Title: "Старое", MediaType: movie.MediaTV})
	if err != nil || old.CastLoaded || len(old.Cast) != 0 {
		t.Fatalf("old: %+v %v", old, err)
	}
	if _, err := movies.Upsert(ctx, movie.Movie{TMDBID: 1, Title: "Не в коллекции", MediaType: movie.MediaMovie}); err != nil {
		t.Fatal(err)
	}
	if _, err := movies.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: old}); err != nil {
		t.Fatal(err)
	}
	pending, err := movies.MoviesWithoutCast(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].ID != old.ID || pending[0].KinopoiskID != 500 || pending[0].MediaType != movie.MediaTV {
		t.Fatalf("without cast: %+v %v", pending, err)
	}
	if err := movies.ReplaceCast(ctx, old.ID, []movie.CastMember{{KinopoiskID: 9, Name: "Актёр", Character: "Герой"}}); err != nil {
		t.Fatal(err)
	}
	if pending, err := movies.MoviesWithoutCast(ctx, 10); err != nil || len(pending) != 0 {
		t.Fatalf("backfilled movie still pending: %+v %v", pending, err)
	}
	reloaded, err := movies.GetByTMDBID(ctx, 438631)
	if err != nil || reloaded.Cast[0].ID != dune.Cast[0].ID {
		t.Fatalf("actor identity must be stable: %+v %v", reloaded.Cast, err)
	}
}

func TestActorFollowsRolesAndCollectionThreshold(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	tx := newManager(t)
	movies := postgres.NewMovieRepository(tx)
	repo := postgres.NewActorRepository(tx)
	directors := postgres.NewDirectorRepository(tx)
	u, other := seedUser(t, 9911), seedUser(t, 9912)

	chalamet := movie.CastMember{TMDBID: 1190668, Name: "Timothée Chalamet", OriginalName: "Timothée Chalamet"}
	zendaya := movie.CastMember{TMDBID: 505710, Name: "Zendaya", OriginalName: "Zendaya"}
	kpOnly := movie.CastMember{KinopoiskID: 900, Name: "Тимоти Шаламе", OriginalName: "Timothée Chalamet", Character: "Пол"}
	save := func(m movie.Movie, status movie.Status, rating int16, add bool) movie.Movie {
		t.Helper()
		m.CastLoaded = true
		saved, err := movies.Upsert(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		if add {
			if _, err := movies.Add(ctx, movie.UserMovie{UserID: u.ID, Movie: saved, Status: status, UserRating: rating}); err != nil {
				t.Fatal(err)
			}
		}
		return saved
	}
	withRole := func(c movie.CastMember, character string) movie.CastMember {
		c.Character = character
		return c
	}
	dune := save(movie.Movie{
		TMDBID: 438631, Title: "Дюна", Year: 2021, MediaType: movie.MediaMovie,
		Cast: []movie.CastMember{withRole(chalamet, "Paul Atreides"), withRole(zendaya, "Chani")},
	}, movie.StatusWatched, 9, true)
	save(movie.Movie{
		TMDBID: 9001, Title: "Леди Бёрд", Year: 2017, MediaType: movie.MediaMovie,
		Cast: []movie.CastMember{withRole(chalamet, "Kyle")},
	}, movie.StatusWantToWatch, 0, true)
	save(movie.Movie{
		KinopoiskID: 501, Title: "Только Кинопоиск", MediaType: movie.MediaMovie,
		Cast: []movie.CastMember{kpOnly},
	}, "", 0, false)
	actorID := dune.Cast[0].ID

	// Zendaya — одна работа в коллекции: в «Мои актёры» не попадает.
	items, total, counts, err := repo.MyActors(ctx, movie.MyActorsFilter{UserID: u.ID}.Normalize())
	if err != nil || total != 1 || counts != (movie.ActorScopeCounts{All: 1, Collection: 1}) || items[0].ID != actorID {
		t.Fatalf("collection threshold: %+v %d %+v %v", items, total, counts, err)
	}
	// Снимок ещё не загружен: считаются только работы коллекции.
	if items[0].FilmographyLoaded || items[0].WorkCount != 2 || items[0].InCollectionCount != 2 {
		t.Fatalf("before sync: %+v", items[0])
	}
	stale, err := repo.StaleActors(ctx, time.Now().Add(time.Hour), 10)
	if err != nil || len(stale) != 1 || stale[0].ID != actorID {
		t.Fatalf("only actors from >= 2 collection works are synced: %+v %v", stale, err)
	}

	// Кинопоиск опознал человека: русское имя и слияние KP-строки в TMDB-строку.
	birth := time.Date(1995, 12, 27, 0, 0, 0, 0, time.UTC)
	id, err := repo.UpsertPerson(ctx, movie.Person{
		TMDBID: 1190668, KinopoiskID: 900, Name: "Тимоти Шаламе", OriginalName: "Timothée Chalamet",
		NameSource: movie.NameSourceKinopoisk, BirthDate: &birth, Biography: "Актёр.", BiographyLang: "ru",
	})
	if err != nil || id != actorID {
		t.Fatalf("upsert person: %d %v (want %d)", id, err, actorID)
	}
	kpMovie, err := movies.GetByTMDBID(ctx, 438631)
	if err != nil || kpMovie.Cast[0].KinopoiskID != 900 || kpMovie.Cast[0].Name != "Тимоти Шаламе" {
		t.Fatalf("merged actor in cast: %+v %v", kpMovie.Cast, err)
	}
	rating := 7.8
	episodes := int32(8)
	works := []movie.ActorWork{
		{MediaType: movie.MediaMovie, TMDBID: 438631, Title: "Дюна", Year: 2021, Rating: &rating, VoteCount: 12000, Character: "Paul Atreides"},
		{MediaType: movie.MediaMovie, TMDBID: 2, Title: "Зови меня своим именем", Year: 2017, VoteCount: 9000, Character: "Elio"},
		{MediaType: movie.MediaTV, TMDBID: 77, Title: "Родина", Year: 2012, VoteCount: 10, EpisodeCount: &episodes, Character: "Finn Walden"},
	}
	if err := repo.ReplaceWorks(ctx, id, works); err != nil {
		t.Fatal(err)
	}

	items, _, _, err = repo.MyActors(ctx, movie.MyActorsFilter{UserID: u.ID, Query: "шаламе"}.Normalize())
	if err != nil || len(items) != 1 {
		t.Fatalf("search by russian name: %+v %v", items, err)
	}
	a := items[0]
	// 3 роли из каталога + «Леди Бёрд» из коллекции, которой нет в снимке.
	if a.Name != "Тимоти Шаламе" || a.KinopoiskID != 900 || !a.FilmographyLoaded || a.WorkCount != 4 ||
		a.InCollectionCount != 2 || a.WatchedCount != 1 || a.WantToWatchCount != 1 ||
		a.AverageUserRating == nil || *a.AverageUserRating != 9 ||
		len(a.KnownFor) != 3 || a.KnownFor[0] != "Дюна" || a.KnownFor[1] != "Зови меня своим именем" {
		t.Fatalf("overview: %+v", a)
	}

	p, err := repo.Profile(ctx, u.ID, id)
	if err != nil || p.Biography != "Актёр." || len(p.Filmography) != 4 {
		t.Fatalf("profile: %+v %v", p, err)
	}
	byTitle := map[string]movie.FilmographyItem{}
	for _, it := range p.Filmography {
		byTitle[it.Title] = it
	}
	if f := byTitle["Дюна"]; f.Character != "Paul Atreides" || f.Status != movie.StatusWatched || f.UserRating != 9 {
		t.Fatalf("dune: %+v", f)
	}
	if f := byTitle["Леди Бёрд"]; f.Character != "Kyle" || f.Status != movie.StatusWantToWatch {
		t.Fatalf("collection-only role keeps character from the cast: %+v", f)
	}
	if f := byTitle["Родина"]; f.EpisodeCount == nil || *f.EpisodeCount != 8 || f.Character != "Finn Walden" {
		t.Fatalf("series: %+v", f)
	}

	if err := repo.Follow(ctx, other.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := repo.Follow(ctx, other.ID, 999999); !errors.Is(err, movie.ErrActorNotFound) {
		t.Fatalf("unknown actor: %v", err)
	}
	theirs, total, counts, err := repo.MyActors(ctx, movie.MyActorsFilter{UserID: other.ID, Scope: movie.DirectorScopeFollowed}.Normalize())
	if err != nil || total != 1 || counts.Followed != 1 || theirs[0].WorkCount != 3 || !theirs[0].Followed {
		t.Fatalf("follower without collection: %+v %+v %v", theirs, counts, err)
	}
	known, err := repo.KnownByTMDB(ctx, other.ID, []int32{1190668, 505710, 525})
	if err != nil || len(known) != 2 || known[1190668].PersonID != id || !known[1190668].Followed || known[505710].Followed {
		t.Fatalf("known: %+v %v", known, err)
	}
	if err := repo.Unfollow(ctx, other.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, total, _, _ := repo.MyActors(ctx, movie.MyActorsFilter{UserID: other.ID}.Normalize()); total != 0 {
		t.Fatalf("unfollowed actor without collection must disappear, total = %d", total)
	}

	// Справочники разделов независимы: актёр не становится режиссёром.
	if _, err := directors.SyncStateByTMDB(ctx, 1190668); !errors.Is(err, movie.ErrDirectorNotFound) {
		t.Fatalf("actor leaked into directors: %v", err)
	}
	if _, err := repo.Profile(ctx, u.ID, 999999); !errors.Is(err, movie.ErrActorNotFound) {
		t.Fatalf("missing profile: %v", err)
	}
	state, err := repo.SyncState(ctx, id)
	if err != nil || state.SyncedAt == nil || state.NameSource != movie.NameSourceKinopoisk {
		t.Fatalf("sync state: %+v %v", state, err)
	}
	if stale, err := repo.StaleActors(ctx, time.Now().Add(-time.Hour), 10); err != nil || len(stale) != 0 {
		t.Fatalf("fresh actor reported stale: %+v %v", stale, err)
	}
}
