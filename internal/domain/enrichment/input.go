package enrichment

import (
	"net/url"
	"strings"
)

var instagramHosts = map[string]struct{}{
	"instagram.com":     {},
	"www.instagram.com": {},
	"m.instagram.com":   {},
	"instagr.am":        {},
	"www.instagr.am":    {},
	"ddinstagram.com":   {},
}

// reelPaths — сегменты, под которыми Instagram отдаёт короткие видео.
var reelPaths = map[string]struct{}{
	"reel": {}, "reels": {}, "p": {}, "tv": {},
}

// ClassifyInput решает, что пользователь прислал: ссылку на рилс или название.
// Ссылка на не-Instagram отвергается явной ошибкой, иначе она молча ушла бы
// в поиск по названию и вернула мусор.
func ClassifyInput(raw string) (Kind, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", ErrEmptyInput
	}

	if !looksLikeURL(s) {
		return KindTitle, s, nil
	}

	withScheme := s
	if !strings.Contains(s, "://") {
		withScheme = "https://" + s
	}

	u, err := url.Parse(withScheme)
	if err != nil {
		return "", "", ErrInvalidReelURL
	}

	host := strings.ToLower(u.Hostname())
	if _, ok := instagramHosts[host]; !ok {
		return "", "", ErrInvalidReelURL
	}

	if !hasReelPath(u.Path) {
		return "", "", ErrInvalidReelURL
	}

	// Трекинговые параметры (igsh, utm_*) только мешают дедупликации ссылок.
	u.RawQuery = ""
	u.Fragment = ""
	u.Scheme = "https"

	return KindReel, u.String(), nil
}

// hasReelPath ищет сегмент рилса с идентификатором после него. Кроме
// /reel/<code> Instagram отдаёт ссылки с префиксом: /<user>/reel/<code>
// со страницы профиля и /share/reel/<token> из меню «Поделиться».
func hasReelPath(path string) bool {
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	for i := 0; i < len(segments)-1 && i < 2; i++ {
		if _, ok := reelPaths[strings.ToLower(segments[i])]; ok {
			return true
		}
	}
	return false
}

func looksLikeURL(s string) bool {
	if strings.ContainsAny(s, " \t\n") {
		return false
	}
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.Contains(lower, "instagram.com/") ||
		strings.Contains(lower, "instagr.am/")
}
