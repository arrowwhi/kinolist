package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

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
		Confirm(ctx context.Context, userID, jobID int64, provider movie.Provider, externalID int32) (movie.UserMovie, error)
		Reject(ctx context.Context, userID, jobID int64) error
		JobForUser(ctx context.Context, userID, jobID int64) (enrichment.Job, error)
	}

	collectionUseCase interface {
		ResolveUser(ctx context.Context, u user.User) (user.User, error)
	}
)

type Bot struct {
	api        *API
	enrichment enrichmentUseCase
	collection collectionUseCase
	log        *zap.Logger
	webAppURL  string
}

func NewBot(api *API, e enrichmentUseCase, c collectionUseCase, log *zap.Logger, webAppURL string) *Bot {
	return &Bot{api: api, enrichment: e, collection: c, log: log, webAppURL: webAppURL}
}

// Setup регистрирует команды и кнопку меню. Ошибки здесь не фатальны:
// бот работает и без обновлённого меню.
func (b *Bot) Setup(ctx context.Context) error {
	commands := []BotCommand{
		{Command: "start", Description: "Начать"},
		{Command: "collection", Description: "Открыть коллекцию"},
		{Command: "help", Description: "Справка"},
	}
	if err := b.api.SetMyCommands(ctx, commands); err != nil {
		return fmt.Errorf("set bot commands: %w", err)
	}

	if b.webAppURL != "" {
		if err := b.api.SetChatMenuButton(ctx, b.webAppURL); err != nil {
			return fmt.Errorf("set chat menu button: %w", err)
		}
	}

	return nil
}

// Handle — единая точка входа для обоих режимов получения апдейтов.
func (b *Bot) Handle(ctx context.Context, u Update) {
	switch {
	case u.Message != nil:
		b.handleMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		b.handleCallback(ctx, u.CallbackQuery)
	}
}

func (b *Bot) handleMessage(ctx context.Context, m *Message) {
	if m.From == nil || m.From.IsBot {
		return
	}

	ctx = logger.With(ctx, zap.Int64("telegram_id", m.From.ID))

	u, err := b.resolveUser(ctx, m.From)
	if err != nil {
		b.log.Error("resolve user failed", zap.Error(err))
		b.reply(ctx, m.Chat.ID, "⚠️ Не получилось вас опознать. Попробуйте ещё раз.")
		return
	}

	text := strings.TrimSpace(m.Text)
	if text == "" {
		b.reply(ctx, m.Chat.ID, "Пришлите ссылку на рилс или название фильма.")
		return
	}

	if strings.HasPrefix(text, "/") {
		b.handleCommand(ctx, m, text, u)
		return
	}

	b.enqueue(ctx, u.ID, m.Chat.ID, text)
}

func (b *Bot) handleCommand(ctx context.Context, m *Message, text string, u user.User) {
	// Команда может прийти как "/start@botname" — имя бота нам не нужно.
	command, _, _ := strings.Cut(strings.TrimPrefix(text, "/"), " ")
	command, _, _ = strings.Cut(command, "@")

	switch command {
	case "start":
		b.send(ctx, SendMessageRequest{
			ChatID:      m.Chat.ID,
			Text:        startText(u.FirstName),
			ParseMode:   parseModeHTML,
			ReplyMarkup: openAppKeyboard(b.webAppURL),
		})
	case "help":
		b.send(ctx, SendMessageRequest{
			ChatID: m.Chat.ID, Text: helpText, ParseMode: parseModeHTML,
		})
	case "collection":
		b.send(ctx, SendMessageRequest{
			ChatID:      m.Chat.ID,
			Text:        "Ваша коллекция:",
			ReplyMarkup: openAppKeyboard(b.webAppURL),
		})
	default:
		b.reply(ctx, m.Chat.ID, "Не знаю такой команды. Посмотрите /help.")
	}
}

