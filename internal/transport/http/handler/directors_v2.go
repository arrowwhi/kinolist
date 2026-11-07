package handler

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
	"github.com/arrowwhi/kinolist/internal/transport/http/converter"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
	directorsuc "github.com/arrowwhi/kinolist/internal/usecase/directors"
)

type directorsUseCase interface {
	MyDirectors(ctx context.Context, f movie.MyDirectorsFilter) (directorsuc.Page, error)
	Profile(ctx context.Context, userID, directorID int64) (movie.DirectorProfile, error)
	Search(ctx context.Context, userID int64, query string) ([]movie.PersonSearchResult, error)
	Import(ctx context.Context, userID int64, tmdbID int32) (movie.DirectorOverview, error)
	FollowByTMDB(ctx context.Context, userID int64, tmdbID int32) (movie.DirectorOverview, error)
	Follow(ctx context.Context, userID, directorID int64) error
	Unfollow(ctx context.Context, userID, directorID int64) error
}

// Directors — раздел «Мои режиссёры»: подписки и фильмографии из каталога.
type Directors struct {
	uc  directorsUseCase
	now func() time.Time
}

func NewDirectors(uc directorsUseCase) *Directors {
	return &Directors{uc: uc, now: time.Now}
}

const (
	orderAsc  = "asc"
	orderDesc = "desc"
)

// parsePaging читает limit/offset: limit положительный, offset неотрицательный.
func parsePaging(q url.Values, limit, offset *int) error {
	for key, target := range map[string]*int{"limit": limit, "offset": offset} {
		if raw := q.Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 || (target == limit && value == 0) {
				return errmap.ErrBadRequest
			}
			*target = value
		}
	}
	return nil
}

// defaultDesc — направление по умолчанию: прогресс и оценки интереснее
// смотреть от больших к меньшим, имена — по алфавиту.
func defaultDesc(sort string) bool {
	return sort == movie.MyDirectorsSortProgress || sort == movie.MyDirectorsSortUserRating
}

func (h *Directors) Mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	q := r.URL.Query()
	f := movie.MyDirectorsFilter{
		UserID: userID, Scope: movie.DirectorScope(q.Get("scope")), Query: q.Get("query"), Sort: q.Get("sort"),
	}
	switch order := q.Get("order"); order {
	case "":
		f.Desc = defaultDesc(f.Sort)
	case orderAsc, orderDesc:
		f.Desc = order == orderDesc
	default:
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	if err := parsePaging(q, &f.Limit, &f.Offset); err != nil {
		writeError(w, r, err)
		return
	}
	page, err := h.uc.MyDirectors(r.Context(), f)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, converter.DirectorOverviewList(page))
}

func (h *Directors) Profile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, movie.ErrDirectorNotFound)
		return
	}
	p, err := h.uc.Profile(r.Context(), userID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, converter.DirectorProfile(p, h.now()))
}

func (h *Directors) Search(w http.ResponseWriter, r *http.Request) {
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

func (h *Directors) Import(w http.ResponseWriter, r *http.Request) {
	h.byTMDB(w, r, h.uc.Import)
}

func (h *Directors) FollowByTMDB(w http.ResponseWriter, r *http.Request) {
	h.byTMDB(w, r, h.uc.FollowByTMDB)
}

func (h *Directors) byTMDB(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int32) (movie.DirectorOverview, error)) {
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
	d, err := action(r.Context(), userID, req.TMDBID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, converter.DirectorOverview(d))
}

func (h *Directors) Follow(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.uc.Follow)
}

func (h *Directors) Unfollow(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.uc.Unfollow)
}

func (h *Directors) toggle(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int64) error) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, movie.ErrDirectorNotFound)
		return
	}
	if err := action(r.Context(), userID, id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
