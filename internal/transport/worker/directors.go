package worker

import (
	"context"
	"time"

	"github.com/arrowwhi/kinolist/internal/pkg/logger"
)

type directorSyncUseCase interface {
	SyncStale(ctx context.Context, limit int) (int, error)
}

// DirectorSync периодически обновляет фильмографии режиссёров, которые
// кому-то нужны: так у «режиссёров из коллекции» появляется полный список
// работ без ожидания на экране профиля.
type DirectorSync struct {
	uc       directorSyncUseCase
	interval time.Duration
	batch    int
}

func NewDirectorSync(uc directorSyncUseCase, interval time.Duration, batch int) *DirectorSync {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	if batch <= 0 {
		batch = 20
	}
	return &DirectorSync{uc: uc, interval: interval, batch: batch}
}

// Run блокируется до отмены контекста. Первый проход — сразу после старта,
// чтобы после деплоя не ждать полный интервал.
func (s *DirectorSync) Run(ctx context.Context) error {
	logger.Info(ctx).Dur("interval", s.interval).Int("batch", s.batch).Msg("director sync started")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.tick(ctx)
		select {
		case <-ctx.Done():
			logger.Info(ctx).Msg("director sync stopped")
			return nil
		case <-ticker.C:
		}
	}
}

func (s *DirectorSync) tick(ctx context.Context) {
	synced, err := s.uc.SyncStale(ctx, s.batch)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		logger.Warn(ctx).Int("synced", synced).Err(err).Msg("director sync finished with errors")
		return
	}
	if synced > 0 {
		logger.Info(ctx).Int("synced", synced).Msg("director filmographies synced")
	}
}