func (b *Bot) enqueue(ctx context.Context, userID, chatID int64, input string) {
	if _, err := b.enrichment.Enqueue(ctx, userID, chatID, input); err != nil {
		b.reply(ctx, chatID, failureText(err))
		return
	}

	b.reply(ctx, chatID, "🔎 Ищу фильм…")
}

func (b *Bot) handleCallback(ctx context.Context, q *CallbackQuery) {
	if q.From == nil {
		return
	}

	ctx = logger.With(ctx, zap.Int64("telegram_id", q.From.ID))

	// Ответить на callback нужно всегда, иначе у пользователя навсегда
	// останется крутящийся индикатор на кнопке.
	answer := "Готово"
	defer func() {
		if err := b.api.AnswerCallbackQuery(ctx, q.ID, answer); err != nil {
			b.log.Warn("answer callback failed", zap.Error(err))
		}
	}()

	u, err := b.resolveUser(ctx, q.From)
	if err != nil {
		answer = "Не удалось вас опознать"
		return
	}

	c, err := parseCallback(q.Data)
	if err != nil {
		b.log.Warn("malformed callback data", zap.Error(err))
		answer = "Кнопка устарела"
		return
	}

	chatID := int64(0)
	messageID := 0
	if q.Message != nil {
		chatID = q.Message.Chat.ID
		messageID = q.Message.MessageID
	}

	// Клавиатуру снимаем сразу: повторное нажатие уже ничего не изменит.
	if chatID != 0 && messageID != 0 {
		if err := b.api.EditMessageReplyMarkup(ctx, chatID, messageID, nil); err != nil {
			b.log.Debug("clear keyboard failed", zap.Error(err))
		}
	}

	switch c.Action {
	case actionConfirm, actionPick:
		answer = b.confirm(ctx, u.ID, chatID, c)
	case actionReject:
		if err := b.enrichment.Reject(ctx, u.ID, c.JobID); err != nil {
			b.log.Warn("reject job failed", zap.Error(err))
		}
		b.reply(ctx, chatID, "Отменил. Пришлите другое название, если нужно.")
		answer = "Отменено"
	case actionOther:
		if err := b.enrichment.Reject(ctx, u.ID, c.JobID); err != nil {
			b.log.Warn("reject job failed", zap.Error(err))
		}
		b.reply(ctx, chatID, "Напишите название фильма текстом — поищу заново.")
		answer = "Хорошо"
	default:
		answer = "Кнопка устарела"
	}
}

func (b *Bot) confirm(ctx context.Context, userID, chatID int64, c callback) string {
	um, err := b.enrichment.Confirm(ctx, userID, c.JobID, c.Provider, c.ExternalID)
	switch {
	case errors.Is(err, movie.ErrAlreadyInCollection):
		b.reply(ctx, chatID, "📌 Этот фильм уже есть в вашей коллекции.")
		return "Уже в коллекции"
	case err != nil:
		b.log.Error("confirm failed", zap.Error(err))
		b.reply(ctx, chatID, failureText(err))
		return "Не получилось"
	}

	b.send(ctx, SendMessageRequest{
		ChatID:      chatID,
		Text:        addedText(um),
		ParseMode:   parseModeHTML,
		ReplyMarkup: openAppKeyboard(b.webAppURL),
	})

	return "Добавлено"
}

func (b *Bot) resolveUser(ctx context.Context, from *User) (user.User, error) {
	return b.collection.ResolveUser(ctx, user.User{
		TelegramID:   from.ID,
		Username:     from.Username,
		FirstName:    from.FirstName,
		LanguageCode: from.LanguageCode,
	})
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if chatID == 0 {
		return
	}
	b.send(ctx, SendMessageRequest{ChatID: chatID, Text: text, ParseMode: parseModeHTML})
}

func (b *Bot) send(ctx context.Context, req SendMessageRequest) {
	if req.LinkPreview == nil {
		req.LinkPreview = &LinkPreviewOptions{IsDisabled: true}
	}
	if _, err := b.api.SendMessage(ctx, req); err != nil {
		b.log.Warn("send message failed", zap.Error(err), zap.Int64("chat_id", req.ChatID))
	}
}
