package movie

import (
	"strings"
	"unicode"
)

// MaxSuggestions — сколько вариантов показываем, когда уверенного совпадения нет.
const MaxSuggestions = 5

// MatchKind — насколько сервис уверен, что нашёл именно тот фильм.
type MatchKind string

const (
	// MatchExact — название и год сошлись, можно предлагать одного кандидата.
	MatchExact MatchKind = "exact"
	// MatchAmbiguous — точного совпадения нет, показываем список на выбор.
	MatchAmbiguous MatchKind = "ambiguous"
	// MatchNone — каталог ничего не вернул.
	MatchNone MatchKind = "none"
)

// Match — результат сопоставления запроса с выдачей каталога.
type Match struct {
	Kind       MatchKind
	Best       Candidate
	Candidates []Candidate
}

// articles отбрасываются при нормализации: "The Godfather" и "Godfather"
// в подписях к рилсам встречаются одинаково часто.
var articles = map[string]struct{}{
	"the": {}, "a": {}, "an": {},
}

// NormalizeTitle приводит название к виду, пригодному для сравнения:
// нижний регистр, без пунктуации, без артиклей, одиночные пробелы.
func NormalizeTitle(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	prevSpace := true
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		case !prevSpace:
			b.WriteRune(' ')
			prevSpace = true
		}
	}

	fields := strings.Fields(b.String())
	// Артикль отбрасываем только в начале и только если он не единственное слово.
	if len(fields) > 1 {
		if _, ok := articles[fields[0]]; ok {
			fields = fields[1:]
		}
	}

	return strings.Join(fields, " ")
}

// titlesMatch сравнивает запрос и с локализованным, и с оригинальным названием:
// подпись к рилсу может быть на любом из двух языков.
func titlesMatch(query string, c Candidate) bool {
	q := NormalizeTitle(query)
	if q == "" {
		return false
	}
	return q == NormalizeTitle(c.Title) || q == NormalizeTitle(c.OriginalTitle)
}

// SelectMatch решает, показывать ли одного кандидата как найденный фильм
// или предложить список на выбор.
//
// Уверенным считается совпадение, когда нормализованное название сошлось и
// либо год совпал, либо год из запроса неизвестен, а претендент с таким
// названием ровно один. Второй случай важен: LLM часто извлекает название
// без года, и требовать год означало бы всегда переспрашивать.
func SelectMatch(query string, year int16, candidates []Candidate) Match {
	if len(candidates) == 0 {
		return Match{Kind: MatchNone}
	}

	var byTitle []Candidate
	for _, c := range candidates {
		if titlesMatch(query, c) {
			byTitle = append(byTitle, c)
		}
	}

	switch {
	case year > 0:
		for _, c := range byTitle {
			if c.Year == year {
				return Match{Kind: MatchExact, Best: c, Candidates: []Candidate{c}}
			}
		}
	case len(byTitle) == 1:
		return Match{Kind: MatchExact, Best: byTitle[0], Candidates: byTitle}
	}

	return Match{Kind: MatchAmbiguous, Candidates: topN(candidates, MaxSuggestions)}
}

func topN(candidates []Candidate, n int) []Candidate {
	if len(candidates) <= n {
		out := make([]Candidate, len(candidates))
		copy(out, candidates)
		return out
	}
	out := make([]Candidate, n)
	copy(out, candidates[:n])
	return out
}
