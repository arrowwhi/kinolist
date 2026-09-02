package telegram

import (
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestCallbackRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   callback
	}{
		{"confirm with candidate", callback{actionConfirm, 42, movie.ProviderTMDB, 438631}},
		{"kinopoisk candidate", callback{actionPick, 7, movie.ProviderKinopoisk, 41519}},
		{"action without candidate", callback{Action: actionReject, JobID: 99}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded := tt.in.encode()
			if len(encoded) > maxCallbackBytes {
				t.Fatalf("callback data %q exceeds Telegram's 64-byte limit", encoded)
			}

			got, err := parseCallback(encoded)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.in {
				t.Fatalf("round trip changed the callback: %+v -> %+v", tt.in, got)
			}
		})
	}
}

func TestParseCallbackRejectsGarbage(t *testing.T) {
	t.Parallel()

	for _, data := range []string{"", "ok", "ok:notanumber", "pk:1:t:xx"} {
		if _, err := parseCallback(data); err == nil {
			t.Fatalf("parseCallback(%q) accepted malformed data", data)
		}
	}
}

func TestChoiceKeyboardAlwaysOffersAnExit(t *testing.T) {
	t.Parallel()

	kb := choiceKeyboard(1, []movie.Candidate{
		{Provider: movie.ProviderTMDB, ExternalID: 1, Title: "Дюна", Year: 2021},
		{Provider: movie.ProviderTMDB, ExternalID: 2, Title: "Дюна", Year: 1984},
	})

	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("rows = %d, want 2 candidates + 1 escape row", len(kb.InlineKeyboard))
	}

	last := kb.InlineKeyboard[len(kb.InlineKeyboard)-1][0]
	if got, _ := parseCallback(last.CallbackData); got.Action != actionReject {
		t.Fatalf("last row action = %q, want reject", got.Action)
	}
}

func TestShortLabelIsTruncated(t *testing.T) {
	t.Parallel()

	long := movie.Candidate{Title: "Очень длинное название фильма которое точно не влезет в кнопку", Year: 2020}
	label := shortLabel(long)

	if len([]rune(label)) > maxButtonLabel {
		t.Fatalf("label is %d runes, want at most %d", len([]rune(label)), maxButtonLabel)
	}
}
