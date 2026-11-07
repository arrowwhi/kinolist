package worker

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type directorSyncUseCase interface {
	SyncStale(ctx context.Context, limit int) (int, error)
}

// DirectorSync периодически обновляет фильмографии режиссёров, которые
// кому-то нужны: так у «режиссёров из коллекции» появляется полный список
// работ без ожидания на экране профиля.
type DirectorSync struct {
	uc       directorSyncUseCase
	log      *zap.Logger
	interval time.Duration
	batch    int
}

func NewDirectorSync(uc directorSyncUseCase, log *zap.Logger, interval time.Duration, batch int) *DirectorSync {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	if batch <= 0 {
		batch = 20
	}
	return &DirectorSync{uc: uc, log: log, interval: interval, batch: batch}
}

// Run блокируется до отмены контекста. Первый проход — сразу после старта,
// чтобы после деплоя не ждать полный интервал.
func (s *DirectorSync) Run(ctx context.Context) error {
	s.log.Info("director sync started", zap.Duration("interval", s.interval), zap.Int("batch", s.batch))
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.tick(ctx)
		select {
		case <-ctx.Done():
			s.log.Info("director sync stopped")
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
		s.log.Warn("director sync finished with errors", zap.Int("synced", synced), zap.Error(err))
		return
	}
	if synced > 0 {
		s.log.Info("director filmographies synced", zap.Int("synced", synced))
	}
}
