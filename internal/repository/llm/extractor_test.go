package llm

import "testing"

func TestParseExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		wantTitle string
		wantYear  int16
		wantErr   bool
	}{
		{
			name: "plain json", raw: `{"title":"Дюна","year":2021,"confidence":0.9}`,
			wantTitle: "Дюна", wantYear: 2021,
		},
		{
			name:      "wrapped in markdown fence",
			raw:       "```json\n{\"title\":\"Fight Club\",\"year\":1999,\"confidence\":0.8}\n```",
			wantTitle: "Fight Club", wantYear: 1999,
		},
		{
			name: "prose around json", raw: `Вот результат: {"title":"Брат","year":1997,"confidence":1}. Готово.`,
			wantTitle: "Брат", wantYear: 1997,
		},
		{
			name: "no movie found", raw: `{"title":"","year":0,"confidence":0}`,
			wantTitle: "",
		},
		{
			name: "implausible year is dropped", raw: `{"title":"Дюна","year":12,"confidence":0.5}`,
			wantTitle: "Дюна", wantYear: 0,
		},
		{
			name: "title is trimmed", raw: `{"title":"  Дюна  ","year":2021,"confidence":0.5}`,
			wantTitle: "Дюна", wantYear: 2021,
		},
		{name: "empty response", raw: "   ", wantTitle: ""},
		{name: "broken json", raw: `{"title": "Дюна"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseExtraction(tt.raw)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Year != tt.wantYear {
				t.Fatalf("year = %d, want %d", got.Year, tt.wantYear)
			}
			if got.Recognized() != (tt.wantTitle != "") {
				t.Fatalf("Recognized() = %v for title %q", got.Recognized(), got.Title)
			}
		})
	}
}
