package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
	"github.com/arrowwhi/kinolist/internal/transport/http/converter"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

func (h *Movie) ListDirectors(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	q := r.URL.Query()
	f := movie.DirectorFilter{UserID: userID, MediaType: movie.MediaType(q.Get("media_type")), Query: q.Get("q"), Sort: q.Get("sort"), Desc: q.Get("order") == "desc"}
	if order := q.Get("order"); order != "" && order != "asc" && order != "desc" {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	for key, target := range map[string]*int{"limit": &f.Limit, "offset": &f.Offset} {
		if raw := q.Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 || (key == "limit" && value == 0) {
				writeError(w, r, errmap.ErrBadRequest)
				return
			}
			*target = value
		}
	}
	page, err := h.uc.ListDirectors(r.Context(), f)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]dto.DirectorStats, 0, len(page.Items))
	for _, d := range page.Items {
		items = append(items, converter.DirectorStats(d))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, dto.DirectorList{Items: items, Total: page.Total, Limit: page.Limit, Offset: page.Offset})
}

func (h *Movie) GetDirector(w http.ResponseWriter, r *http.Request) {
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
	d, err := h.uc.GetDirector(r.Context(), userID, id, movie.MediaType(r.URL.Query().Get("media_type")))
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, converter.DirectorStats(d))
}

func parseDirectorIDs(raw string) ([]int64, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > movie.MaxLimit {
		return nil, errmap.ErrBadRequest
	}
	ids := make([]int64, 0, len(parts))
	seen := map[int64]bool{}
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, errmap.ErrBadRequest
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}
