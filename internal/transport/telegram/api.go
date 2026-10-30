// Package telegram — транспорт бота. Клиент Bot API написан вручную:
// нужны семь методов, а любая библиотека притащила бы свой диспетчер,
// который пришлось бы обходить ради чистой архитектуры.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	apiBase          = "https://api.telegram.org/bot"
	maxResponseBytes = 8 << 20
)

type API struct {
	http  *http.Client
	token string
}

func NewAPI(token string, timeout time.Duration) *API {
	return &API{
		// Запас поверх таймаута long-polling: сам запрос висит до
		// PollTimeout, и обрывать его раньше нельзя.
		http:  &http.Client{Timeout: timeout + 30*time.Second},
		token: token,
	}
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

// APIError — отказ Bot API. Типизирован, чтобы вызывающий код различал
// ошибки по коду через errors.As, а не по тексту.
type APIError struct {
	Method      string
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %s failed (%d): %s", e.Method, e.Code, e.Description)
}

func (a *API) call(ctx context.Context, method string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+a.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s response: %w", method, err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if !parsed.OK {
		return &APIError{Method: method, Code: parsed.ErrorCode, Description: parsed.Description}
	}

	if out != nil && len(parsed.Result) > 0 {
		if err := json.Unmarshal(parsed.Result, out); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}

	return nil
}

func (a *API) GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]Update, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         int(timeout.Seconds()),
		"allowed_updates": []string{"message", "callback_query"},
	}

	var updates []Update
	if err := a.call(ctx, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

func (a *API) SetWebhook(ctx context.Context, url, secret string) error {
	return a.call(ctx, "setWebhook", map[string]any{
		"url":             url,
		"secret_token":    secret,
		"allowed_updates": []string{"message", "callback_query"},
		// Апдейты, накопившиеся пока сервис лежал, чаще всего уже неактуальны.
		"drop_pending_updates": true,
	}, nil)
}

func (a *API) DeleteWebhook(ctx context.Context) error {
	return a.call(ctx, "deleteWebhook", map[string]any{}, nil)
}

func (a *API) SendMessage(ctx context.Context, m SendMessageRequest) (Message, error) {
	var out Message
	if err := a.call(ctx, "sendMessage", m, &out); err != nil {
		return Message{}, err
	}
	return out, nil
}

func (a *API) SendPhoto(ctx context.Context, m SendPhotoRequest) (Message, error) {
	var out Message
	if err := a.call(ctx, "sendPhoto", m, &out); err != nil {
		// Постер мог оказаться недоступен по ссылке — в этом случае
		// сообщение всё равно должно дойти, но уже без картинки.
		if strings.Contains(err.Error(), "wrong file identifier") ||
			strings.Contains(err.Error(), "failed to get HTTP URL content") ||
			strings.Contains(err.Error(), "IMAGE_PROCESS_FAILED") {
			return Message{}, ErrPhotoUnavailable
		}
		return Message{}, err
	}
	return out, nil
}

func (a *API) AnswerCallbackQuery(ctx context.Context, id, text string) error {
	return a.call(ctx, "answerCallbackQuery", map[string]any{
		"callback_query_id": id,
		"text":              text,
	}, nil)
}

func (a *API) EditMessageReplyMarkup(ctx context.Context, chatID int64, messageID int, markup *InlineKeyboardMarkup) error {
	return a.call(ctx, "editMessageReplyMarkup", map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"reply_markup": markup,
	}, nil)
}

func (a *API) SetMyCommands(ctx context.Context, commands []BotCommand) error {
	return a.call(ctx, "setMyCommands", map[string]any{"commands": commands}, nil)
}

func (a *API) SetChatMenuButton(ctx context.Context, webAppURL string) error {
	return a.call(ctx, "setChatMenuButton", map[string]any{
		"menu_button": map[string]any{
			"type":    "web_app",
			"text":    "Коллекция",
			"web_app": map[string]string{"url": webAppURL},
		},
	}, nil)
}
