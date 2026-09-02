package telegram

import (
	"errors"
	"fmt"
	"html"
	"strings"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/pkg/errmap"
)

const parseModeHTML = "HTML"

// maxOverview — Telegram режет подпись к фото на 1024 символах.
const maxOverview = 600

func startText(firstName string) string {
	name := strings.TrimSpace(firstName)
	greeting := "Привет"
	if name != "" {
		greeting = "Привет, " + html.EscapeString(name)
	}

	return greeting + "! Я сохраняю фильмы, которые вы находите в рилсах.\n\n" +
		"• Пришлите ссылку на рилс — я попробую распознать фильм\n" +
		"• Или просто напишите название\n\n" +
		"Вся коллекция с жанрами и описаниями — в мини-приложении."
}

const helpText = "Что я умею:\n\n" +
	"• <b>Ссылка на рилс</b> — распознаю название и найду фильм\n" +
	"• <b>Название текстом</b> — найду фильм напрямую\n" +
	"• <b>/collection</b> — открыть коллекцию\n" +
	"• <b>/help</b> — эта справка\n\n" +
	"Если название в рилсе вшито в видео, я могу его не увидеть — тогда просто напишите название текстом."

func candidateCaption(c movie.Candidate, header string) string {
	var b strings.Builder

	if header != "" {
		b.WriteString(header)
		b.WriteString("\n\n")
	}

	title := c.Title
	if title == "" {
		title = c.OriginalTitle
	}
	b.WriteString("<b>" + html.EscapeString(title) + "</b>")
	if c.Year > 0 {
		fmt.Fprintf(&b, " (%d)", c.Year)
	}
	if c.OriginalTitle != "" && c.OriginalTitle != c.Title {
		b.WriteString("\n<i>" + html.EscapeString(c.OriginalTitle) + "</i>")
	}
	if c.Rating > 0 {
		fmt.Fprintf(&b, "\n⭐ %.1f", c.Rating)
	}
	if overview := strings.TrimSpace(c.Overview); overview != "" {
		b.WriteString("\n\n" + html.EscapeString(truncate(overview, maxOverview)))
	}

	return b.String()
}

func choiceText(candidates []movie.Candidate) string {
	var b strings.Builder
	b.WriteString("Точного совпадения нет. Выберите вариант:\n")

	for i, c := range candidates {
		title := c.Title
		if title == "" {
			title = c.OriginalTitle
		}
		fmt.Fprintf(&b, "\n<b>%d.</b> %s", i+1, html.EscapeString(title))
		if c.Year > 0 {
			fmt.Fprintf(&b, " (%d)", c.Year)
		}
		if overview := strings.TrimSpace(c.Overview); overview != "" {
			b.WriteString("\n<i>" + html.EscapeString(truncate(overview, 160)) + "</i>\n")
		}
	}

	return b.String()
}

func addedText(um movie.UserMovie) string {
	var b strings.Builder
	b.WriteString("✅ <b>" + html.EscapeString(um.Movie.Title) + "</b>")
	if um.Movie.Year > 0 {
		fmt.Fprintf(&b, " (%d)", um.Movie.Year)
	}
	b.WriteString(" добавлен в коллекцию.")

	if len(um.Movie.Genres) > 0 {
		names := make([]string, 0, len(um.Movie.Genres))
		for _, g := range um.Movie.Genres {
			names = append(names, g.NameRU)
		}
		b.WriteString("\n🎭 " + html.EscapeString(strings.Join(names, ", ")))
	}

	return b.String()
}

// failureText объясняет пользователю, что делать дальше. Текст берётся из
// той же таблицы errmap, что и HTTP-ответы, чтобы формулировки не разъезжались.
func failureText(err error) string {
	base := errmap.Map(err).Message

	switch {
	case errors.Is(err, enrichment.ErrTitleNotRecognized),
		errors.Is(err, enrichment.ErrReelUnavailable),
		errors.Is(err, enrichment.ErrInvalidReelURL),
		errors.Is(err, enrichment.ErrFetcherDisabled),
		errors.Is(err, enrichment.ErrExtractorDisabled):
		return "😕 " + base + ".\n\nПришлите название фильма текстом — я найду его напрямую."
	case errors.Is(err, movie.ErrNoCandidates):
		return "😕 " + base + ".\n\nПопробуйте другое написание или добавьте год."
	default:
		return "⚠️ " + base + ". Попробуйте ещё раз чуть позже."
	}
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

// tmdbImageBase дублирует константу HTTP-конвертера: транспорты не
// импортируют друг друга, а домен хранит только относительный путь.
const tmdbImageBase = "https://image.tmdb.org/t/p/w500"

func posterURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return tmdbImageBase + path
}
