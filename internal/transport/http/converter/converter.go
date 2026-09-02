// Package converter переводит доменные модели в DTO и обратно.
// Это единственное место, где домен встречается с представлением.
package converter

import (
	"strings"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/usecase/collection"
)

// tmdbImageBase — база для постеров TMDB. Домен хранит только путь,
// чтобы смена размера картинки не требовала переписывать базу.
const tmdbImageBase = "https://image.tmdb.org/t/p/w500"

// PosterURL достраивает путь TMDB до полного адреса; Кинопоиск отдаёт
// абсолютную ссылку и остаётся как есть.
func PosterURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return tmdbImageBase + path
}

func Genre(g movie.Genre) dto.Genre {
	return dto.Genre{ID: g.ID, NameRU: g.NameRU, NameEN: g.NameEN}
}

func Genres(genres []movie.Genre) []dto.Genre {
	out := make([]dto.Genre, 0, len(genres))
	for _, g := range genres {
		out = append(out, Genre(g))
	}
	return out
}

func Movie(m movie.Movie) dto.Movie {
	return dto.Movie{
		ID: m.ID, TMDBID: m.TMDBID, KinopoiskID: m.KinopoiskID,
		Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year,
		Overview: m.Overview, PosterURL: PosterURL(m.PosterPath),
		RuntimeMin: m.RuntimeMin, Rating: m.Rating, Genres: Genres(m.Genres),
	}
}

func UserMovie(um movie.UserMovie) dto.UserMovie {
	out := dto.UserMovie{
		ID: um.ID, Movie: Movie(um.Movie), Status: string(um.Status),
		UserRating: um.UserRating, Note: um.Note,
		Source: string(um.Source), SourceURL: um.SourceURL,
		AddedAt: um.AddedAt.UTC().Format(time.RFC3339),
	}
	if um.WatchedAt != nil {
		out.WatchedAt = um.WatchedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func MovieList(p collection.Page) dto.MovieList {
	items := make([]dto.UserMovie, 0, len(p.Items))
	for _, um := range p.Items {
		items = append(items, UserMovie(um))
	}
	return dto.MovieList{Items: items, Total: p.Total, Limit: p.Limit, Offset: p.Offset}
}

func Candidate(c movie.Candidate) dto.Candidate {
	return dto.Candidate{
		Provider: string(c.Provider), ExternalID: c.ExternalID,
		Title: c.Title, Year: c.Year, Overview: c.Overview,
		PosterURL: PosterURL(c.PosterPath), Rating: c.Rating,
	}
}

func Job(j enrichment.Job) dto.Job {
	candidates := make([]dto.Candidate, 0, len(j.Candidates))
	for _, c := range j.Candidates {
		candidates = append(candidates, Candidate(c))
	}

	return dto.Job{
		ID: j.ID, Kind: string(j.Kind), Status: string(j.Status),
		QueryTitle: j.QueryTitle, QueryYear: j.QueryYear,
		SourceURL: j.SourceURL, Candidates: candidates,
	}
}

func Me(u user.User) dto.Me {
	return dto.Me{ID: u.ID, Username: u.Username, FirstName: u.FirstName}
}

// Update переводит частичное обновление из DTO в доменную форму.
func Update(req dto.UpdateMovieRequest) movie.Update {
	upd := movie.Update{UserRating: req.UserRating, Note: req.Note}
	if req.Status != nil {
		s := movie.Status(*req.Status)
		upd.Status = &s
	}
	return upd
}
