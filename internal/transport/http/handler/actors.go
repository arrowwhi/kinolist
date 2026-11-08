package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
	"github.com/arrowwhi/kinolist/internal/transport/http/converter"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	actorsuc "github.com/arrowwhi/kinolist/internal/usecase/actors"
)

type actorsUseCase interface {
	MyActors(ctx context.Context, f movie.MyActorsFilter) (actorsuc.Page, error)
	Profile(ctx context.Context, userID, actorID int64) (movie.ActorProfile, error)
	Search(ctx context.Context, userID int64, query string) ([]movie.PersonSearchResult, error)
	Import(ctx context.Context, userID int64, tmdbID int32) (movie.ActorOverview, error)
	FollowByTMDB(ctx context.Context, userID int64, tmdbID int32) (movie.ActorOverview, error)
	Follow(ctx context.Context, userID, actorID int64) error
	Unfollow(ctx context.Context, userID, actorID int64) error
}

// Actors — раздел «Актёры»: подписки и роли из каталога.
type Actors struct {
	uc  actorsUseCase
	now func() time.Time
}

func NewActors(uc actorsUseCase) *Actors {
	return &Actors{uc: uc, now: time.Now}
}

const (
	roleDirector = "director"
	roleActor    = "actor"
)

// PeopleSearch выбирает раздел поиска людей по параметру role: director
// (по умолчанию, как было до актёров) или actor.
func PeopleSearch(directors *Directors, actors *Actors) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("role") {
		case "", roleDirector:
			directors.Search(w, r)
		case roleActor:
			actors.Search(w, r)
		default:
			writeError(w, r, errmap.ErrBadRequest)
		}
	}
}

func (h *Actors) Mine(w http.ResponseWriter, r *http.Request) {
	f, err := parseMyPeople(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	page, err := h.uc.MyActors(r.Context(), f)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, converter.ActorOverviewList(page))
}

func (h *Actors) Profile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, movie.ErrActorNotFound)
		return
	}
	p, err := h.uc.Profile(r.Context(), userID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, converter.ActorProfile(p, h.now()))
}

func (h *Actors) Search(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	items, err := h.uc.Search(r.Context(), userID, strings.TrimSpace(r.URL.Query().Get("query")))
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, converter.PersonSearchList(items))
}

func (h *Actors) Import(w http.ResponseWriter, r *http.Request) {
	h.byTMDB(w, r, h.uc.Import)
}

func (h *Actors) FollowByTMDB(w http.ResponseWriter, r *http.Request) {
	h.byTMDB(w, r, h.uc.FollowByTMDB)
}

func (h *Actors) byTMDB(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int32) (movie.ActorOverview, error)) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	var req dto.TMDBPersonRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	if req.TMDBID <= 0 {
		writeError(w, r, movie.ErrInvalidPersonID)
		return
	}
	a, err := action(r.Context(), userID, req.TMDBID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, converter.ActorOverview(a))
}

func (h *Actors) Follow(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.uc.Follow)
}

func (h *Actors) Unfollow(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.uc.Unfollow)
}

func (h *Actors) toggle(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int64) error) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, movie.ErrActorNotFound)
		return
	}
	if err := action(r.Context(), userID, id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
