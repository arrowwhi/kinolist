package telegram

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// conflictRetry — пауза при 409 от getUpdates. Короткая и без роста:
// конфликт снимается, как только старый экземпляр остановится, и забрать
// опрос нужно сразу, а не через полминуты отката.
const conflictRetry = 3 * time.Second

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
	inConflict := false

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

			var wait time.Duration
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.Code == http.StatusConflict {
				// При rolling-деплое старый контейнер ещё опрашивает Telegram.
				// Это штатно и временно, поэтому пишем один раз, а не на
				// каждый повтор.
				if !inConflict {
					p.log.Info("another bot instance is polling, waiting for it to stop")
					inConflict = true
				}
				wait = conflictRetry
			} else {
				p.log.Warn("get updates failed", zap.Error(err), zap.Duration("retry_in", backoff))
				wait = backoff
				// Экспоненциальный откат не даёт залить Telegram запросами,
				// пока он недоступен.
				if backoff < 30*time.Second {
					backoff *= 2
				}
			}

			select {
			case <-ctx.Done():
				p.wg.Wait()
				return nil
			case <-time.After(wait):
			}
			continue
		}
		backoff = time.Second
		if inConflict {
			p.log.Info("polling resumed")
			inConflict = false
		}

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
