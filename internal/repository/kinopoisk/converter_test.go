package kinopoisk

import "testing"

func TestDetailsRuntimeAndRatingSource(t *testing.T) {
	c := (filmDetails{KinopoiskID: 1, FilmLength: 120, RatingKP: 8}).toCandidate()
	if c.RuntimeMin != 120 || c.RatingProvider != "kinopoisk" || c.RatingVoteCount != nil {
		t.Fatalf("candidate: %+v", c)
	}
}
