package handler

import (
	"context"
	"net/http"

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
	Confirm(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32) (movie.UserMovie, error)
	Reject(ctx context.Context, userID, jobID int64) error
}

type Enrichment struct {
	uc enrichmentUseCase
}

func NewEnrichment(uc enrichmentUseCase) *Enrichment {
	return &Enrichment{uc: uc}
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
		out.Error = errmap.Map(errFromJob(job)).Message
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

	um, err := h.uc.Confirm(r.Context(), userID, id, movie.Provider(req.Provider), req.ExternalID)
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
