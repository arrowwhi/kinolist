package converter

import (
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	directorsuc "github.com/arrowwhi/kinolist/internal/usecase/directors"
)

func date(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func DirectorOverview(d movie.DirectorOverview) dto.DirectorOverview {
	return dto.DirectorOverview{
		Director:            dto.Director{TMDBID: d.TMDBID, KinopoiskID: d.KinopoiskID, Name: d.Name, OriginalName: d.OriginalName},
		ID:                  d.ID,
		PhotoURL:            PosterURL(d.ProfilePath),
		BirthDate:           date(d.BirthDate),
		DeathDate:           date(d.DeathDate),
		PlaceOfBirth:        d.PlaceOfBirth,
		KnownFor:            nonNil(d.KnownFor),
		Followed:            d.Followed,
		FilmographyComplete: d.FilmographyLoaded,
		WorkCount:           d.WorkCount,
		InCollectionCount:   d.InCollectionCount,
		WatchedCount:        d.WatchedCount,
		WantToWatchCount:    d.WantToWatchCount,
		RatedCount:          d.RatedCount,
		AverageUserRating:   d.AverageUserRating,
	}
}

func DirectorOverviewList(p directorsuc.Page) dto.DirectorOverviewList {
	items := make([]dto.DirectorOverview, 0, len(p.Items))
	for _, d := range p.Items {
		items = append(items, DirectorOverview(d))
	}
	return dto.DirectorOverviewList{
		Items: items, Total: p.Total, Limit: p.Limit, Offset: p.Offset,
		Counts: dto.DirectorScopeCounts{All: p.Counts.All, Followed: p.Counts.Followed, Collection: p.Counts.Collection},
	}
}

func DirectorProfile(p movie.DirectorProfile, now time.Time) dto.DirectorProfile {
	items := make([]dto.FilmographyItem, 0, len(p.Filmography))
	for _, w := range p.Filmography {
		items = append(items, dto.FilmographyItem{
			TMDBID: w.TMDBID, MovieID: w.MovieID, MediaType: string(w.MediaType.Normalize()),
			Title: w.Title, OriginalTitle: w.OriginalTitle, Year: w.Year, ReleaseDate: date(w.ReleaseDate),
			PosterURL: PosterURL(w.PosterPath), Rating: w.Rating, VoteCount: w.VoteCount, EpisodeCount: w.EpisodeCount,
			Upcoming:    w.UserMovieID == 0 && w.Upcoming(now),
			UserMovieID: w.UserMovieID, Status: string(w.Status), UserRating: w.UserRating,
		})
	}
	return dto.DirectorProfile{
		DirectorOverview: DirectorOverview(p.DirectorOverview),
		Biography:        p.Biography, BiographyLang: p.BiographyLang, Stale: p.Stale, Filmography: items,
	}
}

// CatalogMovie считает upcoming по тому же правилу, что и фильмография
// режиссёра: работа из коллекции «скоро» не помечается.
func CatalogMovie(t movie.CatalogTitle, now time.Time) dto.CatalogMovie {
	return dto.CatalogMovie{
		Movie:       Movie(t.Movie),
		ReleaseDate: date(t.ReleaseDate),
		Upcoming:    t.UserMovieID == 0 && movie.IsUpcoming(t.ReleaseDate, t.Year, now),
		UserMovieID: t.UserMovieID,
		Status:      string(t.Status),
		UserRating:  t.UserRating,
	}
}

func PersonSearchList(items []movie.PersonSearchResult) dto.PersonSearchList {
	out := make([]dto.PersonSearchResult, 0, len(items))
	for _, p := range items {
		r := dto.PersonSearchResult{
			TMDBID: p.TMDBID, Name: p.Name, OriginalName: p.OriginalName, PhotoURL: PosterURL(p.ProfilePath),
			KnownForDepartment: p.KnownForDepartment, KnownFor: nonNil(p.KnownFor), Followed: p.Followed,
		}
		if p.DirectorID > 0 {
			id := p.DirectorID
			r.ID = &id
		}
		out = append(out, r)
	}
	return dto.PersonSearchList{Items: out}
}
