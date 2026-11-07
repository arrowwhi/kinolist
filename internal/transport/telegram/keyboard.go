package telegram

import (
	"fmt"
	"strconv"
	"strings"

	tg "github.com/arrowwhi/infrastructure/telegram"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// Формат callback_data: "<action>:<jobID>[:<provider>:<externalID>]".
// Telegram ограничивает поле 64 байтами, поэтому действия короткие.
const (
	actionConfirm = "ok"
	actionPick    = "pk"
	actionReject  = "no"
	actionOther   = "ot"

	callbackSeparator = ":"
	maxCallbackBytes  = 64
)

type callback struct {
	Action     string
	JobID      int64
	Provider   movie.Provider
	ExternalID int32
	MediaType  movie.MediaType
}

func (c callback) encode() string {
	parts := []string{c.Action, strconv.FormatInt(c.JobID, 10)}
	if c.Provider != "" {
		parts = append(parts, providerCode(c.Provider), strconv.Itoa(int(c.ExternalID)))
	}
	if c.Provider != "" && c.MediaType != "" {
		parts = append(parts, string(c.MediaType))
	}
	return strings.Join(parts, callbackSeparator)
}

func parseCallback(data string) (callback, error) {
	parts := strings.Split(data, callbackSeparator)
	if len(parts) < 2 {
		return callback{}, fmt.Errorf("malformed callback data %q", data)
	}

	jobID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return callback{}, fmt.Errorf("malformed job id in callback %q", data)
	}

	c := callback{Action: parts[0], JobID: jobID}

	if len(parts) >= 4 {
		externalID, err := strconv.Atoi(parts[3])
		if err != nil {
			return callback{}, fmt.Errorf("malformed external id in callback %q", data)
		}
		c.Provider = providerFromCode(parts[2])
		c.ExternalID = int32(externalID) //nolint:gosec // id кандидата из TMDB/КП не выходит за диапазон int32
	}

	if len(parts) > 5 {
		return callback{}, fmt.Errorf("too many callback fields")
	}
	if len(parts) == 5 {
		c.MediaType = movie.MediaType(parts[4])
		if !c.MediaType.Valid() {
			return callback{}, movie.ErrInvalidMediaType
		}
	}
	return c, nil
}

// providerCode ужимает название источника до одной буквы: в 64 байта
// callback_data полное "kinopoisk" вместе с id уже не всегда влезает.
func providerCode(p movie.Provider) string {
	if p == movie.ProviderKinopoisk {
		return "k"
	}
	return "t"
}

func providerFromCode(code string) movie.Provider {
	if code == "k" {
		return movie.ProviderKinopoisk
	}
	return movie.ProviderTMDB
}

// confirmKeyboard показывается, когда фильм найден уверенно.
func confirmKeyboard(jobID int64, c movie.Candidate) *tg.InlineKeyboardMarkup {
	return &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{
		{
			{Text: "✅ Да, добавить", CallbackData: callback{
				Action: actionConfirm, JobID: jobID, Provider: c.Provider, ExternalID: c.ExternalID, MediaType: c.MediaType.Normalize(),
			}.encode()},
		},
		{
			{Text: "🔍 Это другой результат", CallbackData: callback{Action: actionOther, JobID: jobID}.encode()},
		},
		{
			{Text: "✖️ Отмена", CallbackData: callback{Action: actionReject, JobID: jobID}.encode()},
		},
	}}
}

// choiceKeyboard показывается, когда уверенного совпадения нет.
func choiceKeyboard(jobID int64, candidates []movie.Candidate) *tg.InlineKeyboardMarkup {
	rows := make([][]tg.InlineKeyboardButton, 0, len(candidates)+1)

	for i, c := range candidates {
		data := callback{Action: actionPick, JobID: jobID, Provider: c.Provider, ExternalID: c.ExternalID, MediaType: c.MediaType.Normalize()}.encode()
		if len(data) > maxCallbackBytes {
			continue
		}
		rows = append(rows, []tg.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%d. %s", i+1, shortLabel(c)),
			CallbackData: data,
		}})
	}

	rows = append(rows, []tg.InlineKeyboardButton{
		{Text: "✖️ Ничего из этого", CallbackData: callback{Action: actionReject, JobID: jobID}.encode()},
	})

	return &tg.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func openAppKeyboard(webAppURL string) *tg.InlineKeyboardMarkup {
	if webAppURL == "" {
		return nil
	}
	return &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{
		{{Text: "🎬 Открыть коллекцию", WebApp: &tg.WebAppInfo{URL: webAppURL}}},
	}}
}

// maxButtonLabel держит кнопку в одну строку на телефоне.
const maxButtonLabel = 40

func shortLabel(c movie.Candidate) string {
	title := c.MediaType.Label() + " · " + c.Title
	if c.Title == "" {
		title = c.MediaType.Label() + " · " + c.OriginalTitle
	}
	if c.Year > 0 {
		title = fmt.Sprintf("%s (%d)", title, c.Year)
	}

	runes := []rune(title)
	if len(runes) > maxButtonLabel {
		return string(runes[:maxButtonLabel-1]) + "…"
	}
	return title
}
