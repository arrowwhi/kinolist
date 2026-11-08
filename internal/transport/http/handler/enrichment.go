package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
	"github.com/arrowwhi/kinolist/internal/transport/http/converter"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

type enrichmentUseCase interface {
	Enqueue(ctx context.Context, userID, chatID int64, input string) (enrichment.Job, error)
	JobForUser(ctx context.Context, userID, jobID int64) (enrichment.Job, error)
	Reject(ctx context.Context, userID, jobID int64) error
	ConfirmWithOptions(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32, options movie.ConfirmOptions) (movie.UserMovie, error)
	CandidateDetails(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32, mediaTypes ...movie.MediaType) (movie.Movie, error)
	AddFromCatalog(ctx context.Context, userID int64, provider movie.Provider, externalID int32, options movie.ConfirmOptions) (movie.UserMovie, error)
	CatalogTitle(ctx context.Context, userID int64, provider movie.Provider, externalID int32, mediaType movie.MediaType) (movie.CatalogTitle, error)
}

type Enrichment struct {
	uc  enrichmentUseCase
	now func() time.Time
}

func NewEnrichment(uc enrichmentUseCase) *Enrichment {
	return &Enrichment{uc: uc, now: time.Now}
}

// Create принимает ссылку или название и отвечает 202: поиск идёт в фоне,
// клиент опрашивает задачу по её id.
func (h *Enrichment) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	var req dto.CreateJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}

	// chatID = 0: задача создана из Mini App, уведомлять в чат не нужно.
	job, err := h.uc.Enqueue(r.Context(), userID, 0, req.Input)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusAccepted, converter.Job(job))
}

func (h *Enrichment) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	id, err := pathID(r)
	if err != nil {
		writeError(w, r, enrichment.ErrJobNotFound)
		return
	}

	job, err := h.uc.JobForUser(r.Context(), userID, id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	out := converter.Job(job)
	if job.Status == enrichment.StatusFailed {
		resp := errmap.Map(errFromJob(job))
		out.Error = resp.Message
		out.ErrorCode = resp.Code
	}

	writeJSON(w, http.StatusOK, out)
}

func (h *Enrichment) Confirm(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	id, err := pathID(r)
	if err != nil {
		writeError(w, r, enrichment.ErrJobNotFound)
		return
	}

	var req dto.ConfirmJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}

	um, err := h.uc.ConfirmWithOptions(r.Context(), userID, id, movie.Provider(req.Provider), req.ExternalID, movie.ConfirmOptions{MediaType: movie.MediaType(req.MediaType), Status: movie.Status(req.Status), UserRating: req.UserRating, Review: req.Review, Note: req.Note})
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, converter.UserMovie(um))
}

// AddFromCatalog добавляет работу по внешнему id, без поиска и задачи:
// так работают «＋» в фильмографии режиссёра и кнопки карточки каталога.
func (h *Enrichment) AddFromCatalog(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	var req dto.AddMovieRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	provider := movie.Provider(req.Provider)
	if (provider != movie.ProviderTMDB && provider != movie.ProviderKinopoisk) || req.ExternalID <= 0 {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	mediaType := movie.MediaType(req.MediaType)
	if !mediaType.Valid() {
		writeError(w, r, movie.ErrInvalidMediaType)
		return
	}
	um, err := h.uc.AddFromCatalog(r.Context(), userID, provider, req.ExternalID, movie.ConfirmOptions{
		MediaType: mediaType, Status: movie.Status(req.Status), UserRating: req.UserRating, Review: req.Review,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, converter.UserMovie(um))
}

func (h *Enrichment) Reject(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}

	id, err := pathID(r)
	if err != nil {
		writeError(w, r, enrichment.ErrJobNotFound)
		return
	}

	if err := h.uc.Reject(r.Context(), userID, id); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// errFromJob восстанавливает доменную причину отказа по сохранённому тексту,
// чтобы клиент увидел ту же формулировку, что и в боте. Сырой last_error
// наружу не отдаётся: в нём бывают адреса и ответы внешних сервисов.
func errFromJob(j enrichment.Job) error {
	for _, known := range []error{
		enrichment.ErrTitleNotRecognized,
		enrichment.ErrReelUnavailable,
		enrichment.ErrInvalidReelURL,
		enrichment.ErrFetcherDisabled,
		enrichment.ErrExtractorDisabled,
		movie.ErrNoCandidates,
		movie.ErrProviderUnavailable,
	} {
		if containsCause(j.LastError, known.Error()) {
			return known
		}
	}
	return errUnknownJobFailure
}

func (h *Enrichment) CandidateDetails(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	jobID, err := pathID(r)
	if err != nil {
		writeError(w, r, enrichment.ErrJobNotFound)
		return
	}
	externalID, err := strconv.ParseInt(chi.URLParam(r, "external_id"), 10, 32)
	if err != nil || externalID <= 0 {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	mt := movie.MediaType(chi.URLParam(r, "media_type"))
	if mt == "" {
		mt = movie.MediaType(r.URL.Query().Get("media_type"))
	}
	if mt != "" && !mt.Valid() {
		writeError(w, r, movie.ErrInvalidMediaType)
		return
	}
	m, err := h.uc.CandidateDetails(r.Context(), userID, jobID, movie.Provider(chi.URLParam(r, "provider")), int32(externalID), mt)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, converter.Movie(m))
}

// CatalogTitle отдаёт карточку работы из каталога по внешнему id —
// для работ фильмографии, которых ещё нет в коллекции.
func (h *Enrichment) CatalogTitle(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		writeError(w, r, user.ErrInvalidTelegram)
		return
	}
	provider := movie.Provider(chi.URLParam(r, "provider"))
	if provider != movie.ProviderTMDB && provider != movie.ProviderKinopoisk {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	mediaType := movie.MediaType(chi.URLParam(r, "media_type"))
	if !mediaType.Valid() {
		writeError(w, r, movie.ErrInvalidMediaType)
		return
	}
	externalID, err := strconv.ParseInt(chi.URLParam(r, "external_id"), 10, 32)
	if err != nil || externalID <= 0 {
		writeError(w, r, errmap.ErrBadRequest)
		return
	}
	t, err := h.uc.CatalogTitle(r.Context(), userID, provider, int32(externalID), mediaType)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, converter.CatalogMovie(t, h.now()))
}
