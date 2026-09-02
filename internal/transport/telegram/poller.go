package telegram

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Poller получает апдейты через getUpdates. Режим для локальной разработки:
// не требует публичного адреса.
type Poller struct {
	api     *API
	bot     *Bot
	log     *zap.Logger
	timeout time.Duration

	wg sync.WaitGroup
}

func NewPoller(api *API, bot *Bot, log *zap.Logger, timeout time.Duration) *Poller {
	return &Poller{api: api, bot: bot, log: log, timeout: timeout}
}

// Run блокируется до отмены контекста.
func (p *Poller) Run(ctx context.Context) error {
	// Ранее выставленный webhook блокирует getUpdates — снимаем его.
	if err := p.api.DeleteWebhook(ctx); err != nil {
		p.log.Warn("delete webhook failed", zap.Error(err))
	}

	p.log.Info("telegram poller started")

	var offset int64
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			p.wg.Wait()
			return nil //nolint:nilerr // штатная остановка по отмене контекста — не ошибка
		}

		updates, err := p.api.GetUpdates(ctx, offset, p.timeout)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				p.wg.Wait()
				return nil //nolint:nilerr // штатная остановка по отмене контекста — не ошибка
			}

			p.log.Warn("get updates failed", zap.Error(err), zap.Duration("retry_in", backoff))
			select {
			case <-ctx.Done():
				p.wg.Wait()
				return nil
			case <-time.After(backoff):
			}
			// Экспоненциальный откат не даёт залить Telegram запросами,
			// пока он недоступен.
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			p.dispatch(ctx, u)
		}
	}
}

// dispatch обрабатывает апдейт в отдельной горутине, чтобы медленный ответ
// внешнего API не тормозил разбор очереди.
func (p *Poller) dispatch(ctx context.Context, u Update) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if rec := recover(); rec != nil {
				p.log.Error("panic while handling update", zap.Any("panic", rec))
			}
		}()

		// Контекст обработки не отменяется вместе с polling-циклом:
		// начатый ответ пользователю нужно довести до конца.
		handleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()

		p.bot.Handle(handleCtx, u)
	}()
}
