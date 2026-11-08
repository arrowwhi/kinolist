package movie

import "testing"

func TestJoinCharacters(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"", "  "}, ""},
		{[]string{"Paul Atreides"}, "Paul Atreides"},
		{[]string{"Walter White", "Heisenberg", "walter white"}, "Walter White / Heisenberg"},
		{[]string{"Finn / Finn Walden", "Finn"}, "Finn / Finn Walden"},
	} {
		if got := JoinCharacters(tc.in...); got != tc.want {
			t.Errorf("JoinCharacters(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
