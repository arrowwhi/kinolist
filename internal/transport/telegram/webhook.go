package telegram

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const secretHeader = "X-Telegram-Bot-Api-Secret-Token" //nolint:gosec // имя заголовка, не секрет

// WebhookHandler принимает апдейты по HTTP. Секретный токен обязателен:
// иначе кто угодно, зная путь, отправил бы боту поддельный апдейт.
func WebhookHandler(bot *Bot, secret string, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(secretHeader)), []byte(secret)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var u Update
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&u); err != nil {
			log.Warn("decode telegram update failed", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Telegram повторяет апдейт, если ответ идёт дольше нескольких
		// секунд. Отвечаем сразу, обработку продолжаем в фоне.
		w.WriteHeader(http.StatusOK)

		go func() { //nolint:contextcheck // обработка апдейта намеренно живёт дольше запроса, свой контекст ниже
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic while handling update", zap.Any("panic", rec))
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			bot.Handle(ctx, u)
		}()
	}
}
