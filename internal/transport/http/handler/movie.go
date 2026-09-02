package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
	"github.com/arrowwhi/kinolist/internal/transport/http/converter"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	"github.com/arrowwhi/kinolist/internal/usecase/collection"
)

type collectionUseCase interface {
	List(ctx context.Context, f movie.Filter) (collection.Page, error)
	Get(ctx context.Context, userID, userMovieID int64) (movie.UserMovie, error)
	Update(ctx context.Context, userID, userMovieID int64, upd movie.Update) (movie.UserMovie, error)
	Delete(ctx context.Context, userID, userMovieID int64) error
	Genres(ctx context.Context) ([]movie.Genre, error)
	ResolveUser(ctx context.Context, u user.User) (user.User, error)
}

type Movie struct {
	uc collectionUseCase
}

func NewMovie(uc collectionUseCase) *Movie {
	return &Movie{uc: uc}
}

func (h *Movie) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	filter, err := parseFilter(r, userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	page, err := h.uc.List(r.Context(), filter)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, converter.MovieList(page))
}

func (h *Movie) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	um, err := h.uc.Get(r.Context(), userID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, converter.UserMovie(um))
}

func (h *Movie) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req dto.UpdateMovieRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}

	um, err := h.uc.Update(r.Context(), userID, id, converter.Update(req))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, converter.UserMovie(um))
}

func (h *Movie) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := h.uc.Delete(r.Context(), userID, id); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Movie) Genres(w http.ResponseWriter, r *http.Request) {
	genres, err := h.uc.Genres(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.GenreList{Items: converter.Genres(genres)})
}

func (h *Movie) Me(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.IdentityFrom(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	userID, _ := middleware.UserID(r.Context())

	writeJSON(w, http.StatusOK, dto.Me{
		ID: userID, Username: identity.Username, FirstName: identity.FirstName,
	})
}

func parseFilter(r *http.Request, userID int64) (movie.Filter, error) {
	q := r.URL.Query()

	f := movie.Filter{
		UserID: userID,
		Status: movie.Status(q.Get("status")),
		Query:  q.Get("q"),
		SortBy: movie.SortBy(q.Get("sort")),
		Desc:   q.Get("order") != "asc",
	}

	if raw := q.Get("genre_ids"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 16)
			if err != nil {
				return movie.Filter{}, errmap.ErrBadRequest
			}
			f.GenreIDs = append(f.GenreIDs, int16(id))
		}
	}
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return movie.Filter{}, errmap.ErrBadRequest
		}
		f.Limit = v
	}
	if raw := q.Get("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return movie.Filter{}, errmap.ErrBadRequest
		}
		f.Offset = v
	}

	return f, nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, movie.ErrNotFound
	}
	return id, nil
}
