package handler

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

type pinger interface {
	Ping(ctx context.Context) error
}

const statusKey = "status"

// Health разделяет liveness и readiness: процесс может быть жив, но не
// готов принимать трафик — например, во время остановки.
type Health struct {
	db      pinger
	ready   atomic.Bool
	version string
}

func NewHealth(db pinger, version string) *Health {
	h := &Health{db: db, version: version}
	h.ready.Store(true)
	return h
}

// SetReady используется shutdown'ом: сначала /readyz начинает отдавать
// ошибку, и только потом сервис перестаёт принимать запросы.
func (h *Health) SetReady(ready bool) { h.ready.Store(ready) }

func (h *Health) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{statusKey: "ok", "version": h.version})
}

func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	if !h.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			statusKey: "shutting_down",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			statusKey: "unavailable", "dependency": "postgres",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{statusKey: "ok"})
}
