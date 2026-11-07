package handler

import (
	"net/http"
	"sync/atomic"

	"github.com/arrowwhi/infrastructure/observability/health"
)

const statusKey = "status"

// Health разделяет liveness и readiness: процесс может быть жив, но не
// готов принимать трафик — например, во время остановки.
//
// Проверки зависимостей живут в общем реестре инфраструктуры: тот же реестр
// отдаёт /ready на служебном порту и публикует метрики health.check.*.
type Health struct {
	checks  *health.Registry
	ready   atomic.Bool
	version string
}

func NewHealth(checks *health.Registry, version string) *Health {
	h := &Health{checks: checks, version: version}
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

	if h.checks == nil {
		writeJSON(w, http.StatusOK, map[string]string{statusKey: "ok"})
		return
	}

	report := h.checks.Check(r.Context())
	if !report.Ready() {
		// Наружу — только имена упавших зависимостей, без текстов ошибок.
		failed := make([]string, 0, len(report.Checks))
		for name, res := range report.Checks {
			if res.Status == health.StatusFail && res.Critical {
				failed = append(failed, name)
			}
		}

		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			statusKey: "unavailable", "dependencies": failed,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{statusKey: string(report.Status)})
}
