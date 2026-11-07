package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/arrowwhi/infrastructure/telegram/api"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/pkg/logger"
)

// Notifier доставляет результат фоновой задачи в чат. Воркер знает только
// интерфейс из домена и ничего не знает о Telegram.
type Notifier struct {
	api *api.Client
}

func NewNotifier(client *api.Client) *Notifier {
	return &Notifier{api: client}
}

func (n *Notifier) NotifyOutcome(ctx context.Context, out enrichment.Outcome) error {
	// Задача из Mini App не привязана к чату — её результат пользователь
	// увидит в интерфейсе, слать сообщение некуда.
	if out.Job.ChatID == 0 {
		return nil
	}

	switch {
	case out.Err != nil:
		return n.sendText(ctx, out.Job.ChatID, failureText(out.Err), nil)
	case out.Kind == movie.MatchExact && len(out.Candidates) == 1:
		c := out.Candidates[0]
		return n.sendCandidate(ctx, out.Job.ChatID, c,
			candidateCaption(c, "Нашёл. Это тот фильм?"),
			confirmKeyboard(out.Job.ID, c))
	case len(out.Candidates) > 0:
		return n.sendText(ctx, out.Job.ChatID,
			choiceText(out.Candidates),
			choiceKeyboard(out.Job.ID, out.Candidates))
	default:
		return n.sendText(ctx, out.Job.ChatID, failureText(movie.ErrNoCandidates), nil)
	}
}

// sendCandidate пробует отправить постер картинкой и молча откатывается на
// текст: недоступная обложка не повод терять всё сообщение.
func (n *Notifier) sendCandidate(ctx context.Context, chatID int64, c movie.Candidate, caption string, kb *api.InlineKeyboardMarkup) error {
	if poster := posterURL(c.PosterPath); poster != "" {
		_, err := n.api.SendPhoto(ctx, api.SendPhotoRequest{
			ChatID: chatID, Photo: poster, Caption: caption,
			ParseMode: parseModeHTML, ReplyMarkup: markup(kb),
		})
		if err == nil {
			return nil
		}
		if !isPhotoUnavailable(err) {
			logger.Warn(ctx).Err(err).Msg("send photo failed, falling back to text")
		}
	}

	return n.sendText(ctx, chatID, caption, kb)
}

func (n *Notifier) sendText(ctx context.Context, chatID int64, text string, kb *api.InlineKeyboardMarkup) error {
	_, err := n.api.SendMessage(ctx, api.SendMessageRequest{
		ChatID: chatID, Text: text, ParseMode: parseModeHTML,
		ReplyMarkup: markup(kb), LinkPreviewOptions: &api.LinkPreviewOptions{IsDisabled: true},
	})
	return err
}

// isPhotoUnavailable — Telegram не смог забрать постер по ссылке; сообщение нужно
// повторить текстом.
func isPhotoUnavailable(err error) bool {
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		return false
	}

	for _, s := range []string{"wrong file identifier", "failed to get HTTP URL content", "IMAGE_PROCESS_FAILED"} {
		if strings.Contains(apiErr.Description, s) {
			return true
		}
	}

	return false
}

// markup превращает nil-клавиатуру в nil-интерфейс: иначе в JSON ушло бы
// "reply_markup": null.
func markup(kb *api.InlineKeyboardMarkup) api.ReplyMarkup {
	if kb == nil {
		return nil
	}

	return kb
}
