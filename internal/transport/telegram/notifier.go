package telegram

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// Notifier доставляет результат фоновой задачи в чат. Воркер знает только
// интерфейс из домена и ничего не знает о Telegram.
type Notifier struct {
	api *API
	log *zap.Logger
}

func NewNotifier(api *API, log *zap.Logger) *Notifier {
	return &Notifier{api: api, log: log}
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
func (n *Notifier) sendCandidate(ctx context.Context, chatID int64, c movie.Candidate, caption string, kb *InlineKeyboardMarkup) error {
	if poster := posterURL(c.PosterPath); poster != "" {
		_, err := n.api.SendPhoto(ctx, SendPhotoRequest{
			ChatID: chatID, Photo: poster, Caption: caption,
			ParseMode: parseModeHTML, ReplyMarkup: kb,
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrPhotoUnavailable) {
			n.log.Warn("send photo failed, falling back to text", zap.Error(err))
		}
	}

	return n.sendText(ctx, chatID, caption, kb)
}

func (n *Notifier) sendText(ctx context.Context, chatID int64, text string, kb *InlineKeyboardMarkup) error {
	_, err := n.api.SendMessage(ctx, SendMessageRequest{
		ChatID: chatID, Text: text, ParseMode: parseModeHTML,
		ReplyMarkup: kb, LinkPreview: &LinkPreviewOptions{IsDisabled: true},
	})
	return err
}
