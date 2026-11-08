package enrichment

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

// MaxCaptionGuesses ограничивает число названий из одной подписи: каждое
// из них — отдельный запрос в каталог, а подборки «10 фильмов на вечер»
// иначе сожгут лимиты TMDB на одном рилсе.
const MaxCaptionGuesses = 5

const (
	maxGuessRunes = 80
	maxGuessWords = 10
)

// TitleGuess — название, найденное в подписи по явным признакам.
type TitleGuess struct {
	Title string
	Year  int16
}

var (
	// quotedRe ловит фрагменты в кавычках в пределах одной строки:
	// «ёлочки», „немецкие“, “английские” и "прямые". Одинарные кавычки
	// не берём — их путают с апострофами (don't, Grey's Anatomy).
	quotedRe = regexp.MustCompile(`«([^«»\n]+)»|„([^„“”\n]+)[“”]|“([^“”\n]+)”|"([^"\n]+)"`)

	// yearAfterRe — год сразу после названия: «Дюна» (2021) или «Дюна», 2021.
	yearAfterRe = regexp.MustCompile(`^\s*(?:\(\s*(\d{4})\s*\)|,\s*(\d{4})\b)`)

	// markerRe — «фильм: Престиж», «Сериал — Тьма». Разделитель обязателен:
	// без него «этот фильм просто бомба» дал бы мусорное название.
	markerRe = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(?:фильм|сериал|мультфильм|мультсериал|мульт|кино|аниме|название|movie|film|series|tv show|title)\s*[:—–-]\s*([^\n#@(),.!?;|]+)`)

	// parenYearRe — «Дюна (2021)»: название берётся из текста перед скобкой.
	parenYearRe = regexp.MustCompile(`\(\s*(\d{4})\s*\)`)
)

// CaptionTitles вытаскивает из подписи к рилсу названия, выделенные явно:
// кавычками, маркером «фильм:» или годом в скобках. Возвращает их по
// убыванию надёжности признака (кавычки → маркер → год) и в порядке
// появления в тексте, без дублей. Это дешёвый детерминированный путь —
// LLM нужна только для подписей, где название ничем не выделено.
func CaptionTitles(caption string) []TitleGuess {
	c := guessCollector{seen: map[string]int{}}

	for _, m := range quotedRe.FindAllStringSubmatchIndex(caption, -1) {
		var title string
		for g := 1; g < len(m)/2; g++ {
			if m[2*g] >= 0 {
				title = caption[m[2*g]:m[2*g+1]]
				break
			}
		}
		c.add(title, yearAfter(caption[m[1]:]))
	}

	for _, m := range markerRe.FindAllStringSubmatchIndex(caption, -1) {
		c.add(caption[m[2]:m[3]], yearAfter(caption[m[3]:]))
	}

	for _, m := range parenYearRe.FindAllStringSubmatchIndex(caption, -1) {
		c.add(titleBefore(caption[:m[0]]), parseYear(caption[m[2]:m[3]]))
	}

	return c.guesses
}

// ValidYear отсекает годы, которым нельзя доверять: первый фильм снят в 1888.
func ValidYear(year int) bool {
	return year >= 1888 && year <= 2100
}

type guessCollector struct {
	guesses []TitleGuess
	seen    map[string]int
}

func (c *guessCollector) add(raw string, year int16) {
	title := cleanGuess(raw)
	if title == "" {
		return
	}

	key := movie.NormalizeTitle(title)
	if i, ok := c.seen[key]; ok {
		// То же название встретилось ещё раз, но уже с годом — год пригодится.
		if c.guesses[i].Year == 0 {
			c.guesses[i].Year = year
		}
		return
	}
	if len(c.guesses) >= MaxCaptionGuesses {
		return
	}

	c.seen[key] = len(c.guesses)
	c.guesses = append(c.guesses, TitleGuess{Title: title, Year: year})
}

// cleanGuess обрезает обрамление и отбраковывает то, что не может быть
// названием: эмодзи, хэштеги, упоминания, ссылки и целые предложения.
func cleanGuess(raw string) string {
	if strings.ContainsAny(raw, "#@") {
		return ""
	}
	s := strings.TrimFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	s = strings.Join(strings.Fields(s), " ")

	if s == "" || utf8.RuneCountInString(s) > maxGuessRunes {
		return ""
	}
	if len(strings.Fields(s)) > maxGuessWords {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "http") || strings.Contains(lower, "://") {
		return ""
	}
	if !strings.ContainsFunc(s, unicode.IsLetter) {
		return ""
	}

	return s
}

func yearAfter(s string) int16 {
	m := yearAfterRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	if m[1] != "" {
		return parseYear(m[1])
	}
	return parseYear(m[2])
}

func parseYear(s string) int16 {
	y, err := strconv.ParseInt(s, 10, 16)
	if err != nil || !ValidYear(int(y)) {
		return 0
	}
	return int16(y)
}

// titleBefore берёт хвост текста до ближайшего разделителя: в «топ драм:
// Дюна» или «2) Дюна» названием считается только «Дюна».
func titleBefore(s string) string {
	start := strings.LastIndexFunc(s, isTitleBoundary)
	if start >= 0 {
		_, size := utf8.DecodeRuneInString(s[start:])
		s = s[start+size:]
	}
	// Дефис внутри слова — часть названия («Человек-паук»), а отбитый
	// пробелами — разделитель списка («1 - Дюна»).
	if i := strings.LastIndex(s, " - "); i >= 0 {
		s = s[i+len(" - "):]
	}
	return s
}

func isTitleBoundary(r rune) bool {
	switch r {
	case '\n', ':', '—', '–', '|', '•', '.', '!', '?', ';', ',', ')', '(', '#', '@', '«', '»', '"', '“', '”', '„':
		return true
	}
	return unicode.IsSymbol(r)
}
