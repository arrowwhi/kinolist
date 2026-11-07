// Package telegram — обработчики бота kinolist поверх скелета инфраструктуры
// (github.com/arrowwhi/infrastructure/telegram/bot): получение обновлений
// (polling или webhook), порядок обработки внутри чата, лимиты Bot API, логи,
// метрики и трейсы живут там, здесь — только сценарии kinolist.
package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/arrowwhi/infrastructure/telegram/api"
	tgbot "github.com/arrowwhi/infrastructure/telegram/bot"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/internal/pkg/logger"
)

// Порты объявлены на стороне потребителя: транспорт знает только те методы
// usecase, которые действительно вызывает.
type (
	enrichmentUseCase interface {
		Enqueue(ctx context.Context, userID, chatID int64, input string) (enrichment.Job, error)
		Confirm(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32, mediaTypes ...movie.MediaType) (movie.UserMovie, error)
		Reject(ctx context.Context, userID, jobID int64) error
		JobForUser(ctx context.Context, userID, jobID int64) (enrichment.Job, error)
	}

	collectionUseCase interface {
		ResolveUser(ctx context.Context, u user.User) (user.User, error)
	}
)

type Bot struct {
	enrichment enrichmentUseCase
	collection collectionUseCase
	webAppURL  string
}

func NewBot(e enrichmentUseCase, c collectionUseCase, webAppURL string) *Bot {
	return &Bot{enrichment: e, collection: c, webAppURL: webAppURL}
}

type userKey struct{}

// Register вешает обработчики kinolist на бота инфраструктуры. Команды с
// описанием попадают в меню бота при старте.
func (b *Bot) Register(r *tgbot.Bot) {
	r.Use(b.ignoreBots, b.resolveUserMiddleware)

	r.Command("start", "Начать", b.start)
	r.Command("collection", "Открыть коллекцию", b.collectionCmd)
	r.Command("help", "Справка", b.help)

	r.Callback(actionConfirm, b.confirmCallback)
	r.Callback(actionPick, b.confirmCallback)
	r.Callback(actionReject, b.rejectCallback)
	r.Callback(actionOther, b.otherCallback)

	r.Text(b.search)
	r.Default(b.fallback)
}

// SetupMenuButton ставит кнопку меню чата, открывающую Mini App. Ошибка не
// фатальна: бот работает и без неё.
func (b *Bot) SetupMenuButton(ctx context.Context, client *api.Client) error {
	if b.webAppURL == "" {
		return nil
	}

	return client.Call(ctx, "setChatMenuButton", 0, map[string]any{
		"menu_button": map[string]any{
			"type":    "web_app",
			"text":    "Коллекция",
			"web_app": api.WebAppInfo{URL: b.webAppURL},
		},
	}, nil)
}

// ignoreBots отбрасывает сообщения от других ботов.
func (b *Bot) ignoreBots(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(c *tgbot.Context) error {
		if u := c.User(); u == nil || u.IsBot {
			return nil
		}

		return next(c)
	}
}

// resolveUserMiddleware заводит или находит пользователя kinolist по Telegram-личности.
func (b *Bot) resolveUserMiddleware(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(c *tgbot.Context) error {
		from := c.User()

		c.Context = logger.With(c.Context, "telegram_id", from.ID)

		u, err := b.collection.ResolveUser(c, user.User{
			TelegramID:   from.ID,
			Username:     from.Username,
			FirstName:    from.FirstName,
			LanguageCode: from.LanguageCode,
		})
		if err != nil {
			logger.Error(c).Err(err).Msg("resolve user failed")

			if c.Callback() != nil {
				return c.Answer("Не удалось вас опознать", false)
			}

			b.reply(c, "⚠️ Не получилось вас опознать. Попробуйте ещё раз.")

			return nil
		}

		c.Context = context.WithValue(c.Context, userKey{}, u)

		return next(c)
	}
}

func currentUser(c *tgbot.Context) user.User {
	u, _ := c.Value(userKey{}).(user.User)
	return u
}

func (b *Bot) start(c *tgbot.Context) error {
	b.send(c, startText(currentUser(c).FirstName), openAppKeyboard(b.webAppURL))
	return nil
}

func (b *Bot) help(c *tgbot.Context) error {
	b.send(c, helpText, nil)
	return nil
}

