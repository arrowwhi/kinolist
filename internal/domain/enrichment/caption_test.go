package enrichment

import (
	"reflect"
	"strings"
	"testing"
)

func TestCaptionTitles(t *testing.T) {
	tests := []struct {
		name    string
		caption string
		want    []TitleGuess
	}{
		{name: "пустая подпись", caption: "", want: nil},
		{name: "без признаков", caption: "подпишись, тут лучшее кино 🔥", want: nil},
		{name: "ёлочки", caption: "Пересматриваю «Интерстеллар» уже третий раз", want: []TitleGuess{{Title: "Интерстеллар"}}},
		{name: "ёлочки с годом в скобках", caption: "«Дюна» (2021) — шедевр", want: []TitleGuess{{Title: "Дюна", Year: 2021}}},
		{name: "ёлочки с годом через запятую", caption: "«Престиж», 2006", want: []TitleGuess{{Title: "Престиж", Year: 2006}}},
		{name: "английские кавычки", caption: "Watch “The Dark Knight” tonight", want: []TitleGuess{{Title: "The Dark Knight"}}},
		{name: "немецкие кавычки", caption: "сериал „Тьма“ лучший", want: []TitleGuess{{Title: "Тьма"}}},
		{name: "прямые кавычки", caption: `Фильм "Inception" взорвал мозг`, want: []TitleGuess{{Title: "Inception"}}},
		{
			name:    "несколько названий по порядку",
			caption: "Топ на вечер:\n1. «Остров проклятых»\n2. «Престиж»\n3. «Помни»",
			want:    []TitleGuess{{Title: "Остров проклятых"}, {Title: "Престиж"}, {Title: "Помни"}},
		},
		{name: "дубли схлопываются, год берётся из повтора", caption: "«Дюна» и снова «дюна» (2021)", want: []TitleGuess{{Title: "Дюна", Year: 2021}}},
		{name: "маркер с двоеточием", caption: "Фильм: Остров проклятых (2010)\n#кино", want: []TitleGuess{{Title: "Остров проклятых", Year: 2010}}},
		{name: "маркер через тире", caption: "Сериал — Настоящий детектив 🔥", want: []TitleGuess{{Title: "Настоящий детектив"}}},
		{name: "маркер на английском", caption: "Movie: Fight Club", want: []TitleGuess{{Title: "Fight Club"}}},
		{name: "маркер без разделителя не считается", caption: "этот фильм просто бомба", want: nil},
		{name: "год в скобках после текста", caption: "топ драм: Дюна (2021)", want: []TitleGuess{{Title: "Дюна", Year: 2021}}},
		{name: "цифра в названии сохраняется", caption: "Дюна 2 (2024)", want: []TitleGuess{{Title: "Дюна 2", Year: 2024}}},
		{name: "дефис в названии", caption: "1 - Человек-паук (2002)", want: []TitleGuess{{Title: "Человек-паук", Year: 2002}}},
		{name: "кавычки приоритетнее года в скобках", caption: "Обзор (2020) на «Довод»", want: []TitleGuess{{Title: "Довод"}, {Title: "Обзор", Year: 2020}}},
		{name: "кавычки с эмодзи отбрасываются", caption: `смотри "🔥🔥" и «#кино»`, want: nil},
		{name: "кавычки со ссылкой отбрасываются", caption: `"https://t.me/kino"`, want: nil},
		{name: "неправдоподобный год игнорируется", caption: "«Дюна» (1234)", want: []TitleGuess{{Title: "Дюна"}}},
		{name: "длинная цитата не название", caption: "«это была лучшая ночь в моей жизни и я никогда её не забуду правда»", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CaptionTitles(tt.caption)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("CaptionTitles(%q) = %+v, want %+v", tt.caption, got, tt.want)
			}
		})
	}
}

func TestCaptionTitlesLimit(t *testing.T) {
	var b strings.Builder
	for _, title := range []string{"Один", "Два", "Три", "Четыре", "Пять", "Шесть", "Семь"} {
		b.WriteString("«" + title + "» ")
	}

	got := CaptionTitles(b.String())
	if len(got) != MaxCaptionGuesses {
		t.Fatalf("got %d guesses, want %d", len(got), MaxCaptionGuesses)
	}
	if got[0].Title != "Один" || got[MaxCaptionGuesses-1].Title != "Пять" {
		t.Fatalf("unexpected order: %+v", got)
	}
}
