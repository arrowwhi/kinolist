package movie_test

import (
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
)

func TestNormalizeTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "The Godfather", "godfather"},
		{"strips punctuation", "Spider-Man: No Way Home", "spider man no way home"},
		{"collapses spaces", "  Fight   Club  ", "fight club"},
		{"keeps cyrillic", "Брат 2", "брат 2"},
		{"drops leading article only", "A Star Is Born", "star is born"},
		{"keeps article as sole word", "The", "the"},
		{"keeps inner article", "Killing Of A Sacred Deer", "killing of a sacred deer"},
		{"empty stays empty", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := movie.NormalizeTitle(tt.in); got != tt.want {
				t.Fatalf("NormalizeTitle(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func cand(title, original string, year int16, id int32) movie.Candidate {
	return movie.Candidate{
		Provider:      movie.ProviderTMDB,
		ExternalID:    id,
		Title:         title,
		OriginalTitle: original,
		Year:          year,
	}
}

func TestSelectMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         string
		year          int16
		candidates    []movie.Candidate
		wantKind      movie.MatchKind
		wantBestID    int32
		wantCandCount int
	}{
		{
			name:       "empty catalog result",
			query:      "Начало",
			candidates: nil,
			wantKind:   movie.MatchNone,
		},
		{
			name:       "title and year match",
			query:      "Начало",
			year:       2010,
			candidates: []movie.Candidate{cand("Дюна", "Dune", 2021, 1), cand("Начало", "Inception", 2010, 2)},
			wantKind:   movie.MatchExact,
			wantBestID: 2, wantCandCount: 1,
		},
		{
			name:       "matches original title when query is english",
			query:      "Inception",
			year:       2010,
			candidates: []movie.Candidate{cand("Начало", "Inception", 2010, 2)},
			wantKind:   movie.MatchExact,
			wantBestID: 2, wantCandCount: 1,
		},
		{
			name:       "no year, single title hit is confident",
			query:      "Бойцовский клуб",
			candidates: []movie.Candidate{cand("Бойцовский клуб", "Fight Club", 1999, 7), cand("Клуб", "Club", 2015, 8)},
			wantKind:   movie.MatchExact,
			wantBestID: 7, wantCandCount: 1,
		},
		{
			name:       "no year, several remakes stay ambiguous",
			query:      "Дюна",
			candidates: []movie.Candidate{cand("Дюна", "Dune", 1984, 3), cand("Дюна", "Dune", 2021, 4)},
			wantKind:   movie.MatchAmbiguous, wantCandCount: 2,
		},
		{
			name:       "year given but no candidate matches it",
			query:      "Дюна",
			year:       2000,
			candidates: []movie.Candidate{cand("Дюна", "Dune", 1984, 3), cand("Дюна", "Dune", 2021, 4)},
			wantKind:   movie.MatchAmbiguous, wantCandCount: 2,
		},
		{
			name:       "title never matches, fall back to suggestions",
			query:      "Совсем другое кино",
			candidates: []movie.Candidate{cand("Дюна", "Dune", 2021, 4)},
			wantKind:   movie.MatchAmbiguous, wantCandCount: 1,
		},
		{
			name:  "suggestions are capped",
			query: "клуб",
			candidates: []movie.Candidate{
				cand("A", "A", 2001, 1), cand("B", "B", 2002, 2), cand("C", "C", 2003, 3),
				cand("D", "D", 2004, 4), cand("E", "E", 2005, 5), cand("F", "F", 2006, 6),
			},
			wantKind: movie.MatchAmbiguous, wantCandCount: movie.MaxSuggestions,
		},
		{
			name:       "punctuation difference still matches",
			query:      "Человек-паук: Нет пути домой",
			year:       2021,
			candidates: []movie.Candidate{cand("Человек паук. Нет пути домой", "Spider-Man: No Way Home", 2021, 9)},
			wantKind:   movie.MatchExact,
			wantBestID: 9, wantCandCount: 1,
		},
		{
			name:       "empty query is never a confident match",
			query:      "   ",
			candidates: []movie.Candidate{cand("Дюна", "Dune", 2021, 4)},
			wantKind:   movie.MatchAmbiguous, wantCandCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := movie.SelectMatch(tt.query, tt.year, tt.candidates)

			if got.Kind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", got.Kind, tt.wantKind)
			}
			if len(got.Candidates) != tt.wantCandCount {
				t.Fatalf("candidates = %d, want %d", len(got.Candidates), tt.wantCandCount)
			}
			if tt.wantKind == movie.MatchExact && got.Best.ExternalID != tt.wantBestID {
				t.Fatalf("best = %d, want %d", got.Best.ExternalID, tt.wantBestID)
			}
		})
	}
}

func TestSelectMatchDoesNotAliasInput(t *testing.T) {
	t.Parallel()

	in := []movie.Candidate{cand("A", "A", 2001, 1), cand("B", "B", 2002, 2)}
	got := movie.SelectMatch("нет такого", 0, in)

	got.Candidates[0] = cand("mutated", "mutated", 1900, 99)
	if in[0].Title != "A" {
		t.Fatal("SelectMatch returned a slice aliasing its input")
	}
}
