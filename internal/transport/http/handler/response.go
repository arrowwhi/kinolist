package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
	"github.com/arrowwhi/kinolist/internal/pkg/logger"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Заголовки уже ушли — остаётся только зафиксировать факт.
		logger.WarnDetached().Err(err).Msg("encode response failed")
	}
}

// writeError — единственный путь ошибки к клиенту: код и текст берутся
// из таблицы errmap, оригинальное сообщение остаётся в логе.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	resp := errmap.Map(err)

	if errmap.IsInternal(err) {
		logger.Error(r.Context()).Err(err).Msg("request error")
	} else {
		logger.Debug(r.Context()).Err(err).Msg("request rejected")
	}

	body := dto.Error{Code: resp.Code, Message: resp.Message}
	var duplicate *movie.AlreadyInCollectionError
	if errors.As(err, &duplicate) {
		body.ExistingUserMovieID = duplicate.UserMovieID
	}
	writeJSON(w, resp.Status, body)
}

func decodeJSON(r *http.Request, target any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

var errUnknownJobFailure = errors.New("enrichment job failed")

// containsCause ищет текст доменной ошибки внутри сохранённой строки:
// last_error пишется через %w-обёртки, поэтому сравнение по вхождению —
// единственный способ восстановить причину из БД.
func containsCause(stored, cause string) bool {
	return stored != "" && strings.Contains(stored, cause)
}