func (b *Bot) collectionCmd(c *tgbot.Context) error {
	b.send(c, "Ваша коллекция:", openAppKeyboard(b.webAppURL))
	return nil
}

// search ставит название в очередь обогащения; ответ с кандидатами пришлёт воркер.
func (b *Bot) search(c *tgbot.Context) error {
	text := strings.TrimSpace(c.Text())
	if text == "" {
		b.reply(c, "Пришлите название фильма или сериала.")
		return nil
	}

	if _, err := b.enrichment.Enqueue(c, currentUser(c).ID, c.ChatID(), text); err != nil {
		b.reply(c, failureText(err))
		return nil
	}

	b.reply(c, "🔎 Ищу фильм…")

	return nil
}

// fallback — неизвестные команды, сообщения без текста и устаревшие кнопки.
func (b *Bot) fallback(c *tgbot.Context) error {
	switch {
	case c.Callback() != nil:
		return c.Answer("Кнопка устарела", false)
	case c.Command() != "":
		b.reply(c, "Не знаю такой команды. Посмотрите /help.")
	default:
		b.reply(c, "Пришлите название фильма или сериала.")
	}

	return nil
}

// parseButton разбирает data кнопки и снимает клавиатуру: повторное нажатие уже
// ничего не изменит.
func (b *Bot) parseButton(c *tgbot.Context) (callback, bool) {
	cb, err := parseCallback(c.CallbackData())
	if err != nil {
		logger.Warn(c).Err(err).Msg("malformed callback data")
		_ = c.Answer("Кнопка устарела", false)

		return callback{}, false
	}

	if m := c.Message(); m != nil {
		if err := c.API().EditMessageReplyMarkup(c, m.Chat.ID, m.MessageID, nil); err != nil && !api.IsMessageNotModified(err) {
			logger.Debug(c).Err(err).Msg("clear keyboard failed")
		}
	}

	return cb, true
}

func (b *Bot) confirmCallback(c *tgbot.Context) error {
	cb, ok := b.parseButton(c)
	if !ok {
		return nil
	}

	um, err := b.enrichment.Confirm(c, currentUser(c).ID, cb.JobID, cb.Provider, cb.ExternalID, cb.MediaType)
	switch {
	case errors.Is(err, movie.ErrAlreadyInCollection):
		b.reply(c, "📌 Этот фильм уже есть в вашей коллекции.")
		return c.Answer("Уже в коллекции", false)
	case err != nil:
		logger.Error(c).Err(err).Msg("confirm failed")
		b.reply(c, failureText(err))
		return c.Answer("Не получилось", false)
	}

	b.send(c, addedText(um), openAppKeyboard(b.webAppURL))

	return c.Answer("Добавлено", false)
}

func (b *Bot) rejectCallback(c *tgbot.Context) error {
	cb, ok := b.parseButton(c)
	if !ok {
		return nil
	}

	if err := b.enrichment.Reject(c, currentUser(c).ID, cb.JobID); err != nil {
		logger.Warn(c).Err(err).Msg("reject job failed")
	}

	b.reply(c, "Отменил. Пришлите другое название, если нужно.")

	return c.Answer("Отменено", false)
}

func (b *Bot) otherCallback(c *tgbot.Context) error {
	cb, ok := b.parseButton(c)
	if !ok {
		return nil
	}

	if err := b.enrichment.Reject(c, currentUser(c).ID, cb.JobID); err != nil {
		logger.Warn(c).Err(err).Msg("reject job failed")
	}

	b.reply(c, "Напишите название фильма текстом — поищу заново.")

	return c.Answer("Хорошо", false)
}

func (b *Bot) reply(c *tgbot.Context, text string) {
	b.send(c, text, nil)
}

// send отправляет HTML-сообщение без превью ссылок. Ошибка отправки только
// логируется: пользователю её всё равно не показать.
func (b *Bot) send(c *tgbot.Context, text string, kb *api.InlineKeyboardMarkup) {
	if c.ChatID() == 0 {
		return
	}

	opts := []tgbot.SendOption{tgbot.WithParseMode(parseModeHTML), tgbot.WithoutPreview()}
	if kb != nil {
		opts = append(opts, tgbot.WithKeyboard(kb))
	}

	if _, err := c.Reply(text, opts...); err != nil {
		logger.Warn(c).Err(err).Int64("chat_id", c.ChatID()).Msg("send message failed")
	}
}
