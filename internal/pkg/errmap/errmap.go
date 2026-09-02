// Package errmap — единственное место, где доменные ошибки превращаются
// в коды и тексты для клиента. Детали инфраструктуры наружу не уходят:
// текст ошибки Postgres пользователь не видит никогда.
package errmap

import (
	"context"
	"errors"
	"net/http"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
)

// ErrBadRequest — транспортная ошибка разбора запроса. Живёт здесь, а не в
// пакете transport, чтобы таблица соответствий осталась единственной.
var ErrBadRequest = errors.New("malformed request")

// Response — то, что уходит клиенту вместо ошибки.
type Response struct {
	Status  int
	Code    string
	Message string
}

// entry связывает доменную ошибку с ответом. Одна таблица, одно место правки.
type entry struct {
	err  error
	resp Response
}

var table = []entry{
	{ErrBadRequest, Response{http.StatusBadRequest, "bad_request", "Некорректный запрос"}},

	{movie.ErrNotFound, Response{http.StatusNotFound, "movie_not_found", "Фильм не найден"}},
	{user.ErrNotFound, Response{http.StatusNotFound, "user_not_found", "Пользователь не найден"}},
	{enrichment.ErrJobNotFound, Response{http.StatusNotFound, "job_not_found", "Задача не найдена"}},

	{movie.ErrAlreadyInCollection, Response{http.StatusConflict, "already_in_collection", "Фильм уже в коллекции"}},
	{enrichment.ErrJobNotConfirmable, Response{http.StatusConflict, "job_not_confirmable", "Эта задача уже завершена"}},

	{movie.ErrInvalidStatus, Response{http.StatusBadRequest, "invalid_status", "Недопустимый статус фильма"}},
	{movie.ErrInvalidRating, Response{http.StatusBadRequest, "invalid_rating", "Оценка должна быть от 1 до 10"}},
	{movie.ErrEmptyQuery, Response{http.StatusBadRequest, "empty_query", "Пустой поисковый запрос"}},
	{movie.ErrNoCandidates, Response{http.StatusNotFound, "no_candidates", "Ничего не нашлось по этому названию"}},
	{enrichment.ErrEmptyInput, Response{http.StatusBadRequest, "empty_input", "Пустой запрос"}},
	{enrichment.ErrInvalidReelURL, Response{http.StatusBadRequest, "invalid_reel_url", "Это не ссылка на рилс Instagram"}},
	{enrichment.ErrCandidateNotInJob, Response{http.StatusBadRequest, "candidate_not_in_job", "Этот вариант не относится к задаче"}},
	{enrichment.ErrTitleNotRecognized, Response{http.StatusUnprocessableEntity, "title_not_recognized", "Не удалось распознать название фильма"}},
	{user.ErrInvalidTelegram, Response{http.StatusUnauthorized, "invalid_telegram", "Недействительные данные Telegram"}},

	{enrichment.ErrFetcherDisabled, Response{http.StatusServiceUnavailable, "fetcher_disabled", "Разбор ссылок сейчас недоступен, пришлите название текстом"}},
	{enrichment.ErrExtractorDisabled, Response{http.StatusServiceUnavailable, "extractor_disabled", "Распознавание названий сейчас недоступно, пришлите название текстом"}},
	{enrichment.ErrReelUnavailable, Response{http.StatusBadGateway, "reel_unavailable", "Не удалось получить содержимое рилса"}},
	{movie.ErrProviderUnavailable, Response{http.StatusBadGateway, "catalog_unavailable", "Кинобаза временно недоступна"}},
}

var (
	internal = Response{http.StatusInternalServerError, "internal", "Что-то пошло не так"}
	canceled = Response{499, "canceled", "Запрос отменён"}
	timedOut = Response{http.StatusGatewayTimeout, "timeout", "Превышено время ожидания"}
)

// Map переводит ошибку в ответ клиенту. Неизвестная ошибка всегда становится
// 500 с обобщённым текстом — так внутренние детали не утекают.
func Map(err error) Response {
	if err == nil {
		return Response{Status: http.StatusOK}
	}

	for _, e := range table {
		if errors.Is(err, e.err) {
			return e.resp
		}
	}

	switch {
	case errors.Is(err, context.Canceled):
		return canceled
	case errors.Is(err, context.DeadlineExceeded):
		return timedOut
	default:
		return internal
	}
}

// IsInternal сообщает логгеру, что ошибку нужно писать уровнем error:
// ожидаемые доменные исходы шумят в логах зря.
func IsInternal(err error) bool {
	return Map(err).Status >= http.StatusInternalServerError
}
