package worker

import (
	"context"
	"time"

	"github.com/arrowwhi/kinolist/internal/pkg/logger"
)

type personSyncUseCase interface {
	SyncStale(ctx context.Context, limit int) (int, error)
}

// PersonSync периодически обновляет снимки фильмографий людей, которые
// кому-то нужны: так у «режиссёров/актёров из коллекции» появляется полный
// список работ без ожидания на экране профиля. Для актёров тот же проход
// догружает состав работ, добавленных до появления актёров.
type PersonSync struct {
	uc       personSyncUseCase
	interval time.Duration
	batch    int
	// section — поле лога: director или actor.
	section string
}

func NewDirectorSync(uc personSyncUseCase, interval time.Duration, batch int) *PersonSync {
	return newPersonSync(uc, interval, batch, "director")
}

func NewActorSync(uc personSyncUseCase, interval time.Duration, batch int) *PersonSync {
	return newPersonSync(uc, interval, batch, "actor")
}

func newPersonSync(uc personSyncUseCase, interval time.Duration, batch int, section string) *PersonSync {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	if batch <= 0 {
		batch = 20
	}
	return &PersonSync{uc: uc, interval: interval, batch: batch, section: section}
}

// Run блокируется до отмены контекста. Первый проход — сразу после старта,
// чтобы после деплоя не ждать полный интервал.
func (s *PersonSync) Run(ctx context.Context) error {
	ctx = logger.With(ctx, "section", s.section)
	logger.Info(ctx).Dur("interval", s.interval).Int("batch", s.batch).Msg("person sync started")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.tick(ctx)
		select {
		case <-ctx.Done():
			logger.Info(ctx).Msg("person sync stopped")
			return nil
		case <-ticker.C:
		}
	}
}

func (s *PersonSync) tick(ctx context.Context) {
	synced, err := s.uc.SyncStale(ctx, s.batch)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		logger.Warn(ctx).Int("synced", synced).Err(err).Msg("person sync finished with errors")
		return
	}
	if synced > 0 {
		logger.Info(ctx).Int("synced", synced).Msg("person filmographies synced")
	}
}
