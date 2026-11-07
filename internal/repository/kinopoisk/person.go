package kinopoisk

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// maxNameCandidates — сколько однофамильцев проверяем по году рождения:
// каждая проверка стоит отдельного запроса к лимитированному API.
const maxNameCandidates = 3

// ResolveRussianName ищет человека по латинскому имени и подтверждает
// совпадение годом рождения. Без года принимается только единственный
// кандидат с точно таким же именем.
func (c *Client) ResolveRussianName(ctx context.Context, name string, birthYear int) (string, int32, bool, error) {
	want := normalizeName(name)
	if want == "" {
		return "", 0, false, nil
	}
	var resp struct {
		Items []struct {
			KinopoiskID int32  `json:"kinopoiskId"`
			NameRU      string `json:"nameRu"`
			NameEN      string `json:"nameEn"`
		} `json:"items"`
	}
	if err := c.get(ctx, "/api/v1/persons", url.Values{"name": {name}, "page": {"1"}}, &resp); err != nil {
		return "", 0, false, err
	}
	type candidate struct {
		id     int32
		nameRU string
	}
	var matches []candidate
	for _, item := range resp.Items {
		if item.KinopoiskID > 0 && strings.TrimSpace(item.NameRU) != "" && normalizeName(item.NameEN) == want {
			matches = append(matches, candidate{item.KinopoiskID, strings.TrimSpace(item.NameRU)})
		}
	}
	if birthYear == 0 {
		if len(matches) == 1 {
			return matches[0].nameRU, matches[0].id, true, nil
		}
		return "", 0, false, nil
	}
	for i, m := range matches {
		if i >= maxNameCandidates {
			break
		}
		var staff struct {
			NameRU   string `json:"nameRu"`
			Birthday string `json:"birthday"`
		}
		if err := c.get(ctx, "/api/v1/staff/"+strconv.Itoa(int(m.id)), nil, &staff); err != nil {
			return "", 0, false, err
		}
		if len(staff.Birthday) >= 4 {
			if year, err := strconv.Atoi(staff.Birthday[:4]); err == nil && year == birthYear {
				nameRU := strings.TrimSpace(staff.NameRU)
				if nameRU == "" {
					nameRU = m.nameRU
				}
				return nameRU, m.id, true, nil
			}
		}
	}
	return "", 0, false, nil
}

// normalizeName сравнивает имена без регистра, пунктуации и диакритики
// латиницы: «Denis Villeneuve» и «Dénis Villeneuve» — одно и то же имя.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if folded, ok := latinFold[r]; ok {
			r = folded
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var latinFold = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a', 'ā': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o', 'ø': 'o', 'ō': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y', 'ÿ': 'y', 'ł': 'l', 'ś': 's', 'š': 's', 'ž': 'z', 'č': 'c',
}
